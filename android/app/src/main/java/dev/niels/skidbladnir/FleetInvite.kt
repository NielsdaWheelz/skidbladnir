package dev.niels.skidbladnir

import java.nio.charset.CharacterCodingException
import java.time.DateTimeException
import java.time.Instant
import java.util.Base64
import kotlinx.serialization.Serializable
import kotlinx.serialization.decodeFromString

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
/** An origin the phone cannot pin pairs as origin-less: shown, never reachable. */
@Serializable internal data class FleetMachine(val machine: String, val name: String, val origin: String? = null) {
    fun paired(): PairedMachine = PairedMachine(MachineHandle(machine), MachineLabel(name), origin?.let(MachineOrigin::parse))
}
/** The hub endpoint plus a one-use, five-minute join token; the hub alone enforces use and expiry. */
@Serializable internal data class FleetInvite(val fleetId: String, val hub: FleetMachine, val token: String, val expiresAt: String)
@Serializable internal data class AndroidPush(
    val id: String, val projectId: String, val applicationId: String, val senderId: String, val apiKey: String,
)
/** What the phone reads from the hub's fleet view; the hub sends more and the phone ignores it. */
@Serializable internal data class PublicFleet(val hub: String, val machines: List<FleetMachine>, val androidPush: AndroidPush? = null)
/** Scanned text is human ingress and the only place fleet values are checked: null on any malformed or expired line. */
internal fun parseFleetInvite(encoded: String): FleetInvite? = try {
    require(encoded.startsWith("skid://connect/") && encoded.length <= 8_192)
    val text = fleetBytes(encoded.removePrefix("skid://connect/"), 4_096).decodeToString(throwOnInvalidSequence = true)
    val invite = productJson.decodeFromString<FleetInvite>(text)
    val hub = invite.hub
    require(invite.fleetId.matches(Regex("[0-9a-f]{64}")) && isCanonicalBase64Url256(invite.token))
    require(hub.machine.matches(Regex("mh-[0-9a-f]{32}")))
    require(hub.name.isNotEmpty() && hub.name == hub.name.trim() && !hub.name.hasDisplayUnsafeCodePoint())
    require(hub.origin != null && MachineOrigin.parse(hub.origin)?.encoded?.removeSuffix("/") == hub.origin)
    require(Instant.parse(invite.expiresAt).isAfter(Instant.now()))
    invite
} catch (_: IllegalArgumentException) { null } // includes SerializationException
  catch (_: CharacterCodingException) { null }
  catch (_: DateTimeException) { null }
