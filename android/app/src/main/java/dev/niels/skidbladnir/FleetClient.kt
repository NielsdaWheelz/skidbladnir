package dev.niels.skidbladnir

import java.io.IOException
import java.security.GeneralSecurityException
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

/** The expected failures of hub and fleet-store work: null, so the caller shows its failure state. Anything else is a defect. */
internal inline fun <T> fleetAttempt(block: () -> T): T? = try {
    block()
} catch (_: IOException) { null }
  catch (_: IllegalArgumentException) { null } // includes SerializationException
  catch (_: IllegalStateException) { null }
  catch (_: GeneralSecurityException) { null }

internal sealed interface Fetch {
    data class Fetched(val credentials: List<MachineCredential>) : Fetch
    data object Unauthorized : Fetch
    data object Failed : Fetch
}

/** The phone talks to one fixed hub. A staged identity is saved before any request, so a lost join reply recovers. */
internal class FleetClient(private val store: MachineStore) {
    @Volatile private var stream: Call? = null
    fun connect(invite: FleetInvite): List<MachineCredential>? {
        val saved = store.association()
        require(saved == null || saved.fleetId == invite.fleetId)
        if (saved != null) {
            // Repair keeps the identity; a staged identity may already be enrolled by a lost reply.
            val result = fetch(saved, (if (saved.joined) invite.hub else saved.invitation.hub).paired())
            if (result is Fetch.Fetched) return result.credentials
            if (saved.joined && result != Fetch.Unauthorized) return null
        }
        val random = SecureRandom()
        val staged = if (saved == null || saved.joined) PhoneFleet(fleetId = invite.fleetId,
            deviceId = "d-" + ByteArray(16).also(random::nextBytes).joinToString("") { "%02x".format(it) },
            bearer = fleetBase64(ByteArray(32).also(random::nextBytes)), invitation = invite, view = saved?.view)
            else saved.copy(invitation = invite)
        store.save(staged)
        return join(staged)
    }
    fun resume(): List<MachineCredential>? {
        val value = store.association()?.takeUnless { it.joined } ?: return null
        val result = fetch(value, value.invitation.hub.paired())
        return if (result is Fetch.Fetched) result.credentials else join(value)
    }
    fun refresh(timeoutMillis: Long = 15_000): List<MachineCredential>? {
        val value = fleetAttempt { store.association() } ?: return null
        val hub = value.hubCredential() ?: return null
        return (fetch(value, hub.machine, timeoutMillis) as? Fetch.Fetched)?.credentials
    }
    internal fun fetch(value: PhoneFleet, hub: PairedMachine, timeoutMillis: Long = 15_000): Fetch =
        exchange(value, GatewayClient.authorizedRequest(value.credential(hub), listOf("v1", "fleet")).get().build(), 200, timeoutMillis)
    private fun join(value: PhoneFleet): List<MachineCredential>? {
        val verifier = fleetBase64(MessageDigest.getInstance("SHA-256").digest(fleetBytes(value.bearer, 32)))
        val body = JsonObject(mapOf("token" to JsonPrimitive(value.invitation.token), "deviceId" to JsonPrimitive(value.deviceId),
            "kind" to JsonPrimitive("phone"), "name" to JsonPrimitive("Phone"), "verifier" to JsonPrimitive(verifier)))
        val request = GatewayClient.authorizedRequest(value.credential(value.invitation.hub.paired()), listOf("v1", "fleet", "join"))
            .removeHeader("Authorization").post(body.toString().toRequestBody("application/json".toMediaType())).build()
        return (exchange(value, request, 201, 15_000) as? Fetch.Fetched)?.credentials
    }
    /** One hub round trip whose success is a fleet view committed for exactly this identity. */
    private fun exchange(value: PhoneFleet, request: Request, success: Int, timeoutMillis: Long): Fetch = fleetAttempt {
        val call = GatewayClient.http.newCall(request).apply { timeout().timeout(timeoutMillis, TimeUnit.MILLISECONDS) }
        call.execute().use { response ->
            when (response.code) {
                success -> {
                    val bytes = checkNotNull(response.body).byteStream().readNBytes(1_048_577)
                    require(bytes.size <= 1_048_576)
                    Fetch.Fetched(store.accept(productJson.decodeFromString<PublicFleet>(bytes.decodeToString()), value))
                }
                401 -> Fetch.Unauthorized
                else -> Fetch.Failed
            }
        }
    } ?: Fetch.Failed
    /** A revision event is only a wake-up; the refetched view is the state. Returns when the stream ends; the caller retries. */
    suspend fun watch(changed: () -> Unit) {
        val hub = fleetAttempt { store.association()?.hubCredential() } ?: return
        val request = GatewayClient.authorizedRequest(hub, listOf("v1", "fleet", "events"))
            .header("Accept", "text/event-stream").get().build()
        val call = GatewayClient.http.newBuilder().readTimeout(0, TimeUnit.SECONDS).callTimeout(0, TimeUnit.SECONDS).build().newCall(request)
        stream = call
        try {
            // The blocking reads ignore coroutine cancellation: a cancel that came before this registration
            // stops here, and one after it reaches the call through stopWatching.
            currentCoroutineContext().ensureActive()
            refresh()?.let { changed() }
            call.execute().use { response ->
                if (response.code != 200) return
                val source = checkNotNull(response.body).source()
                while (!call.isCanceled()) {
                    if (source.readUtf8LineStrict(1_024).isEmpty()) refresh()?.let { changed() }
                }
            }
        } catch (_: IOException) { /* the foreground owner retries the invalidated stream */ }
        finally { if (stream === call) stream = null }
    }
    fun stopWatching() { stream?.cancel() }
}
