package dev.niels.skidbladnir

import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.assertTextEquals
import androidx.compose.ui.test.assertTextContains
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.semantics.getOrNull
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.Locale
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MultiMachineUiInstrumentedTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    @Test
    fun compactDashboardTopBarOwnsTheCreateActionWithoutMachineAdministration() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        Column {
                            DashboardTopBar(
                                summary = "4 tmux sessions across 2 machines",
                                refreshing = false,
                                canForge = true,
                                onRefresh = {},
                                onNewAgent = {},
                            )
                            UnreadableMachineStrip(
                                UnreadableStoredMachine(),
                            )
                        }
                    }
                }
            }
            compose.onNodeWithTag("dashboard-top-bar").assertIsDisplayed()
            compose.onNodeWithTag("new-agent").assertIsDisplayed().assertIsEnabled()
            val topBarBounds = compose.onNodeWithTag("dashboard-top-bar").getUnclippedBoundsInRoot()
            val newAgentBounds = compose.onNodeWithTag("new-agent").getUnclippedBoundsInRoot()
            assertTrue("dashboard top bar exceeds 64 dp", topBarBounds.bottom.value - topBarBounds.top.value <= 64f)
            assertTrue(
                "New agent is not in the trailing half of the dashboard top bar",
                newAgentBounds.left.value > (topBarBounds.left.value + topBarBounds.right.value) / 2f,
            )
            compose.onNodeWithText("Add machine").assertDoesNotExist()
            compose.onNodeWithText("Rename").assertDoesNotExist()
            compose.onNodeWithText("Remove machine").assertDoesNotExist()
            compose.onNodeWithText("Remove pairing").assertDoesNotExist()
            compose.onNodeWithText("Provisioning repair is required outside this app.", substring = true)
                .assertIsDisplayed()
        }
    }

    @Test
    fun machinePressureRestoresMetricsHistoryAndCapabilityDetails() {
        val current = PressureSample(
            sampledAt = "2026-08-26T12:00:00Z",
            level = PressureLevel.Warm,
            reasons = listOf(PressureReason.Load),
            metrics = PressureMetrics(
                cpuPercent = 34.0,
                normalizedLoad = 1.25,
                diskAvailablePercent = 61.0,
                memoryPressure = SystemMemoryPressure.Warning,
            ),
            missing = listOf(PressureMetric.SwapUsedPercent),
        )
        val response = PressureResponse(
            unsupported = listOf(
                PressureMetric.CpuPsiSomeAvg60Percent,
                PressureMetric.IoPsiFullAvg60Percent,
                PressureMetric.MemoryAvailablePercent,
                PressureMetric.MemoryPsiFullAvg60Percent,
            ),
            current = current,
            history = listOf(
                current.copy(
                    sampledAt = "2026-08-26T11:59:55Z",
                    level = PressureLevel.Normal,
                    reasons = emptyList(),
                    metrics = current.metrics.copy(
                        normalizedLoad = 0.4,
                        memoryPressure = SystemMemoryPressure.Normal,
                    ),
                ),
                current,
            ),
        )

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        MachinePressureStrip(
                            "MacBook",
                            PressureState.Stale(response, GatewayFailure.Transport),
                        )
                    }
                }
            }
            compose.onNodeWithText("MACBOOK WARM").assertIsDisplayed()
            compose.onNodeWithText("STALE").assertIsDisplayed()
            compose.onNodeWithText("CPU 34%", substring = true).assertIsDisplayed()
            compose.onNodeWithText("load 1.25", substring = true).assertIsDisplayed()
            compose.onNodeWithText("disk 61% free", substring = true).assertIsDisplayed()
            compose.onNodeWithText("memory pressure WARNING", substring = true).assertIsDisplayed()
            compose.onNodeWithText("Recent pressure history · up to 15 min").assertIsDisplayed()
            compose.onNodeWithContentDescription("MacBook pressure history: normal, warm").assertIsDisplayed()
            compose.onNodeWithText("Missing: swap used").assertIsDisplayed()
            compose.onNodeWithText(
                "Unsupported: CPU PSI, I/O PSI, memory available, memory PSI",
            ).assertIsDisplayed()
            compose.onNodeWithText("Pressure: load").assertIsDisplayed()
        }
    }

    @Test
    fun staleTerminalKillConfirmationDisablesConfirmButKeepsCancelAvailable() {
        val handle = requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"))
        val machine = PairedMachine(
            handle,
            requireNotNull(MachineLabel.parse("Devbox")),
            requireNotNull(MachineOrigin.parse("https://devbox.example.ts.net:8443")),
        )
        val session = AgentSession(
            id = "${'$'}1",
            name = "ga-durinn",
            identityToken = "v1-0123456789abcdef0123456789abcdef.100.200.1",
            attachedClients = 1,
            attention = false,
            status = SessionStatus(SessionStatusKind.Working, SessionStatusSignal.Lifecycle, "2026-08-26T11:59:55Z"),
        )
        val target = AgentTarget(handle, session)
        val terminal = SkidbladnirUiState.Terminal(
            machine,
            target,
            machineCanMutate = true,
            attempt = 1,
            connection = TerminalUiStatus.Connected(1, TerminalGeometry.Owner),
            kill = KillState(machine, target, pending = false),
        )
        val projection = synchronizeTerminalMachineState(
            terminal,
            MachineState(
                machine,
                MachineAccess.Ready,
                InventoryState.Stale(
                    InventorySnapshot(
                        SessionsResponse(MachineSummary(handle, MachinePlatform.Linux), "2026-08-26T12:00:00Z", emptyList(), listOf(session)),
                        receivedAtElapsedMillis = 1_000,
                    ),
                    GatewayFailure.Transport,
                ),
                PressureState.Reading,
            ),
        )

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        KillConfirmation(
                            state = requireNotNull(projection.kill),
                            actionAdmissible = terminalActionAdmissible(
                                projection.machineCanMutate,
                                projection.connection,
                            ),
                            onDismiss = {},
                            onConfirm = {},
                        )
                    }
                }
            }
            compose.onNodeWithTag("kill-confirm").assertIsNotEnabled()
            compose.onNodeWithText("Cancel").assertIsEnabled()
        }
    }

    @Test
    fun twoHealthyPairingsRenderAndRouteThroughTheProductionUi() {
        val arguments = InstrumentationRegistry.getArguments()
        assumeTrue(
            "NOT_RUN: pass skidbladnir.multiMachineUi=true only for the approved physical UI journey",
            arguments.getString(UI_OPT_IN) == "true",
        )
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val credentials = MachineStore(context).read().credentials
        assertEquals("UI journey requires exactly two existing production pairings", 2, credentials.size)
        val expectedHandles = setOf(
            requireNotNull(MachineHandle.parse(requireNotNull(arguments.getString(DEVBOX_MACHINE)))),
            requireNotNull(MachineHandle.parse(requireNotNull(arguments.getString(MACBOOK_MACHINE)))),
        )
        assertEquals(
            "UI journey pairings do not match the requested machines",
            expectedHandles,
            credentials.map { it.machine.handle }.toSet(),
        )
        assertEquals(
            "UI journey requires independent bearer authorities",
            2,
            credentials.map { it.bearer.encoded }.distinct().size,
        )
        val client = GatewayClient()
        try {
            val inventories = credentials.associateWith { credential ->
                gatewaySuccess(client.listSessions(credential))
            }
            val targets = credentials.associateWith { credential ->
                val session = inventories.getValue(credential).sessions.firstOrNull()
                requireNotNull(session) {
                    "UI journey requires one pre-existing session on machine ${credential.machine.handle.encoded}"
                }
                AgentTarget(credential.machine.handle, session)
            }
            assertTrue(inventories.values.all { it.profiles.isNotEmpty() })

            ActivityScenario.launch(MainActivity::class.java).use { scenario ->
                compose.onNodeWithText("Add machine").assertDoesNotExist()
                compose.onNodeWithText("Rename").assertDoesNotExist()
                compose.onNodeWithText("Remove machine").assertDoesNotExist()
                compose.onNodeWithTag("new-agent").assertIsDisplayed()
                credentials.forEach { credential ->
                    waitForTag(machineStripTag(credential), 30_000)
                    compose.onNodeWithTag(stripLabelTag(credential), useUnmergedTree = true)
                        .assertTextContains(credential.machine.label.text.uppercase(Locale.ROOT), substring = true)
                }

                scenario.recreate()
                credentials.forEach { credential ->
                    waitForTag("machine-state-fresh-${credential.machine.handle.encoded}", 30_000)
                    compose.onNodeWithTag(stripLabelTag(credential), useUnmergedTree = true)
                        .assertTextContains(credential.machine.label.text.uppercase(Locale.ROOT), substring = true)
                }

                val first = credentials[0]
                val second = credentials[1]

                compose.onNodeWithText(
                    "${inventories.values.sumOf { it.sessions.size }} tmux",
                    substring = true,
                ).assertIsDisplayed()

                credentials.forEach { credential ->
                    val target = targets.getValue(credential)
                    compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(target)))
                    compose.onNodeWithTag(cardTag(target)).assertIsDisplayed()
                    compose.onNodeWithTag(cardPillTag(target), useUnmergedTree = true)
                        .assertTextEquals(credential.machine.label.text)
                }

                compose.onNodeWithTag("new-agent").performClick()
                waitForTag("forge-sheet", 10_000)
                compose.onNodeWithTag(forgeMachineTag(first)).assertIsNotSelected()
                compose.onNodeWithTag(forgeMachineTag(second)).assertIsNotSelected()
                compose.onNodeWithTag(forgeMachineTag(first)).assertIsEnabled()
                compose.onNodeWithTag(forgeMachineTag(second)).assertIsEnabled()
                compose.onAllNodesWithTag(forgeProfileTag(first)).assertCountEquals(0)
                compose.onAllNodesWithTag(forgeProfileTag(second)).assertCountEquals(0)
                compose.onNodeWithTag(forgeMachineTag(first)).performClick()
                compose.onNodeWithTag(forgeMachineTag(first)).assertIsSelected()
                val firstProfiles = inventories.getValue(first).profiles
                compose.onAllNodesWithTag(forgeProfileTag(first)).assertCountEquals(firstProfiles.size)
                compose.onAllNodesWithTag(forgeProfileTag(first))[0].performClick()
                compose.onAllNodesWithTag(forgeProfileTag(first))[0].assertIsSelected()
                compose.onAllNodesWithTag(forgeProfileTag(second)).assertCountEquals(0)
                scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() }
                compose.waitUntil(10_000) {
                    compose.onAllNodesWithTag("forge-sheet").fetchSemanticsNodes().isEmpty()
                }

                selectMachine(first)
                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(targets.getValue(first))))
                compose.onNodeWithTag(cardTag(targets.getValue(first))).assertIsDisplayed()
                compose.onNodeWithTag(cardPillTag(targets.getValue(first)), useUnmergedTree = true)
                    .assertTextEquals(first.machine.label.text)
                compose.onNodeWithTag(cardTag(targets.getValue(second))).assertDoesNotExist()
                compose.onNodeWithText(
                    "${inventories.getValue(first).sessions.size} tmux",
                    substring = true,
                ).assertIsDisplayed()
                selectMachine(second)
                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(targets.getValue(second))))
                compose.onNodeWithTag(cardTag(targets.getValue(second))).assertIsDisplayed()
                compose.onNodeWithTag(cardPillTag(targets.getValue(second)), useUnmergedTree = true)
                    .assertTextEquals(second.machine.label.text)
                compose.onNodeWithTag(cardTag(targets.getValue(first))).assertDoesNotExist()

                val killCredential = credentials[0]
                val killTarget = targets.getValue(killCredential)
                selectMachine(killCredential)
                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(killTarget)))
                compose.onNodeWithTag(killTag(killTarget)).performClick()
                compose.onNodeWithText(
                    "Kill ${killTarget.session.name} on ${killCredential.machine.label.text}?",
                ).assertIsDisplayed()
                compose.onNodeWithText("Cancel").performClick()
            }
        } finally {
            client.closeAsync()
        }
    }

    @Test
    fun genuinelyUnavailablePairingDisablesMachineMutations() {
        val arguments = InstrumentationRegistry.getArguments()
        val encodedHandle = arguments.getString(FAILED_MACHINE)
        assumeTrue(
            "NOT_RUN: pass skidbladnir.failedMachine=<handle> with that real gateway unavailable",
            !encodedHandle.isNullOrBlank(),
        )
        val failedHandle = requireNotNull(MachineHandle.parse(requireNotNull(encodedHandle)))
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val credentials = MachineStore(context).read().credentials
        assertEquals(2, credentials.size)
        assertTrue(credentials.any { it.machine.handle == failedHandle })
        val failed = credentials.single { it.machine.handle == failedHandle }
        val healthy = credentials.single { it.machine.handle != failedHandle }
        val readiness = context.cacheDir.resolve(OUTAGE_READY_FILE)
        assertTrue("Could not clear stale outage coordination marker", !readiness.exists() || readiness.delete())
        val client = GatewayClient()
        val failedTarget = AgentTarget(
            failedHandle,
            requireNotNull(gatewaySuccess(client.listSessions(failed)).sessions.firstOrNull()) {
                "Outage journey requires one pre-existing session on ${failed.machine.handle.encoded}"
            },
        )

        try {
            ActivityScenario.launch(MainActivity::class.java).use {
                waitForTag("machine-state-fresh-${failedHandle.encoded}", 30_000)
                waitForTag("machine-state-fresh-${healthy.machine.handle.encoded}", 30_000)

                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(failedTarget)))
                compose.onNodeWithTag(killTag(failedTarget)).performClick()
                compose.onNodeWithText(
                    "Kill ${failedTarget.session.name} on ${failed.machine.label.text}?",
                ).assertIsDisplayed()
                compose.onNodeWithTag("kill-confirm").assertIsEnabled()
                assertTrue("Could not publish outage coordination marker", readiness.createNewFile())

                waitForTag("machine-state-stale-${failedHandle.encoded}", 30_000)
                val healthyAtOutage = singleTagWithPrefix(inventoryReceivedPrefix(healthy))
                compose.waitUntil(30_000) {
                    singleTagWithPrefix(inventoryReceivedPrefix(healthy)) != healthyAtOutage
                }
                waitForTag("machine-state-fresh-${healthy.machine.handle.encoded}", 30_000)
                waitForTag("machine-nonmutating-${failedHandle.encoded}", 30_000)
                waitForTag("machine-actionable-${healthy.machine.handle.encoded}", 30_000)
                compose.onNodeWithTag(stripLabelTag(failed), useUnmergedTree = true)
                    .assertTextContains(failed.machine.label.text.uppercase(Locale.ROOT), substring = true)
                compose.onNodeWithTag(stripLabelTag(healthy), useUnmergedTree = true)
                    .assertTextContains(healthy.machine.label.text.uppercase(Locale.ROOT), substring = true)
                compose.onNodeWithTag("kill-confirm").assertIsNotEnabled()
                compose.onNodeWithText("Cancel").assertIsEnabled().performClick()
                compose.onNodeWithTag(filterTag(healthy)).performClick()
                compose.onNodeWithTag("new-agent").assertIsEnabled()
                compose.onNodeWithTag("machine-filter-${failedHandle.encoded}").performClick()
                compose.onNodeWithTag("new-agent").assertIsNotEnabled()
            }
        } finally {
            assertTrue("Could not clear outage coordination marker", !readiness.exists() || readiness.delete())
            client.closeAsync()
        }
    }

    private fun selectMachine(credential: MachineCredential) {
        compose.onNodeWithTag(filterTag(credential)).performClick()
        waitForTag(cardTagForMachine(credential), 10_000, prefix = true)
    }

    private fun waitForTag(tag: String, timeoutMillis: Long, prefix: Boolean = false) {
        compose.waitUntil(timeoutMillis) {
            if (prefix) {
                compose.onAllNodes(hasTagPrefix(tag)).fetchSemanticsNodes().isNotEmpty()
            } else {
                compose.onAllNodesWithTag(tag).fetchSemanticsNodes().isNotEmpty()
            }
        }
    }

    private fun hasTagPrefix(prefix: String) = androidx.compose.ui.test.SemanticsMatcher(
        "TestTag starts with '$prefix'",
    ) { node ->
        node.config.getOrNull(androidx.compose.ui.semantics.SemanticsProperties.TestTag)?.startsWith(prefix) == true
    }

    private fun singleTagWithPrefix(prefix: String): String {
        val nodes = compose.onAllNodes(hasTagPrefix(prefix), useUnmergedTree = true).fetchSemanticsNodes()
        assertEquals("Expected one inventory observation tag for $prefix", 1, nodes.size)
        return requireNotNull(
            nodes.single().config.getOrNull(androidx.compose.ui.semantics.SemanticsProperties.TestTag),
        )
    }

    private fun <Value> gatewaySuccess(result: GatewayResult<Value>): Value {
        assertTrue("Expected gateway success", result is GatewayResult.Success)
        return (result as GatewayResult.Success).value
    }

    private fun machineStripTag(credential: MachineCredential) =
        "machine-strip-${credential.machine.handle.encoded}"

    private fun stripLabelTag(credential: MachineCredential) =
        "machine-strip-label-${credential.machine.handle.encoded}"

    private fun filterTag(credential: MachineCredential) =
        "machine-filter-${credential.machine.handle.encoded}"

    private fun cardTagForMachine(credential: MachineCredential) =
        "agent-card-${credential.machine.handle.encoded}-"

    private fun cardTag(target: AgentTarget) =
        "agent-card-${target.machineHandle.encoded}-${target.session.id}"

    private fun cardPillTag(target: AgentTarget) =
        "agent-machine-pill-${target.machineHandle.encoded}-${target.session.id}"

    private fun killTag(target: AgentTarget) =
        "agent-kill-${target.machineHandle.encoded}-${target.session.id}"

    private fun forgeMachineTag(credential: MachineCredential) =
        "forge-machine-${credential.machine.handle.encoded}"

    private fun forgeProfileTag(credential: MachineCredential) =
        "forge-profile-${credential.machine.handle.encoded}"

    private fun inventoryReceivedPrefix(credential: MachineCredential) =
        "machine-inventory-received-${credential.machine.handle.encoded}-"

    private companion object {
        const val UI_OPT_IN = "skidbladnir.multiMachineUi"
        const val FAILED_MACHINE = "skidbladnir.failedMachine"
        const val DEVBOX_MACHINE = "skidbladnir.devboxMachine"
        const val MACBOOK_MACHINE = "skidbladnir.macbookMachine"
        const val OUTAGE_READY_FILE = "skidbladnir-multi-machine-outage-ready"
    }
}
