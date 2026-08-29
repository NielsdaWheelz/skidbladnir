package dev.niels.skidbladnir

import java.io.IOException
import java.nio.charset.StandardCharsets
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.decodeFromJsonElement
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

private val jsonMediaType = "application/json; charset=utf-8".toMediaType()
internal const val MAXIMUM_HTTP_BODY_BYTES = 64 * 1024

internal class GatewayBearer private constructor(internal val encoded: String) {
    companion object {
        fun parse(candidate: String): GatewayBearer? =
            candidate.takeIf(::isCanonicalBase64Url256)?.let(::GatewayBearer)
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
@Serializable private data class WirePairingMachine(val handle: String, val platform: MachinePlatform)
@Serializable private data class WirePairingResponse(val machine: WirePairingMachine, val bearer: String)

internal data class PairingResponse(
    val machine: MachineSummary,
    val bearer: GatewayBearer,
)

internal fun decodePairingResponse(encoded: String): PairingResponse = decodeProtocol {
    val wire = productJson.decodeFromJsonElement<WirePairingResponse>(strictJsonObject(encoded))
    PairingResponse(
        machine = MachineSummary(
            requireNotNull(MachineHandle.parse(wire.machine.handle)),
            wire.machine.platform,
        ),
        bearer = requireNotNull(GatewayBearer.parse(wire.bearer)),
    )
}

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

    fun redeemPairing(machine: FleetInviteMachine): GatewayResult<PairingResponse> = executeJson(
        request = pairingRequest(machine),
        expectedStatus = 200,
        decode = ::decodePairingResponse,
        decodeFailure = ::decodePairingHttpFailure,
    )

    internal fun pairingRequest(machine: FleetInviteMachine): Request {
        val url = machine.machine.origin.encoded.toHttpUrl().newBuilder()
            .addPathSegment("v1")
            .addPathSegment("pairings")
            .build()
        return Request.Builder()
            .url(url)
            .header("Authorization", "Skidbladnir-Invite ${machine.pairingInviteToken.encoded}")
            .header("Accept", "application/json")
            .header("Skidbladnir-Machine", machine.machine.handle.encoded)
            .post(ByteArray(0).toRequestBody())
            .build()
    }

    fun createSession(credential: MachineCredential, draft: ForgeDraft): GatewayResult<TmuxSession> {
        require(draft.machineHandle == credential.machine.handle)
        return executeJson(
            request = authorizedRequest(credential, listOf("v1", "sessions"))
                .post(encodeCreateSessionRequest(draft).toRequestBody(jsonMediaType))
                .build(),
            expectedStatus = 201,
            decode = ::decodeTmuxSession,
        )
    }

    fun killSession(credential: MachineCredential, target: SessionTarget): GatewayResult<Unit> {
        require(target.machineHandle == credential.machine.handle)
        return executeBodyless(killRequest(credential, target))
    }

    internal fun killRequest(credential: MachineCredential, target: SessionTarget): Request {
        require(target.machineHandle == credential.machine.handle)
        return authorizedRequest(credential, listOf("v1", "sessions", target.session.tmuxId))
            .delete(encodeKillSessionRequest(target.session).toRequestBody(jsonMediaType))
            .build()
    }

    fun renameSession(
        credential: MachineCredential,
        target: SessionTarget,
        newTmuxName: String,
    ): GatewayResult<Unit> {
        require(target.machineHandle == credential.machine.handle)
        return executeBodyless(
            request = renameRequest(credential, target, newTmuxName),
            decodeFailure = ::decodeRenameHttpFailure,
        )
    }

    internal fun renameRequest(
        credential: MachineCredential,
        target: SessionTarget,
        newTmuxName: String,
    ): Request {
        require(target.machineHandle == credential.machine.handle)
        return authorizedRequest(credential, listOf("v1", "sessions", target.session.tmuxId))
            .patch(encodeRenameSessionRequest(target, newTmuxName).toRequestBody(jsonMediaType))
            .build()
    }

    internal fun terminalRequest(credential: MachineCredential, target: SessionTarget): Request {
        require(target.machineHandle == credential.machine.handle)
        return authorizedRequest(credential, listOf("v1", "sessions", target.session.tmuxId, "terminal"))
            .header("Skidbladnir-Session-Identity", target.session.identityToken)
            .build()
    }

    /**
     * Every ordinary bearer request is bound to a pinned machine; there is no headerless variant.
     * A gateway that answers this origin with another installation's identity fails with
     * `409 MachineIdentityMismatch` before it discloses anything.
     */
    private fun authorizedRequest(credential: MachineCredential, segments: List<String>): Request.Builder {
        val url = credential.machine.origin.encoded.toHttpUrl().newBuilder()
            .apply { segments.forEach(::addPathSegment) }
            .build()
        return Request.Builder()
            .url(url)
            .header("Authorization", "Bearer ${credential.bearer.encoded}")
            .header("Accept", "application/json")
            .header("Skidbladnir-Machine", credential.machine.handle.encoded)
    }

    private fun <Value> executeJson(
        request: Request,
        expectedStatus: Int,
        decode: (String) -> Value,
        decodeFailure: (Int, String) -> GatewayFailure = ::decodeGatewayHttpFailure,
    ): GatewayResult<Value> = try {
        http.newCall(request).execute().use { response ->
            decodeGatewayResponse(response, expectedStatus, decode, decodeFailure)
        }
    } catch (_: IOException) {
        GatewayResult.Failure(GatewayFailure.Transport)
    }

    private fun executeBodyless(
        request: Request,
        decodeFailure: (Int, String) -> GatewayFailure = ::decodeGatewayHttpFailure,
    ): GatewayResult<Unit> = executeJson(
        request = request,
        expectedStatus = 204,
        decode = { encoded ->
            if (encoded.isNotEmpty()) throw SerializationException("bodyless response was not empty")
        },
        decodeFailure = decodeFailure,
    )

}

internal fun <Value> decodeGatewayResponse(
    response: Response,
    expectedStatus: Int,
    decode: (String) -> Value,
    decodeFailure: (Int, String) -> GatewayFailure = ::decodeGatewayHttpFailure,
): GatewayResult<Value> {
    if (response.code in setOf(502, 503, 504)) return GatewayResult.Failure(GatewayFailure.Transport)
    val bodylessSuccess = response.code == expectedStatus && expectedStatus == 204
    val body = response.body
    if (!bodylessSuccess) {
        val mediaType = body?.contentType()
        if (mediaType?.type != "application" || mediaType.subtype != "json") {
            return GatewayResult.Failure(GatewayFailure.Transport)
        }
    }
    // OkHttp presents this ResponseBody after transparent content decompression. Reading one byte
    // beyond the owned 64 KiB protocol bound distinguishes an exact-limit body without buffering
    // an attacker-controlled response through ResponseBody.string().
    val bytes = body?.byteStream()?.readNBytes(MAXIMUM_HTTP_BODY_BYTES + 1) ?: ByteArray(0)
    if (bytes.size > MAXIMUM_HTTP_BODY_BYTES) return GatewayResult.Failure(GatewayFailure.Transport)
    val encoded = String(bytes, StandardCharsets.UTF_8)
    if (bodylessSuccess) {
        if (encoded.isNotEmpty()) return GatewayResult.Failure(GatewayFailure.Transport)
        return GatewayResult.Success(decodeProtocol { decode(encoded) })
    }
    return if (response.code != expectedStatus) {
        GatewayResult.Failure(decodeFailure(response.code, encoded))
    } else {
        GatewayResult.Success(decodeProtocol { decode(encoded) })
    }
}

internal fun decodeGatewayHttpFailure(status: Int, encoded: String): GatewayFailure = decodeProtocol {
    if (status == 502 || status == 503 || status == 504) return@decodeProtocol GatewayFailure.Transport
    val response = productJson.decodeFromJsonElement<ErrorResponse>(strictJsonObject(encoded))
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

internal fun decodePairingHttpFailure(status: Int, encoded: String): GatewayFailure {
    val failure = decodeGatewayHttpFailure(status, encoded)
    if (failure is GatewayFailure.Api && failure.code !in setOf(
            ApiErrorCode.PairingInviteRejected,
            ApiErrorCode.InvalidRequest,
            ApiErrorCode.InternalError,
        )
    ) throw ProtocolDecodeException("pairing route error set")
    return failure
}

internal fun decodeRenameHttpFailure(status: Int, encoded: String): GatewayFailure {
    val failure = decodeGatewayHttpFailure(status, encoded)
    if (failure is GatewayFailure.Api && failure.code !in setOf(
            ApiErrorCode.Unauthenticated,
            ApiErrorCode.InvalidRequest,
            ApiErrorCode.RequestTooLarge,
            ApiErrorCode.SessionNameInvalid,
            ApiErrorCode.SessionNameConflict,
            ApiErrorCode.SessionNotFound,
            ApiErrorCode.SessionIdentityMismatch,
            ApiErrorCode.MachineIdentityMismatch,
            ApiErrorCode.InternalError,
        )
    ) throw ProtocolDecodeException("rename route error set")
    return failure
}

private fun apiErrorHttpStatus(code: ApiErrorCode): Int = when (code) {
    ApiErrorCode.Unauthenticated -> 401
    ApiErrorCode.PairingInviteRejected -> 401
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
