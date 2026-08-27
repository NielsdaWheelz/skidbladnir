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

/** Owns the real Android Keystore + preferences boundary for atomic fleet installation and repair. */
@RunWith(AndroidJUnit4::class)
class MachineStoreInstrumentedTest {
    private val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    private val storage = MachineStorage(TEST_PREFERENCES, TEST_KEY_ALIAS)
    private val store = MachineStore(context, storage)

    private val fleet = listOf(
        credential("mh-11111111111111111111111111111111", "Arch", "https://arch.example.ts.net:8443/", "A".repeat(43)),
        credential("mh-22222222222222222222222222222222", "Devbox", "https://devbox.example.ts.net:8443/", "B" + "A".repeat(42)),
        credential("mh-33333333333333333333333333333333", "MacBook", "https://macbook.example.ts.net:8443/", "C" + "A".repeat(42)),
    )

    @Before
    fun clearFixture() = resetFixture()

    @After
    fun removeFixture() = resetFixture()

    @Test
    fun connectInstallsOneExactEncryptedFleetOnlyIntoAnEmptyStore() {
        assertEquals(FleetInstallation.Installed, store.installFixedFleet(fleet))
        assertEquals(MachineStoreRead(fleet, emptyList()), store.read())
        assertNoPlaintextBearers(fleet)

        val replacement = fleet.mapIndexed { index, credential ->
            credential.copy(bearer = bearer(('D'.code + index).toChar()))
        }
        assertEquals(FleetInstallation.StoreNotEmpty, store.installFixedFleet(replacement))
        assertEquals(MachineStoreRead(fleet, emptyList()), store.read())

        resetFixture()
        assertEquals(FleetInstallation.InvalidFleet, store.installFixedFleet(fleet.dropLast(1)))
        assertTrue(preferences().all.isEmpty())
        assertEquals(FleetInstallation.InvalidFleet, store.installFixedFleet(fleet.reversed()))
        assertTrue(preferences().all.isEmpty())
    }

    @Test
    fun reconnectRotatesAllBearersForOnlyTheExactInstalledIdentities() {
        assertEquals(FleetInstallation.Installed, store.installFixedFleet(fleet))
        val oldNonces = fleet.associate { credential ->
            credential.machine.handle to preferences().getString(
                storage.field(credential.machine.handle.encoded, "nonce"),
                null,
            )
        }
        val reconnected = fleet.mapIndexed { index, credential ->
            credential.copy(bearer = bearer(('D'.code + index).toChar()))
        }

        assertEquals(FleetReconnection.Reconnected, store.reconnectFixedFleet(reconnected))
        assertEquals(MachineStoreRead(reconnected, emptyList()), store.read())
        reconnected.forEach { credential ->
            assertNotEquals(
                oldNonces.getValue(credential.machine.handle),
                preferences().getString(storage.field(credential.machine.handle.encoded, "nonce"), null),
            )
        }
        assertNoPlaintextBearers(reconnected)

        val wrongIdentity = reconnected.toMutableList().also { candidates ->
            candidates[0] = candidates[0].copy(
                machine = candidates[0].machine.copy(
                    origin = requireNotNull(MachineOrigin.parse("https://replacement.example.ts.net:8443/")),
                ),
            )
        }
        assertEquals(FleetReconnection.FleetMismatch, store.reconnectFixedFleet(wrongIdentity))
        assertEquals(MachineStoreRead(reconnected, emptyList()), store.read())
    }

    @Test
    fun incompleteOrCorruptStorageNeverBecomesAPartialReadableFleet() {
        assertEquals(FleetInstallation.Installed, store.installFixedFleet(fleet))
        preferences().edit()
            .putString(
                storage.field(fleet.last().machine.handle.encoded, "origin"),
                "https://changed.example.ts.net:8443/",
            )
            .commit()

        val quarantined = store.read()
        assertTrue(quarantined.credentials.isEmpty())
        assertEquals(3, quarantined.unreadable.size)
        assertEquals(FleetReconnection.FleetMismatch, store.reconnectFixedFleet(fleet))
        assertEquals(FleetInstallation.StoreNotEmpty, store.installFixedFleet(fleet))

        resetFixture()
        assertEquals(FleetInstallation.Installed, store.installFixedFleet(fleet))
        preferences().edit()
            .putString(
                storage.field(fleet.first().machine.handle.encoded, "origin"),
                "https://ARCH.example.ts.net:8443/",
            )
            .commit()
        val noncanonicalOrigin = store.read()
        assertTrue(noncanonicalOrigin.credentials.isEmpty())
        assertEquals(3, noncanonicalOrigin.unreadable.size)
        assertEquals(FleetReconnection.FleetMismatch, store.reconnectFixedFleet(fleet))
        assertEquals(FleetInstallation.StoreNotEmpty, store.installFixedFleet(fleet))

        preferences().edit().putString("legacy.bearer", "must-not-be-read").commit()
        assertTrue(store.read().unreadable.single().collectionWide)
        preferences().edit().putString(storage.handlesField, "corrupt-index").commit()
        val corruptIndex = store.read()
        assertTrue(corruptIndex.credentials.isEmpty())
        assertTrue(corruptIndex.unreadable.single().collectionWide)
    }

    @Test
    fun collidingLabelsOriginsOrBearersQuarantineTheWholeFleet() {
        val cases = listOf(
            "label" to fleet[1].copy(
                machine = fleet[1].machine.copy(label = requireNotNull(MachineLabel.parse("arch"))),
            ),
            "origin" to fleet[1].copy(machine = fleet[1].machine.copy(origin = fleet[0].machine.origin)),
            "bearer" to fleet[1].copy(bearer = fleet[0].bearer),
        )
        cases.forEach { (collision, colliding) ->
            resetFixture()
            installFixture(listOf(fleet[0], colliding, fleet[2]))

            val observed = store.read()
            assertTrue("a $collision collision exposed a partial fleet", observed.credentials.isEmpty())
            assertEquals("a $collision collision did not quarantine every slot", 3, observed.unreadable.size)
        }
    }

    private fun assertNoPlaintextBearers(credentials: List<MachineCredential>) {
        val persisted = preferences().all.values.joinToString()
        credentials.forEach { credential ->
            assertFalse("a bearer reached storage in plaintext", persisted.contains(credential.bearer.encoded))
        }
    }

    private fun preferences() = storage.preferences(context)

    private fun bearer(prefix: Char): GatewayBearer = requireNotNull(GatewayBearer.parse(prefix + "A".repeat(42)))

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
        check(editor.commit()) { "could not install the machine fixture" }
    }

    private fun resetFixture() {
        check(preferences().edit().clear().commit()) { "could not clear the machine fixture" }
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(TEST_KEY_ALIAS)) keyStore.deleteEntry(TEST_KEY_ALIAS)
    }

    private companion object {
        const val TEST_PREFERENCES = "skidbladnir.machines.instrumented-test"
        const val TEST_KEY_ALIAS = "skidbladnir.machine-bearers.instrumented-test"
    }
}
