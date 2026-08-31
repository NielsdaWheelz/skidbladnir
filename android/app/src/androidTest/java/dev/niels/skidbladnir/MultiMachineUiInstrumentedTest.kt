package dev.niels.skidbladnir

import android.os.SystemClock
import android.view.KeyEvent
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.width
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assertContentDescriptionEquals
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertTextContains
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.filterToOne
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onChildren
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.swipe
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.DpRect
import androidx.compose.ui.unit.dp
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.time.Instant
import kotlin.math.absoluteValue
import kotlin.math.roundToInt
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
    fun dashboardHeaderKeepsTitleAndExposesWholeFleetReconnectWithoutCreateOrMachineAdministration() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    MaterialTheme {
                        Column {
                            DashboardTopBar(
                                summary = "4 tmux sessions across 2 machines",
                                onReconnect = {},
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
            compose.onNodeWithText("Reconnect fleet").assertIsDisplayed().assertIsEnabled()
            compose.onNodeWithText("Refresh").assertDoesNotExist()

            // The create action left the header for the Forge seal (forge-seal.md,
            // "Hard cut and cleanup"), which is anchored over the grid, not here.
            compose.onNodeWithTag("new-session").assertDoesNotExist()
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
            var density = 0f

            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                scenario.onActivity { activity ->
                    density = activity.resources.displayMetrics.density
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
                val grid = compose.onNodeWithTag("sessions-grid").assertIsDisplayed()
                val gridBoundsAtRest = grid.getUnclippedBoundsInRoot()
                if (fixtureName == "short") {
                    assertNoGoldInGridGutter(
                        label = "short resting collection",
                        grid = grid,
                        gridBounds = gridBoundsAtRest,
                        gutterHeight = 12.dp,
                        density = density,
                    )
                }
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

                val pullingIndicatorEvidence = if (fixtureName == "short") {
                    val cardBounds = requireNotNull(firstCardBoundsAtRest)
                    val gridBounds = gridBoundsAtRest
                    grid.performTouchInput {
                        down(percentOffset(0.5f, 0.2f))
                        moveTo(percentOffset(0.5f, 0.9f), delayMillis = 300)
                    }

                    compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true)
                        .assertCountEquals(1)
                    val pullingIndicator = compose.onNode(
                        hasProgressSemantics(),
                        useUnmergedTree = true,
                    ).assertIsDisplayed().assertHasNoClickAction()
                    val pullingConfig = pullingIndicator.fetchSemanticsNode().config
                    val pullingProgress = pullingConfig.getOrNull(
                        SemanticsProperties.ProgressBarRangeInfo,
                    )
                    val semanticsBounds = pullingIndicator.getUnclippedBoundsInRoot()
                    val goldBounds = goldBoundsInGridGutter(
                        grid = grid,
                        gridBounds = gridBounds,
                        gutterHeight = cardBounds.top - gridBounds.top,
                        density = density,
                    )
                    assertTrue(
                        "short held pull must expose determinate progress: " +
                            "progress=$pullingProgress grid=$gridBounds card=$cardBounds " +
                            "semantics=$semanticsBounds gold=$goldBounds",
                        pullingProgress != null && pullingProgress != ProgressBarRangeInfo.Indeterminate,
                    )
                    assertEquals(
                        "short held pull must not add a content description: " +
                            "grid=$gridBounds card=$cardBounds semantics=$semanticsBounds gold=$goldBounds",
                        null,
                        pullingConfig.getOrNull(SemanticsProperties.ContentDescription),
                    )
                    assertEquals(
                        "short held pull must not expose a role: " +
                            "grid=$gridBounds card=$cardBounds semantics=$semanticsBounds gold=$goldBounds",
                        null,
                        pullingConfig.getOrNull(SemanticsProperties.Role),
                    )
                    val pullingCustomActions = pullingConfig.getOrNull(SemanticsActions.CustomActions)
                    assertTrue(
                        "short held pull must not expose custom actions: " +
                            "actions=$pullingCustomActions grid=$gridBounds card=$cardBounds " +
                            "semantics=$semanticsBounds gold=$goldBounds",
                        pullingCustomActions.isNullOrEmpty(),
                    )
                    assertEquals(
                        "short held pull moved the first card: " +
                            "grid=$gridBounds cardAtRest=$cardBounds " +
                            "semantics=$semanticsBounds gold=$goldBounds",
                        cardBounds,
                        requireNotNull(firstCard).getUnclippedBoundsInRoot(),
                    )
                    assertEquals("short held pull dispatched before release", 0, dispatchCount)

                    grid.performTouchInput { up() }
                    semanticsBounds to goldBounds
                } else {
                    pull(beyondThreshold = true)
                    null
                }
                retained.assertIsDisplayed()

                if (!live) {
                    val noLiveOutcome = requireNotNull(inventoryOutcome).assertIsDisplayed()
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    assertEquals("$fixtureName inert pull dispatched", 0, dispatchCount)
                    if (fixtureName == "unreachable") {
                        assertNoGoldInGridGutter(
                            label = "unreachable inert collection after pull",
                            grid = grid,
                            gridBounds = gridBoundsAtRest,
                            gutterHeight = 12.dp,
                            density = density,
                        )
                    }
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
                val indicatorConfig = indicator.fetchSemanticsNode().config
                assertEquals(
                    "accepted pull must expose indeterminate checking progress",
                    ProgressBarRangeInfo.Indeterminate,
                    indicatorConfig.getOrNull(SemanticsProperties.ProgressBarRangeInfo),
                )
                assertEquals(
                    "$fixtureName checking progress must not expose a role",
                    null,
                    indicatorConfig.getOrNull(SemanticsProperties.Role),
                )
                val checkingCustomActions = indicatorConfig.getOrNull(SemanticsActions.CustomActions)
                assertTrue(
                    "$fixtureName checking progress must not expose custom actions: " +
                        "actions=$checkingCustomActions",
                    checkingCustomActions.isNullOrEmpty(),
                )
                compose.onAllNodes(hasProgressSemantics(), useUnmergedTree = true).assertCountEquals(1)
                assertEquals("$fixtureName threshold pull did not dispatch once", 1, dispatchCount)
                val indicatorSemanticsBounds = indicator.getUnclippedBoundsInRoot()
                val contentBounds = firstCardBoundsAtRest ?: retainedBoundsAtRest
                lateinit var indicatorGoldBounds: DpRect
                compose.waitUntil(
                    conditionDescription = "$fixtureName checking progress paints full-opacity Gold: " +
                        "grid=$gridBoundsAtRest content=$contentBounds " +
                        "semantics=$indicatorSemanticsBounds",
                    timeoutMillis = 1_000,
                ) {
                    goldBoundsInGridGutter(
                        grid = grid,
                        gridBounds = gridBoundsAtRest,
                        gutterHeight = contentBounds.top - gridBoundsAtRest.top,
                        density = density,
                    )?.let { bounds ->
                        indicatorGoldBounds = bounds
                        true
                    } ?: false
                }
                if (fixtureName == "short") {
                    val (pullingSemanticsBounds, pullingGoldBounds) =
                        requireNotNull(pullingIndicatorEvidence)
                    assertRefreshBoundaryBounds(
                        grid = gridBoundsAtRest,
                        card = requireNotNull(firstCardBoundsAtRest),
                        pullingSemantics = pullingSemanticsBounds,
                        pullingGold = pullingGoldBounds,
                        checkingSemantics = indicatorSemanticsBounds,
                        checkingGold = indicatorGoldBounds,
                    )
                }
                assertEquals(
                    "$fixtureName checking moved retained content",
                    retainedBoundsAtRest,
                    retained.getUnclippedBoundsInRoot(),
                )
                assertNoOverlap("$fixtureName first-row text", indicatorGoldBounds, retainedBoundsAtRest)
                firstCardBoundsAtRest?.let { bounds ->
                    assertEquals(
                        "$fixtureName checking moved the first card",
                        bounds,
                        requireNotNull(firstCard).getUnclippedBoundsInRoot(),
                    )
                    assertNoOverlap("$fixtureName first card", indicatorGoldBounds, bounds)
                }
                firstKillBoundsAtRest?.let { bounds ->
                    assertEquals(
                        "$fixtureName checking moved the first Kill control",
                        bounds,
                        requireNotNull(firstKill).getUnclippedBoundsInRoot(),
                    )
                    assertNoOverlap("$fixtureName first-row Kill control", indicatorGoldBounds, bounds)
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
                        indicatorSemanticsBounds,
                        indicator.getUnclippedBoundsInRoot(),
                    )
                }

                if (sessions.size > 1) {
                    compose.runOnIdle { refreshing = false }
                    compose.onNodeWithContentDescription(CHECKING_SESSIONS).assertDoesNotExist()
                    assertNoProgressSemantics()
                    val firstCardTag = cardTag(sessions.first())
                    compose.onNodeWithTag("sessions-grid").performScrollToNode(hasTestTag(cardTag(sessions.last())))
                    compose.onNodeWithTag(firstCardTag).assertDoesNotExist()
                    val dispatchesBeforeAwayPull = dispatchCount
                    compose.onNodeWithTag("sessions-grid").performTouchInput {
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

                    val grid = compose.onNodeWithTag("sessions-grid")
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
                compose.onAllNodesWithTag("new-session").assertCountEquals(1)
                val forgeSeal = compose.onNodeWithTag("new-session")
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
    fun machinePressureRailIsCompactAccessibleAndDisclosesOnlyItsMachine() {
        val pressureMachine = PairedMachine(
            OTHER_MACHINE.handle,
            requireNotNull(MachineLabel.parse("MacBook")),
            OTHER_MACHINE.origin,
        )
        val macCurrent = PressureSample(
            sampledAt = Instant.parse("2026-08-26T12:00:00Z"),
            level = PressureLevel.Hot,
            phase = PressurePhase.Recovering,
            reasons = listOf(PressureReason.Load),
            signals = listOf(
                PressureSignal.Measured(
                    PressureValue.CpuPercent(34.0),
                    PressureSignalState.Informational,
                ),
                PressureSignal.Measured(
                    PressureValue.NormalizedLoad(1.25),
                    PressureSignalState.Warm,
                ),
                PressureSignal.Missing(PressureMetric.SwapUsedPercent),
                PressureSignal.Measured(
                    PressureValue.DiskAvailablePercent(61.0),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.MemoryPressure(SystemMemoryPressure.Warning),
                    PressureSignalState.Warm,
                ),
            ),
        )
        val macResponse = PressureResponse(
            unsupported = listOf(
                PressureMetric.CpuPsiSomeAvg60Percent,
                PressureMetric.IoPsiFullAvg60Percent,
                PressureMetric.MemoryAvailablePercent,
                PressureMetric.MemoryPsiFullAvg60Percent,
            ),
            current = macCurrent,
            history = listOf(
                PressureHistorySample(Instant.parse("2026-08-26T11:59:40Z"), PressureLevel.Normal),
                PressureHistorySample(Instant.parse("2026-08-26T11:59:45Z"), PressureLevel.Warm),
                PressureHistorySample(Instant.parse("2026-08-26T11:59:50Z"), PressureLevel.Hot),
                PressureHistorySample(Instant.parse("2026-08-26T11:59:55Z"), PressureLevel.Unknown),
                PressureHistorySample(macCurrent.sampledAt, macCurrent.level),
            ),
        )
        val devCurrent = macCurrent.copy(
            level = PressureLevel.Normal,
            phase = PressurePhase.Steady,
            reasons = emptyList(),
            signals = listOf(
                PressureSignal.Measured(
                    PressureValue.CpuPercent(12.0),
                    PressureSignalState.Informational,
                ),
                PressureSignal.Measured(
                    PressureValue.NormalizedLoad(0.3),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.MemoryAvailablePercent(72.0),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.SwapUsedPercent(0.0),
                    PressureSignalState.Informational,
                ),
                PressureSignal.Measured(
                    PressureValue.DiskAvailablePercent(80.0),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.CpuPsiSomeAvg60Percent(0.0),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.MemoryPsiFullAvg60Percent(0.0),
                    PressureSignalState.Normal,
                ),
                PressureSignal.Measured(
                    PressureValue.IoPsiFullAvg60Percent(0.0),
                    PressureSignalState.Normal,
                ),
            ),
        )
        val devResponse = PressureResponse(
            unsupported = listOf(PressureMetric.MemoryPressure),
            current = devCurrent,
            history = listOf(PressureHistorySample(devCurrent.sampledAt, devCurrent.level)),
        )
        val placementSession = session(0)
        val placementTarget = SessionTarget(pressureMachine.handle, placementSession)
        val macMachine = MachineState(
            machine = pressureMachine,
            access = MachineAccess.Ready,
            inventory = InventoryState.Stale(
                InventorySnapshot(
                    SessionsResponse(
                        MachineSummary(pressureMachine.handle, MachinePlatform.Darwin),
                        macCurrent.sampledAt,
                        emptyList(),
                        emptyList(),
                    ),
                    1_000,
                ),
                GatewayFailure.Transport,
            ),
            pressure = PressureState.Fresh(macResponse),
        )
        var dashboard by mutableStateOf(
            SkidbladnirUiState.Dashboard(
                machines = listOf(
                    MachineState(
                        machine = TEST_MACHINE,
                        access = MachineAccess.Ready,
                        inventory = InventoryState.Fresh(snapshot(emptyList())),
                        pressure = PressureState.Fresh(devResponse),
                    ),
                    macMachine,
                ),
                selectedMachine = null,
                refreshing = false,
                forge = null,
                forgeRecovery = null,
                kill = null,
            ),
        )
        val macHandle = pressureMachine.handle.encoded
        val railTag = "machine-strip-$macHandle"
        val devRailTag = "machine-strip-${TEST_MACHINE.handle.encoded}"
        var largeText by mutableStateOf(false)
        var density = 0f
        var controller: SkidbladnirController? = null

        try {
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                scenario.onActivity { activity ->
                    density = activity.resources.displayMetrics.density
                    val dashboardController = SkidbladnirController(activity.applicationContext)
                    controller = dashboardController
                    activity.setContent {
                        CompositionLocalProvider(
                            LocalDensity provides Density(density, if (largeText) 2f else 1f),
                        ) {
                            MaterialTheme {
                                Box(
                                    Modifier
                                        .width(if (largeText) 320.dp else 360.dp)
                                        .fillMaxHeight(),
                                ) {
                                    DashboardMain(dashboard, dashboardController, onVerify = {})
                                }
                            }
                        }
                    }
                }

                compose.onNodeWithTag(railTag).assertDoesNotExist()
                compose.onNodeWithTag(devRailTag).assertDoesNotExist()
                compose.onNodeWithText("Prior sessions are STALE", substring = true)
                    .assertIsDisplayed()
                scenario.onActivity {
                    dashboard = dashboard.copy(selectedMachine = pressureMachine.handle)
                }
                val rail = compose.onNodeWithTag(railTag)
                rail.assertIsDisplayed().assertHasClickAction().assertContentDescriptionEquals(
                    "MacBook. Fresh pressure. Recovering from hot. Cause: load. " +
                        "CPU 34% i, informational; MEM WARNING W, warm; SWAP NO DATA ?, no data; " +
                        "LOAD 1.3 W, warm; DISK 61% N, normal. " +
                        "Trend: 2 earlier runs over 10 seconds; hot 5 seconds, " +
                        "then unknown 5 seconds, then hot 5 seconds.",
                )
                assertEquals(
                    "the pressure rail must expose one button role",
                    Role.Button,
                    rail.fetchSemanticsNode().config.getOrNull(SemanticsProperties.Role),
                )
                compose.onAllNodes(
                    hasAnyAncestor(hasTestTag(railTag)) and hasClickAction(),
                    useUnmergedTree = true,
                ).assertCountEquals(0)
                val railBounds = rail.getUnclippedBoundsInRoot()
                val height = railBounds.bottom - railBounds.top
                assertTrue(
                    "default 360dp rail must be 68–76dp, was $height",
                    height >= 68.dp - 0.001.dp && height <= 76.dp + 0.001.dp,
                )
                scenario.onActivity {
                    dashboard = dashboard.copy(
                        machines = dashboard.machines.map { machine ->
                            if (machine.machine.handle == pressureMachine.handle) {
                                machine.copy(
                                    inventory = InventoryState.Fresh(
                                        InventorySnapshot(
                                            SessionsResponse(
                                                MachineSummary(
                                                    pressureMachine.handle,
                                                    MachinePlatform.Darwin,
                                                ),
                                                macCurrent.sampledAt,
                                                listOf(TEST_PROFILE),
                                                listOf(placementSession),
                                            ),
                                            SystemClock.elapsedRealtime(),
                                        ),
                                    ),
                                )
                            } else {
                                machine
                            }
                        },
                    )
                }
                compose.onNodeWithText("Prior sessions are STALE", substring = true)
                    .assertDoesNotExist()
                val placementCard = compose.onNodeWithTag(cardTag(placementTarget))
                    .assertIsDisplayed()
                assertRailToCardGap(
                    size = "360dp / 1.0x",
                    rail = rail.getUnclippedBoundsInRoot(),
                    card = placementCard.getUnclippedBoundsInRoot(),
                )
                val header = compose.onNodeWithText(
                    "MacBook RECOVERING FROM HOT · LOAD",
                    useUnmergedTree = true,
                ).assertIsDisplayed()
                val metricGroups = listOf(
                    "CPU 34% i",
                    "MEM WARNING W",
                    "SWAP NO DATA ?",
                    "LOAD 1.3 W",
                    "DISK 61% N",
                )
                metricGroups.forEach { group ->
                    compose.onNodeWithText(group, useUnmergedTree = true).assertHasNoClickAction()
                }

                val headerPixels = header.captureToImage().toPixelMap()
                var lastBoneColumn: Int? = null
                var firstEmberColumn: Int? = null
                for (x in 0 until headerPixels.width) {
                    for (y in 0 until headerPixels.height) {
                        when (headerPixels[x, y].toArgb()) {
                            Bone.toArgb() -> lastBoneColumn = x
                            Ember.toArgb() -> if (firstEmberColumn == null) firstEmberColumn = x
                        }
                    }
                }
                assertTrue(
                    "header must render neutral machine text before the hot status accent; " +
                        "lastBone=$lastBoneColumn firstEmber=$firstEmberColumn",
                    lastBoneColumn != null && firstEmberColumn != null &&
                        lastBoneColumn < firstEmberColumn,
                )

                // Visible text owns metric identity. This one row tag exists only for the
                // rendered no-container proof, which has no semantic query.
                val metricPixels = compose.onNodeWithTag(
                    "pressure-metrics-$macHandle",
                    useUnmergedTree = true,
                ).captureToImage().toPixelMap()
                val quietAccentColors = setOf(Frost.toArgb(), Moss.toArgb())
                var backgroundPixels = 0
                var bonePixels = 0
                var mutedPixels = 0
                var goldPixels = 0
                var quietAccentPixels = 0
                for (x in 0 until metricPixels.width) {
                    for (y in 0 until metricPixels.height) {
                        val color = metricPixels[x, y].toArgb()
                        if (color == RaisedSurface.toArgb()) backgroundPixels += 1
                        if (color == Bone.toArgb()) bonePixels += 1
                        if (color == Muted.toArgb()) mutedPixels += 1
                        if (color == Gold.toArgb()) goldPixels += 1
                        if (color in quietAccentColors) quietAccentPixels += 1
                    }
                }
                val metricArea = metricPixels.width * metricPixels.height
                assertTrue(
                    "metric row capture must contain pixels, was ${metricPixels.width}x${metricPixels.height}",
                    metricArea > 0,
                )
                assertTrue(
                    "flat metric text must leave at least two thirds RaisedSurface around glyphs; " +
                        "background=$backgroundPixels/$metricArea",
                    backgroundPixels * 3 >= metricArea * 2,
                )
                assertTrue(
                    "metric row must render Bone/Muted/Gold ink without Frost/Moss; " +
                        "bone=$bonePixels muted=$mutedPixels gold=$goldPixels " +
                        "frostOrMoss=$quietAccentPixels",
                    bonePixels > 0 && mutedPixels > 0 && goldPixels > 0 && quietAccentPixels == 0,
                )
                compose.onNodeWithText("Recent pressure history", substring = true, useUnmergedTree = true)
                    .assertDoesNotExist()
                compose.onNodeWithText("up to 15 min", substring = true, useUnmergedTree = true)
                    .assertDoesNotExist()
                compose.onNodeWithText("Unsupported:", substring = true, useUnmergedTree = true)
                    .assertDoesNotExist()
                compose.onNodeWithText("Missing:", substring = true, useUnmergedTree = true)
                    .assertDoesNotExist()
                compose.onNodeWithText("Pressure:", substring = true, useUnmergedTree = true)
                    .assertDoesNotExist()
                compose.onNodeWithContentDescription("Recent pressure history", substring = true)
                    .assertDoesNotExist()
                compose.onNodeWithContentDescription("Unsupported", substring = true).assertDoesNotExist()

                val history = compose.onNodeWithTag(
                    "pressure-history-band-$macHandle",
                    useUnmergedTree = true,
                )
                val historyBoundsAtDefault = history.getUnclippedBoundsInRoot()
                assertEquals(
                    "history height changed",
                    16.dp,
                    historyBoundsAtDefault.bottom - historyBoundsAtDefault.top,
                )
                val pixels = history.captureToImage().toPixelMap()
                val topPadding = (5f * density).roundToInt()
                val drawHeight = pixels.height - topPadding
                val levels = listOf(
                    PressureLevel.Normal to Moss,
                    PressureLevel.Warm to Gold,
                    PressureLevel.Hot to Ember,
                    PressureLevel.Unknown to Muted,
                    PressureLevel.Hot to Ember,
                )
                val proportions = listOf(0.25f, 0.58f, 1f, 0.42f, 1f)
                val barWidth = pixels.width.toFloat() / levels.size
                levels.forEachIndexed { index, (_, color) ->
                    val x = ((index + 0.5f) * barWidth).toInt()
                    val firstColorRow = (0 until pixels.height).first {
                        pixels[x, it].toArgb() == color.toArgb()
                    }
                    val expectedTop = topPadding + drawHeight * (1f - proportions[index])
                    assertTrue(
                        "history level ${levels[index].first} changed height: " +
                            "expected top $expectedTop, actual $firstColorRow",
                        (firstColorRow - expectedTop).absoluteValue <= 1f,
                    )
                    if (index < levels.lastIndex) {
                        val gapX = (((index + 1) * barWidth) - 0.25f).toInt()
                        val gap = pixels[gapX, pixels.height - 1]
                        val nextColor = levels[index + 1].second
                        assertTrue(
                            "history sample gap lost its background at index $index: " +
                                "gap=$gap left=$color right=$nextColor",
                            gap.toArgb() != color.toArgb() &&
                                gap.toArgb() != nextColor.toArgb() &&
                                (
                                    gap.red < minOf(color.red, nextColor.red) ||
                                        gap.green < minOf(color.green, nextColor.green) ||
                                        gap.blue < minOf(color.blue, nextColor.blue)
                                ),
                        )
                    }
                }

                InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_TAB)
                compose.waitForIdle()
                rail.performSemanticsAction(androidx.compose.ui.semantics.SemanticsActions.RequestFocus)
                rail.assertIsFocused().performClick()
                compose.onNodeWithText("MacBook pressure").assertIsDisplayed()
                compose.onNodeWithText("Devbox pressure").assertDoesNotExist()
                compose.onNodeWithText("system memory pressure").assertIsDisplayed()
                compose.onNodeWithText("WARNING").assertIsDisplayed()
                compose.onNodeWithText("NO DATA").assertIsDisplayed()
                compose.onNodeWithText("Unsupported", substring = true).assertDoesNotExist()
                compose.onAllNodesWithTag(
                    "pressure-history-band-$macHandle",
                    useUnmergedTree = true,
                ).assertCountEquals(1)
                repeat(2) {
                    compose.onNodeWithTag("pressure-details-sheet-$macHandle").performTouchInput {
                        swipe(
                            start = percentOffset(0.5f, 0.85f),
                            end = percentOffset(0.5f, 0.15f),
                            durationMillis = 500,
                        )
                    }
                }
                compose.onNodeWithContentDescription("Dismiss MacBook pressure details")
                    .assertIsDisplayed()
                    .performClick()
                compose.onNodeWithText("MacBook pressure").assertDoesNotExist()
                rail.assertIsDisplayed().assertIsFocused()

                scenario.onActivity { largeText = true }
                val largeRail = compose.onNodeWithTag(railTag)
                val largeBounds = largeRail.getUnclippedBoundsInRoot()
                val largeCardBounds = compose.onNodeWithTag(cardTag(placementTarget))
                    .assertIsDisplayed()
                    .getUnclippedBoundsInRoot()
                assertRailToCardGap(
                    size = "320dp / 2.0x",
                    rail = largeBounds,
                    card = largeCardBounds,
                )
                val headerBounds = compose.onNodeWithTag(
                    "machine-strip-label-$macHandle",
                    useUnmergedTree = true,
                ).getUnclippedBoundsInRoot()
                val historyBounds = compose.onNodeWithTag(
                    "pressure-history-band-$macHandle",
                    useUnmergedTree = true,
                ).getUnclippedBoundsInRoot()
                assertTrue(
                    "320dp large-text header clipped outside rail: rail=$largeBounds header=$headerBounds",
                    headerBounds.top >= largeBounds.top && headerBounds.bottom <= largeBounds.bottom,
                )
                assertTrue(
                    "320dp large-text history clipped outside rail: rail=$largeBounds history=$historyBounds",
                    historyBounds.top >= largeBounds.top && historyBounds.bottom <= largeBounds.bottom,
                )
                compose.onNodeWithText("DISK 61% N", useUnmergedTree = true)
                    .performScrollTo()
                    .assertIsDisplayed()
                    .assertHasNoClickAction()

                largeRail.performClick()
                compose.onNodeWithText("MacBook pressure").assertIsDisplayed()
                InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(KeyEvent.KEYCODE_BACK)
                compose.waitUntil(10_000) {
                    compose.onAllNodes(hasText("MacBook pressure")).fetchSemanticsNodes().isEmpty()
                }
                compose.onNodeWithText("MacBook pressure").assertDoesNotExist()
                largeRail.assertIsDisplayed().assertIsFocused()

                largeRail.performClick()
                compose.onNodeWithText("MacBook pressure").assertIsDisplayed()
                val machines = dashboard.machines
                scenario.onActivity {
                    dashboard = dashboard.copy(
                        machines = dashboard.machines.filterNot { machine ->
                            machine.machine.handle == pressureMachine.handle
                        },
                    )
                }
                compose.waitUntil(10_000) {
                    compose.onAllNodes(hasText("MacBook pressure")).fetchSemanticsNodes().isEmpty()
                }
                scenario.onActivity { dashboard = dashboard.copy(machines = machines) }
                compose.onNodeWithTag(railTag).assertIsDisplayed()
                compose.onNodeWithText("MacBook pressure").assertDoesNotExist()
                scenario.onActivity { dashboard = dashboard.copy(selectedMachine = null) }
                compose.onNodeWithTag(railTag).assertDoesNotExist()
                compose.onNodeWithTag(devRailTag).assertDoesNotExist()
            }
        } finally {
            controller?.close()
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
        val session = TmuxSession(
            tmuxId = "${'$'}1",
            tmuxName = "ga-durinn",
            identityToken = "v1-0123456789abcdef0123456789abcdef.100.200.1",
            character = CharacterSummary("norse.durinn", "Durinn"),
            attachedClients = 1,
            activity = SessionActivity.Active,
            agent = AgentRuntime(AgentProvider.Codex, pid = 1234),
        )
        val target = SessionTarget(handle, session)
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
        val failedTarget = SessionTarget(
            failedHandle,
            requireNotNull(gatewaySuccess(client.listSessions(failed)).sessions.firstOrNull()) {
                "Outage journey requires one pre-existing session on ${failed.machine.handle.encoded}"
            },
        )

        try {
            ActivityScenario.launch(MainActivity::class.java).use {
                waitForInventoryObservation(failed, 30_000)
                healthy.forEach { waitForInventoryObservation(it, 30_000) }

                compose.onNodeWithTag("sessions-grid").performScrollToNode(hasTestTag(cardTag(failedTarget)))
                compose.onNodeWithTag(killTag(failedTarget)).performClick()
                compose.onNodeWithText(
                    "Kill ${failedTarget.session.tmuxName} on ${failed.machine.label.text}?",
                ).assertIsDisplayed()
                compose.onNodeWithTag("kill-confirm").assertIsEnabled()
                assertTrue("Could not publish outage coordination marker", readiness.createNewFile())

                waitForDisabledTag("kill-confirm", 30_000)
                val healthyAtOutage = requireNotNull(inventoryObservation(healthyProbe))
                compose.waitUntil(30_000) {
                    inventoryObservation(healthyProbe).let { it != null && it != healthyAtOutage }
                }
                healthy.forEach { waitForInventoryObservation(it, 30_000) }
                compose.onNodeWithTag(filterTag(failed), useUnmergedTree = true)
                    .assertTextContains(failed.machine.label.text, substring = true)
                healthy.forEach { credential ->
                    compose.onNodeWithTag(filterTag(credential), useUnmergedTree = true)
                        .assertTextContains(credential.machine.label.text, substring = true)
                }
                credentials.forEach { credential ->
                    compose.onNodeWithTag(pressureRailTag(credential)).assertDoesNotExist()
                }
                compose.onNodeWithTag("kill-confirm").assertIsNotEnabled()
                compose.onNodeWithText("Cancel").assertIsEnabled().performClick()

                compose.onNodeWithTag("sessions-grid").performScrollToNode(hasTestTag(cardTag(failedTarget)))
                compose.onNodeWithTag(killTag(failedTarget)).assertIsNotEnabled()

                compose.onNodeWithTag(filterTag(healthyProbe)).performClick()
                compose.onNodeWithTag(pressureRailTag(healthyProbe)).assertIsDisplayed()
                compose.onNodeWithTag("new-session").assertIsEnabled()
                compose.onNodeWithTag(filterTag(failed)).performClick()
                compose.onNodeWithTag(pressureRailTag(failed)).assertIsDisplayed()
                compose.onNodeWithTag("new-session").assertIsNotEnabled()
            }
        } finally {
            assertTrue("Could not clear outage coordination marker", !readiness.exists() || readiness.delete())
            client.closeAsync()
        }
    }

    private fun pull(beyondThreshold: Boolean) {
        compose.onNodeWithTag("sessions-grid").performTouchInput {
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

    private fun snapshot(sessions: List<TmuxSession>) = InventorySnapshot(
        SessionsResponse(
            MachineSummary(TEST_MACHINE.handle, MachinePlatform.Linux),
            OBSERVED_AT,
            listOf(TEST_PROFILE),
            sessions,
        ),
        SystemClock.elapsedRealtime(),
    )

    private fun session(index: Int): TmuxSession = TmuxSession(
        tmuxId = "session-$index",
        tmuxName = "skidbladnir-work-$index",
        identityToken = "identity-$index",
        character = CharacterSummary("dwarf-$index", "Dwarf $index"),
        launchProfile = TEST_PROFILE.key,
        attachedClients = 0,
        activity = SessionActivity.Active,
        agent = AgentRuntime(AgentProvider.Codex, pid = 1234, profile = TEST_PROFILE.key),
    )

    private fun cardTag(session: TmuxSession) =
        "session-card-${TEST_MACHINE.handle.encoded}-${session.tmuxId}"

    private fun killTag(session: TmuxSession) =
        "session-kill-${TEST_MACHINE.handle.encoded}-${session.tmuxId}"

    private fun assertNoOverlap(label: String, indicator: DpRect, content: DpRect) {
        assertTrue(
            "$label is obscured by checking progress: indicator=$indicator, content=$content",
            indicator.right <= content.left || content.right <= indicator.left ||
                indicator.bottom <= content.top || content.bottom <= indicator.top,
        )
    }

    private fun assertRefreshBoundaryBounds(
        grid: DpRect,
        card: DpRect,
        pullingSemantics: DpRect,
        pullingGold: DpRect?,
        checkingSemantics: DpRect,
        checkingGold: DpRect,
    ) {
        val expectedIndicator = DpRect(
            left = grid.left + 12.dp,
            top = grid.top,
            right = grid.right - 12.dp,
            bottom = grid.top + 2.dp,
        )
        val failures = mutableListOf<String>()
        fun compare(label: String, expected: Dp, actual: Dp) {
            val delta = (actual - expected).value.absoluteValue
            if (delta > 1f) failures += "$label expected=$expected actual=$actual delta=${delta}dp"
        }

        compare("resting card top", grid.top + 12.dp, card.top)
        listOf(
            "pulling" to pullingSemantics,
            "checking" to checkingSemantics,
        ).forEach { (phase, semantics) ->
            compare("$phase semantics left", expectedIndicator.left, semantics.left)
            compare("$phase semantics right", expectedIndicator.right, semantics.right)
        }
        if (pullingGold == null) {
            failures += "pulling painted no full-opacity Gold in the pre-card gutter"
        } else {
            compare("pulling Gold left", expectedIndicator.left, pullingGold.left)
            compare("pulling Gold top", expectedIndicator.top, pullingGold.top)
            compare(
                "pulling Gold height",
                expectedIndicator.bottom - expectedIndicator.top,
                pullingGold.bottom - pullingGold.top,
            )
            if (pullingGold.right > expectedIndicator.right + 1.dp) {
                failures += "pulling Gold escaped horizontal band expected=$expectedIndicator actual=$pullingGold"
            }
        }
        compare("checking Gold top", expectedIndicator.top, checkingGold.top)
        compare(
            "checking Gold height",
            expectedIndicator.bottom - expectedIndicator.top,
            checkingGold.bottom - checkingGold.top,
        )
        if (checkingGold.left < expectedIndicator.left - 1.dp ||
            checkingGold.right > expectedIndicator.right + 1.dp
        ) {
            failures += "checking Gold escaped horizontal band expected=$expectedIndicator actual=$checkingGold"
        }
        assertTrue(
            "refresh boundary geometry mismatch within 1dp: failures=$failures; " +
                "expectedIndicator=$expectedIndicator grid=$grid card=$card " +
                "pullingSemantics=$pullingSemantics pullingGold=$pullingGold " +
                "checkingSemantics=$checkingSemantics checkingGold=$checkingGold",
            failures.isEmpty(),
        )
    }

    private fun assertNoGoldInGridGutter(
        label: String,
        grid: SemanticsNodeInteraction,
        gridBounds: DpRect,
        gutterHeight: Dp,
        density: Float,
    ) {
        val goldBounds = goldBoundsInGridGutter(grid, gridBounds, gutterHeight, density)
        assertEquals(
            "$label must have no full-opacity Gold progress pixels: " +
                "grid=$gridBounds gutterHeight=$gutterHeight gold=$goldBounds",
            null,
            goldBounds,
        )
    }

    private fun goldBoundsInGridGutter(
        grid: SemanticsNodeInteraction,
        gridBounds: DpRect,
        gutterHeight: Dp,
        density: Float,
    ): DpRect? {
        val pixels = grid.captureToImage().toPixelMap()
        val bottom = (gutterHeight.value * density).roundToInt().coerceIn(0, pixels.height)
        val gold = Gold.toArgb()
        var left = pixels.width
        var top = bottom
        var right = -1
        var lastRow = -1
        for (x in 0 until pixels.width) {
            for (y in 0 until bottom) {
                if (pixels[x, y].toArgb() == gold) {
                    left = minOf(left, x)
                    top = minOf(top, y)
                    right = maxOf(right, x)
                    lastRow = maxOf(lastRow, y)
                }
            }
        }
        if (right < 0) return null
        return DpRect(
            left = gridBounds.left + (left / density).dp,
            top = gridBounds.top + (top / density).dp,
            right = gridBounds.left + ((right + 1) / density).dp,
            bottom = gridBounds.top + ((lastRow + 1) / density).dp,
        )
    }

    private fun assertRailToCardGap(size: String, rail: DpRect, card: DpRect) {
        val gap = card.top - rail.bottom
        assertTrue(
            "$size selected ready/fresh one-card rail-to-card gap must be 16dp +/- 1dp: " +
                "actual=$gap rail=$rail card=$card",
            (gap - 16.dp).value.absoluteValue <= 1f,
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

    private fun waitForInventoryObservation(credential: MachineCredential, timeoutMillis: Long) {
        compose.waitUntil(timeoutMillis) { inventoryObservation(credential) != null }
    }

    private fun waitForDisabledTag(tag: String, timeoutMillis: Long) {
        compose.waitUntil(timeoutMillis) {
            compose.onAllNodesWithTag(tag).fetchSemanticsNodes().singleOrNull()
                ?.config
                ?.getOrNull(SemanticsProperties.Disabled) != null
        }
    }

    /** The machine filter's own record of when its freshest inventory arrived. */
    private fun inventoryObservation(credential: MachineCredential): Long? =
        compose.onAllNodesWithTag(filterTag(credential), useUnmergedTree = true)
            .fetchSemanticsNodes()
            .singleOrNull()
            ?.config
            ?.getOrNull(MachineInventoryObservationKey)

    private fun <Value> gatewaySuccess(result: GatewayResult<Value>): Value {
        assertTrue("Expected gateway success", result is GatewayResult.Success)
        return (result as GatewayResult.Success).value
    }

    private fun filterTag(credential: MachineCredential) =
        "machine-filter-${credential.machine.handle.encoded}"

    private fun pressureRailTag(credential: MachineCredential) =
        "machine-strip-${credential.machine.handle.encoded}"

    private fun cardTag(target: SessionTarget) =
        "session-card-${target.machineHandle.encoded}-${target.session.tmuxId}"

    private fun killTag(target: SessionTarget) =
        "session-kill-${target.machineHandle.encoded}-${target.session.tmuxId}"

    private companion object {
        const val CHECKING_SESSIONS = "Checking tmux sessions"
        const val FAILED_MACHINE = "skidbladnir.failedMachine"
        const val OUTAGE_READY_FILE = "skidbladnir-multi-machine-outage-ready"
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
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
        val TEST_PROFILE = ProfileChoice(
            requireNotNull(ProfileKey.parse("work")),
            "Codex · Work",
            AgentProvider.Codex,
        )
    }
}
