package dev.niels.skidbladnir

import java.io.IOException
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.concurrent.TimeUnit
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement
import okhttp3.Call
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody

/** Phone requests only public snapshots. A saved pending identity survives uncertain join outcomes. */
internal class FleetClient(private val store: MachineStore) {
    private val gateway = GatewayClient()
    @Volatile private var stream: Call? = null
    fun connect(invite: FleetInvite): List<MachineCredential>? {
        var association = store.association()
        require(association == null || association.fleetId == invite.fleetId && association.publicKey == invite.publicKey)
        if (association == null || association.bearer == null) association = stage(invite, association)
        // Invitation proof binds this endpoint to the pinned fleet key before bearer reuse.
        val recovered = if (association.joined) refresh(association, invite.hub.paired()) ?: refresh()
            else refresh(association, association.invitation.hub.paired())
        if (recovered != null) return recovered
        // A signed removal may have committed while the recovery request was in flight.
        association = store.association() ?: return null
        if (association.bearer == null) association = stage(invite, association)
        if (association.joined) return null
        if (association.invitation != invite) { association = association.copy(invitation = invite); store.save(association) }
        return join(association)
    }
    private fun stage(invite: FleetInvite, previous: PhoneFleet?): PhoneFleet {
        val random = SecureRandom()
        val value = PhoneFleet(fleetId = invite.fleetId, publicKey = invite.publicKey,
            deviceId = "d-" + ByteArray(16).also(random::nextBytes).joinToString("") { "%02x".format(it) },
            bearer = fleetBase64(ByteArray(32).also(random::nextBytes)), invitation = invite, envelope = previous?.envelope)
        store.save(value)
        return value
    }
    fun resume(): List<MachineCredential>? {
        val value = store.association()?.takeIf { it.bearer != null } ?: return null
        refresh(value)?.let { return it }
        val current = store.association() ?: return null
        return if (current.bearer != null && !current.joined) join(current) else null
    }
    private fun join(value: PhoneFleet): List<MachineCredential>? {
        val verifier = fleetBase64(MessageDigest.getInstance("SHA-256").digest(fleetBytes(checkNotNull(value.bearer), 32)))
        val body = JsonObject(mapOf("token" to JsonPrimitive(value.invitation.token), "deviceId" to JsonPrimitive(value.deviceId),
            "kind" to JsonPrimitive("phone"), "name" to JsonPrimitive("Phone"), "verifier" to JsonPrimitive(verifier)))
        val request = gateway.authorizedRequest(MachineCredential(value.invitation.hub.paired(), checkNotNull(GatewayBearer.parse(checkNotNull(value.bearer)))), listOf("v1", "fleet", "join"))
            .removeHeader("Authorization").post(body.toString().toRequestBody("application/json".toMediaType())).build()
        return try {
            gateway.http.newCall(request).execute().use { response ->
                if (response.code != 201) return null
                store.accept(decodeEnvelope(response), value)
            }
        } catch (_: IOException) { null }
    }
    fun refresh(timeoutMillis: Long = 15_000): List<MachineCredential>? {
        val value = store.association()?.takeIf { it.bearer != null } ?: return null
        val public = value.envelope?.let { verifyPublicFleet(it, value.publicKey) }
        val hosts = (public?.machines?.sortedBy { it.machine != public.activeHub } ?: listOf(value.invitation.hub)).filter { it.origin != null }
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMillis)
        for ((index, host) in hosts.withIndex()) {
            val remaining = TimeUnit.NANOSECONDS.toMillis(deadline - System.nanoTime())
            if (remaining <= 0) break
            refresh(value, host.paired(), maxOf(1, remaining / (hosts.size - index)))?.let { return it }
            if (store.association()?.bearer == null) return null
        }
        return null
    }
    fun refresh(value: PhoneFleet, hub: PairedMachine = value.invitation.hub.paired(), timeoutMillis: Long = 15_000): List<MachineCredential>? = try {
        val credential = MachineCredential(hub, checkNotNull(GatewayBearer.parse(checkNotNull(value.bearer))))
        val call = gateway.http.newCall(gateway.authorizedRequest(credential, listOf("v1", "fleet")).get().build())
            .apply { timeout().timeout(timeoutMillis, TimeUnit.MILLISECONDS) }
        call.execute().use { response ->
            if (response.code == 403) { acceptRemoval(response); return null }
            if (response.code != 200) return null
            store.accept(decodeEnvelope(response), value)
        }
    } catch (_: IOException) { null }
      catch (_: IllegalArgumentException) { null }
      catch (_: kotlinx.serialization.SerializationException) { null }
      catch (_: java.security.GeneralSecurityException) { null }
    private fun decodeEnvelope(response: okhttp3.Response): FleetEnvelope {
        require(response.body?.contentType()?.let { it.type == "application" && it.subtype == "json" } == true)
        val bytes = checkNotNull(response.body).byteStream().readNBytes(5_600_001)
        require(bytes.size <= 5_600_000)
        return productJson.decodeFromJsonElement(notificationJson(bytes.decodeToString(throwOnInvalidSequence = true)))
    }
    fun acceptRemoval(response: okhttp3.Response): Boolean {
        if (response.code != 403) return false
        val bytes = response.peekBody(5_600_001).bytes()
        if (bytes.size > 5_600_000) return false
        return try {
            val value = notificationJson(bytes.decodeToString(throwOnInvalidSequence = true))
            value.requireExactKeys(setOf("code", "message", "removal"))
            require((value.getValue("code") as JsonPrimitive).content == "DeviceRemoved")
            store.remove(productJson.decodeFromJsonElement(value.getValue("removal")))
        } catch (_: IllegalArgumentException) { false }
          catch (_: kotlinx.serialization.SerializationException) { false }
          catch (_: java.security.GeneralSecurityException) { false }
    }
    /** A revision is only a wake-up; the signed snapshot alone can change fleet authority. */
    fun watch(changed: () -> Unit) {
        val value = store.association() ?: return
        val hub = value.hubCredential() ?: return
        val request = gateway.authorizedRequest(hub, listOf("v1", "fleet", "events"))
            .header("Accept", "text/event-stream").get().build()
        val call = gateway.http.newBuilder().readTimeout(0, TimeUnit.SECONDS).callTimeout(0, TimeUnit.SECONDS).build().newCall(request)
        stream = call
        try {
            refresh()?.let { changed() }
            if (store.association()?.hubCredential() != hub) return
            call.execute().use { response ->
                if (response.code == 403) { if (acceptRemoval(response)) changed(); return }
                if (response.code != 200 || response.body?.contentType()?.toString() != "text/event-stream") return
                val source = checkNotNull(response.body).source()
                while (!call.isCanceled()) {
                    val line = source.readUtf8LineStrict(1_024)
                    if (line.isEmpty()) {
                        refresh()?.let { changed() }
                        if (store.association()?.hubCredential() != hub) return
                    }
                }
            }
        } catch (_: IOException) { /* the foreground owner retries the invalidated stream */ }
        finally { if (stream === call) stream = null }
    }
    fun stopWatching() { stream?.cancel() }
}
