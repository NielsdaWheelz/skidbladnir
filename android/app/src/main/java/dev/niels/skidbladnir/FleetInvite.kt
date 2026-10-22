package dev.niels.skidbladnir

import java.nio.charset.CharacterCodingException
import java.time.Instant
import java.util.Base64
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.decodeFromJsonElement

internal fun fleetBase64(bytes: ByteArray): String = Base64.getUrlEncoder().withoutPadding().encodeToString(bytes)
internal fun fleetBytes(encoded: String, maximum: Int): ByteArray {
    require(encoded.length <= (maximum * 4 + 2) / 3)
    val bytes = Base64.getUrlDecoder().decode(encoded)
    require(bytes.size <= maximum && fleetBase64(bytes) == encoded)
    return bytes
}
internal fun isCanonicalBase64Url256(candidate: String): Boolean = try {
    candidate.length == 43 && fleetBytes(candidate, 32).size == 32
} catch (_: IllegalArgumentException) { false }
@Serializable internal data class FleetMachine(val machine: String, val name: String, val origin: String? = null) {
    init {
        require(MachineHandle.parse(machine) != null && MachineLabel.parse(name) != null)
        require(origin == null || MachineOrigin.parse(origin)?.encoded?.removeSuffix("/") == origin)
    }
    fun paired(): PairedMachine = PairedMachine(checkNotNull(MachineHandle.parse(machine)),
        checkNotNull(MachineLabel.parse(name)), origin?.let { checkNotNull(MachineOrigin.parse(it)) })
}
/** The hub endpoint plus a one-use, five-minute join token; the hub alone enforces use and expiry. */
@Serializable internal data class FleetInvite(val fleetId: String, val hub: FleetMachine, val token: String, val expiresAt: String) {
    init {
        require(fleetId.matches(Regex("[0-9a-f]{64}")) && isCanonicalBase64Url256(token) && hub.origin != null)
        acceptWireInstant(expiresAt)
    }
}
@Serializable internal data class AndroidPush(
    val id: String, val projectId: String, val applicationId: String, val senderId: String, val apiKey: String,
)
/** What the phone reads from the hub's fleet view; the hub sends more and the phone ignores it. */
@Serializable internal data class PublicFleet(val hub: String, val machines: List<FleetMachine>, val androidPush: AndroidPush? = null)
/** Scanned text is human ingress: one strict parse, null on any malformed or expired line. */
internal fun parseFleetInvite(encoded: String): FleetInvite? = try {
    require(encoded.startsWith("skid://connect/") && encoded.length <= 8_192)
    val text = fleetBytes(encoded.removePrefix("skid://connect/"), 4_096).decodeToString(throwOnInvalidSequence = true)
    val invite = productJson.decodeFromJsonElement<FleetInvite>(nonNullJsonObject(text))
    require(acceptWireInstant(invite.expiresAt).isAfter(Instant.now()))
    invite
} catch (_: IllegalArgumentException) { null } // includes SerializationException
  catch (_: CharacterCodingException) { null }
