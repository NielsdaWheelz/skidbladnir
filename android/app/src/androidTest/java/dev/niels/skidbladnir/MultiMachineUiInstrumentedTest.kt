package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertTextContains
import androidx.compose.ui.test.filterToOne
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onChildren
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.swipe
import androidx.compose.ui.unit.DpRect
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.time.Instant
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
    fun theDashboardHeaderKeepsItsTitleWithoutACreateAffordanceOrMachineAdministration() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        Column {
                            DashboardTopBar(
                                summary = "4 tmux sessions across 2 machines",
                            )
                            UnreadableMachineStrip(
                                UnreadableStoredMachine(),
                            )
                        }
                    }
                }
            }
            compose.onNodeWithTag("dashboard-top-bar").assertIsDisplayed()
            // Read through the tag, not the word: "dashboard-title" survives this
            // delta (forge-seal.md, "Hard cut and cleanup") and this is its reader,
            // so the tag cannot rot into an unreferenced constant.
            compose.onNodeWithTag("dashboard-title", useUnmergedTree = true)
                .onChildren()
                .filterToOne(hasText("Dwarves"))
                .assertIsDisplayed()
            compose.onNodeWithText("Refresh").assertDoesNotExist()

            // The create action left the header for the Forge seal (forge-seal.md,
            // "Hard cut and cleanup"), which is anchored over the grid, not here.
            compose.onNodeWithTag("new-agent").assertDoesNotExist()
            compose.onNodeWithText("New dwarf").assertDoesNotExist()

            compose.onNodeWithText("Add machine").assertDoesNotExist()
            compose.onNodeWithText("Rename").assertDoesNotExist()
            compose.onNodeWithText("Remove machine").assertDoesNotExist()
            compose.onNodeWithText("Remove pairing").assertDoesNotExist()
            // The unreadable strip is NoticePanel's only title+body consumer, so it is the
            // only place the banner collapse can drop an optional title and leave a body
            // orphaned under no heading.
            compose.onNodeWithText("Unreadable pairing").assertIsDisplayed()
            compose.onNodeWithText("Reset the app data, then connect again.", substring = true)
                .assertIsDisplayed()
        }
    }

    @OptIn(ExperimentalMaterial3Api::class)
    @Test
    fun dwarfCollectionPullKeepsContentAndExposesOnlyActiveCheckingProgress() {
        val fixtures = listOf(
            "empty" to dashboard(
                InventoryState.Fresh(snapshot(emptyList())),
            ),
            "short" to dashboard(
                InventoryState.Fresh(snapshot(listOf(session(0)))),
            ),
            "populated" to dashboard(
                InventoryState.Fresh(snapshot(List(20, ::session))),
            ),
            "reading" to dashboard(InventoryState.Reading),
            "stale-retained" to dashboard(
                InventoryState.Stale(snapshot(listOf(session(0))), GatewayFailure.Transport),
            ),
            "unreachable" to dashboard(
                InventoryState.Unreachable(GatewayFailure.Transport),
            ),
            "auth-required" to dashboard(
                inventory = InventoryState.Unreachable(
                    GatewayFailure.Api(ApiErrorCode.Unauthenticated),
                ),
                access = MachineAccess.AuthRequired,
            ),
        )

        fixtures.forEach { (fixtureName, dashboard) ->
            val machine = dashboard.machines.single()
            val sessions = machine.inventory.lastSnapshot()?.inventory?.sessions.orEmpty()
            val live = machine.access == MachineAccess.Ready
            var refreshing by mutableStateOf(false)
            var dispatchCount = 0

            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                scenario.onActivity { activity ->
                    activity.setContent {
                        MaterialTheme {
                            DashboardDwarfCollection(
                                state = dashboard.copy(refreshing = refreshing),
                                onVerify = {
                                    dispatchCount += 1
                                    refreshing = !refreshing
                                },
                                onOpen = { error("pull opened a tmux session") },
                                onKill = { error("pull killed a tmux session") },
                            )
                        }
                    }
                }

                val retained = if (sessions.isEmpty()) {
                    compose.onNodeWithText(
                        if (machine.inventory is InventoryState.Fresh) {
                            "No tmux sessions"
                        } else {
                            "Sessions not current"
                        },
                    )
                } else {
                    compose.onNodeWithText(sessions.first().character.displayName, useUnmergedTree = true)
                }
                retained.assertIsDisplayed()
                val inventoryOutcome = when {
                    machine.access == MachineAccess.AuthRequired -> compose.onNodeWithText(
                        "Devbox: authentication required; its sessions may be out of date.",
                    ).assertIsDisplayed()
                    machine.inventory == InventoryState.Reading -> compose.onNodeWithText(
                        "Devbox: reading tmux sessions.",
                    ).assertIsDisplayed()
                    machine.inventory is InventoryState.Stale -> null
                    machine.inventory is InventoryState.Unreachable -> compose.onNodeWithText(
                        "Devbox: unavailable; its sessions cannot be read.",
                    ).assertIsDisplayed()
                    else -> null
                }
                compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                assertNoProgressSemantics()
                assertEquals("$fixtureName must begin without a verification", 0, dispatchCount)

                val retainedBoundsAtRest = retained.getUnclippedBoundsInRoot()
                val firstCard = sessions.firstOrNull()?.let {
                    compose.onNodeWithTag(cardTag(it)).assertIsDisplayed()
                }
                val firstCardBoundsAtRest = firstCard?.getUnclippedBoundsInRoot()
                val firstKill = sessions.firstOrNull()?.let {
                    compose.onNodeWithTag(killTag(it)).assertIsDisplayed()
                }
                val firstKillBoundsAtRest = firstKill?.getUnclippedBoundsInRoot()
                if (machine.inventory is InventoryState.Stale) {
                    requireNotNull(firstKill).assertIsNotEnabled()
                }
                val inventoryOutcomeBoundsAtRest = inventoryOutcome?.getUnclippedBoundsInRoot()

                if (fixtureName == "empty") {
                    pull(beyondThreshold = false)
                    retained.assertIsDisplayed()
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    assertEquals("$fixtureName below-threshold pull dispatched", 0, dispatchCount)
                }

                pull(beyondThreshold = true)
                retained.assertIsDisplayed()

                if (!live) {
                    val noLiveOutcome = requireNotNull(inventoryOutcome).assertIsDisplayed()
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    assertEquals("$fixtureName inert pull dispatched", 0, dispatchCount)
                    assertEquals(
                        "$fixtureName inert pull moved its access outcome",
                        retainedBoundsAtRest,
                        retained.getUnclippedBoundsInRoot(),
                    )
                    assertEquals(
                        "$fixtureName inert pull moved its access explanation",
                        requireNotNull(inventoryOutcomeBoundsAtRest),
                        noLiveOutcome.getUnclippedBoundsInRoot(),
                    )
                    return@use
                }

                val indicator = compose.onNodeWithContentDescription(CHECKING_SESSIONS)
                    .assertIsDisplayed()
                    .assertHasNoClickAction()
                assertEquals(
                    "accepted pull must expose indeterminate checking progress",
                    ProgressBarRangeInfo.Indeterminate,
                    indicator.fetchSemanticsNode().config.getOrNull(SemanticsProperties.ProgressBarRangeInfo),
                )
                compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(1)
                assertEquals("$fixtureName threshold pull did not dispatch once", 1, dispatchCount)
                val indicatorBounds = indicator.getUnclippedBoundsInRoot()
                assertEquals(
                    "$fixtureName checking moved retained content",
                    retainedBoundsAtRest,
                    retained.getUnclippedBoundsInRoot(),
                )
                assertNoOverlap("$fixtureName first-row text", indicatorBounds, retainedBoundsAtRest)
                firstCardBoundsAtRest?.let { bounds ->
                    assertEquals(
                        "$fixtureName checking moved the first card",
                        bounds,
                        requireNotNull(firstCard).getUnclippedBoundsInRoot(),
                    )
                    assertNoOverlap("$fixtureName first card", indicatorBounds, bounds)
                }
                firstKillBoundsAtRest?.let { bounds ->
                    assertEquals(
                        "$fixtureName checking moved the first Kill control",
                        bounds,
                        requireNotNull(firstKill).getUnclippedBoundsInRoot(),
                    )
                    assertNoOverlap("$fixtureName first-row Kill control", indicatorBounds, bounds)
                }
                if (machine.inventory is InventoryState.Stale) {
                    requireNotNull(firstKill).assertIsNotEnabled()
                }

                if (fixtureName == "short") {
                    pull(beyondThreshold = true)
                    retained.assertIsDisplayed()
                    compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(1)
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertIsDisplayed()
                    assertEquals("$fixtureName pull while checking dispatched again", 1, dispatchCount)
                    assertEquals(
                        "a pull while checking must not move the retained collection",
                        retainedBoundsAtRest,
                        retained.getUnclippedBoundsInRoot(),
                    )
                    assertEquals(
                        "a pull while checking must not create or move progress",
                        indicatorBounds,
                        indicator.getUnclippedBoundsInRoot(),
                    )
                }

                if (sessions.size > 1) {
                    compose.runOnIdle { refreshing = false }
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    val firstCardTag = cardTag(sessions.first())
                    compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(sessions.last())))
                    compose.onNodeWithTag(firstCardTag).assertDoesNotExist()
                    val dispatchesBeforeAwayPull = dispatchCount
                    compose.onNodeWithTag("agents-grid").performTouchInput {
                        swipe(
                            start = percentOffset(0.5f, 0.35f),
                            end = percentOffset(0.5f, 0.55f),
                            durationMillis = 1_000,
                        )
                    }
                    compose.onNodeWithTag(firstCardTag).assertDoesNotExist()
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    assertEquals(
                        "a pull that remained away from top dispatched",
                        dispatchesBeforeAwayPull,
                        dispatchCount,
                    )

                    val grid = compose.onNodeWithTag("agents-grid")
                    grid.performScrollToNode(hasTestTag(firstCardTag))
                    val firstAtTop = compose.onNodeWithTag(firstCardTag)
                        .assertIsDisplayed()
                        .getUnclippedBoundsInRoot()
                    val gridBounds = grid.getUnclippedBoundsInRoot()
                    val gridHeight = gridBounds.bottom - gridBounds.top
                    grid.performTouchInput {
                        swipe(
                            start = percentOffset(0.5f, 0.65f),
                            end = percentOffset(0.5f, 0.55f),
                            durationMillis = 1_000,
                        )
                    }
                    val firstAwayFromTop = compose.onNodeWithTag(firstCardTag)
                        .assertIsDisplayed()
                        .getUnclippedBoundsInRoot()
                    val measuredOffset = firstAtTop.top - firstAwayFromTop.top
                    assertTrue(
                        "the continuous pull precondition did not scroll away from the top",
                        measuredOffset > 0.dp,
                    )
                    assertTrue(
                        "the continuous pull cannot cover its measured offset plus the Material threshold",
                        gridHeight * 0.9f > measuredOffset + PullToRefreshDefaults.PositionalThreshold,
                    )
                    val dispatchesBeforeContinuousPull = dispatchCount
                    grid.performTouchInput {
                        swipe(
                            start = percentOffset(0.5f, 0.05f),
                            end = percentOffset(0.5f, 0.95f),
                            durationMillis = 1_000,
                        )
                    }
                    compose.onNodeWithTag(firstCardTag).assertIsDisplayed()
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS)
                        .assertIsDisplayed()
                        .assertHasNoClickAction()
                    compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(1)
                    assertEquals(
                        "one continuous away-to-top pull must dispatch once after crossing threshold",
                        dispatchesBeforeContinuousPull + 1,
                        dispatchCount,
                    )
                }
            }
        }
    }

    @Test
    fun dashboardChromeAndHorizontalFiltersStayOutsideTheCollectionPullOwner() {
        val recoveryDraft = ForgeDraft(
            machineHandle = TEST_MACHINE.handle,
            cwd = "/src",
            profile = TEST_PROFILE.key,
            optionalTmuxName = "",
            objective = "",
        )
        val staleMachine = MachineState(
            machine = TEST_MACHINE,
            access = MachineAccess.Ready,
            inventory = InventoryState.Stale(
                snapshot(listOf(session(0))),
                GatewayFailure.Transport,
            ),
            pressure = PressureState.Reading,
        )
        val unreachableMachine = MachineState(
            machine = OTHER_MACHINE,
            access = MachineAccess.Ready,
            inventory = InventoryState.Unreachable(GatewayFailure.Transport),
            pressure = PressureState.Reading,
        )
        var dashboard by mutableStateOf(
            SkidbladnirUiState.Dashboard(
                machines = listOf(staleMachine, unreachableMachine),
                selectedMachine = null,
                refreshing = false,
                forge = null,
                forgeRecovery = ForgeRecovery.RefreshRequired(recoveryDraft),
                kill = null,
            ),
        )
        var dispatchCount = 0
        var controller: SkidbladnirController? = null

        try {
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                scenario.onActivity { activity ->
                    val dashboardController = SkidbladnirController(activity.applicationContext)
                    controller = dashboardController
                    activity.setContent {
                        MaterialTheme {
                            DashboardMain(
                                state = dashboard,
                                controller = dashboardController,
                                onVerify = {
                                    dispatchCount += 1
                                    dashboard = dashboard.copy(refreshing = true)
                                },
                            )
                        }
                    }
                }

                compose.onNodeWithText(
                    "Devbox: Could not reach this machine over your Tailnet. " +
                        "Prior sessions are STALE; actions disabled. Pull down to check again.",
                ).assertIsDisplayed()
                compose.onNodeWithText(
                    "${OTHER_MACHINE.label.text}: Could not reach this machine over your Tailnet. " +
                        "Pull down to check again.",
                ).assertIsDisplayed()
                compose.onNodeWithText(
                    "Devbox: create outcome unknown. " +
                        "Pull down to check again before reviewing this draft.",
                ).assertIsDisplayed()

                val topBar = compose.onNodeWithTag("dashboard-top-bar")
                val filters = compose.onNodeWithTag("machine-filters")
                val firstFilter = compose.onNodeWithTag("machine-filter-all")
                val lastFilter = compose.onNodeWithTag("machine-filter-${OTHER_MACHINE.handle.encoded}")
                compose.onAllNodesWithTag("new-agent").assertCountEquals(1)
                val forgeSeal = compose.onNodeWithTag("new-agent")
                val topBarBounds = topBar.getUnclippedBoundsInRoot()
                val filterBounds = filters.getUnclippedBoundsInRoot()
                val forgeSealBounds = forgeSeal.getUnclippedBoundsInRoot()
                val firstFilterLeft = firstFilter.getUnclippedBoundsInRoot().left
                val lastFilterBounds = lastFilter.getUnclippedBoundsInRoot()
                assertTrue(
                    "the real filter fixture does not overflow its viewport: " +
                        "last=$lastFilterBounds, viewport=$filterBounds",
                    lastFilterBounds.right > filterBounds.right,
                )

                filters.performTouchInput {
                    swipe(
                        start = percentOffset(0.85f, 0.5f),
                        end = percentOffset(0.15f, 0.5f),
                        durationMillis = 500,
                    )
                }
                assertTrue(
                    "the real machine filters did not scroll horizontally",
                    firstFilter.getUnclippedBoundsInRoot().left < firstFilterLeft,
                )
                compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                assertNoProgressSemantics()
                assertEquals("horizontal filters dispatched collection verification", 0, dispatchCount)
                assertEquals("horizontal filters moved the fixed top bar", topBarBounds, topBar.getUnclippedBoundsInRoot())
                assertEquals("horizontal filters moved their viewport", filterBounds, filters.getUnclippedBoundsInRoot())

                pull(beyondThreshold = true)
                compose.onNodeWithContentDescription(CHECKING_SESSIONS)
                    .assertIsDisplayed()
                    .assertHasNoClickAction()
                compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(1)
                assertEquals("collection pull did not dispatch once", 1, dispatchCount)
                assertEquals("collection pull moved the fixed top bar", topBarBounds, topBar.getUnclippedBoundsInRoot())
                assertEquals("collection pull moved the fixed filters", filterBounds, filters.getUnclippedBoundsInRoot())
                assertEquals(
                    "collection pull moved the fixed Forge seal",
                    forgeSealBounds,
                    forgeSeal.getUnclippedBoundsInRoot(),
                )
            }
        } finally {
            controller?.close()
        }
    }

    @Test
    fun forgeRecoveryCopyNamesOnlyTheAvailableRepair() {
        val draft = ForgeDraft(
            machineHandle = TEST_MACHINE.handle,
            cwd = "/src",
            profile = TEST_PROFILE.key,
            optionalTmuxName = "",
            objective = "",
        )
        val refreshRequired = ForgeRecovery.RefreshRequired(draft)
        val ready = dashboard(InventoryState.Fresh(snapshot(emptyList())))
        val otherMachine = MachineState(
            machine = OTHER_MACHINE,
            access = MachineAccess.Ready,
            inventory = InventoryState.Reading,
            pressure = PressureState.Reading,
        )
        val cases = listOf(
            Triple(
                "all machines",
                ready,
                "Devbox: create outcome unknown. Pull down to check again before reviewing this draft.",
            ),
            Triple(
                "target selected",
                ready.copy(selectedMachine = TEST_MACHINE.handle),
                "Devbox: create outcome unknown. Pull down to check again before reviewing this draft.",
            ),
            Triple(
                "another machine selected",
                ready.copy(
                    machines = ready.machines + otherMachine,
                    selectedMachine = OTHER_MACHINE.handle,
                ),
                "Devbox: create outcome unknown. " +
                    "Select Devbox, then pull down to check again before reviewing this draft.",
            ),
            Triple(
                "fleet reconnect",
                dashboard(
                    inventory = InventoryState.Unreachable(
                        GatewayFailure.Api(ApiErrorCode.Unauthenticated),
                    ),
                    access = MachineAccess.AuthRequired,
                ),
                "Devbox: create outcome unknown. Reconnect fleet before reviewing this draft.",
            ),
            Triple(
                "identity reset",
                dashboard(
                    inventory = InventoryState.Unreachable(
                        GatewayFailure.Api(ApiErrorCode.MachineIdentityMismatch),
                    ),
                    access = MachineAccess.IdentityChanged,
                ),
                "Devbox: create outcome unknown. " +
                    "Fleet reset is required before reviewing this draft.",
            ),
            Triple(
                "missing target",
                ready.copy(machines = emptyList()),
                "Machine: create outcome unknown. " +
                    "Fleet reset is required before reviewing this draft.",
            ),
        )

        cases.forEach { (name, state, expected) ->
            assertEquals(name, expected, forgeRecoveryMessage(state, refreshRequired))
        }
        assertEquals(
            "review-ready copy",
            "Devbox refreshed. Review its sessions before resuming this draft.",
            forgeRecoveryMessage(ready, ForgeRecovery.ReviewReady(draft)),
        )
    }

    @Test
    fun machinePressureRestoresMetricsHistoryAndCapabilityDetails() {
        val current = PressureSample(
            sampledAt = Instant.parse("2026-08-26T12:00:00Z"),
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
                    sampledAt = Instant.parse("2026-08-26T11:59:55Z"),
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
                            inventoryStale = false,
                            supporting = null,
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
            tmuxName = "ga-durinn",
            identityToken = "v1-0123456789abcdef0123456789abcdef.100.200.1",
            character = CharacterSummary("norse.durinn", "Durinn"),
            attachedClients = 1,
            attention = false,
            status = SessionStatus(
                SessionStatusKind.Working,
                SessionStatusSignal.Lifecycle,
                Instant.parse("2026-08-26T11:59:55Z"),
            ),
        )
        val target = AgentTarget(handle, session)
        val stale = MachineState(
            machine = machine,
            access = MachineAccess.Ready,
            inventory = InventoryState.Stale(
                InventorySnapshot(
                    SessionsResponse(
                        MachineSummary(handle, MachinePlatform.Linux),
                        Instant.parse("2026-08-26T12:00:00Z"),
                        emptyList(),
                        listOf(session),
                    ),
                    receivedAtElapsedMillis = 1_000,
                ),
                GatewayFailure.Transport,
            ),
            pressure = PressureState.Reading,
        )
        val terminal = SkidbladnirUiState.Terminal(
            machine = stale,
            target = target,
            attempt = 1,
            connection = TerminalUiStatus.Connected(1, TerminalGeometry.Owner),
            kill = KillState(machine, target, pending = false),
        )

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        KillConfirmation(
                            state = requireNotNull(terminal.kill),
                            actionAdmissible = terminalActionAdmissible(
                                terminal.machine.canMutate,
                                terminal.connection,
                            ),
                            onDismiss = {},
                            onConfirm = {},
                        )
                    }
                }
            }
            compose.onNodeWithTag("kill-confirm").assertIsNotEnabled()
            compose.onNodeWithText("Cancel").assertIsEnabled()
            compose.onNodeWithText(
                "Devbox inventory is not fresh. Kill is disabled. " +
                    "Cancel, return to Dwarves, then pull down to check again.",
            ).assertIsDisplayed()
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
        val credentials = MachineStore(context, MachineStorage.production).read().credentials
        assertEquals(3, credentials.size)
        assertTrue(credentials.any { it.machine.handle == failedHandle })
        val failed = credentials.single { it.machine.handle == failedHandle }
        val healthy = credentials.filter { it.machine.handle != failedHandle }
        val healthyProbe = healthy.first()
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
                waitForTag(freshMachineTag(failed), 30_000)
                healthy.forEach { waitForTag(freshMachineTag(it), 30_000) }

                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(failedTarget)))
                compose.onNodeWithTag(killTag(failedTarget)).performClick()
                compose.onNodeWithText(
                    "Kill ${failedTarget.session.tmuxName} on ${failed.machine.label.text}?",
                ).assertIsDisplayed()
                compose.onNodeWithTag("kill-confirm").assertIsEnabled()
                assertTrue("Could not publish outage coordination marker", readiness.createNewFile())

                waitForTag("machine-state-stale-${failedHandle.encoded}", 30_000)
                val healthyAtOutage = requireNotNull(inventoryObservation(healthyProbe))
                compose.waitUntil(30_000) {
                    inventoryObservation(healthyProbe).let { it != null && it != healthyAtOutage }
                }
                healthy.forEach { waitForTag(freshMachineTag(it), 30_000) }
                compose.onNodeWithTag(stripLabelTag(failed), useUnmergedTree = true)
                    .assertTextContains(failed.machine.label.text.uppercase(Locale.ROOT), substring = true)
                healthy.forEach { credential ->
                    compose.onNodeWithTag(stripLabelTag(credential), useUnmergedTree = true)
                        .assertTextContains(credential.machine.label.text.uppercase(Locale.ROOT), substring = true)
                }
                compose.onNodeWithTag("kill-confirm").assertIsNotEnabled()
                compose.onNodeWithText("Cancel").assertIsEnabled().performClick()

                compose.onNodeWithTag("agents-grid").performScrollToNode(hasTestTag(cardTag(failedTarget)))
                compose.onNodeWithTag(killTag(failedTarget)).assertIsNotEnabled()

                compose.onNodeWithTag(filterTag(healthyProbe)).performClick()
                compose.onNodeWithTag("new-agent").assertIsEnabled()
                compose.onNodeWithTag(filterTag(failed)).performClick()
                compose.onNodeWithTag("new-agent").assertIsNotEnabled()
            }
        } finally {
            assertTrue("Could not clear outage coordination marker", !readiness.exists() || readiness.delete())
            client.closeAsync()
        }
    }

    private fun pull(beyondThreshold: Boolean) {
        compose.onNodeWithTag("agents-grid").performTouchInput {
            swipe(
                start = percentOffset(0.5f, 0.2f),
                end = percentOffset(0.5f, if (beyondThreshold) 0.9f else 0.3f),
                durationMillis = 300,
            )
        }
    }

    private fun dashboard(
        inventory: InventoryState,
        access: MachineAccess = MachineAccess.Ready,
    ): SkidbladnirUiState.Dashboard =
        SkidbladnirUiState.Dashboard(
            machines = listOf(
                MachineState(
                    machine = TEST_MACHINE,
                    access = access,
                    inventory = inventory,
                    pressure = PressureState.Reading,
                ),
            ),
            selectedMachine = null,
            refreshing = false,
            forge = null,
            forgeRecovery = null,
            kill = null,
        )

    private fun snapshot(sessions: List<AgentSession>) = InventorySnapshot(
        SessionsResponse(
            MachineSummary(TEST_MACHINE.handle, MachinePlatform.Linux),
            OBSERVED_AT,
            listOf(TEST_PROFILE),
            sessions,
        ),
        SystemClock.elapsedRealtime(),
    )

    private fun session(index: Int): AgentSession = AgentSession(
        id = "session-$index",
        tmuxName = "skidbladnir-work-$index",
        identityToken = "identity-$index",
        character = CharacterSummary("dwarf-$index", "Dwarf $index"),
        profile = TEST_PROFILE.key.encoded,
        attachedClients = 0,
        attention = false,
        status = SessionStatus(
            SessionStatusKind.Working,
            SessionStatusSignal.Lifecycle,
            SIGNAL_AT,
        ),
    )

    private fun cardTag(session: AgentSession) =
        "agent-card-${TEST_MACHINE.handle.encoded}-${session.id}"

    private fun killTag(session: AgentSession) =
        "agent-kill-${TEST_MACHINE.handle.encoded}-${session.id}"

    private fun assertNoOverlap(label: String, indicator: DpRect, content: DpRect) {
        assertTrue(
            "$label is obscured by checking progress: indicator=$indicator, content=$content",
            indicator.right <= content.left || content.right <= indicator.left ||
                indicator.bottom <= content.top || content.bottom <= indicator.top,
        )
    }

    private fun assertNoProgressSemantics() {
        compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(0)
    }

    private fun hasProgressSemantics() = androidx.compose.ui.test.SemanticsMatcher(
        "Has progress semantics",
    ) { node ->
        node.config.getOrNull(SemanticsProperties.ProgressBarRangeInfo) != null
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

    /** The machine strip's own record of when its freshest inventory arrived. */
    private fun inventoryObservation(credential: MachineCredential): Long? =
        compose.onAllNodesWithTag(freshMachineTag(credential), useUnmergedTree = true)
            .fetchSemanticsNodes()
            .singleOrNull()
            ?.config
            ?.getOrNull(MachineInventoryObservationKey)

    private fun <Value> gatewaySuccess(result: GatewayResult<Value>): Value {
        assertTrue("Expected gateway success", result is GatewayResult.Success)
        return (result as GatewayResult.Success).value
    }

    private fun freshMachineTag(credential: MachineCredential) =
        "machine-state-fresh-${credential.machine.handle.encoded}"

    private fun stripLabelTag(credential: MachineCredential) =
        "machine-strip-label-${credential.machine.handle.encoded}"

    private fun filterTag(credential: MachineCredential) =
        "machine-filter-${credential.machine.handle.encoded}"

    private fun cardTag(target: AgentTarget) =
        "agent-card-${target.machineHandle.encoded}-${target.session.id}"

    private fun killTag(target: AgentTarget) =
        "agent-kill-${target.machineHandle.encoded}-${target.session.id}"

    private companion object {
        const val CHECKING_SESSIONS = "Checking tmux sessions"
        const val FAILED_MACHINE = "skidbladnir.failedMachine"
        const val OUTAGE_READY_FILE = "skidbladnir-multi-machine-outage-ready"
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val SIGNAL_AT: Instant = Instant.parse("2026-08-26T11:59:55Z")
        val TEST_MACHINE = PairedMachine(
            requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")),
            requireNotNull(MachineLabel.parse("Devbox")),
            requireNotNull(MachineOrigin.parse("https://devbox.example.ts.net:8443")),
        )
        val OTHER_MACHINE = PairedMachine(
            requireNotNull(MachineHandle.parse("mh-fedcba9876543210fedcba9876543210")),
            requireNotNull(MachineLabel.parse("MacBook Pro Across The Far Tailnet Realm")),
            requireNotNull(MachineOrigin.parse("https://macbook.example.ts.net:8443")),
        )
        val TEST_PROFILE = ProfileChoice(requireNotNull(ProfileKey.parse("work")), "Codex · Work")
    }
}
