package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import java.util.Locale
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val MACHINE_KEY_ALIAS = "skidbladnir.machine-bearers.v1"
private const val MACHINE_PREFERENCES = "skidbladnir.machines"
private const val MACHINE_HANDLES = "machine.handles"
private const val MACHINE_INDEX_QUARANTINE = "machine-index"
private const val CIPHER_TRANSFORMATION = "AES/GCM/NoPadding"

internal data class UnreadableStoredMachine(
    val encodedHandle: String,
    val handle: MachineHandle?,
    val collectionWide: Boolean = false,
)
internal data class MachineStoreRead(
    val credentials: List<MachineCredential>,
    val unreadable: List<UnreadableStoredMachine>,
)

@SuppressLint("UseKtx")
internal class MachineStore(
    context: Context,
    private val keyAlias: String = MACHINE_KEY_ALIAS,
) {
    private val preferences = context.getSharedPreferences(MACHINE_PREFERENCES, Context.MODE_PRIVATE)

    fun read(): MachineStoreRead = synchronized(persistenceLock) {
        val credentials = mutableListOf<MachineCredential>()
        val unreadable = mutableListOf<UnreadableStoredMachine>()
        val storedHandles = try {
            handles()
        } catch (_: Exception) {
            return@synchronized MachineStoreRead(
                emptyList(),
                listOf(UnreadableStoredMachine(MACHINE_INDEX_QUARANTINE, null, collectionWide = true)),
            )
        }
        storedHandles.sorted().forEach { encodedHandle ->
            val handle = MachineHandle.parse(encodedHandle)
            val machine = try {
                readMachine(encodedHandle)
            } catch (_: Exception) {
                unreadable += UnreadableStoredMachine(encodedHandle, handle)
                return@forEach
            }
            try {
                credentials += readCredential(machine)
            } catch (_: Exception) {
                unreadable += UnreadableStoredMachine(encodedHandle, handle)
            }
        }
        MachineStoreRead(
            credentials.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) },
            unreadable,
        )
    }

    private fun readMachine(encodedHandle: String): PairedMachine {
        val handle = MachineHandle.parse(encodedHandle) ?: throw IOException("invalid stored machine handle")
        val label = MachineLabel.parse(requireField(handle, "label")) ?: throw IOException("invalid stored machine label")
        val origin = MachineOrigin.parse(requireField(handle, "origin")) ?: throw IOException("invalid stored machine origin")
        return PairedMachine(handle, label, origin)
    }

    private fun readCredential(machine: PairedMachine): MachineCredential {
        val handle = machine.handle
        val origin = machine.origin
        val nonce = decodeField(handle, "nonce")
        val ciphertext = decodeField(handle, "ciphertext")
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, requireKey(), GCMParameterSpec(128, nonce))
        cipher.updateAAD(associatedData(handle, origin))
        val bearer = GatewayBearer.parse(
            String(cipher.doFinal(ciphertext), StandardCharsets.UTF_8),
        ) ?: throw IOException("invalid stored bearer")
        return MachineCredential(machine, bearer)
    }

    fun add(credential: MachineCredential) = synchronized(persistenceLock) {
        val existing = requireComplete(read())
        require(existing.none { it.machine.handle == credential.machine.handle })
        requireUnique(existing, credential)
        persist(credential, existing.map { it.machine.handle.encoded }.toSet() + credential.machine.handle.encoded)
    }

    fun rotateBearer(credential: MachineCredential) = synchronized(persistenceLock) {
        val existing = requireComplete(read())
        val machine = existing.singleOrNull { it.machine.handle == credential.machine.handle }?.machine
            ?: throw IOException("machine is not paired")
        require(machine == credential.machine)
        require(existing.none {
            it.machine.handle != credential.machine.handle && it.bearer == credential.bearer
        })
        persist(credential, handles())
    }

    fun rename(handle: MachineHandle, label: MachineLabel) = synchronized(persistenceLock) {
        val existing = requireComplete(read())
        val credential = existing.singleOrNull { it.machine.handle == handle } ?: throw IOException("machine is not paired")
        val renamed = credential.copy(machine = credential.machine.copy(label = label))
        requireUnique(existing.filterNot { it.machine.handle == handle }, renamed)
        if (!preferences.edit().putString(field(handle, "label"), label.text).commit()) {
            throw IOException("could not persist machine label")
        }
    }

    fun remove(handle: MachineHandle) = synchronized(persistenceLock) {
        removeEncoded(handle.encoded)
    }

    fun removeUnreadable(encodedHandle: String) = synchronized(persistenceLock) {
        val unreadable = read().unreadable.singleOrNull { it.encodedHandle == encodedHandle }
            ?: throw IOException("stored machine is readable or absent")
        if (unreadable.collectionWide) {
            if (!preferences.edit().clear().commit()) throw IOException("could not clear unreadable machine index")
        } else {
            removeEncoded(encodedHandle)
        }
    }

    private fun removeEncoded(encodedHandle: String) {
        val existing = handles()
        if (encodedHandle !in existing) throw IOException("machine is not paired")
        val remaining = existing - encodedHandle
        val editor = preferences.edit()
            .putStringSet(MACHINE_HANDLES, remaining)
            .remove(field(encodedHandle, "label"))
            .remove(field(encodedHandle, "origin"))
            .remove(field(encodedHandle, "ciphertext"))
            .remove(field(encodedHandle, "nonce"))
        if (!editor.commit()) throw IOException("could not remove machine")
    }

    fun resetAll() = synchronized(persistenceLock) {
        if (!preferences.edit().clear().commit()) throw IOException("could not clear machines")
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(keyAlias)) keyStore.deleteEntry(keyAlias)
    }

    private fun persist(credential: MachineCredential, handles: Set<String>) {
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, requireKey())
        cipher.updateAAD(associatedData(credential.machine.handle, credential.machine.origin))
        val ciphertext = cipher.doFinal(credential.bearer.encoded.toByteArray(StandardCharsets.UTF_8))
        val handle = credential.machine.handle
        val editor = preferences.edit()
            .putStringSet(MACHINE_HANDLES, handles)
            .putString(field(handle, "label"), credential.machine.label.text)
            .putString(field(handle, "origin"), credential.machine.origin.encoded)
            .putString(field(handle, "ciphertext"), Base64.encodeToString(ciphertext, Base64.NO_WRAP))
            .putString(field(handle, "nonce"), Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
        if (!editor.commit()) throw IOException("could not persist machine")
    }

    private fun handles(): Set<String> = preferences.getStringSet(MACHINE_HANDLES, emptySet())?.toSet()
        ?: throw IOException("invalid stored machine index")

    private fun requireField(handle: MachineHandle, name: String): String =
        preferences.getString(field(handle, name), null) ?: throw IOException("incomplete stored machine")

    private fun decodeField(handle: MachineHandle, name: String): ByteArray = try {
        Base64.decode(requireField(handle, name), Base64.NO_WRAP)
    } catch (failure: IllegalArgumentException) {
        throw IOException("invalid stored machine encryption", failure)
    }

    private fun field(handle: MachineHandle, name: String): String = field(handle.encoded, name)

    private fun field(encodedHandle: String, name: String): String = "machine.$encodedHandle.$name"

    private fun associatedData(handle: MachineHandle, origin: MachineOrigin): ByteArray =
        "dev.niels.skidbladnir.machine.bearer.v1\u0000${handle.encoded}\u0000${origin.encoded}"
            .toByteArray(StandardCharsets.UTF_8)

    private fun requireComplete(read: MachineStoreRead): List<MachineCredential> {
        if (read.unreadable.isNotEmpty()) throw IOException("stored machine collection is incomplete")
        return read.credentials
    }

    private fun requireUnique(existing: List<MachineCredential>, credential: MachineCredential) {
        require(existing.none { it.machine.label.text.equals(credential.machine.label.text, ignoreCase = true) })
        require(existing.none { it.machine.origin == credential.machine.origin })
        require(existing.none { it.bearer == credential.bearer })
    }

    private fun requireKey(): SecretKey {
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (keyStore.getKey(keyAlias, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").run {
            init(
                KeyGenParameterSpec.Builder(
                    keyAlias,
                    KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
                )
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .setKeySize(256)
                    .build(),
            )
            generateKey()
        }
    }

    private companion object {
        val persistenceLock = Any()
    }
}
