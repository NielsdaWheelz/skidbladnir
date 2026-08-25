package dev.niels.skidbladnir

import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
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
        val report = ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val ready = CountDownLatch(1)
            scenario.onActivity { ready.countDown() }
            assertTrue("MainActivity did not launch", ready.await(5, TimeUnit.SECONDS))
            PlatformPreflightReport.collect(context)
        }
        val json = report.toJson()

        assertEquals("android-target-preflight.v1", report.schema)
        assertEquals("SM-S906W", report.observed.model)
        assertEquals(36, report.observed.api)
        assertTrue(report.observed.webViewPackage?.isNotBlank() == true)
        assertEquals("com.google.android.inputmethod.latin", report.observed.imePackage)
        assertEquals(true, report.observed.tailscaleInstalled)
        val checks = report.checks.associateBy { it.id }
        assertEquals(PreflightStatus.PASS, checks.getValue("target-device").status)
        assertEquals(PreflightStatus.PASS, checks.getValue("api-36").status)
        assertEquals(PreflightStatus.PASS, checks.getValue("webview-runtime").status)
        assertEquals(PreflightStatus.PASS, checks.getValue("gboard-selected").status)
        assertEquals(PreflightStatus.PASS, checks.getValue("tailscale-client-present").status)
        assertEquals(PreflightStatus.NOT_RUN, report.overall)
        assertTrue(json.startsWith("{\n    \"schema\": \"android-target-preflight.v1\""))
        assertFalse(json.contains("serial"))
        assertFalse(json.contains("bearer"))
        assertFalse(json.contains("objective"))
        assertFalse(json.contains("terminal"))
    }
}
