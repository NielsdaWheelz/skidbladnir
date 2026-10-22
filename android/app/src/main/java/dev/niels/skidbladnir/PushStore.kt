package dev.niels.skidbladnir

import android.content.Context
import android.util.AtomicFile
import java.io.IOException
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString

/** The fcm destination last registered for target; the hub activates it on an authenticated PUT. */
@Serializable internal data class PushRegistration(val generation: Long, val fid: String)
@Serializable internal data class PushTarget(val configuration: String, val fleetId: String, val deviceId: String)
internal data class PushAttempt(val target: PushTarget, val generationFloor: Long)
@Serializable private data class PushState(
    val generationFloor: Long = 0, val configuration: String? = null, val resetRequired: Boolean = false,
    val target: PushTarget? = null, val registration: PushRegistration? = null,
)

/** One atomic push-only record preserves the generation floor across disable, options and pairing changes. */
internal class PushStore(context: Context) {
    private val file = AtomicFile(java.io.File(context.filesDir, "push-v3.json"))
    private fun state(): PushState =
        if (!file.baseFile.exists()) PushState() else productJson.decodeFromString(file.readFully().decodeToString())
    private fun write(value: PushState) {
        val output = file.startWrite()
        try {
            output.write(productJson.encodeToString(value).encodeToByteArray())
            file.finishWrite(output)
        } catch (failure: IOException) { file.failWrite(output); throw failure }
    }
    fun read(): PushRegistration? = synchronized(lock) { state().registration }
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
        old.registration?.takeIf { it.fid == fid }?.let { return@synchronized it }
        val registration = PushRegistration(old.generationFloor + 1, fid)
        write(old.copy(generationFloor = registration.generation, registration = registration))
        registration
    }
    /** The hub refused this registration: the next attempt drops the cached route and takes a new generation. */
    fun renew(expected: PushRegistration) = synchronized(lock) {
        val old = state()
        if (old.registration == expected) write(old.copy(registration = null, resetRequired = true))
    }
    private companion object { val lock = Any() }
}
