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
internal class NotificationClient {
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

    fun enroll(
        credential: MachineCredential, subscription: NotificationSubscription, receiverTag: String,
    ): NotificationEnrollmentResult = try {
        val encoded = productJson.encodeToString(subscription)
        require(encoded.encodeToByteArray().size <= 4_096)
        val request = gateway.authorizedRequest(credential, listOf("v1", "notifications", "receivers", "android"))
            .header("If-Match", "\"$receiverTag\"")
            .put(encoded.toRequestBody("application/json".toMediaType())).build()
        val call = gateway.http.newCall(request).apply { timeout().timeout(3, TimeUnit.SECONDS) }
        call.execute().use { response ->
            when (response.code) {
                204 -> if (response.body?.byteStream()?.read() == -1) NotificationEnrollmentResult.Installed
                    else NotificationEnrollmentResult.Unavailable
                412 -> NotificationEnrollmentResult.Conflict
                else -> NotificationEnrollmentResult.Unavailable
            }
        }
    } catch (_: IOException) {
        NotificationEnrollmentResult.Unavailable
    }
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
