package dev.niels.skidbladnir

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/** One-shot operator bootstrap for the fixed collection; excluded from the routine platform suite. */
@RunWith(AndroidJUnit4::class)
class MachineProvisioningInstrumentedTest {
    @Test
    fun provisionAuthenticatedFixedCollectionFromPrivateStaging() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val staging = File(context.cacheDir, STAGING_FILE)
        assertTrue("private provisioning input is required", staging.isFile)
        val bytes = try {
            staging.readBytes()
        } finally {
            assertTrue("plaintext provisioning input was not deleted", staging.delete())
        }
        assertTrue("provisioning input exceeds its fixed bound", bytes.size in 1..MAXIMUM_INPUT_BYTES)
        val payload = try {
            strictJson.decodeFromString<ProvisioningPayload>(bytes.toString(Charsets.UTF_8))
        } catch (_: Exception) {
            throw AssertionError("provisioning input is malformed")
        }
        assertEquals(setOf("Devbox", "MacBook"), payload.machines.map { it.label }.toSet())
        assertEquals(2, payload.machines.size)
        val credentials = payload.machines.map { record ->
            MachineCredential(
                PairedMachine(
                    MachineHandle.parse(record.handle)
                        ?: throw AssertionError("provisioning handle is malformed"),
                    MachineLabel.parse(record.label)
                        ?: throw AssertionError("provisioning label is malformed"),
                    MachineOrigin.parse(record.origin)
                        ?: throw AssertionError("provisioning origin is malformed"),
                ),
                GatewayBearer.parse(record.bearer)
                    ?: throw AssertionError("provisioning bearer is malformed"),
            )
        }
        val store = MachineStore(context, MachineStorage.production)
        assertEquals(
            MachineProvisioning.Provisioned,
            store.provisionFixedCollection(credentials),
        )
        val observed = store.read()
        assertTrue(observed.unreadable.isEmpty())
        assertEquals(credentials.sortedBy { it.machine.label.text }, observed.credentials)
        val persisted = MachineStorage.production.preferences(context).all.values.joinToString()
        credentials.forEach { credential ->
            assertFalse("a provisioned bearer reached preferences in plaintext", persisted.contains(credential.bearer.encoded))
        }
    }

    @Serializable
    private data class ProvisioningPayload(val machines: List<ProvisioningMachine>)

    @Serializable
    private data class ProvisioningMachine(
        val handle: String,
        val label: String,
        val origin: String,
        val bearer: String,
    )

    private companion object {
        const val STAGING_FILE = "skidbladnir-machine-provisioning.json"
        const val MAXIMUM_INPUT_BYTES = 4_096
        val strictJson = Json {
            ignoreUnknownKeys = false
            isLenient = false
            explicitNulls = true
        }
    }
}
