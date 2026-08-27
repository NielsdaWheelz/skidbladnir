package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.content.SharedPreferences
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.GeneralSecurityException
import java.security.KeyStore
import java.util.Locale
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

private const val CIPHER_TRANSFORMATION = "AES/GCM/NoPadding"
private const val GCM_TAG_BITS = 128
private const val BEARER_KEY_BITS = 256
private const val MACHINE_HANDLES_FIELD = "machine.handles"
private const val BEARER_ASSOCIATED_CONTEXT = "dev.niels.skidbladnir.machine.bearer.v1"

/** The at-rest form of one bearer: base64 AES-GCM ciphertext plus its base64 nonce. */
internal data class SealedBearer(val ciphertext: String, val nonce: String)

/**
 * Single owner of the externally provisioned at-rest format: which preference file holds the
 * collection, how a machine's fields are keyed inside it, which Android Keystore entry protects the
 * bearers, and the AES-256-GCM sealing whose AAD binds every bearer to its handle and origin.
 *
 * The app never provisions machines, so the format is owned here rather than in a writer: the
 * production read path, the bearer-repair write path, and the instrumented persistence gate all
 * speak this one definition.
 */
internal class MachineStorage(
    private val preferencesName: String,
    private val keyAlias: String,
) {
    val handlesField: String = MACHINE_HANDLES_FIELD

    fun preferences(context: Context): SharedPreferences =
        context.getSharedPreferences(preferencesName, Context.MODE_PRIVATE)

    fun field(encodedHandle: String, name: String): String = "machine.$encodedHandle.$name"

    fun seal(machine: PairedMachine, bearer: GatewayBearer): SealedBearer {
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD(associatedData(machine))
        val ciphertext = cipher.doFinal(bearer.encoded.toByteArray(StandardCharsets.UTF_8))
        return SealedBearer(
            ciphertext = Base64.encodeToString(ciphertext, Base64.NO_WRAP),
            nonce = Base64.encodeToString(cipher.iv, Base64.NO_WRAP),
        )
    }

    fun open(machine: PairedMachine, sealed: SealedBearer): GatewayBearer {
        val cipher = Cipher.getInstance(CIPHER_TRANSFORMATION)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(GCM_TAG_BITS, decode(sealed.nonce)))
        cipher.updateAAD(associatedData(machine))
        val plaintext = String(cipher.doFinal(decode(sealed.ciphertext)), StandardCharsets.UTF_8)
        return GatewayBearer.parse(plaintext) ?: throw IOException("stored bearer is not canonical")
    }

    private fun decode(value: String): ByteArray = try {
        Base64.decode(value, Base64.NO_WRAP)
    } catch (failure: IllegalArgumentException) {
        throw IOException("stored machine encryption is malformed", failure)
    }

    private fun associatedData(machine: PairedMachine): ByteArray =
        "$BEARER_ASSOCIATED_CONTEXT\u0000${machine.handle.encoded}\u0000${machine.origin.encoded}"
            .toByteArray(StandardCharsets.UTF_8)

    private fun key(): SecretKey {
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
                    .setKeySize(BEARER_KEY_BITS)
                    .build(),
            )
            generateKey()
        }
    }

    companion object {
        val production = MachineStorage("skidbladnir.machines", "skidbladnir.machine-bearers.v1")
    }
}

internal data class UnreadableStoredMachine(
    val collectionWide: Boolean = false,
)

internal data class MachineStoreRead(
    val credentials: List<MachineCredential>,
    val unreadable: List<UnreadableStoredMachine>,
)

internal sealed interface MachineProvisioning {
    data object Provisioned : MachineProvisioning
    data object AlreadyProvisioned : MachineProvisioning
    data object InvalidCollection : MachineProvisioning
    data object StorageUnavailable : MachineProvisioning
}

internal sealed interface BearerRotation {
    data object Rotated : BearerRotation
    /** The candidate bearer already authorizes a different installed machine. */
    data object BearerInUse : BearerRotation
    /** The store no longer holds this exact machine, or the collection is incomplete. */
    data object MachineUnavailable : BearerRotation
    data object StorageUnavailable : BearerRotation
}

internal class MachineStore(context: Context, private val storage: MachineStorage) {
    private val preferences = storage.preferences(context)

    /**
     * Reads the externally provisioned collection. This is the app's ingress for machine
     * credentials, so every entry is validated here and the uniqueness architecture §6 promises —
     * handle, case-insensitive label, origin, and bearer — is enforced across the collection.
     * Entries that fail validation or collide are quarantined rather than returned.
     */
    fun read(): MachineStoreRead = synchronized(persistenceLock) { readLocked() }

    /**
     * Installs the fixed two-machine collection exactly once for the external operator boundary.
     * The app has no caller for this method: only signed instrumentation can reach it. Any existing
     * production preference, including a partial or quarantined collection, blocks replacement.
     */
    fun provisionFixedCollection(credentials: List<MachineCredential>): MachineProvisioning =
        synchronized(persistenceLock) {
            if (preferences.all.isNotEmpty()) return@synchronized MachineProvisioning.AlreadyProvisioned
            val handles = credentials.map { it.machine.handle.encoded }.toSet()
            if (credentials.size != 2 || handles.size != 2 || uniqueCredentials(credentials).size != 2) {
                return@synchronized MachineProvisioning.InvalidCollection
            }
            val sealedCredentials = try {
                credentials.map { it to storage.seal(it.machine, it.bearer) }
            } catch (_: GeneralSecurityException) {
                // justify-ignore-error: no preference mutation has occurred; the operator receives a
                // closed storage outcome and must repair the device Keystore boundary.
                return@synchronized MachineProvisioning.StorageUnavailable
            }
            val editor = preferences.edit().putStringSet(storage.handlesField, handles)
            sealedCredentials.forEach { (credential, sealed) ->
                putCredential(editor, credential, sealed)
            }
            if (!editor.commit()) return@synchronized MachineProvisioning.StorageUnavailable

            val observed = readLocked()
            val expected = credentials.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) }
            if (observed.credentials != expected || observed.unreadable.isNotEmpty()) {
                // The store was proven empty on entry, so clearing only this failed transaction is
                // the safe rollback. A failed rollback remains visibly unavailable to the app.
                preferences.edit().clear().commit()
                return@synchronized MachineProvisioning.StorageUnavailable
            }
            MachineProvisioning.Provisioned
        }

    fun rotateBearer(credential: MachineCredential): BearerRotation = synchronized(persistenceLock) {
        val stored = readLocked()
        if (stored.unreadable.isNotEmpty()) return@synchronized BearerRotation.MachineUnavailable
        if (stored.credentials.none { it.machine == credential.machine }) {
            return@synchronized BearerRotation.MachineUnavailable
        }
        if (stored.credentials.any {
                it.machine.handle != credential.machine.handle && it.bearer == credential.bearer
            }
        ) {
            return@synchronized BearerRotation.BearerInUse
        }
        try {
            persist(credential, handles())
        } catch (_: IOException) {
            // justify-ignore-error: the encrypted preference write either lands or does not; the
            // product only needs the caller to know that verification outran storage.
            return@synchronized BearerRotation.StorageUnavailable
        } catch (_: GeneralSecurityException) {
            // justify-ignore-error: the Keystore entry is unusable on this device; repair is the
            // same product outcome as a failed write.
            return@synchronized BearerRotation.StorageUnavailable
        }
        BearerRotation.Rotated
    }

    private fun readLocked(): MachineStoreRead {
        val storedHandles = try {
            handles()
        } catch (_: IOException) {
            // justify-ignore-error: the collection index is externally provisioned, so an
            // unreadable index is the modeled collection-wide quarantine, not a defect.
            return MachineStoreRead(emptyList(), listOf(UnreadableStoredMachine(collectionWide = true)))
        }
        val readable = mutableListOf<MachineCredential>()
        var quarantined = 0
        storedHandles.sorted().forEach { encodedHandle ->
            val credential = try {
                readCredential(readMachine(encodedHandle))
            } catch (_: IOException) {
                // justify-ignore-error: a stored entry that does not parse is an opaque quarantine
                // slot; its untrusted plaintext must never reach the product or a request.
                quarantined += 1
                return@forEach
            } catch (_: GeneralSecurityException) {
                // justify-ignore-error: a bearer that fails its handle/origin-bound AAD is exactly
                // the same opaque quarantine slot.
                quarantined += 1
                return@forEach
            }
            readable += credential
        }
        val unique = uniqueCredentials(readable)
        return MachineStoreRead(
            unique.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) },
            List(quarantined + readable.size - unique.size) { UnreadableStoredMachine() },
        )
    }

    /**
     * Distinct handles are guaranteed by the index being a set of encoded handles; labels
     * (case-insensitively), origins, and bearer bytes are enforced here. Every member of a
     * colliding group is quarantined, because nothing in the store says which one is authoritative.
     */
    private fun uniqueCredentials(credentials: List<MachineCredential>): List<MachineCredential> {
        val labels = credentials.groupingBy { it.machine.label.text.lowercase(Locale.ROOT) }.eachCount()
        val origins = credentials.groupingBy { it.machine.origin }.eachCount()
        val bearers = credentials.groupingBy { it.bearer }.eachCount()
        return credentials.filter {
            labels.getValue(it.machine.label.text.lowercase(Locale.ROOT)) == 1 &&
                origins.getValue(it.machine.origin) == 1 &&
                bearers.getValue(it.bearer) == 1
        }
    }

    private fun readMachine(encodedHandle: String): PairedMachine {
        val handle = MachineHandle.parse(encodedHandle) ?: throw IOException("invalid stored machine handle")
        val label = MachineLabel.parse(requireField(encodedHandle, "label"))
            ?: throw IOException("invalid stored machine label")
        val origin = MachineOrigin.parse(requireField(encodedHandle, "origin"))
            ?: throw IOException("invalid stored machine origin")
        return PairedMachine(handle, label, origin)
    }

    private fun readCredential(machine: PairedMachine): MachineCredential = MachineCredential(
        machine,
        storage.open(
            machine,
            SealedBearer(
                ciphertext = requireField(machine.handle.encoded, "ciphertext"),
                nonce = requireField(machine.handle.encoded, "nonce"),
            ),
        ),
    )

    @SuppressLint("UseKtx") // justify-override: a credential write must fail loudly, and the KTX
    // edit {} helper discards the commit result this branch depends on.
    private fun persist(credential: MachineCredential, handles: Set<String>) {
        val sealed = storage.seal(credential.machine, credential.bearer)
        val editor = preferences.edit()
            .putStringSet(storage.handlesField, handles)
        putCredential(editor, credential, sealed)
        if (!editor.commit()) throw IOException("could not persist machine")
    }

    private fun putCredential(
        editor: SharedPreferences.Editor,
        credential: MachineCredential,
        sealed: SealedBearer,
    ) {
        val handle = credential.machine.handle.encoded
        editor
            .putString(storage.field(handle, "label"), credential.machine.label.text)
            .putString(storage.field(handle, "origin"), credential.machine.origin.encoded)
            .putString(storage.field(handle, "ciphertext"), sealed.ciphertext)
            .putString(storage.field(handle, "nonce"), sealed.nonce)
    }

    private fun handles(): Set<String> {
        val stored = try {
            preferences.getStringSet(storage.handlesField, emptySet())
        } catch (failure: ClassCastException) {
            // justify-ignore-error: preferences are provisioned outside the app, so a wrongly typed
            // index entry is unreadable input rather than a broken app invariant.
            throw IOException("stored machine index is not a handle set", failure)
        }
        return stored?.toSet() ?: throw IOException("stored machine index is absent")
    }

    private fun requireField(encodedHandle: String, name: String): String {
        val value = try {
            preferences.getString(storage.field(encodedHandle, name), null)
        } catch (failure: ClassCastException) {
            // justify-ignore-error: the same externally provisioned boundary; a wrongly typed field
            // quarantines its machine.
            throw IOException("stored machine field is not text", failure)
        }
        return value ?: throw IOException("incomplete stored machine")
    }

    private companion object {
        val persistenceLock = Any()
    }
}
