package dev.niels.skidbladnir

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class PlatformPreflightInstrumentedTest {
    @Test
    fun noDeviceResultContractIsExplicitlyNotRun() {
        val report = PlatformPreflightReport.noDevice()

        assertEquals("android-target-preflight.v1", report.schema)
        assertEquals("SM-S906W", report.target.model)
        assertEquals(36, report.target.api)
        assertEquals(PreflightStatus.NOT_RUN, report.overall)
        assertTrue(report.checks.isNotEmpty())
        assertTrue(report.checks.all { it.status == PreflightStatus.NOT_RUN })
        assertTrue(report.checks.all { it.reason == "SM-S906W is not attached" })
    }

    @Test
    fun attachedResultIsHostReadableWithoutSensitiveFields() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val report = PlatformPreflightReport.collect(context)
        val json = report.toJson()

        assertEquals("android-target-preflight.v1", report.schema)
        assertTrue(json.startsWith("{\n    \"schema\": \"android-target-preflight.v1\""))
        assertFalse(json.contains("serial"))
        assertFalse(json.contains("bearer"))
        assertFalse(json.contains("objective"))
        assertFalse(json.contains("terminal"))
    }
}
