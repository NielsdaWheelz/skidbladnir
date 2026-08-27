package dev.niels.skidbladnir

import android.content.Context
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.security.KeyStore
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Exercises the app's ingress for externally provisioned machine credentials against the real
 * Android Keystore and preference storage. Fixtures are written through the same [MachineStorage]
 * the product reads with, so the at-rest format has exactly one definition on both sides.
 */
@RunWith(AndroidJUnit4::class)
class MachineStoreInstrumentedTest {
    private val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    private val storage = MachineStorage(TEST_PREFERENCES, TEST_KEY_ALIAS)
    private val store = MachineStore(context, storage)

    private val devbox = credential(
        "mh-0123456789abcdef0123456789abcdef",
        "Devbox",
        "https://devbox.example.ts.net:8443",
        "A".repeat(43),
    )
    private val macBook = credential(
        "mh-fedcba9876543210fedcba9876543210",
        "MacBook",
        "https://macbook.example.ts.net:8443",
        "B" + "A".repeat(42),
    )

    @Before
    fun clearFixture() = resetFixture()

    @After
    fun removeFixture() = resetFixture()

    @Test
    fun provisionedBearersRoundTripEncryptedAndRotateWithAFreshNonce() {
        installFixture(listOf(devbox, macBook))

        assertEquals(listOf(devbox, macBook), store.read().credentials)
        assertNoPlaintextBearer(devbox, macBook)

        val nonceField = storage.field(devbox.machine.handle.encoded, "nonce")
        val firstNonce = preferences().getString(nonceField, null)
        val rotatedBearer = requireNotNull(GatewayBearer.parse("C" + "A".repeat(42)))
        val rotated = devbox.copy(bearer = rotatedBearer)

        assertEquals(BearerRotation.Rotated, store.rotateBearer(rotated))
        assertNotEquals(firstNonce, preferences().getString(nonceField, null))
        assertEquals(listOf(rotated, macBook), store.read().credentials)
        assertNoPlaintextBearer(rotated, macBook)
    }

    @Test
    fun rotationRefusesAnotherMachineBearerAndAnIncompleteCollection() {
        installFixture(listOf(devbox, macBook))

        assertEquals(
            BearerRotation.BearerInUse,
            store.rotateBearer(macBook.copy(bearer = devbox.bearer)),
        )
        assertEquals(listOf(devbox, macBook), store.read().credentials)

        preferences().edit()
            .putString(
                storage.field(macBook.machine.handle.encoded, "origin"),
                "https://changed.example.ts.net:8443/",
            )
            .commit()

        val partitioned = store.read()
        assertEquals(listOf(devbox), partitioned.credentials)
        assertEquals(listOf(UnreadableStoredMachine()), partitioned.unreadable)

        val fresh = requireNotNull(GatewayBearer.parse("C" + "A".repeat(42)))
        assertEquals(
            "an incomplete collection must not accept a credential write",
            BearerRotation.MachineUnavailable,
            store.rotateBearer(devbox.copy(bearer = fresh)),
        )

        preferences().edit().putString(storage.handlesField, "corrupt-index").commit()
        val corruptIndex = store.read()
        assertTrue(corruptIndex.credentials.isEmpty())
        assertTrue(corruptIndex.unreadable.single().collectionWide)
    }

    @Test
    fun collidingLabelsOriginsOrBearersQuarantineEveryColliderAtTheBoundary() {
        val other = credential(
            "mh-00112233445566778899aabbccddeeff",
            "devbox",
            "https://other.example.ts.net:8443",
            "D" + "A".repeat(42),
        )

        listOf(
            "case-insensitive label" to other,
            "origin" to other.copy(
                machine = other.machine.copy(
                    label = requireNotNull(MachineLabel.parse("Other")),
                    origin = devbox.machine.origin,
                ),
            ),
            "bearer" to other.copy(
                machine = other.machine.copy(label = requireNotNull(MachineLabel.parse("Other"))),
                bearer = devbox.bearer,
            ),
        ).forEach { (collision, colliding) ->
            resetFixture()
            installFixture(listOf(devbox, macBook, colliding))

            val partitioned = store.read()
            assertEquals(
                "a duplicate $collision must not resolve to one usable machine",
                listOf(macBook),
                partitioned.credentials,
            )
            assertEquals(
                "both sides of a duplicate $collision must be quarantined",
                listOf(UnreadableStoredMachine(), UnreadableStoredMachine()),
                partitioned.unreadable,
            )
        }
    }

    private fun assertNoPlaintextBearer(vararg credentials: MachineCredential) {
        val persisted = preferences().all.values.joinToString()
        credentials.forEach {
            assertFalse(
                "a bearer reached storage in plaintext",
                persisted.contains(it.bearer.encoded),
            )
        }
    }

    private fun preferences() = storage.preferences(context)

    private fun credential(handle: String, label: String, origin: String, bearer: String): MachineCredential =
        MachineCredential(
            PairedMachine(
                requireNotNull(MachineHandle.parse(handle)),
                requireNotNull(MachineLabel.parse(label)),
                requireNotNull(MachineOrigin.parse(origin)),
            ),
            requireNotNull(GatewayBearer.parse(bearer)),
        )

    private fun installFixture(credentials: List<MachineCredential>) {
        val editor = preferences().edit()
            .putStringSet(storage.handlesField, credentials.map { it.machine.handle.encoded }.toSet())
        credentials.forEach { credential ->
            val sealed = storage.seal(credential.machine, credential.bearer)
            val handle = credential.machine.handle.encoded
            editor
                .putString(storage.field(handle, "label"), credential.machine.label.text)
                .putString(storage.field(handle, "origin"), credential.machine.origin.encoded)
                .putString(storage.field(handle, "ciphertext"), sealed.ciphertext)
                .putString(storage.field(handle, "nonce"), sealed.nonce)
        }
        check(editor.commit()) { "could not install the provisioned machine fixture" }
    }

    private fun resetFixture() {
        check(preferences().edit().clear().commit()) { "could not clear the provisioned machine fixture" }
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(TEST_KEY_ALIAS)) keyStore.deleteEntry(TEST_KEY_ALIAS)
    }

    private companion object {
        const val TEST_PREFERENCES = "skidbladnir.machines.instrumented-test"
        const val TEST_KEY_ALIAS = "skidbladnir.machine-bearers.instrumented-test"
    }
}
