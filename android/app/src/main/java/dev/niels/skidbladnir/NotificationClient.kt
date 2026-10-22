package dev.niels.skidbladnir

import java.io.IOException
import java.nio.charset.CharacterCodingException
import java.util.concurrent.TimeUnit
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

internal enum class NotificationEnrollmentResult { Installed, Conflict, Unavailable }

/** One pinned bearer transport. Its total deadline includes TLS, body and decoding. */
internal class NotificationClient(private val context: android.content.Context) {
    private val gateway = GatewayClient()

    fun state(credential: MachineCredential): GatewayResult<ObserverSnapshot> = try {
        val request = gateway.authorizedRequest(credential, listOf("v1", "notifications", "state")).get().build()
        val call = gateway.http.newCall(request).apply { timeout().timeout(3, TimeUnit.SECONDS) }
        call.execute().use { response ->
            decodeNotificationState(response)
        }
    } catch (_: IOException) {
        GatewayResult.Failure(GatewayFailure.Transport)
    }

    fun receiver(credential: MachineCredential): ReceiverStatus? = try {
        val request = gateway.authorizedRequest(credential, listOf("v1", "notifications", "receivers", "me")).get().build()
        gateway.http.newCall(request).apply { timeout().timeout(3, TimeUnit.SECONDS) }.execute().use(::receiverStatus)
    } catch (_: IOException) { null }

    private fun receiverStatus(response: Response): ReceiverStatus? {
        if (response.code != 200) return null
        val body = checkNotNull(response.body)
        require(body.contentType()?.let { it.type == "application" && it.subtype == "json" } == true)
        val bytes = body.byteStream().readNBytes(4_097)
        require(bytes.size <= 4_096)
        return productJson.decodeFromJsonElement<ReceiverStatus>(notificationJson(bytes.decodeToString(throwOnInvalidSequence = true)))
    }

    fun enroll(credential: MachineCredential, registration: PushRegistration): ReceiverStatus? = try {
        val proof = registration.challenge
        val path = listOf("v1", "notifications", "receivers", "me") + if (proof == null) emptyList() else listOf("confirm")
        val fields = if (proof == null) mapOf("configuration" to JsonPrimitive(registration.configuration), "generation" to JsonPrimitive(registration.generation),
            "fid" to JsonPrimitive(registration.fid), "clientNonce" to JsonPrimitive(registration.clientNonce))
            else mapOf("configuration" to JsonPrimitive(registration.configuration), "generation" to JsonPrimitive(registration.generation), "challenge" to JsonPrimitive(proof))
        val body = kotlinx.serialization.json.JsonObject(fields).toString().toRequestBody("application/json".toMediaType())
        val builder = gateway.authorizedRequest(credential, path)
        val request = (if (proof == null) builder.put(body) else builder.post(body)).build()
        gateway.http.newCall(request).apply { timeout().timeout(5, TimeUnit.SECONDS) }.execute().use { response ->
            if (response.code == 409) {
                if (context.firebasePush().target() == PushTarget(registration.configuration, registration.fleetId, registration.deviceId)) {
                    if (proof != null) PushStore(context).rejectChallenge(registration) else PushStore(context).renew(registration)
                }
                return null
            }
            receiverStatus(response)
        }
    } catch (_: IOException) { null }

}

/** Observer unavailability has a closed body; ordinary gateway failures keep their existing contract. */
private fun decodeNotificationState(response: Response): GatewayResult<ObserverSnapshot> {
    if (response.code == 503) return decodeProtocol {
        val body = requireNotNull(response.body)
        val mediaType = body.contentType()
        require(mediaType?.type == "application" && mediaType.subtype == "json")
        val bytes = body.byteStream().readNBytes(1_048_577)
        require(bytes.size <= 1_048_576)
        val encoded = try {
            bytes.decodeToString(throwOnInvalidSequence = true)
        } catch (_: CharacterCodingException) {
            throw SerializationException("invalid notification error UTF-8")
        }
        val failure = strictJsonObject(encoded)
        val code = failure["code"] as? JsonPrimitive
        val message = failure["message"] as? JsonPrimitive
        require(failure.keys == setOf("code", "message") && code?.isString == true && message?.isString == true)
        require(code.content == "NotificationsUnavailable" && message.content == "notifications unavailable")
        GatewayResult.Failure(GatewayFailure.Transport)
    }
    return decodeGatewayResponse(response, 200, { encoded ->
        try {
            productJson.decodeFromJsonElement<ObserverSnapshot>(notificationJson(encoded))
        } catch (_: java.time.DateTimeException) {
            throw SerializationException("invalid observer time")
        }
    }, decodeFailure = { status, encoded ->
        val failure = decodeGatewayHttpFailure(status, encoded)
        if (failure is GatewayFailure.Api && failure.code !in setOf(
                ApiErrorCode.Unauthenticated, ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge,
                ApiErrorCode.MachineIdentityMismatch, ApiErrorCode.InternalError,
            )) throw ProtocolDecodeException("notification route error set")
        failure
    }, maximumBodyBytes = 1_048_576)
}
