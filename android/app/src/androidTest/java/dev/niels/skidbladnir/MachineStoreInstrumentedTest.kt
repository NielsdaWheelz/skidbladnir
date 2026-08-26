package dev.niels.skidbladnir

import android.content.Context
import android.content.ContextWrapper
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.IOException
import java.nio.charset.StandardCharsets
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MachineStoreInstrumentedTest {
    @Test
    fun corruptedPairingPreservesHealthyMachineAndSharedBearerIsRejected() {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val testPreferences = "skidbladnir.machines.isolation-instrumented-test"
        val context = object : ContextWrapper(targetContext) {
            override fun getSharedPreferences(name: String?, mode: Int) =
                targetContext.getSharedPreferences(testPreferences, mode)
        }
        val keyAlias = "skidbladnir.machine-bearers.isolation-instrumented-test"
        val store = MachineStore(context, keyAlias)
        val devbox = credential(
            "mh-0123456789abcdef0123456789abcdef",
            "Devbox",
            "https://devbox.example.ts.net:8443",
            "A".repeat(43),
        )
        val macBook = credential(
            "mh-fedcba9876543210fedcba9876543210",
            "MacBook",
            "https://macbook.example.ts.net:8443",
            "B" + "A".repeat(42),
        )

        try {
            installFixture(targetContext, testPreferences, keyAlias, listOf(devbox, macBook))

            assertThrows(IllegalArgumentException::class.java) {
                store.rotateBearer(macBook.copy(bearer = devbox.bearer))
            }
            assertEquals(listOf(devbox, macBook), store.read().credentials)

            val preferences = context.getSharedPreferences(testPreferences, Context.MODE_PRIVATE)
            preferences.edit()
                .putString(
                    "machine.${macBook.machine.handle.encoded}.origin",
                    "https://changed.example.ts.net:8443/",
                )
                .commit()

            val partitioned = store.read()
            assertEquals(listOf(devbox), partitioned.credentials)
            assertEquals(listOf(UnreadableStoredMachine()), partitioned.unreadable)
            assertEquals(
                setOf(devbox.machine.handle.encoded, macBook.machine.handle.encoded),
                preferences.getStringSet("machine.handles", emptySet()),
            )

            assertThrows(IOException::class.java) {
                store.rotateBearer(devbox.copy(bearer = requireNotNull(GatewayBearer.parse("C" + "A".repeat(42)))))
            }

            preferences.edit().putString("machine.handles", "corrupt-index").commit()
            val corruptIndex = store.read()
            assertTrue(corruptIndex.credentials.isEmpty())
            assertTrue(corruptIndex.unreadable.single().collectionWide)
        } finally {
            resetFixture(targetContext, testPreferences, keyAlias)
        }
    }

    @Test
    fun twoMachineBearersRoundTripEncryptedWithFreshNonceAndBoundAad() {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val testPreferences = "skidbladnir.machines.instrumented-test"
        val context = object : ContextWrapper(targetContext) {
            override fun getSharedPreferences(name: String?, mode: Int) =
                targetContext.getSharedPreferences(testPreferences, mode)
        }
        val keyAlias = "skidbladnir.machine-bearers.instrumented-test"
        val store = MachineStore(context, keyAlias)
        val devbox = credential(
            "mh-0123456789abcdef0123456789abcdef",
            "Devbox",
            "https://devbox.example.ts.net:8443",
            "A".repeat(43),
        )
        val macBook = credential(
            "mh-fedcba9876543210fedcba9876543210",
            "MacBook",
            "https://macbook.example.ts.net:8443",
            "B" + "A".repeat(42),
        )

        try {
            installFixture(targetContext, testPreferences, keyAlias, listOf(devbox, macBook))

            val restored = store.read().credentials
            assertEquals(listOf(devbox.machine.handle, macBook.machine.handle), restored.map { it.machine.handle })
            assertTrue(restored.zip(listOf(devbox, macBook)).all { (actual, expected) -> actual == expected })
            val preferences = context.getSharedPreferences(testPreferences, Context.MODE_PRIVATE)
            val persisted = preferences.all.values.joinToString()
            assertFalse(persisted.contains(devbox.bearer.encoded))
            assertFalse(persisted.contains(macBook.bearer.encoded))

            val nonceKey = "machine.${devbox.machine.handle.encoded}.nonce"
            val firstNonce = preferences.getString(nonceKey, null)
            val rotatedBearer = requireNotNull(GatewayBearer.parse("C" + "A".repeat(42)))
            store.rotateBearer(devbox.copy(bearer = rotatedBearer))
            assertTrue(firstNonce != preferences.getString(nonceKey, null))

            val rotated = store.read().credentials.single { it.machine.handle == devbox.machine.handle }
            assertEquals(devbox.machine, rotated.machine)
            assertEquals(rotatedBearer, rotated.bearer)

            preferences.edit()
                .putString(
                    "machine.${macBook.machine.handle.encoded}.origin",
                    "https://changed.example.ts.net:8443/",
                )
                .commit()
            val unreadable = store.read()
            assertEquals(listOf(rotated), unreadable.credentials)
            assertEquals(listOf(UnreadableStoredMachine()), unreadable.unreadable)
        } finally {
            resetFixture(targetContext, testPreferences, keyAlias)
        }
    }

    private fun credential(handle: String, label: String, origin: String, bearer: String): MachineCredential =
        MachineCredential(
            PairedMachine(
                requireNotNull(MachineHandle.parse(handle)),
                requireNotNull(MachineLabel.parse(label)),
                requireNotNull(MachineOrigin.parse(origin)),
            ),
            requireNotNull(GatewayBearer.parse(bearer)),
        )

    private fun installFixture(
        context: Context,
        preferencesName: String,
        keyAlias: String,
        credentials: List<MachineCredential>,
    ) {
        resetFixture(context, preferencesName, keyAlias)
        val key = fixtureKey(keyAlias)
        val editor = context.getSharedPreferences(preferencesName, Context.MODE_PRIVATE).edit()
            .putStringSet("machine.handles", credentials.map { it.machine.handle.encoded }.toSet())
        credentials.forEach { credential ->
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.ENCRYPT_MODE, key)
            cipher.updateAAD(fixtureAssociatedData(credential.machine))
            val ciphertext = cipher.doFinal(credential.bearer.encoded.toByteArray(StandardCharsets.UTF_8))
            val prefix = "machine.${credential.machine.handle.encoded}"
            editor
                .putString("$prefix.label", credential.machine.label.text)
                .putString("$prefix.origin", credential.machine.origin.encoded)
                .putString("$prefix.ciphertext", Base64.encodeToString(ciphertext, Base64.NO_WRAP))
                .putString("$prefix.nonce", Base64.encodeToString(cipher.iv, Base64.NO_WRAP))
        }
        check(editor.commit()) { "could not install encrypted machine fixture" }
    }

    private fun resetFixture(context: Context, preferencesName: String, keyAlias: String) {
        check(context.getSharedPreferences(preferencesName, Context.MODE_PRIVATE).edit().clear().commit()) {
            "could not clear encrypted machine fixture"
        }
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(keyAlias)) keyStore.deleteEntry(keyAlias)
    }

    private fun fixtureKey(keyAlias: String): SecretKey = KeyGenerator.getInstance(
        KeyProperties.KEY_ALGORITHM_AES,
        "AndroidKeyStore",
    ).run {
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

    private fun fixtureAssociatedData(machine: PairedMachine): ByteArray =
        "dev.niels.skidbladnir.machine.bearer.v1\u0000${machine.handle.encoded}\u0000${machine.origin.encoded}"
            .toByteArray(StandardCharsets.UTF_8)

}
