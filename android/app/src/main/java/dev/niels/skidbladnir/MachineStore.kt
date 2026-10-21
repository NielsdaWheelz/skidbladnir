package dev.niels.skidbladnir

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.AtomicFile
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.decodeFromJsonElement

internal enum class FleetQuarantine { Index, Pairings }
internal data class MachineStoreRead(
    val credentials: List<MachineCredential>, val quarantine: FleetQuarantine?, val envelope: FleetEnvelope?,
)
@Serializable internal data class PhoneFleet(
    val schema: Int = 1, val fleetId: String, val publicKey: String, val deviceId: String,
    val bearer: String? = null, val invitation: FleetInvite, val envelope: FleetEnvelope? = null, val joined: Boolean = false,
) {
    init {
        require(schema == 1 && fleetId == fleetFingerprint(publicKey))
        require(deviceId.matches(Regex("d-[0-9a-f]{32}")) && (bearer == null || GatewayBearer.parse(bearer) != null))
        require(invitation.fleetId == fleetId && invitation.publicKey == publicKey)
        envelope?.let { verifyPublicFleet(it, publicKey) }
    }
    fun androidPush(): AndroidPush? = if (joined && bearer != null)
        envelope?.let { verifyPublicFleet(it, publicKey).androidPush } else null
    fun credentials(): List<MachineCredential> {
        val credential = bearer?.let(GatewayBearer::parse) ?: return emptyList()
        if (!joined) return emptyList()
        return envelope?.let { verifyPublicFleet(it, publicKey).machines }.orEmpty().map { MachineCredential(it.paired(), credential) }
    }
    fun hubCredential(): MachineCredential? {
        val credential = bearer?.let(GatewayBearer::parse) ?: return null
        val hub = envelope?.let { verifyPublicFleet(it, publicKey) }?.let { value -> value.machines.single { it.machine == value.activeHub } }
            ?: invitation.hub
        if (hub.origin == null) return null
        return MachineCredential(hub.paired(), credential)
    }
}
/** One encrypted atomic record owns the phone credential, staged join and signed public cache. */
internal class MachineStore(private val context: Context) {
    private val file = AtomicFile(java.io.File(context.filesDir, "fleet-v1.json"))
    fun association(): PhoneFleet? = synchronized(lock) {
        if (!file.baseFile.exists()) return@synchronized null
        val bytes = file.openRead().use { it.readNBytes(6_000_001) }
        require(bytes.size in 13..6_000_000)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12)))
        cipher.updateAAD("skidbladnir.phone-fleet.v1".encodeToByteArray())
        productJson.decodeFromJsonElement<PhoneFleet>(notificationJson(cipher.doFinal(bytes.copyOfRange(12, bytes.size)).decodeToString(throwOnInvalidSequence = true)))
    }
    /** Pairing commits and push admissions share this boundary; nested push writes take its lock second. */
    fun <T> withAssociation(block: (PhoneFleet?) -> T): T = synchronized(lock) { block(association()) }
    fun read(): MachineStoreRead = try {
        val value = association()
        MachineStoreRead(value?.credentials().orEmpty(), null, value?.envelope)
    } catch (_: IOException) { MachineStoreRead(emptyList(), FleetQuarantine.Pairings, null) }
      catch (_: GeneralSecurityException) { MachineStoreRead(emptyList(), FleetQuarantine.Pairings, null) }
      catch (_: IllegalArgumentException) { MachineStoreRead(emptyList(), FleetQuarantine.Pairings, null) }
      catch (_: kotlinx.serialization.SerializationException) { MachineStoreRead(emptyList(), FleetQuarantine.Pairings, null) }
    fun save(value: PhoneFleet) = synchronized(lock) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD("skidbladnir.phone-fleet.v1".encodeToByteArray())
        val bytes = cipher.iv + cipher.doFinal(productJson.encodeToString(value).encodeToByteArray())
        val stream = file.startWrite()
        try {
            stream.write(bytes)
            stream.fd.sync()
            file.finishWrite(stream)
            if (!file.openRead().use { it.readNBytes(bytes.size + 1) }.contentEquals(bytes)) throw IOException("fleet commit unavailable")
        }
        catch (failure: IOException) { file.failWrite(stream); throw failure }
    }
    fun accept(envelope: FleetEnvelope, expected: PhoneFleet): List<MachineCredential> = synchronized(lock) {
        val value = checkNotNull(association())
        require(value.deviceId == expected.deviceId && value.bearer == expected.bearer)
        verifyPublicFleet(envelope, value.publicKey, value.envelope)
        val next = value.copy(envelope = envelope, joined = true)
        save(next)
        next.credentials()
    }
    fun remove(proof: FleetEnvelope): Boolean = synchronized(lock) {
        val value = association() ?: return@synchronized false
        val removal = notificationJson(verifiedFleetPayload(proof, value.publicKey, "member-removed"))
        removal.requireExactKeys(setOf("schema", "kind", "fleetId", "deviceId", "authorityEpoch", "stateRevision"))
        fun text(key: String) = (removal.getValue(key) as kotlinx.serialization.json.JsonPrimitive).content
        val current = value.envelope?.let { verifyPublicFleet(it, value.publicKey) }
        require(text("schema") == "1" && text("kind") == "member-removed" && text("fleetId") == value.fleetId && text("deviceId") == value.deviceId)
        val epoch = text("authorityEpoch").toLong()
        val revision = text("stateRevision").toLong()
        require(epoch > 0 && revision > 0)
        require(current == null || epoch >= current.authorityEpoch && revision >= current.stateRevision)
        save(value.copy(bearer = null, joined = false))
        true
    }
    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey("skidbladnir.phone-fleet.v1", null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(KeyGenParameterSpec.Builder("skidbladnir.phone-fleet.v1", KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).setKeySize(256).build())
            generateKey()
        }
    }
    private companion object { val lock = Any() }
}
