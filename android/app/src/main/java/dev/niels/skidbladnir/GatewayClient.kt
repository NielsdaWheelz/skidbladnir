package dev.niels.skidbladnir

import java.io.IOException
import java.net.Proxy
import java.util.concurrent.TimeUnit
import kotlin.coroutines.resume
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import okhttp3.Call
import okhttp3.Callback
import okhttp3.Dispatcher
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

private val jsonMediaType = "application/json; charset=utf-8".toMediaType()
private const val MAXIMUM_HTTP_BODY_BYTES = 64 * 1024
private const val AGENT_CALL_TIMEOUT_MILLIS = 15_000L

internal sealed interface GatewayResult<out Value> {
    data class Success<Value>(val value: Value) : GatewayResult<Value>
    data class Failure(val failure: GatewayFailure) : GatewayResult<Nothing>
}

internal enum class MutationDispatch { NotSent, Unknown }

internal sealed interface GatewayFailure {
    /** A gateway error; [message] is the gateway's copy. [dispatch] is null where the gateway did not say. */
    data class Api(val code: ApiErrorCode, val message: String, val dispatch: MutationDispatch? = null) : GatewayFailure
    data object Transport : GatewayFailure
}

internal fun gatewayFailureMessage(failure: GatewayFailure): String = when (failure) {
    is GatewayFailure.Api -> failure.message
    GatewayFailure.Transport -> "Could not reach this machine over your Tailnet."
}

internal fun createFailureIsDefinitive(failure: GatewayFailure): Boolean =
    failure is GatewayFailure.Api && failure.dispatch == MutationDispatch.NotSent

@Serializable private class ErrorEnvelope(val code: String, val message: String, val dispatch: String? = null)

/**
 * Total: a proxy status (403, 502–504) or a body that is not a gateway error envelope is a lost response.
 * A code this release does not know is still the gateway's answer: it reads as the generic
 * [ApiErrorCode.InternalError], with the gateway's own message and dispatch.
 */
internal fun decodeGatewayHttpFailure(status: Int, encoded: String): GatewayFailure {
    if (status == 403 || status in 502..504) return GatewayFailure.Transport
    return try {
        val envelope = productJson.decodeFromString<ErrorEnvelope>(encoded)
        val dispatch = when (envelope.dispatch) {
            "not_sent" -> MutationDispatch.NotSent
            "unknown" -> MutationDispatch.Unknown
            else -> null
        }
        val code = ApiErrorCode.entries.firstOrNull { it.name == envelope.code } ?: ApiErrorCode.InternalError
        GatewayFailure.Api(code, envelope.message, dispatch)
    } catch (_: IllegalArgumentException) { // includes SerializationException
        GatewayFailure.Transport
    }
}

@Serializable private data class CreateSessionRequest(
    val kind: String,
    val cwd: String,
    val profile: String? = null,
    val optionalTmuxName: String? = null,
    val objective: String? = null,
    val group: String? = null,
)
@Serializable private data class DirectoryListingRequest(val directory: String)
@Serializable private data class CloseTerminalRequest(val identityToken: String)
@Serializable private data class WireSessionNaming(val mode: NameMode, val name: String? = null)
@Serializable private data class RenameSessionRequest(
    val identityToken: String,
    val expectedNaming: WireSessionNaming,
    val naming: WireSessionNaming,
)

private fun SessionNaming.toWire(): WireSessionNaming = when (this) {
    SessionNaming.Automatic -> WireSessionNaming(NameMode.Automatic)
    is SessionNaming.Manual -> WireSessionNaming(NameMode.Manual, name)
}

@Serializable private data class GroupRequest(val identityToken: String, val group: String)
@Serializable private data class SessionIdentityRequest(val identityToken: String)
@Serializable private data class WireDirectorySearchRequest(val terms: List<String>)
@Serializable private data class TerminalControlRequest(val identityToken: String, val paneId: String)

internal object GatewayClient {
    /**
     * The process's one http client. Calls are asynchronous and share its dispatcher, so one host may use every
     * slot: an open terminal's websocket reader holds one for as long as it is attached. Connection retries,
     * redirects and proxies are off, so a call is sent once, straight to its pinned origin.
     */
    internal val http: OkHttpClient = OkHttpClient.Builder()
        .dispatcher(Dispatcher().apply { maxRequestsPerHost = maxRequests })
        .proxy(Proxy.NO_PROXY)
        .retryOnConnectionFailure(false)
        .followRedirects(false)
        .followSslRedirects(false)
        .pingInterval(15, TimeUnit.SECONDS)
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .build()

    suspend fun listSessions(credential: MachineCredential): GatewayResult<SessionsResponse> =
        call(credential, "GET", listOf("v1", "sessions"), null, 200, ::decodeSessionsResponse)

    suspend fun readPressure(credential: MachineCredential): GatewayResult<PressureResponse> =
        call(credential, "GET", listOf("v1", "pressure"), null, 200, ::decodePressureResponse)

    suspend fun listDirectory(credential: MachineCredential, directory: HomeDirectory): GatewayResult<DirectoryListing> = call(
        credential, "POST", listOf("v1", "directory-listings"),
        productJson.encodeToString(DirectoryListingRequest(directory.encoded)), 200, ::decodeDirectoryListing,
    )

    suspend fun searchDirectories(credential: MachineCredential, terms: List<String>): GatewayResult<DirectorySearchResult> = call(
        credential, "POST", listOf("v1", "directory-searches"),
        productJson.encodeToString(WireDirectorySearchRequest(terms)), 200, ::decodeDirectorySearch,
    )

    suspend fun readTerminalContext(credential: MachineCredential, connectionId: String): GatewayResult<TerminalContext> = call(
        credential, "GET", listOf("v1", "terminal-contexts", connectionId), null,
        200, { productJson.decodeFromString<TerminalContext>(it) },
    )

    suspend fun createSession(credential: MachineCredential, draft: ForgeDraft): GatewayResult<TmuxSession> = call(
        credential, "POST", listOf("v1", "sessions"),
        productJson.encodeToString(
            CreateSessionRequest(
                when (draft.launch) {
                    is LaunchChoice.Agent -> "agent"
                    LaunchChoice.Terminal -> "terminal"
                },
                draft.cwd,
                (draft.launch as? LaunchChoice.Agent)?.profile?.encoded,
                draft.optionalTmuxName.ifEmpty { null },
                draft.objective.ifEmpty { null },
                draft.group?.text,
            ),
        ),
        201, ::decodeCreatedSession,
    )

    suspend fun createShell(credential: MachineCredential, source: SessionTarget): GatewayResult<TmuxSession> = call(
        credential, "POST", listOf("v1", "sessions", source.session.tmuxId, "shell"),
        productJson.encodeToString(SessionIdentityRequest(source.session.identityToken)), 201, ::decodeCreatedSession,
    )

    suspend fun stopTerminal(credential: MachineCredential, target: SessionTarget): GatewayResult<TerminalWriteResult> = call(
        credential, "POST", listOf("v1", "sessions", target.session.tmuxId, "terminal", "stop"),
        productJson.encodeToString(TerminalControlRequest(target.session.identityToken, target.session.activePaneId)),
        200, { productJson.decodeFromString<TerminalWriteResult>(it) }, AGENT_CALL_TIMEOUT_MILLIS,
    )

    suspend fun interruptAndCloseTerminal(credential: MachineCredential, target: SessionTarget): GatewayResult<TerminalCloseResult> = call(
        credential, "POST", listOf("v1", "sessions", target.session.tmuxId, "terminal", "close"),
        productJson.encodeToString(TerminalControlRequest(target.session.identityToken, target.session.activePaneId)),
        200, { productJson.decodeFromString<TerminalCloseResult>(it) }, AGENT_CALL_TIMEOUT_MILLIS,
    )

    suspend fun closeTerminal(credential: MachineCredential, target: SessionTarget): GatewayResult<Unit> = call(
        credential, "DELETE", listOf("v1", "sessions", target.session.tmuxId),
        productJson.encodeToString(CloseTerminalRequest(target.session.identityToken)), 204, {},
    )

    suspend fun renameSession(
        credential: MachineCredential,
        target: SessionTarget,
        expected: SessionNaming,
        naming: SessionNaming,
    ): GatewayResult<Unit> = call(
        credential, "PATCH", listOf("v1", "sessions", target.session.tmuxId),
        productJson.encodeToString(RenameSessionRequest(target.session.identityToken, expected.toWire(), naming.toWire())),
        204, {},
    )

    suspend fun setSessionGroup(credential: MachineCredential, target: SessionTarget, label: GroupLabel?): GatewayResult<Unit> = call(
        credential, "PUT", listOf("v1", "sessions", target.session.tmuxId, "group"),
        productJson.encodeToString(GroupRequest(target.session.identityToken, label?.text.orEmpty())), 204, {},
    )

    internal fun terminalRequest(credential: MachineCredential, target: SessionTarget): Request =
        authorizedRequest(credential, listOf("v1", "sessions", target.session.tmuxId, "terminal"))
            .header("Skidbladnir-Session-Identity", target.session.identityToken)
            .build()

    /**
     * Every ordinary bearer request is bound to a pinned machine; there is no headerless variant.
     * A gateway that answers this origin with another installation's identity fails with
     * `409 MachineIdentityMismatch` before it discloses anything.
     */
    internal fun authorizedRequest(credential: MachineCredential, segments: List<String>): Request.Builder {
        val url = checkNotNull(credential.machine.origin).encoded.toHttpUrl().newBuilder()
            .apply { segments.forEach(::addPathSegment) }
            .build()
        return Request.Builder()
            .url(url)
            .header("Authorization", "Bearer ${credential.bearer.encoded}")
            .header("Accept", "application/json")
            .header("Skidbladnir-Machine", credential.machine.handle.encoded)
    }

    /**
     * One round trip, cancelled with its caller. A failed connection, a body over 64 KiB, a body that is not
     * UTF-8 and a body that does not decode are all [GatewayFailure.Transport]: the response is lost.
     */
    private suspend fun <Value> call(
        credential: MachineCredential,
        method: String,
        segments: List<String>,
        body: String?,
        expectedStatus: Int,
        decode: (String) -> Value,
        callTimeoutMillis: Long? = null,
    ): GatewayResult<Value> = suspendCancellableCoroutine { continuation ->
        val call = http.newCall(authorizedRequest(credential, segments).method(method, body?.toRequestBody(jsonMediaType)).build())
        if (callTimeoutMillis != null) call.timeout().timeout(callTimeoutMillis, TimeUnit.MILLISECONDS)
        continuation.invokeOnCancellation { call.cancel() }
        call.enqueue(object : Callback {
            override fun onFailure(call: Call, e: IOException) {
                continuation.resume(GatewayResult.Failure(GatewayFailure.Transport))
            }

            override fun onResponse(call: Call, response: Response) {
                val result: GatewayResult<Value> = try {
                    response.use {
                        // One byte past the bound tells an exact-limit body from a larger one without buffering it.
                        val bytes = it.body?.byteStream()?.readNBytes(MAXIMUM_HTTP_BODY_BYTES + 1) ?: ByteArray(0)
                        if (bytes.size > MAXIMUM_HTTP_BODY_BYTES) return@use GatewayResult.Failure(GatewayFailure.Transport)
                        val text = bytes.decodeToString(throwOnInvalidSequence = true)
                        if (it.code == expectedStatus) GatewayResult.Success(decode(text))
                        else GatewayResult.Failure(decodeGatewayHttpFailure(it.code, text))
                    }
                } catch (_: IOException) { // includes CharacterCodingException
                    GatewayResult.Failure(GatewayFailure.Transport)
                } catch (_: IllegalArgumentException) { // includes SerializationException
                    GatewayResult.Failure(GatewayFailure.Transport)
                }
                continuation.resume(result)
            }
        })
    }
}
