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
internal const val FLEET_QUARANTINE_FIELD = "machine.collection.quarantined"

/** The at-rest form of one bearer: base64 AES-GCM ciphertext plus its base64 nonce. */
internal data class SealedBearer(val ciphertext: String, val nonce: String)

/**
 * Single owner of the paired-fleet at-rest format: which preference file holds the
 * collection, how a machine's fields are keyed inside it, which Android Keystore entry protects the
 * bearers, and the AES-256-GCM sealing whose AAD binds every bearer to its handle and origin.
 *
 * The production read path, atomic Connect/Reconnect writers, and the instrumented persistence
 * gate all speak this one definition.
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

    /**
     * Destructive whole-fleet quarantine for an unconfirmed preference rollback. Removing this
     * alias makes either encrypted snapshot that can survive disk unreadable after restart. There
     * is no in-app repair after this boundary; the user must reset app data and connect again.
     */
    fun destroyBearerKeyForQuarantine() {
        try {
            val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
            if (keyStore.containsAlias(keyAlias)) keyStore.deleteEntry(keyAlias)
            // justify-service-invariant-check: Android Keystore exposes deletion only through this
            // synchronous query/delete/query boundary; the alias is private to this storage owner.
            check(!keyStore.containsAlias(keyAlias)) { "fleet credential key survived quarantine" }
        } catch (_: GeneralSecurityException) {
            // justify-defect: returning with this alias present could resurrect a disk-confirmed
            // credential target after restart, so Keystore failure cannot become a UI outcome.
            throw IllegalStateException("fleet credential key quarantine failed")
        } catch (_: IOException) {
            // justify-defect: Android Keystore failed to load, so deletion cannot be proven.
            throw IllegalStateException("fleet credential key quarantine failed")
        }
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

internal sealed interface FleetInstallation {
    data object Installed : FleetInstallation
    data object StoreNotEmpty : FleetInstallation
    data object InvalidFleet : FleetInstallation
    data object StorageUnavailable : FleetInstallation
}

internal sealed interface FleetReconnection {
    data object Reconnected : FleetReconnection
    data object FleetMismatch : FleetReconnection
    data object StorageUnavailable : FleetReconnection
}

internal class MachineStore(context: Context, private val storage: MachineStorage) {
    private val preferences = storage.preferences(context)

    /**
     * Reads the paired collection. This is the app's ingress for machine
     * credentials, so every entry is validated here and the uniqueness architecture §6 promises —
     * handle, case-insensitive label, origin, and bearer — is enforced across the collection.
     * Any incomplete, invalid, or colliding collection is quarantined as a whole so no partial
     * fleet becomes request authority.
     */
    fun read(): MachineStoreRead = synchronized(persistenceLock) { readLocked() }

    /**
     * Installs the exact fixed fleet once. Any existing preference, including a partial or
     * quarantined collection, blocks replacement. All bearers are sealed before the single commit.
     */
    fun installFixedFleet(credentials: List<MachineCredential>): FleetInstallation =
        synchronized(persistenceLock) {
            if (preferences.all.isNotEmpty()) return@synchronized FleetInstallation.StoreNotEmpty
            if (!isExactFleet(credentials)) return@synchronized FleetInstallation.InvalidFleet
            if (!commitFleet(credentials, MachineStoreRead(emptyList(), emptyList()))) {
                return@synchronized FleetInstallation.StorageUnavailable
            }
            FleetInstallation.Installed
        }

    /** Rotates all three bearers only for the exact complete installed identities. */
    fun reconnectFixedFleet(credentials: List<MachineCredential>): FleetReconnection = synchronized(persistenceLock) {
        val stored = readLocked()
        if (
            stored.unreadable.isNotEmpty() ||
            stored.credentials.size != 3 ||
            !isExactFleet(credentials) ||
            stored.credentials.map { it.machine } != credentials.map { it.machine }
        ) return@synchronized FleetReconnection.FleetMismatch
        if (!commitFleet(credentials, stored)) return@synchronized FleetReconnection.StorageUnavailable
        FleetReconnection.Reconnected
    }

    private fun readLocked(): MachineStoreRead {
        if (preferences.all.isEmpty()) return MachineStoreRead(emptyList(), emptyList())
        val storedHandles = try {
            handles()
        } catch (_: IOException) {
            // justify-ignore-error: an unreadable persisted index is the modeled collection-wide
            // quarantine, not a defect or an invitation to trust partial fields.
            return MachineStoreRead(emptyList(), listOf(UnreadableStoredMachine(collectionWide = true)))
        }
        if (storedHandles.size != 3) {
            return MachineStoreRead(emptyList(), listOf(UnreadableStoredMachine(collectionWide = true)))
        }
        val expectedFields = setOf(storage.handlesField) + storedHandles.flatMap { handle ->
            listOf("label", "origin", "ciphertext", "nonce").map { name -> storage.field(handle, name) }
        }
        if (preferences.all.keys != expectedFields) {
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
        if (
            quarantined != 0 ||
            unique.size != 3 ||
            unique.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) }.map { it.machine.label.text } !=
            listOf("Arch", "Devbox", "MacBook")
        ) {
            return MachineStoreRead(emptyList(), List(storedHandles.size) { UnreadableStoredMachine() })
        }
        return MachineStoreRead(
            unique.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) },
            emptyList(),
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

    private fun isExactFleet(credentials: List<MachineCredential>): Boolean =
        credentials.map { it.machine.label.text } == listOf("Arch", "Devbox", "MacBook") &&
            uniqueCredentials(credentials).size == 3 &&
            credentials.map { it.machine.handle }.distinct().size == 3

    @SuppressLint("UseKtx") // justify-override: this credential transaction must observe commit.
    private fun commitFleet(
        credentials: List<MachineCredential>,
        priorRead: MachineStoreRead,
    ): Boolean {
        val sealed = try {
            credentials.map { it to storage.seal(it.machine, it.bearer) }
        } catch (_: GeneralSecurityException) {
            // justify-ignore-error: every seal happens before preference mutation, so a broken
            // Keystore leaves the prior collection byte-for-byte observable.
            return false
        }
        val target = linkedMapOf<String, Any>(
            storage.handlesField to credentials.map { it.machine.handle.encoded }.toSet(),
        )
        sealed.forEach { (credential, bearer) -> putEncodedCredential(target, credential, bearer) }
        val committed = replacePreferencesWithVerifiedRollback(
            preferences = preferences,
            target = target,
            verifyTarget = { readLocked() == MachineStoreRead(credentials, emptyList()) },
            onUnconfirmedRollback = storage::destroyBearerKeyForQuarantine,
        )
        if (committed) return true

        val recovered = readLocked()
        val safe = if (priorRead.credentials.isEmpty() && priorRead.unreadable.isEmpty()) {
            recovered.credentials.isEmpty()
        } else {
            recovered == priorRead ||
                (recovered.credentials.isEmpty() && recovered.unreadable.isNotEmpty())
        }
        if (!safe) forceFleetQuarantine(preferences)
        val proven = readLocked()
        // justify-service-invariant-check: SharedPreferences has no type that can promise its
        // documented process-visible editor mutation; reaching this branch means the platform
        // neither restored the exact prior fleet nor exposed the quarantine we just wrote.
        check(
            if (priorRead.credentials.isEmpty() && priorRead.unreadable.isEmpty()) {
                proven.credentials.isEmpty()
            } else {
                proven == priorRead || (proven.credentials.isEmpty() && proven.unreadable.isNotEmpty())
            },
        ) { "failed fleet transaction did not fail closed" }
        return false
    }

    private fun putEncodedCredential(
        encoded: MutableMap<String, Any>,
        credential: MachineCredential,
        sealed: SealedBearer,
    ) {
        val handle = credential.machine.handle.encoded
        encoded[storage.field(handle, "label")] = credential.machine.label.text
        encoded[storage.field(handle, "origin")] = credential.machine.origin.encoded
        encoded[storage.field(handle, "ciphertext")] = sealed.ciphertext
        encoded[storage.field(handle, "nonce")] = sealed.nonce
    }

    private fun handles(): Set<String> {
        val stored = try {
            preferences.getStringSet(storage.handlesField, null)
        } catch (failure: ClassCastException) {
            // justify-ignore-error: a wrongly typed persisted index is unreadable input and must
            // remain quarantined rather than becoming request authority.
            throw IOException("stored machine index is not a handle set", failure)
        }
        return stored?.toSet() ?: throw IOException("stored machine index is absent")
    }

    private fun requireField(encodedHandle: String, name: String): String {
        val value = try {
            preferences.getString(storage.field(encodedHandle, name), null)
        } catch (failure: ClassCastException) {
            // justify-ignore-error: a wrongly typed persisted field quarantines the collection.
            throw IOException("stored machine field is not text", failure)
        }
        return value ?: throw IOException("incomplete stored machine")
    }

    private companion object {
        val persistenceLock = Any()
    }
}

/**
 * Replaces one preference collection while treating commit's Boolean as disk acknowledgement only:
 * Android mutates the process-visible map before that result is known. Any rejected write restores
 * and verifies the exact encoded pre-state; an unconfirmed restoration becomes one opaque marker.
 */
@SuppressLint("UseKtx") // justify-override: every credential transition must observe commit.
internal fun replacePreferencesWithVerifiedRollback(
    preferences: SharedPreferences,
    target: Map<String, Any>,
    verifyTarget: () -> Boolean,
    onUnconfirmedRollback: () -> Unit,
): Boolean {
    val prior = snapshotEncodedPreferences(preferences) ?: run {
        forceFleetQuarantine(preferences)
        return false
    }
    val canonicalTarget = copyEncodedPreferences(target) ?: run {
        forceFleetQuarantine(preferences)
        return false
    }
    val committed = replaceEncodedPreferences(preferences, canonicalTarget)
    if (
        committed &&
        snapshotEncodedPreferences(preferences) == canonicalTarget &&
        verifyTarget()
    ) return true

    val restored = replaceEncodedPreferences(preferences, prior)
    if (restored && snapshotEncodedPreferences(preferences) == prior) return false
    if (committed) {
        // A disk-confirmed target plus an unconfirmed restore can otherwise become readable after
        // restart. Destroy durable decryption authority before exposing the process quarantine.
        try {
            onUnconfirmedRollback()
        } finally {
            forceFleetQuarantine(preferences)
        }
    } else {
        forceFleetQuarantine(preferences)
    }
    return false
}

@SuppressLint("UseKtx") // justify-override: quarantine must be process-visible before returning.
private fun forceFleetQuarantine(preferences: SharedPreferences) {
    replaceEncodedPreferences(preferences, mapOf(FLEET_QUARANTINE_FIELD to true))
    // justify-service-invariant-check: SharedPreferences.Editor mutates process memory even when
    // commit reports a disk failure; the boundary provides no stronger typed acknowledgement.
    check(snapshotEncodedPreferences(preferences) == mapOf(FLEET_QUARANTINE_FIELD to true)) {
        "could not quarantine failed fleet storage"
    }
}

@SuppressLint("UseKtx") // justify-override: restoration must observe commit.
private fun replaceEncodedPreferences(
    preferences: SharedPreferences,
    encoded: Map<String, Any>,
): Boolean {
    val editor = preferences.edit().clear()
    encoded.toSortedMap().forEach { (key, value) ->
        when (value) {
            is String -> editor.putString(key, value)
            is Set<*> -> editor.putStringSet(key, value.filterIsInstance<String>().toSet())
            is Int -> editor.putInt(key, value)
            is Long -> editor.putLong(key, value)
            is Float -> editor.putFloat(key, value)
            is Boolean -> editor.putBoolean(key, value)
            else -> return false
        }
    }
    return editor.commit()
}

private fun snapshotEncodedPreferences(preferences: SharedPreferences): Map<String, Any>? =
    copyEncodedPreferences(preferences.all)

private fun copyEncodedPreferences(encoded: Map<String, *>): Map<String, Any>? {
    val copied = linkedMapOf<String, Any>()
    encoded.toSortedMap().forEach { (key, value) ->
        copied[key] = when (value) {
            is String, is Int, is Long, is Float, is Boolean -> value
            is Set<*> -> {
                if (value.any { it !is String }) return null
                value.filterIsInstance<String>().toSet()
            }
            else -> return null
        }
    }
    return copied
}
