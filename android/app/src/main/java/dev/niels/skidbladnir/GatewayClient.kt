package dev.niels.skidbladnir

import java.io.IOException
import java.util.Base64
import java.util.concurrent.TimeUnit
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

private val gatewayOrigin = "https://dev-server-cpx11.tail6340bd.ts.net:8443".toHttpUrl()
private val jsonMediaType = "application/json; charset=utf-8".toMediaType()

internal class GatewayBearer private constructor(internal val encoded: String) {
    companion object {
        fun parse(candidate: String): GatewayBearer? {
            if (candidate.length != 43 || candidate.any { it !in 'A'..'Z' && it !in 'a'..'z' && it !in '0'..'9' && it != '-' && it != '_' }) {
                return null
            }
            val decoded = try {
                Base64.getUrlDecoder().decode(candidate)
            } catch (_: IllegalArgumentException) {
                return null
            }
            if (decoded.size != 32 || Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) != candidate) {
                return null
            }
            return GatewayBearer(candidate)
        }
    }

    override fun toString(): String = "GatewayBearer(redacted)"
}

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
    GatewayFailure.Transport -> "Could not reach the devbox."
}

internal fun createFailureIsDefinitive(failure: GatewayFailure): Boolean = when (failure) {
    GatewayFailure.Transport -> false
    is GatewayFailure.Api -> when (failure.code) {
        ApiErrorCode.Unauthenticated,
        ApiErrorCode.InvalidRequest,
        ApiErrorCode.RequestTooLarge,
        ApiErrorCode.WorkingDirectoryInvalid,
        ApiErrorCode.WorkingDirectoryUnavailable,
        ApiErrorCode.ProfileUnknown,
        ApiErrorCode.SessionNameInvalid,
        ApiErrorCode.ObjectiveInvalid,
        ApiErrorCode.SessionNameConflict,
        -> true
        ApiErrorCode.SessionNotFound,
        ApiErrorCode.SessionIdentityMismatch,
        ApiErrorCode.SessionGroupedConflict,
        ApiErrorCode.InternalError,
        ApiErrorCode.ReconnectRequired,
        -> false
    }
}

internal fun killFailureIsDefinitive(failure: GatewayFailure): Boolean = when (failure) {
    GatewayFailure.Transport -> false
    is GatewayFailure.Api -> when (failure.code) {
        ApiErrorCode.Unauthenticated,
        ApiErrorCode.InvalidRequest,
        ApiErrorCode.RequestTooLarge,
        ApiErrorCode.SessionNotFound,
        ApiErrorCode.SessionIdentityMismatch,
        ApiErrorCode.SessionGroupedConflict,
        -> true
        ApiErrorCode.WorkingDirectoryInvalid,
        ApiErrorCode.WorkingDirectoryUnavailable,
        ApiErrorCode.ProfileUnknown,
        ApiErrorCode.SessionNameInvalid,
        ApiErrorCode.ObjectiveInvalid,
        ApiErrorCode.SessionNameConflict,
        ApiErrorCode.InternalError,
        ApiErrorCode.ReconnectRequired,
        -> false
    }
}

@Serializable
private data class ErrorResponse(val code: String, val message: String)

internal class GatewayClient {
    internal val http = OkHttpClient.Builder()
        .retryOnConnectionFailure(false)
        .followRedirects(false)
        .followSslRedirects(false)
        .pingInterval(15, TimeUnit.SECONDS)
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .build()

    fun listSessions(bearer: GatewayBearer): GatewayResult<SessionsResponse> = executeJson(
        request = authorizedRequest(bearer, listOf("v1", "sessions")).get().build(),
        expectedStatus = 200,
        decode = ::decodeSessionsResponse,
    )

    fun readPressure(bearer: GatewayBearer): GatewayResult<PressureResponse> = executeJson(
        request = authorizedRequest(bearer, listOf("v1", "pressure")).get().build(),
        expectedStatus = 200,
        decode = ::decodePressureResponse,
    )

    fun createSession(bearer: GatewayBearer, draft: ForgeDraft): GatewayResult<AgentSession> = executeJson(
        request = authorizedRequest(bearer, listOf("v1", "sessions"))
            .post(encodeCreateSessionRequest(draft).toRequestBody(jsonMediaType))
            .build(),
        expectedStatus = 201,
        decode = ::decodeAgentSession,
    )

    fun killSession(bearer: GatewayBearer, session: AgentSession): GatewayResult<Unit> = executeJson(
        request = authorizedRequest(bearer, listOf("v1", "sessions", session.id))
            .delete(encodeKillSessionRequest(session).toRequestBody(jsonMediaType))
            .build(),
        expectedStatus = 204,
        decode = { encoded ->
            if (encoded.isNotEmpty()) throw SerializationException("kill response was not empty")
        },
    )

    internal fun terminalRequest(bearer: GatewayBearer, session: AgentSession): Request =
        authorizedRequest(bearer, listOf("v1", "sessions", session.id, "terminal"))
            .header("Skidbladnir-Session-Identity", session.identityToken)
            .build()

    private fun authorizedRequest(bearer: GatewayBearer, segments: List<String>): Request.Builder {
        val url = gatewayOrigin.newBuilder()
            .apply { segments.forEach(::addPathSegment) }
            .build()
        return Request.Builder()
            .url(url)
            .header("Authorization", "Bearer ${bearer.encoded}")
            .header("Accept", "application/json")
    }

    private fun <Value> executeJson(
        request: Request,
        expectedStatus: Int,
        decode: (String) -> Value,
    ): GatewayResult<Value> {
        return try {
            http.newCall(request).execute().use { response ->
                val encoded = response.body?.string().orEmpty()
                if (response.code != expectedStatus) {
                    return GatewayResult.Failure(decodeFailure(response.code, encoded))
                }
                GatewayResult.Success(decodeProtocol { decode(encoded) })
            }
        } catch (_: IOException) {
            GatewayResult.Failure(GatewayFailure.Transport)
        }
    }

    private fun decodeFailure(status: Int, encoded: String): GatewayFailure {
        return decodeProtocol {
            val response = productJson.decodeFromString<ErrorResponse>(encoded)
            val code = parseApiErrorCode(response.code)
            if (code == ApiErrorCode.ReconnectRequired ||
                status != apiErrorHttpStatus(code) ||
                response.message != apiErrorMessage(code)
            ) {
                throw SerializationException("HTTP error response disagreed with the owned protocol")
            }
            GatewayFailure.Api(code)
        }
    }
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
    ApiErrorCode.SessionNameConflict, ApiErrorCode.SessionIdentityMismatch, ApiErrorCode.SessionGroupedConflict -> 409
    ApiErrorCode.SessionNotFound -> 404
    ApiErrorCode.InternalError -> 500
    ApiErrorCode.ReconnectRequired -> throw SerializationException("terminal error has no HTTP status")
}
