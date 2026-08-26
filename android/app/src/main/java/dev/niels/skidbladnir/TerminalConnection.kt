package dev.niels.skidbladnir

import android.os.Handler
import android.os.Looper
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.serialization.SerializationException
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import okio.ByteString.Companion.toByteString

private const val MAXIMUM_TERMINAL_FRAME_BYTES = 64 * 1024
private const val MAXIMUM_TERMINAL_QUEUE_BYTES = 1024 * 1024L

internal interface TerminalConnectionObserver {
    fun onPresence(attachedClients: Int, geometry: TerminalGeometry)
    fun onFailure(code: ApiErrorCode)
}

private data class PendingResize(
    val encoded: String,
    val byteCount: Int,
)

internal class TerminalConnection(
    private val client: GatewayClient,
    private val bearer: GatewayBearer,
    private val session: AgentSession,
    private val page: TerminalPage,
    private val observer: TerminalConnectionObserver,
) : WebSocketListener() {
    private val stopped = AtomicBoolean(false)
    private val monitor = Any()
    private val main = Handler(Looper.getMainLooper())
    private var socket: WebSocket? = null
    private var started = false
    private var opened = false
    private var connected = false
    private var pendingResize: PendingResize? = null
    private var resizeDrainScheduled = false
    private val resizeDrain = Runnable(::drainResize)

    fun start() {
        synchronized(monitor) {
            check(!started) // justify-service-invariant-check: each connection object owns exactly one WebSocket lifetime.
            started = true
            if (stopped.get()) return
            socket = client.http.newWebSocket(client.terminalRequest(bearer, session), this)
        }
    }

    override fun onOpen(webSocket: WebSocket, response: Response) {
        synchronized(monitor) {
            if (stopped.get()) {
                webSocket.cancel()
                return
            }
            opened = true
            flushResizeLocked()
        }
    }

    override fun onMessage(webSocket: WebSocket, text: String) {
        if (stopped.get()) return
        if (text.utf8ByteCountWithin(MAXIMUM_TERMINAL_FRAME_BYTES) == null) {
            throw ProtocolDecodeException(
                SerializationException("owned terminal text frame exceeded the protocol bound"),
            ) // justify-defect: only the gateway writes server text frames, so an oversized frame is a same-system contract violation.
        }
        val event = decodeTerminalServerEvent(text)
        when (event) {
            is TerminalServerEvent.Hello -> synchronized(monitor) {
                if (stopped.get()) return
                if (connected) {
                    throw ProtocolDecodeException(
                        SerializationException("terminal sent Hello more than once"),
                    ) // justify-defect: the gateway owns the closed terminal event sequence.
                }
                connected = true
                observer.onPresence(event.attachedClients, event.geometry)
            }
            is TerminalServerEvent.Presence -> synchronized(monitor) {
                if (stopped.get()) return
                if (!connected) {
                    throw ProtocolDecodeException(
                        SerializationException("terminal sent Presence before Hello"),
                    ) // justify-defect: the gateway owns the closed terminal event sequence.
                }
                observer.onPresence(event.attachedClients, event.geometry)
            }
            is TerminalServerEvent.Error -> fail(event.code)
        }
    }

    override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
        synchronized(monitor) {
            if (stopped.get()) return
            if (!connected || bytes.size > MAXIMUM_TERMINAL_FRAME_BYTES) {
                throw ProtocolDecodeException(
                    SerializationException("terminal binary frame violated ordering or size"),
                ) // justify-defect: the gateway serializes Hello before bounded PTY output.
            }
            page.write(bytes.toByteArray())
        }
    }

    override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
        if (!stopped.get()) fail(ApiErrorCode.ReconnectRequired)
    }

    override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
        if (!stopped.get()) fail(ApiErrorCode.ReconnectRequired)
    }

    override fun onFailure(webSocket: WebSocket, throwable: Throwable, response: Response?) {
        if (!stopped.get()) fail(ApiErrorCode.ReconnectRequired)
    }

    fun resize(columns: Int, rows: Int) {
        val encoded = try {
            encodeTerminalResize(columns, rows)
        } catch (failure: IllegalArgumentException) {
            throw ProtocolDecodeException(failure) // justify-defect: the owned page emits only geometry inside the closed protocol bounds.
        }
        synchronized(monitor) {
            if (stopped.get()) return
            pendingResize = PendingResize(encoded, encoded.toByteArray(Charsets.UTF_8).size)
            flushResizeLocked()
        }
    }

    fun send(bytes: ByteArray) {
        synchronized(monitor) {
            val activeSocket = socket
            if (!connected || stopped.get() || activeSocket == null) return
            val pendingResizeBytes = pendingResize?.byteCount ?: 0
            if (bytes.size.toLong() + pendingResizeBytes + activeSocket.queueSize() >
                MAXIMUM_TERMINAL_QUEUE_BYTES
            ) {
                fail(ApiErrorCode.ReconnectRequired)
                return
            }
            flushResizeBeforeInputLocked(activeSocket)
            if (stopped.get()) return
            var offset = 0
            while (offset < bytes.size) {
                val byteCount = minOf(MAXIMUM_TERMINAL_FRAME_BYTES, bytes.size - offset)
                if (!activeSocket.send(bytes.toByteString(offset, byteCount))) {
                    fail(ApiErrorCode.ReconnectRequired)
                    return
                }
                offset += byteCount
            }
        }
    }

    fun detach() {
        if (!stopped.compareAndSet(false, true)) return
        synchronized(monitor) {
            connected = false
            pendingResize = null
            resizeDrainScheduled = false
            main.removeCallbacks(resizeDrain)
            socket?.send(encodeTerminalDetach())
            socket?.close(1000, "Detach")
            socket = null
        }
    }

    fun terminalUnavailable() {
        fail(ApiErrorCode.ReconnectRequired)
    }

    private fun fail(code: ApiErrorCode) {
        if (!stopped.compareAndSet(false, true)) return
        synchronized(monitor) {
            connected = false
            pendingResize = null
            resizeDrainScheduled = false
            main.removeCallbacks(resizeDrain)
            socket?.cancel()
            socket = null
        }
        observer.onFailure(code)
    }

    private fun drainResize() {
        synchronized(monitor) {
            resizeDrainScheduled = false
            if (!stopped.get()) flushResizeLocked()
        }
    }

    private fun flushResizeLocked() {
        val activeSocket = socket ?: return
        val resize = pendingResize ?: return
        if (!opened) return
        val queueSize = activeSocket.queueSize()
        if (queueSize + resize.byteCount > MAXIMUM_TERMINAL_QUEUE_BYTES) {
            fail(ApiErrorCode.ReconnectRequired)
            return
        }
        if (queueSize > 0) {
            scheduleResizeDrainLocked()
            return
        }
        pendingResize = null
        if (!activeSocket.send(resize.encoded)) {
            fail(ApiErrorCode.ReconnectRequired)
        }
    }

    private fun scheduleResizeDrainLocked() {
        if (resizeDrainScheduled) return
        resizeDrainScheduled = true
        main.postDelayed(resizeDrain, 25)
    }

    private fun flushResizeBeforeInputLocked(activeSocket: WebSocket) {
        val resize = pendingResize ?: return
        pendingResize = null
        resizeDrainScheduled = false
        main.removeCallbacks(resizeDrain)
        if (!activeSocket.send(resize.encoded)) fail(ApiErrorCode.ReconnectRequired)
    }
}
