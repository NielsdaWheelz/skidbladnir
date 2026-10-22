package dev.niels.skidbladnir

import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody

internal enum class NotificationEnrollmentResult { Installed, Conflict, Unavailable }

/** Hub state is skid's own output: decode what the phone reads and ignore the rest. */
private val hubJson = Json { ignoreUnknownKeys = true }

/** The hub's notification routes over the pinned bearer transport; every failure reads as unavailable. */
internal class NotificationClient {
    private val gateway = GatewayClient()

    fun state(credential: MachineCredential): HubNotifications? {
        val request = gateway.authorizedRequest(credential, listOf("v1", "notifications", "state")).get().build()
        return try {
            gateway.http.newCall(request).apply { timeout().timeout(3, TimeUnit.SECONDS) }.execute().use { response ->
                if (response.code != 200) return null
                val bytes = checkNotNull(response.body).byteStream().readNBytes(1_048_577)
                if (bytes.size > 1_048_576) null else hubJson.decodeFromString<HubNotifications>(bytes.decodeToString())
            }
        } catch (_: IOException) { null }
          catch (_: IllegalArgumentException) { null } // includes SerializationException
          catch (_: IllegalStateException) { null }
    }

    fun handled(credential: MachineCredential, slot: String, episode: Long, readyGeneration: Long): Boolean =
        send(credential, "POST", listOf("v1", "notifications", "handled"),
            mapOf("slot" to JsonPrimitive(slot), "episode" to JsonPrimitive(episode), "readyGeneration" to JsonPrimitive(readyGeneration))) == 204

    fun register(credential: MachineCredential, configuration: String, generation: Long, fid: String): NotificationEnrollmentResult =
        when (send(credential, "PUT", listOf("v1", "notifications", "receivers", "me"),
            mapOf("configuration" to JsonPrimitive(configuration), "generation" to JsonPrimitive(generation), "fid" to JsonPrimitive(fid)))) {
            204 -> NotificationEnrollmentResult.Installed
            409 -> NotificationEnrollmentResult.Conflict
            else -> NotificationEnrollmentResult.Unavailable
        }

    private fun send(credential: MachineCredential, method: String, segments: List<String>, fields: Map<String, JsonPrimitive>): Int? {
        val body = JsonObject(fields).toString().toRequestBody("application/json".toMediaType())
        val request = gateway.authorizedRequest(credential, segments).method(method, body).build()
        return try {
            gateway.http.newCall(request).apply { timeout().timeout(5, TimeUnit.SECONDS) }.execute().use { it.code }
        } catch (_: IOException) { null }
    }
}
