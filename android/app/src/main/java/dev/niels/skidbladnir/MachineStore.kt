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
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString

internal data class MachineStoreRead(val credentials: List<MachineCredential>, val unreadable: Boolean, val view: PublicFleet?)
/** The phone's one membership: its bearer, the invitation it joined with, and the hub's last view. */
@Serializable internal data class PhoneFleet(
    val fleetId: String, val deviceId: String, val bearer: String, val invitation: FleetInvite,
    val view: PublicFleet? = null, val joined: Boolean = false,
) {
    fun androidPush(): AndroidPush? = if (joined) view?.androidPush else null
    fun credential(machine: PairedMachine): MachineCredential = MachineCredential(machine, checkNotNull(GatewayBearer.parse(bearer)))
    fun credentials(): List<MachineCredential> = if (joined) view?.machines.orEmpty().map { credential(it.paired()) } else emptyList()
    fun hubCredential(): MachineCredential? {
        val hub = view?.let { value -> value.machines.firstOrNull { it.machine == value.hub } } ?: invitation.hub
        return if (hub.origin == null) null else credential(hub.paired())
    }
}
/** One encrypted atomic record owns the phone credential, staged join and cached hub view. */
internal class MachineStore(private val context: Context) {
    private val file = AtomicFile(java.io.File(context.filesDir, "fleet-v2.json"))
    fun association(): PhoneFleet? = synchronized(lock) {
        if (!file.baseFile.exists()) return@synchronized null
        val bytes = file.openRead().use { it.readNBytes(2_000_001) }
        require(bytes.size in 13..2_000_000)
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12)))
        cipher.updateAAD("skidbladnir.phone-fleet.v1".encodeToByteArray())
        productJson.decodeFromString<PhoneFleet>(cipher.doFinal(bytes.copyOfRange(12, bytes.size)).decodeToString(throwOnInvalidSequence = true))
    }
    /** Pairing commits and push admissions share this boundary; nested push writes take its lock second. */
    fun <T> withAssociation(block: (PhoneFleet?) -> T): T = synchronized(lock) { block(association()) }
    fun read(): MachineStoreRead = try {
        val value = association()
        MachineStoreRead(value?.credentials().orEmpty(), false, value?.view)
    } catch (_: IOException) { MachineStoreRead(emptyList(), true, null) }
      catch (_: GeneralSecurityException) { MachineStoreRead(emptyList(), true, null) }
      catch (_: IllegalArgumentException) { MachineStoreRead(emptyList(), true, null) }
      catch (_: kotlinx.serialization.SerializationException) { MachineStoreRead(emptyList(), true, null) }
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
    /** Commits a hub view only for the identity that requested it; a concurrent re-stage wins. */
    fun accept(view: PublicFleet, expected: PhoneFleet): List<MachineCredential> = synchronized(lock) {
        val value = checkNotNull(association())
        require(value.deviceId == expected.deviceId && value.bearer == expected.bearer)
        val next = value.copy(view = view, joined = true)
        save(next)
        next.credentials()
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
