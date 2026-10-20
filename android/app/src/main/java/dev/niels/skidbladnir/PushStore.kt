package dev.niels.skidbladnir

import android.content.Context
import android.util.AtomicFile
import java.io.IOException
import java.security.SecureRandom
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal data class PushRegistration(
    val configuration: String, val fleetId: String, val deviceId: String, val generation: Long,
    val fid: String, val clientNonce: String, val activeGeneration: Long = 0,
    val status: String = "pending", val challenge: String? = null, val hint: NotificationHint? = null,
    val transportUnavailable: Boolean = false,
) {
    init {
        require(configuration.matches(Regex("[0-9a-f]{64}")) && fleetId.matches(Regex("[0-9a-f]{64}")) && deviceId.matches(Regex("d-[0-9a-f]{32}")))
        require(generation in 1 until Long.MAX_VALUE && activeGeneration in 0..generation)
        require(fid.matches(Regex("[A-Za-z0-9_-]{1,256}")) && isCanonicalBase64Url256(clientNonce))
        require(status in setOf("unconfigured", "active", "pending", "unavailable", "removed"))
        require(challenge == null || challenge.matches(Regex("[A-Za-z0-9_-]{1,128}")))
    }
}
@Serializable internal data class ReceiverStatus(
    val configuration: String? = null, val activeGeneration: Long, val desiredGeneration: Long, val status: String,
) {
    init {
        require(configuration == null || configuration.matches(Regex("[0-9a-f]{64}")))
        require(activeGeneration >= 0 && desiredGeneration >= activeGeneration)
        require(status in setOf("disabled", "unconfigured", "active", "pending", "unavailable", "removed"))
        require((status == "disabled") == (configuration == null))
        require(status != "active" || activeGeneration > 0 && activeGeneration == desiredGeneration)
    }
}
@Serializable private data class PushState(
    val schema: Int = 2, val generationFloor: Long = 0, val configuration: String? = null,
    val resetRequired: Boolean = false, val target: PushTarget? = null, val registration: PushRegistration? = null,
) {
    init {
        require(schema == 2 && generationFloor in 0 until Long.MAX_VALUE)
        require(configuration == null || configuration.matches(Regex("[0-9a-f]{64}")))
        require(!resetRequired || configuration != null)
        require(target == null || target.configuration == configuration)
        require(registration == null || target != null && registration.configuration == configuration && registration.fleetId == target.fleetId &&
            registration.deviceId == target.deviceId && registration.generation == generationFloor)
    }
}
@Serializable internal data class PushTarget(val configuration: String, val fleetId: String, val deviceId: String)
internal data class PushAttempt(val target: PushTarget, val generationFloor: Long)

/** One atomic push-only record preserves the generation floor across disable, options and pairing changes. */
internal class PushStore(context: Context) {
    private val file = AtomicFile(java.io.File(context.filesDir, "push-v2.json"))
    private fun state(): PushState {
        if (!file.baseFile.exists()) return PushState()
        val bytes = file.openRead().use { it.readNBytes(8_193) }
        require(bytes.size <= 8_192)
        return productJson.decodeFromJsonElement(notificationJson(bytes.decodeToString(throwOnInvalidSequence = true)))
    }
    fun read(): PushRegistration? = synchronized(lock) { state().registration }
    private fun write(value: PushState) {
        val bytes = productJson.encodeToString(value).encodeToByteArray()
        require(bytes.size <= 8_192)
        val output = file.startWrite()
        try {
            output.write(bytes)
            output.fd.sync()
            file.finishWrite(output)
            if (!file.openRead().use { it.readNBytes(bytes.size + 1) }.contentEquals(bytes)) throw IOException("push commit unavailable")
        } catch (failure: IOException) { file.failWrite(output); throw failure }
    }
    fun reconcile(target: PushTarget?) = synchronized(lock) {
        val old = state()
        val next = when {
            target == null -> old.copy(target = null, registration = null)
            old.configuration != target.configuration -> old.copy(configuration = target.configuration, resetRequired = true, target = target, registration = null)
            old.target != target -> old.copy(target = target, registration = null)
            else -> old
        }
        if (next != old) write(next)
    }
    fun resetRequired(): Boolean = synchronized(lock) { state().resetRequired }
    fun resetCompleted(configuration: String) = synchronized(lock) {
        val old = state()
        if (old.configuration == configuration) write(old.copy(resetRequired = false))
    }
    fun attempt(target: PushTarget): PushAttempt = synchronized(lock) {
        val current = state()
        require(current.target == target && !current.resetRequired)
        PushAttempt(target, current.generationFloor)
    }
    fun registered(attempt: PushAttempt, fid: String): PushRegistration? = synchronized(lock) {
        val old = state()
        if (old.target != attempt.target || old.resetRequired || old.generationFloor != attempt.generationFloor) return@synchronized null
        val previous = old.registration?.takeIf { it.matches(attempt.target) }
        if (previous?.fid == fid && !previous.transportUnavailable) return@synchronized previous
        val generation = old.generationFloor + 1
        val registration = PushRegistration(configuration = attempt.target.configuration,
            fleetId = attempt.target.fleetId, deviceId = attempt.target.deviceId, generation = generation, fid = fid,
            clientNonce = fleetBase64(ByteArray(32).also(SecureRandom()::nextBytes)), activeGeneration = previous?.activeGeneration ?: 0)
        write(old.copy(generationFloor = generation, registration = registration))
        registration
    }
    fun status(expected: PushRegistration, status: ReceiverStatus, confirmed: Boolean = false): Boolean = synchronized(lock) {
        val old = state()
        val current = old.registration ?: return@synchronized false
        if (!current.sameTuple(expected) || status.configuration != current.configuration || status.desiredGeneration != current.generation ||
            status.activeGeneration < current.activeGeneration) return@synchronized false
        write(old.copy(registration = current.copy(activeGeneration = status.activeGeneration, status = status.status,
            transportUnavailable = current.transportUnavailable && !(confirmed && status.status == "active"))))
        true
    }
    fun registrationUnavailable(expected: PushRegistration) = synchronized(lock) {
        val old = state()
        old.registration?.takeIf { it.sameTuple(expected) }?.let { write(old.copy(registration = it.copy(status = "unavailable"))) }
    }
    fun renew(expected: PushRegistration) = synchronized(lock) {
        val old = state()
        if (old.registration?.sameTuple(expected) == true) write(old.copy(registration = null, resetRequired = true))
    }
    fun rejectChallenge(expected: PushRegistration) = synchronized(lock) {
        val old = state()
        old.registration?.takeIf { it.sameTuple(expected) && it.challenge == expected.challenge }?.let {
            write(old.copy(registration = it.copy(challenge = null, status = "pending")))
        }
    }
    fun unavailable(target: PushTarget) = synchronized(lock) {
        val old = state()
        old.registration?.takeIf { it.matches(target) }?.let { write(old.copy(registration = it.copy(transportUnavailable = true))) }
    }
    fun accept(target: PushTarget, data: Map<String, String>): Boolean = synchronized(lock) {
        val old = state()
        val current = old.registration ?: return@synchronized false
        if (old.resetRequired || !current.matches(target) || data["configuration"] != target.configuration ||
            data["fleetId"] != current.fleetId || data["deviceId"] != current.deviceId) return@synchronized false
        val next = when (data["kind"]) {
            "enrollment" -> {
                if (data.keys != setOf("kind", "fleetId", "deviceId", "configuration", "generation", "clientNonce", "challenge") ||
                    data["generation"] != current.generation.toString() || data["clientNonce"] != current.clientNonce) return@synchronized false
                val challenge = checkNotNull(data["challenge"])
                if (!challenge.matches(Regex("[A-Za-z0-9_-]{1,128}"))) return@synchronized false
                current.copy(challenge = challenge)
            }
            "hint" -> {
                if (data.keys != setOf("kind", "schema", "fleetId", "deviceId", "configuration", "receiverGeneration", "epoch", "revision", "urgency") ||
                    data["schema"] != "1" || current.activeGeneration == 0L || data["receiverGeneration"] != current.activeGeneration.toString() ||
                    data["urgency"] !in setOf("attention", "sync")) return@synchronized false
                val revision = data["revision"]?.takeIf { it.matches(Regex("0|[1-9][0-9]*")) }?.toLongOrNull() ?: return@synchronized false
                val hint = NotificationHint(1, checkNotNull(data["epoch"]), revision)
                if (current.hint?.epoch == hint.epoch && current.hint.revision > hint.revision) return@synchronized false
                current.copy(hint = hint)
            }
            else -> return@synchronized false
        }
        write(old.copy(registration = next))
        true
    }
    private fun PushRegistration.sameTuple(other: PushRegistration): Boolean =
        configuration == other.configuration && fleetId == other.fleetId && deviceId == other.deviceId &&
            generation == other.generation && fid == other.fid && clientNonce == other.clientNonce
    private fun PushRegistration.matches(target: PushTarget): Boolean =
        configuration == target.configuration && fleetId == target.fleetId && deviceId == target.deviceId
    private companion object { val lock = Any() }
}
