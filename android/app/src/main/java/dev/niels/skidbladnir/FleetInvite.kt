package dev.niels.skidbladnir

import java.security.MessageDigest
import java.security.KeyFactory
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.time.Instant
import java.util.Base64
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
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
internal fun fleetFingerprint(publicKey: String): String {
    val key = fleetBytes(publicKey, 32)
    require(key.size == 32)
    return MessageDigest.getInstance("SHA-256").digest(key).joinToString("") { "%02x".format(it) }
}
@Serializable internal data class FleetMachine(val machine: String, val name: String, val origin: String? = null) {
    init {
        require(MachineHandle.parse(machine) != null && MachineLabel.parse(name) != null)
        require(origin == null || MachineOrigin.parse(origin)?.encoded?.removeSuffix("/") == origin)
    }
    fun paired(): PairedMachine = PairedMachine(checkNotNull(MachineHandle.parse(machine)),
        checkNotNull(MachineLabel.parse(name)), origin?.let { checkNotNull(MachineOrigin.parse(it)) })
}
@Serializable internal data class FleetInvite(
    val schema: Int, val fleetId: String, val publicKey: String, val hub: FleetMachine,
    val token: String, val expiresAt: String, val proof: FleetEnvelope,
) {
    init {
        require(schema == 1 && fleetFingerprint(publicKey) == fleetId && isCanonicalBase64Url256(token))
        require(hub.origin != null)
        acceptWireInstant(expiresAt)
        val claims = productJson.decodeFromJsonElement<InvitationClaims>(notificationJson(verifiedFleetPayload(proof, publicKey, "invitation")))
        require(claims == InvitationClaims(1, "invitation", fleetId, hub, token, expiresAt))
    }
}
@Serializable internal data class InvitationClaims(
    val schema: Int, val kind: String, val fleetId: String, val hub: FleetMachine, val token: String, val expiresAt: String,
)
@Serializable internal data class FleetEnvelope(val payload: String, val signature: String)
@Serializable internal data class AndroidPush(
    val id: String, val projectId: String, val applicationId: String, val senderId: String, val apiKey: String,
) {
    init {
        require(listOf(projectId, applicationId, senderId, apiKey).all { it.matches(Regex("[A-Za-z0-9:_.-]{1,256}")) })
        val bytes = "skidbladnir.android-push.v1\n$projectId\n$applicationId\n$senderId\n$apiKey".encodeToByteArray()
        require(id == MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) })
    }
}
@Serializable internal data class PublicFleet(
    val schema: Int, val kind: String, val fleetId: String, val authorityEpoch: Long,
    val stateRevision: Long, val configRevision: Long, val activeHub: String, val machines: List<FleetMachine>,
    val androidPush: AndroidPush? = null,
) {
    init {
        require(schema == 1 && kind == "fleet" && fleetId.matches(Regex("[0-9a-f]{64}")))
        require(authorityEpoch > 0 && stateRevision > 0 && configRevision > 0)
        require(machines.isNotEmpty() && machines.distinctBy { it.machine }.size == machines.size)
        require(machines.map { it.name.lowercase(java.util.Locale.ROOT) }.distinct().size == machines.size)
        require(machines.mapNotNull { it.origin }.let { it.distinct().size == it.size })
        require(machines.single { it.machine == activeHub }.machine == activeHub)
    }
}
internal fun verifiedFleetPayload(envelope: FleetEnvelope, publicKey: String, kind: String): String {
    val rawKey = fleetBytes(publicKey, 32)
    require(rawKey.size == 32)
    val payload = fleetBytes(envelope.payload, 4 * 1_024 * 1_024)
    val signature = fleetBytes(envelope.signature, 64)
    require(signature.size == 64)
    val key = KeyFactory.getInstance("Ed25519").generatePublic(X509EncodedKeySpec(
        byteArrayOf(0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00) + rawKey,
    ))
    require(Signature.getInstance("Ed25519").run {
        initVerify(key)
        update("skidbladnir.$kind.v1\u0000".encodeToByteArray())
        update(payload)
        verify(signature)
    })
    return payload.decodeToString(throwOnInvalidSequence = true)
}
internal fun verifyPublicFleet(envelope: FleetEnvelope, publicKey: String, previous: FleetEnvelope? = null): PublicFleet {
    val value = productJson.decodeFromJsonElement<PublicFleet>(notificationJson(verifiedFleetPayload(envelope, publicKey, "fleet")))
    require(value.fleetId == fleetFingerprint(publicKey))
    if (previous != null) {
        val prior = verifyPublicFleet(previous, publicKey)
        require(value.authorityEpoch >= prior.authorityEpoch && value.stateRevision >= prior.stateRevision && value.configRevision >= prior.configRevision)
        require(value.authorityEpoch != prior.authorityEpoch || value.stateRevision != prior.stateRevision || envelope.payload == previous.payload)
    }
    return value
}
internal fun parseFleetInvite(encoded: String): FleetInvite? = try {
    require(encoded.startsWith("skid://connect/") && encoded.length <= 8_192)
    val text = fleetBytes(encoded.removePrefix("skid://connect/"), 4_096).decodeToString(throwOnInvalidSequence = true)
    val invite = productJson.decodeFromJsonElement<FleetInvite>(notificationJson(text))
    require(acceptWireInstant(invite.expiresAt).isAfter(Instant.now()))
    invite
} catch (_: IllegalArgumentException) { null }
  catch (_: kotlinx.serialization.SerializationException) { null }
  catch (_: java.security.GeneralSecurityException) { null }
