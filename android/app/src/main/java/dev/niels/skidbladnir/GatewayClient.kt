package dev.niels.skidbladnir

import java.io.IOException
import java.util.Base64
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

private val jsonMediaType = "application/json; charset=utf-8".toMediaType()

internal class GatewayBearer private constructor(internal val encoded: String) {
    companion object {
        fun parse(candidate: String): GatewayBearer? {
            if (candidate.length != 43 || candidate.any {
                    it !in 'A'..'Z' && it !in 'a'..'z' && it !in '0'..'9' && it != '-' && it != '_'
                }
            ) return null
            val decoded = try { Base64.getUrlDecoder().decode(candidate) } catch (_: IllegalArgumentException) { return null }
            if (decoded.size != 32 || Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) != candidate) return null
            return GatewayBearer(candidate)
        }
    }

    override fun equals(other: Any?): Boolean = other is GatewayBearer && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = "GatewayBearer(redacted)"
}

internal data class MachineCredential(
    val machine: PairedMachine,
    val bearer: GatewayBearer,
)

internal sealed interface GatewayResult<out Value> {
    data class Success<Value>(val value: Value) : GatewayResult<Value>
    data class Failure(val failure: GatewayFailure) : GatewayResult<Nothing>
}

internal sealed interface GatewayFailure {
    data class Api(val code: ApiErrorCode) : GatewayFailure
    data object Transport : GatewayFailure
}

internal fun gatewayFailureMessage(failure: GatewayFailure): String = when (failure) {
    is GatewayFailure.Api -> apiErrorMessage(failure.code)
    GatewayFailure.Transport -> "Could not reach this machine over your Tailnet."
}

internal fun createFailureIsDefinitive(failure: GatewayFailure): Boolean = when (failure) {
    GatewayFailure.Transport -> false
    is GatewayFailure.Api -> failure.code in setOf(
        ApiErrorCode.Unauthenticated,
        ApiErrorCode.MachineIdentityMismatch,
        ApiErrorCode.InvalidRequest,
        ApiErrorCode.RequestTooLarge,
        ApiErrorCode.WorkingDirectoryInvalid,
        ApiErrorCode.WorkingDirectoryUnavailable,
        ApiErrorCode.ProfileUnknown,
        ApiErrorCode.SessionNameInvalid,
        ApiErrorCode.ObjectiveInvalid,
        ApiErrorCode.SessionNameConflict,
    )
}

internal fun killFailureIsDefinitive(failure: GatewayFailure): Boolean = when (failure) {
    GatewayFailure.Transport -> false
    is GatewayFailure.Api -> failure.code in setOf(
        ApiErrorCode.Unauthenticated,
        ApiErrorCode.MachineIdentityMismatch,
        ApiErrorCode.InvalidRequest,
        ApiErrorCode.RequestTooLarge,
        ApiErrorCode.SessionNotFound,
        ApiErrorCode.SessionIdentityMismatch,
        ApiErrorCode.SessionGroupedConflict,
    )
}

@Serializable private data class ErrorResponse(val code: String, val message: String)

internal class GatewayClient {
    private val closeScheduled = AtomicBoolean(false)

    internal val http = OkHttpClient.Builder()
        .retryOnConnectionFailure(false)
        .followRedirects(false)
        .followSslRedirects(false)
        .pingInterval(15, TimeUnit.SECONDS)
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .build()

    fun closeAsync() {
        if (!closeScheduled.compareAndSet(false, true)) return
        val dispatcher = http.dispatcher
        val executor = dispatcher.executorService
        executor.execute {
            try {
                dispatcher.cancelAll()
                http.connectionPool.evictAll()
            } finally {
                executor.shutdown()
            }
        }
    }

    fun pair(origin: MachineOrigin, bearer: GatewayBearer): GatewayResult<SessionsResponse> = executeJson(
        request = request(origin.encoded.toHttpUrl(), bearer, null, listOf("v1", "sessions")).get().build(),
        expectedStatus = 200,
        decode = ::decodeSessionsResponse,
    )

    fun listSessions(credential: MachineCredential): GatewayResult<SessionsResponse> = executeJson(
        request = authorizedRequest(credential, listOf("v1", "sessions")).get().build(),
        expectedStatus = 200,
        decode = ::decodeSessionsResponse,
    )

    fun readPressure(credential: MachineCredential): GatewayResult<PressureResponse> = executeJson(
        request = authorizedRequest(credential, listOf("v1", "pressure")).get().build(),
        expectedStatus = 200,
        decode = ::decodePressureResponse,
    )

    fun createSession(credential: MachineCredential, draft: ForgeDraft): GatewayResult<AgentSession> {
        require(draft.machineHandle == credential.machine.handle)
        return executeJson(
            request = authorizedRequest(credential, listOf("v1", "sessions"))
                .post(encodeCreateSessionRequest(draft).toRequestBody(jsonMediaType))
                .build(),
            expectedStatus = 201,
            decode = ::decodeAgentSession,
        )
    }

    fun killSession(credential: MachineCredential, target: AgentTarget): GatewayResult<Unit> {
        require(target.machineHandle == credential.machine.handle)
        return executeJson(
            request = killRequest(credential, target),
            expectedStatus = 204,
            decode = { encoded ->
                if (encoded.isNotEmpty()) throw SerializationException("kill response was not empty")
            },
        )
    }

    internal fun killRequest(credential: MachineCredential, target: AgentTarget): Request {
        require(target.machineHandle == credential.machine.handle)
        return authorizedRequest(credential, listOf("v1", "sessions", target.session.id))
            .delete(encodeKillSessionRequest(target.session).toRequestBody(jsonMediaType))
            .build()
    }

    internal fun terminalRequest(credential: MachineCredential, target: AgentTarget): Request {
        require(target.machineHandle == credential.machine.handle)
        return authorizedRequest(credential, listOf("v1", "sessions", target.session.id, "terminal"))
            .header("Skidbladnir-Session-Identity", target.session.identityToken)
            .build()
    }

    private fun authorizedRequest(credential: MachineCredential, segments: List<String>): Request.Builder = request(
        origin = credential.machine.origin.encoded.toHttpUrl(),
        bearer = credential.bearer,
        machineHandle = credential.machine.handle,
        segments = segments,
    )

    private fun request(
        origin: HttpUrl,
        bearer: GatewayBearer,
        machineHandle: MachineHandle?,
        segments: List<String>,
    ): Request.Builder {
        val url = origin.newBuilder().apply { segments.forEach(::addPathSegment) }.build()
        return Request.Builder()
            .url(url)
            .header("Authorization", "Bearer ${bearer.encoded}")
            .header("Accept", "application/json")
            .apply { machineHandle?.let { header("Skidbladnir-Machine", it.encoded) } }
    }

    private fun <Value> executeJson(
        request: Request,
        expectedStatus: Int,
        decode: (String) -> Value,
    ): GatewayResult<Value> = try {
        http.newCall(request).execute().use { response ->
            val encoded = response.body?.string().orEmpty()
            if (response.code != expectedStatus) {
                GatewayResult.Failure(decodeGatewayHttpFailure(response.code, encoded))
            } else {
                GatewayResult.Success(decodeProtocol { decode(encoded) })
            }
        }
    } catch (_: IOException) {
        GatewayResult.Failure(GatewayFailure.Transport)
    }

}

internal fun decodeGatewayHttpFailure(status: Int, encoded: String): GatewayFailure = decodeProtocol {
    if (status == 502 || status == 503 || status == 504) return@decodeProtocol GatewayFailure.Transport
    val response = productJson.decodeFromString<ErrorResponse>(encoded)
    val code = parseApiErrorCode(response.code)
    if (
        code == ApiErrorCode.ReconnectRequired ||
        status != apiErrorHttpStatus(code) ||
        response.message != apiErrorMessage(code)
    ) {
        throw SerializationException("HTTP error response disagreed with the owned protocol")
    }
    GatewayFailure.Api(code)
}

private fun apiErrorHttpStatus(code: ApiErrorCode): Int = when (code) {
    ApiErrorCode.Unauthenticated -> 401
    ApiErrorCode.InvalidRequest -> 400
    ApiErrorCode.RequestTooLarge -> 413
    ApiErrorCode.WorkingDirectoryInvalid,
    ApiErrorCode.WorkingDirectoryUnavailable,
    ApiErrorCode.ProfileUnknown,
    ApiErrorCode.SessionNameInvalid,
    ApiErrorCode.ObjectiveInvalid,
    -> 422
    ApiErrorCode.SessionNameConflict,
    ApiErrorCode.SessionIdentityMismatch,
    ApiErrorCode.SessionGroupedConflict,
    ApiErrorCode.MachineIdentityMismatch,
    -> 409
    ApiErrorCode.SessionNotFound -> 404
    ApiErrorCode.InternalError -> 500
    ApiErrorCode.ReconnectRequired -> throw SerializationException("terminal error has no HTTP status")
}
