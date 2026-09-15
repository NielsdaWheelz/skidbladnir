package dev.niels.skidbladnir

import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import java.security.KeyStore
import java.security.cert.CertificateFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import okhttp3.Call
import okhttp3.EventListener
import okhttp3.Response
import kotlinx.serialization.Serializable
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Requires the real HTTPS gateway on an empty test-owned isolated tmux socket.
 * The fixture file contains credentials, machineHandle and cwd; never print its contents.
 * The other fixed-fleet peers may be unavailable. No owned HTTP response is intercepted.
 */
@RunWith(AndroidJUnit4::class)
class ShellsInstrumentedTest {
    @get:Rule val compose = createEmptyComposeRule()

    @Test
    fun terminalCreationKeepsExactTargetsAndCurrentNavigation() {
        val path = InstrumentationRegistry.getArguments().getString("shellsFixture")
        assumeTrue("phone shell owner boundary NOT_RUN without its real fixture", path != null)
        val fixture = decodeProtocol {
            productJson.decodeFromString<ShellFixture>(File(
                InstrumentationRegistry.getInstrumentation().targetContext.filesDir, requireNotNull(path),
            ).readText())
        }
        val credentials = fixture.credentials.map { item ->
            MachineCredential(PairedMachine(
                requireNotNull(MachineHandle.parse(item.handle)),
                requireNotNull(MachineLabel.parse(item.label)),
                requireNotNull(MachineOrigin.parse(item.origin)),
            ), requireNotNull(GatewayBearer.parse(item.bearer)))
        }
        val credential = credentials.single { it.machine.handle.encoded == fixture.machineHandle }
        require(credential.machine.origin.encoded == "https://127.0.0.1:8443/") {
            "shell fixture must use the test-owned device loopback reverse"
        }
        val context = InstrumentationRegistry.getInstrumentation().targetContext.applicationContext
        val storage = MachineStorage("shells-test-${System.nanoTime()}", "shells-test-${System.nanoTime()}")
        val store = MachineStore(context, storage)
        assertEquals(FleetInstallation.Installed, store.installFixedFleet(credentials))
        val certificate = CertificateFactory.getInstance("X.509")
            .generateCertificate(fixture.tlsCertificatePem.byteInputStream())
        val trustStore = KeyStore.getInstance(KeyStore.getDefaultType()).apply {
            load(null)
            setCertificateEntry("shell-fixture", certificate)
        }
        val trust = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm()).apply {
            init(trustStore)
        }.trustManagers.single() as X509TrustManager
        val tls = SSLContext.getInstance("TLS").apply { init(null, arrayOf(trust), null) }
        fun gatewayClient(listener: EventListener): GatewayClient = GatewayClient(GatewayClient().http.newBuilder()
            .sslSocketFactory(tls.socketFactory, trust)
            .eventListener(listener)
            .build())
        val api = gatewayClient(EventListener.NONE)
        fun inventory(): SessionsResponse = when (val result = api.listSessions(credential)) {
            is GatewayResult.Success -> result.value
            is GatewayResult.Failure -> error("real shell fixture inventory failed")
        }
        assertTrue("fixture must start empty with no agent profiles", inventory().let { it.profiles.isEmpty() && it.sessions.isEmpty() })
        val hold = AtomicReference<CreationHold?>()
        fun client() = gatewayClient(object : EventListener() {
            override fun responseHeadersEnd(call: Call, response: Response) {
                if (call.request().method != "POST" || !call.request().url.encodedPath.startsWith("/v1/sessions")) return
                val pending = hold.getAndSet(null) ?: return
                pending.arrived.countDown()
                check(pending.release.await(30, TimeUnit.SECONDS)) { "creation completion hold expired" }
            }
        })
        var controller: SkidbladnirController? = null
        var releaseOnFailure: CreationHold? = null
        var fontScale by mutableStateOf(1f)
        try {
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                var entry = DashboardEntryState()
                fun mount() = scenario.onActivity { activity ->
                    entry.install(activity.savedStateRegistry)
                    val active = SkidbladnirController(context, entry, storage, client())
                    controller = active
                    activity.setContent {
                        val density = LocalDensity.current
                        CompositionLocalProvider(LocalDensity provides Density(density.density, fontScale)) {
                            NidavellirTheme {
                                Box(Modifier.width(320.dp).fillMaxHeight()) {
                                    SkidbladnirApp(active, entry, FleetScanner(activity)) {}
                                }
                            }
                        }
                    }
                    active.start()
                }
                mount()
                fun readyDashboard(expectedSessions: Int) = compose.waitUntil(15_000) {
                    (controller?.state as? SkidbladnirUiState.Dashboard)?.machines?.any {
                        it.machine.handle == credential.machine.handle && it.canMutate &&
                            it.inventory.lastSnapshot()?.inventory?.sessions?.size == expectedSessions
                    } == true
                }
                fun attached() = compose.waitUntil(15_000) {
                    (controller?.state as? SkidbladnirUiState.Terminal)?.connection is TerminalUiStatus.Connected
                }
                fun beginHold(): CreationHold = CreationHold().also {
                    check(hold.compareAndSet(null, it))
                    releaseOnFailure = it
                }
                fun terminal(): SkidbladnirUiState.Terminal = compose.runOnIdle {
                    checkNotNull(controller?.state as? SkidbladnirUiState.Terminal)
                }
                readyDashboard(0)
                compose.onNodeWithContentDescription("New dwarf").performClick()
                compose.onNodeWithTag("forge-machine-${credential.machine.handle.encoded}").performClick()
                compose.onNodeWithText("Terminal", substring = false).performClick()
                compose.runOnIdle {
                    controller!!.updateForgeDraft { it.copy(cwd = fixture.cwd, space = SpaceDraft.Chosen("phone-shells")) }
                }
                val pressureBefore = compose.runOnIdle {
                    (controller!!.state as SkidbladnirUiState.Dashboard).machines.single {
                        it.machine.handle == credential.machine.handle
                    }.pressure
                }
                val first = beginHold()
                compose.onNodeWithTag("forge-submit").performScrollTo().performClick()
                assertTrue("real gateway completed standalone creation", first.arrived.await(15, TimeUnit.SECONDS))
                assertEquals("one real session while response is pending", 1, inventory().sessions.size)
                // An ordinary pressure/inventory publication must preserve this submitted form.
                compose.runOnIdle { controller!!.verifyVisibleInventory() }
                compose.waitUntil(10_000) {
                    (controller?.state as? SkidbladnirUiState.Dashboard)?.machines?.single {
                        it.machine.handle == credential.machine.handle
                    }?.pressure != pressureBefore
                }
                compose.onNodeWithTag("forge-submit").assertIsNotEnabled()
                first.release.countDown()
                attached()
                val source = terminal().target
                assertEquals("phone-shells", source.session.space?.text)
                assertEquals(null, source.session.launchProfile)
                compose.runOnIdle { fontScale = 2f }
                compose.waitForIdle()
                val controls = listOf("Detach", "Rename ${source.session.tmuxName} on ${credential.machine.label.text}",
                    "new terminal here", "Terminal text size", killActionLabel(credential.machine.label, source))
                    .map { compose.onNodeWithContentDescription(it).assertIsDisplayed().fetchSemanticsNode().boundsInRoot }
                val minimum = with(compose.density) { 48.dp.toPx() }
                controls.forEach { bounds -> assertTrue("header action keeps its touch target", bounds.width >= minimum && bounds.height >= minimum) }
                controls.zipWithNext().forEach { (left, right) -> assertTrue("header controls do not overlap", left.right <= right.left) }
                val header = compose.onNodeWithTag("terminal-header").fetchSemanticsNode().boundsInRoot
                controls.forEach { bounds ->
                    assertTrue("every action stays in the same header row", bounds.top >= header.top && bounds.bottom <= header.bottom &&
                        bounds.left >= header.left && bounds.right <= header.right)
                }
                compose.runOnIdle { fontScale = 1f }
                val second = beginHold()
                compose.onNodeWithContentDescription("new terminal here").assertIsEnabled().performClick()
                assertTrue(second.arrived.await(15, TimeUnit.SECONDS))
                assertTrue("source remains attached until confirmed completion", terminal().target == source)
                compose.onNodeWithContentDescription("new terminal here").assertIsNotEnabled()
                compose.runOnIdle { controller!!.newTerminalHere() }
                assertEquals("duplicate action made exactly one new session", 2, inventory().sessions.size)
                second.release.countDown()
                compose.waitUntil(15_000) { (controller?.state as? SkidbladnirUiState.Terminal)?.target != source }
                attached()
                val created = terminal().target
                assertTrue("new terminal has a distinct exact lifetime", !sameSessionLifetime(source, created))
                assertEquals(source.session.space, created.session.space)
                assertTrue("source survived creation", inventory().sessions.any { it.identityToken == source.session.identityToken })

                // Actual platform back returns to the existing collection and the new membership.
                scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
                readyDashboard(2)
                assertTrue(entry.space.matches(created.session.space))
                compose.onNodeWithTag("session-card-${credential.machine.handle.encoded}-${created.session.tmuxId}").performClick()
                attached()
                val late = beginHold()
                compose.onNodeWithContentDescription("new terminal here").performClick()
                assertTrue(late.arrived.await(15, TimeUnit.SECONDS))
                compose.onNodeWithContentDescription("Detach").performClick()
                late.release.countDown()
                readyDashboard(3)
                assertEquals(3, inventory().sessions.size)
                assertTrue("late create cannot reopen attachment", controller?.state is SkidbladnirUiState.Dashboard)

                // A real invalid working directory is rejected without another session.
                compose.onNodeWithContentDescription("New dwarf").performClick()
                compose.onNodeWithTag("forge-machine-${credential.machine.handle.encoded}").performClick()
                compose.onNodeWithText("Terminal", substring = false).performClick()
                compose.runOnIdle { controller!!.updateForgeDraft { it.copy(cwd = fixture.cwd + "/missing-shell-directory") } }
                compose.onNodeWithTag("forge-submit").performScrollTo().performClick()
                compose.waitUntil(15_000) { (controller?.state as? SkidbladnirUiState.Dashboard)?.forge?.failure is ForgeFailure.Definite }
                compose.onNodeWithTag("forge-failure").assertIsDisplayed()
                assertEquals(3, inventory().sessions.size)
                compose.runOnIdle { controller!!.dismissForge() }

                // A different credential generation owns the screen before a late response arrives.
                compose.onNodeWithTag("session-card-${credential.machine.handle.encoded}-${created.session.tmuxId}").performClick()
                attached()
                val rotated = beginHold()
                compose.onNodeWithContentDescription("new terminal here").performClick()
                assertTrue(rotated.arrived.await(15, TimeUnit.SECONDS))
                val changedCredentials = credentials.map {
                    if (it == credential) it.copy(bearer = requireNotNull(GatewayBearer.parse("B".repeat(42) + "A"))) else it
                }
                assertEquals(FleetReconnection.Reconnected, store.reconnectFixedFleet(changedCredentials))
                compose.runOnIdle { controller!!.stopForBackground(); controller!!.start() }
                rotated.release.countDown()
                compose.waitUntil(15_000) {
                    (controller?.state as? SkidbladnirUiState.Dashboard)?.machines?.any {
                        it.machine.handle == credential.machine.handle && it.access == MachineAccess.AuthRequired
                    } == true
                }
                assertEquals(4, inventory().sessions.size)
                assertEquals(FleetReconnection.Reconnected, store.reconnectFixedFleet(credentials))
                compose.runOnIdle { controller!!.stopForBackground(); controller!!.start() }
                readyDashboard(4)

                // Activity recreation restores only the content-free dashboard capsule.
                compose.onNodeWithTag("session-card-${credential.machine.handle.encoded}-${created.session.tmuxId}").performClick()
                attached()
                val recreated = beginHold()
                compose.onNodeWithContentDescription("new terminal here").performClick()
                assertTrue(recreated.arrived.await(15, TimeUnit.SECONDS))
                compose.runOnIdle { controller!!.close() }
                scenario.recreate()
                entry = DashboardEntryState()
                mount()
                recreated.release.countDown()
                readyDashboard(5)
                assertFalse("recreation never restores an attachment", controller?.state is SkidbladnirUiState.Terminal)
                assertEquals("recreation never replays a creation", 5, inventory().sessions.size)
            }
        } finally {
            releaseOnFailure?.release?.countDown()
            InstrumentationRegistry.getInstrumentation().runOnMainSync { controller?.close() }
            // The fixture starts empty and is exclusive to this test; every observed session was created above.
            inventory().sessions.forEach { session ->
                assertTrue("remove only this test's exact session", api.killSession(credential, SessionTarget(credential.machine.handle, session)) is GatewayResult.Success)
            }
            storage.preferences(context).edit().clear().commit()
            storage.destroyBearerKeyForQuarantine()
            api.closeAsync()
        }
    }

    @Serializable
    private data class ShellFixture(
        val credentials: List<FixtureCredential>,
        val machineHandle: String,
        val cwd: String,
        val tlsCertificatePem: String,
    )

    @Serializable
    private data class FixtureCredential(val handle: String, val label: String, val origin: String, val bearer: String)

    private class CreationHold {
        val arrived = CountDownLatch(1)
        val release = CountDownLatch(1)
    }
}
