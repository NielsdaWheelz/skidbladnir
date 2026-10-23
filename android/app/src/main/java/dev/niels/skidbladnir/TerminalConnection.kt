package dev.niels.skidbladnir

import android.os.Handler
import android.os.Looper
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okio.ByteString
import okio.ByteString.Companion.toByteString

// The gateway reads at most this much per client frame (terminal.MaximumFrameBytes).
private const val MAXIMUM_TERMINAL_FRAME_BYTES = 64 * 1024
private const val MAXIMUM_TERMINAL_QUEUE_BYTES = 1024 * 1024L

// A gateway text frame as the phone reads it (rules.md trust boundaries): its kind, Hello's and
// Presence's attached-client count and an Error's code. Everything else is ignored.
@Serializable private class TerminalFrame(
    val kind: String,
    val attachedClients: Int? = null,
    val error: TerminalFrameError? = null,
)
@Serializable private class TerminalFrameError(val code: String)
@Serializable private data class TerminalResize(val kind: String, val columns: Int, val rows: Int)

private fun resizeFrame(columns: Int, rows: Int): String =
    productJson.encodeToString(TerminalResize("Resize", columns, rows))

// Hello and Presence both report the attached-client count; the phone reads nothing else from Hello.
internal interface TerminalConnectionObserver {
    fun onConnected(attachedClients: Int)
    fun onFailure(code: ApiErrorCode)
}

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
    private var opened = false

    // The gateway creates no PTY until a valid Resize arrives, so the measured
    // grid waits here for open and is the first client frame; a grid change
    // before open replaces it.
    private var openingResize = resizeFrame(initialViewport.columns, initialViewport.rows)

    fun start() {
        // Holding the monitor keeps a fast fail() from running before `socket` is set.
        synchronized(monitor) {
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
            if (!webSocket.send(openingResize)) fail(ApiErrorCode.ReconnectRequired)
        }
    }

    // Undecodable text, an unknown kind or a missing field the phone reads loses the connection.
    override fun onMessage(webSocket: WebSocket, text: String) {
        val frame = try {
            productJson.decodeFromString<TerminalFrame>(text)
        } catch (_: IllegalArgumentException) { // includes SerializationException
            return fail(ApiErrorCode.ReconnectRequired)
        }
        when (frame.kind) {
            "Hello", "Presence" -> {
                val attachedClients = frame.attachedClients ?: return fail(ApiErrorCode.ReconnectRequired)
                main.post { if (!stopped.get()) observer.onConnected(attachedClients) }
            }
            // An unknown code reads as InternalError, as in decodeGatewayHttpFailure. The frame's message
            // is not carried: the controller renders apiErrorMessage (docs/issues/terminal-error-copy.md).
            "Error" -> {
                val code = frame.error?.code ?: return fail(ApiErrorCode.ReconnectRequired)
                fail(ApiErrorCode.entries.firstOrNull { it.name == code } ?: ApiErrorCode.InternalError)
            }
            else -> fail(ApiErrorCode.ReconnectRequired)
        }
    }

    override fun onMessage(webSocket: WebSocket, bytes: ByteString) {
        // Unbounded until main runs it: the budget in LockedTerminalWebView.write
        // counts a frame only then.
        val payload = bytes.toByteArray()
        main.post { if (!stopped.get()) page.write(payload) }
    }

    override fun onClosing(webSocket: WebSocket, code: Int, reason: String) = fail(ApiErrorCode.ReconnectRequired)

    override fun onClosed(webSocket: WebSocket, code: Int, reason: String) = fail(ApiErrorCode.ReconnectRequired)

    // A refused upgrade carries the gateway's http error envelope; any other failure loses the connection.
    override fun onFailure(webSocket: WebSocket, throwable: Throwable, response: Response?) {
        val refusal = response?.use { decodeGatewayHttpFailure(it.code, it.body?.string().orEmpty()) }
        fail((refusal as? GatewayFailure.Api)?.code ?: ApiErrorCode.ReconnectRequired)
    }

    // After open each Resize goes out at once; OkHttp's queue keeps it ahead of
    // later input.
    fun resize(columns: Int, rows: Int) {
        val encoded = resizeFrame(columns, rows)
        synchronized(monitor) {
            if (stopped.get()) return
            if (!opened) {
                openingResize = encoded
            } else if (!checkNotNull(socket).send(encoded)) {
                fail(ApiErrorCode.ReconnectRequired)
            }
        }
    }

    fun send(bytes: ByteArray) {
        synchronized(monitor) {
            val activeSocket = socket
            if (stopped.get() || activeSocket == null) return
            if (bytes.size + activeSocket.queueSize() > MAXIMUM_TERMINAL_QUEUE_BYTES) {
                fail(ApiErrorCode.ReconnectRequired)
                return
            }
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
            socket?.send("""{"kind":"Detach"}""")
            socket?.close(1000, "Detach")
            socket = null
        }
    }

    fun terminalUnavailable() = fail(ApiErrorCode.ReconnectRequired)

    private fun fail(code: ApiErrorCode) {
        if (!stopped.compareAndSet(false, true)) return
        synchronized(monitor) {
            socket?.cancel()
            socket = null
        }
        main.post { observer.onFailure(code) }
    }
}
