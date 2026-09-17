package dev.niels.skidbladnir

import android.os.Handler
import android.os.Looper
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import okio.ByteString.Companion.toByteString

private const val MAXIMUM_TERMINAL_FRAME_BYTES = 64 * 1024
private const val MAXIMUM_TERMINAL_QUEUE_BYTES = 1024 * 1024L

internal sealed interface TerminalServerEvent {
    data class Hello(val attachedClients: Int) : TerminalServerEvent
    data class Presence(val attachedClients: Int) : TerminalServerEvent
    data class Error(val code: ApiErrorCode) : TerminalServerEvent
}
@Serializable private data class TerminalErrorPayload(val code: String, val message: String)
@Serializable private data class TerminalErrorEnvelope(val kind: String, val error: TerminalErrorPayload)
@Serializable private data class TerminalResize(val kind: String, val columns: Int, val rows: Int)
@Serializable private data class TerminalDetach(val kind: String)

internal fun decodeTerminalServerEvent(encoded: String): TerminalServerEvent = decodeProtocol {
    val objectValue = strictJsonObject(encoded)
    when (val kind = objectValue.requiredString("kind")) {
        "Hello", "Presence" -> {
            objectValue.requireExactKeys(setOf("kind", "attachedClients"))
            val attachedClients = objectValue.requiredPositiveInt("attachedClients")
            if (kind == "Hello") TerminalServerEvent.Hello(attachedClients)
            else TerminalServerEvent.Presence(attachedClients)
        }
        "Error" -> {
            objectValue.requireExactKeys(setOf("kind", "error"))
            objectValue.requiredObject("error").requireExactKeys(setOf("code", "message"))
            val payload = productJson.decodeFromJsonElement<TerminalErrorEnvelope>(objectValue).error
            val code = parseApiErrorCode(payload.code)
            if (code !in setOf(ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge, ApiErrorCode.ReconnectRequired, ApiErrorCode.TerminalConfigurationUnsupported, ApiErrorCode.InternalError)) {
                throw SerializationException("error code is outside the terminal protocol")
            }
            if (payload.message != apiErrorMessage(code)) throw SerializationException("incorrect API error message")
            TerminalServerEvent.Error(code)
        }
        else -> throw SerializationException("unknown terminal event kind")
    }
}

// One geometry bound: the page publishes only fitted whole-cell grids inside it
// (and ViewportTooSmall below it), and the resize transport carries nothing else.
internal val TERMINAL_COLUMNS_RANGE = 20..1024
internal val TERMINAL_ROWS_RANGE = 5..512

internal fun encodeTerminalResize(columns: Int, rows: Int): String {
    if (columns !in TERMINAL_COLUMNS_RANGE || rows !in TERMINAL_ROWS_RANGE) {
        throw IllegalArgumentException("terminal geometry out of bounds")
    }
    return productJson.encodeToString(TerminalResize("Resize", columns, rows))
}
internal fun encodeTerminalDetach(): String = productJson.encodeToString(TerminalDetach("Detach"))

// kotlinx's tree decoder coerces quoted digits into numbers; presence counts stay JSON numbers.
private fun JsonObject.requiredPositiveInt(key: String): Int {
    val member = this[key]
    if (member !is JsonPrimitive || member.isString) throw SerializationException("missing or non-number $key")
    return member.content.toIntOrNull()?.takeIf { it >= 1 }
        ?: throw SerializationException("$key is not a positive integer")
}

internal interface TerminalConnectionObserver {
    fun onPresence(attachedClients: Int)
    fun onFailure(code: ApiErrorCode)
}

private data class PendingResize(
    val encoded: String,
    val byteCount: Int,
)

internal class TerminalConnection(
    private val client: GatewayClient,
    private val credential: MachineCredential,
    private val target: SessionTarget,
    initialViewport: TerminalViewport.Fitted,
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

    // The gateway creates no PTY until a valid Resize arrives, so a
    // connection cannot exist without its measured geometry already queued as the
    // first client frame the open-time flush will send.
    init {
        resize(initialViewport.columns, initialViewport.rows)
    }

    fun start() {
        synchronized(monitor) {
            check(!started) // justify-service-invariant-check: each connection object owns exactly one WebSocket lifetime.
            started = true
            if (stopped.get()) return
            socket = client.http.newWebSocket(client.terminalRequest(credential, target), this)
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
            // justify-defect: only the gateway writes server text frames, so an oversized frame is
            // a same-system contract violation.
            throw ProtocolDecodeException("terminal text frame exceeded the protocol bound")
        }
        val event = decodeTerminalServerEvent(text)
        when (event) {
            is TerminalServerEvent.Hello -> synchronized(monitor) {
                if (stopped.get()) return
                // justify-defect: the gateway owns the closed terminal event sequence.
                if (connected) throw ProtocolDecodeException("terminal sent Hello more than once")
                connected = true
                observer.onPresence(event.attachedClients)
            }
            is TerminalServerEvent.Presence -> synchronized(monitor) {
                if (stopped.get()) return
                // justify-defect: the gateway owns the closed terminal event sequence.
                if (!connected) throw ProtocolDecodeException("terminal sent Presence before Hello")
                observer.onPresence(event.attachedClients)
            }
            is TerminalServerEvent.Error -> fail(event.code)
        }
    }

    override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
        synchronized(monitor) {
            if (stopped.get()) return
            // justify-defect: the gateway serializes Hello before bounded PTY output.
            if (!connected || bytes.size > MAXIMUM_TERMINAL_FRAME_BYTES) {
                throw ProtocolDecodeException("terminal binary frame violated ordering or size")
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
        if (stopped.get()) return
        val code = if (response == null) {
            terminalUpgradeFailureCode(null, null)
        } else {
            response.use {
                terminalUpgradeFailureCode(it.code, it.body?.string().orEmpty())
            }
        }
        fail(code)
    }

    fun resize(columns: Int, rows: Int) {
        val encoded = try {
            encodeTerminalResize(columns, rows)
        } catch (_: IllegalArgumentException) {
            // justify-defect: the owned page emits only geometry inside the closed protocol bounds,
            // and the reason stays a fixed literal so no terminal content can reach a log.
            throw ProtocolDecodeException("terminal geometry left the closed protocol bounds")
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

internal fun terminalUpgradeFailureCode(status: Int?, encoded: String?): ApiErrorCode {
    if (status == null) {
        require(encoded == null)
        return ApiErrorCode.ReconnectRequired
    }
    val failure = decodeGatewayHttpFailure(status, requireNotNull(encoded))
    return when (failure) {
        is GatewayFailure.Api -> failure.code
        GatewayFailure.Transport -> ApiErrorCode.ReconnectRequired
    }
}
