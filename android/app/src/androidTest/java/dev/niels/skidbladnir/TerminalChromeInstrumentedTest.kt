package dev.niels.skidbladnir

import android.content.Context
import android.os.SystemClock
import android.view.View
import android.view.ViewGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.PixelMap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assertContentDescriptionEquals
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import java.security.KeyStore
import java.time.Instant
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TerminalChromeInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun terminalChromeRendersHeaderTextSizeRecoveryAndRenameOverTheProductionScreen() {
        compose.mainClock.autoAdvance = false
        val session = TmuxSession(
            tmuxId = "session-9",
            tmuxName = "skidbladnir-personal-9",
            identityToken = "identity-9",
            character = CharacterSummary("dwarf-9", "Dwarf 9"),
            launchProfile = requireNotNull(ProfileKey.parse("personal")),
            attachedClients = 1,
            activity = SessionActivity.Active,
            agent = AgentRuntime(AgentProvider.Codex, pid = 1234),
        )
        val machine = PairedMachine(
            handle = MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")!!,
            label = MachineLabel.parse("Devbox")!!,
            origin = MachineOrigin.parse("https://devbox.example:8443/")!!,
        )
        val machineState = MachineState(
            machine = machine,
            access = MachineAccess.Ready,
            inventory = InventoryState.Fresh(
                InventorySnapshot(
                    inventory = SessionsResponse(
                        machine = MachineSummary(machine.handle, MachinePlatform.Linux),
                        observedAt = Instant.parse("2026-08-27T12:00:00Z"),
                        profiles = emptyList(),
                        sessions = listOf(session),
                    ),
                    receivedAtElapsedMillis = SystemClock.elapsedRealtime(),
                ),
            ),
            pressure = PressureState.Reading,
        )
        val controller = SkidbladnirController(
            InstrumentationRegistry.getInstrumentation().targetContext.applicationContext,
            DashboardEntryState(),
        )
        val target = SessionTarget(machine.handle, session)
        val connected = TerminalUiStatus.Connected(1)
        val connectedActionsAdmissible = terminalActionAdmissible(machineState.canMutate, connected)
        var terminalState by mutableStateOf(
            SkidbladnirUiState.Terminal(
                machine = machineState,
                target = target,
                attempt = 1,
                connection = TerminalUiStatus.Verifying,
                viewport = TerminalViewport.Pending,
                textSize = TerminalTextSizeState.Reading,
                kill = null,
            ),
        )
        var showRenameHarness by mutableStateOf(false)
        var renameState by mutableStateOf<RenameState?>(null)
        var headerFontScale by mutableStateOf(1f)
        var hostView: View? = null
        val identity = "Rename ${session.tmuxName} on ${machine.label.text}"

        try {
            compose.setContent {
                MaterialTheme {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = Ink,
                        contentColor = Bone,
                    ) {
                        hostView = LocalView.current
                        if (!showRenameHarness) {
                            CompositionLocalProvider(
                                LocalDensity provides Density(LocalDensity.current.density, headerFontScale),
                            ) {
                                TerminalScreen(
                                    state = terminalState,
                                    controller = controller,
                                    onDetach = {},
                                )
                            }
                        } else {
                            Column(modifier = Modifier.fillMaxSize()) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 12.dp, vertical = 8.dp),
                                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    HeaderChip(
                                        label = "Detach",
                                        spokenName = null,
                                        enabled = true,
                                        onClick = {},
                                    )
                                    TerminalRenameControl(
                                        machine = machine,
                                        target = target,
                                        presence = "1 client",
                                        presenceColor = Moss,
                                        enabled = connectedActionsAdmissible,
                                        onClick = { renameState = beginRename(target) },
                                        modifier = Modifier.weight(1f),
                                    )
                                    KillButton(
                                        machineLabel = machine.label,
                                        target = target,
                                        enabled = true,
                                        onClick = {},
                                    )
                                }
                            }
                            renameState?.let { current ->
                                SessionRenameSheet(
                                    machine = machine,
                                    terminalTarget = target,
                                    state = current,
                                    terminalActionsAdmissible = connectedActionsAdmissible,
                                    onDraftChange = { renameState = updateRenameDraft(current, it) },
                                    onDismiss = { renameState = dismissRename(current) },
                                    onSubmit = {
                                        renameState = beginRenameSending(
                                            current,
                                            target,
                                            terminalActionsAdmissible = connectedActionsAdmissible,
                                        ) ?: current
                                    },
                                )
                            }
                        }
                    }
                }
            }
            compose.mainClock.advanceTimeByFrame()

            val detachNodes = compose.onAllNodesWithText("Detach").fetchSemanticsNodes()
            assertEquals(
                "the terminal header must expose exactly one control whose complete visible " +
                    "label is \"Detach\"; matching semantics nodes=$detachNodes",
                1,
                detachNodes.size,
            )
            val detach = compose.onNodeWithText("Detach")
            detach.assertIsDisplayed().assertHasClickAction()
            val detachNode = detach.fetchSemanticsNode()
            val semantics = detachNode.config
            assertEquals(
                "the visible Detach text must be the control's sole spoken content; " +
                    "rendered text=${semantics.getOrNull(SemanticsProperties.Text)?.map { it.text }}",
                listOf("Detach"),
                semantics.getOrNull(SemanticsProperties.Text)?.map { it.text },
            )
            assertTrue(
                "Detach must not replace its visible label with custom accessibility prose; " +
                    "contentDescription=${semantics.getOrNull(SemanticsProperties.ContentDescription)}",
                semantics.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty(),
            )
            assertEquals(
                "Detach must carry Role.Button so its literal label is announced as an action; " +
                    "semantics=$semantics",
                Role.Button,
                semantics.getOrNull(SemanticsProperties.Role),
            )

            val pixels = detach.captureToImage().toPixelMap()
            val outer = pixels.corners(with(compose.density) { 1.dp.roundToPx() })
            val inner = pixels.corners(with(compose.density) { 3.dp.roundToPx() })
            val ground = Ink.toArgb()
            assertTrue(
                "Detach must render the same 4dp cut at all four corners: a point 1dp in must " +
                    "remain bare Ink and a point 3dp in must be material at every corner. " +
                    "outer=$outer, inner=$inner, ground=${Integer.toHexString(ground)}, " +
                    "image=${pixels.width}x${pixels.height}",
                outer.values.all { it == ground } && inner.values.all { it != ground },
            )

            val header = "${machine.label.text} · ${session.tmuxName}"
            val headerNodes = compose.onAllNodesWithText(header).fetchSemanticsNodes()
            assertEquals(
                "shortening Detach must retain one readable machine and tmux-session identity; " +
                    "expected=\"$header\", matching semantics nodes=$headerNodes",
                1,
                headerNodes.size,
            )
            compose.onNodeWithText(header).assertIsDisplayed()

            val renameNodes = compose.onAllNodesWithText("Rename").fetchSemanticsNodes()
            assertEquals(
                "the active Terminal must expose one literal Rename affordance in its middle " +
                    "identity block; matching semantics nodes=$renameNodes",
                1,
                renameNodes.size,
            )
            val rename = compose.onNodeWithContentDescription(identity)
            rename.assertIsDisplayed().assertHasClickAction()
                .assertContentDescriptionEquals(identity).assertIsNotEnabled()
            val renameNode = rename.fetchSemanticsNode()
            assertEquals(
                "the middle identity action must announce Role.Button; semantics=${renameNode.config}",
                Role.Button,
                renameNode.config.getOrNull(SemanticsProperties.Role),
            )
            assertEquals(
                "the middle identity action must expose current presence as state; " +
                    "semantics=${renameNode.config}",
                "Verifying machine and session",
                renameNode.config.getOrNull(SemanticsProperties.StateDescription),
            )
            val presence = "${machine.label.text} · Verifying machine and session"
            assertEquals(
                "the existing machine/presence line must remain literal below Rename",
                1,
                compose.onAllNodesWithText(presence).fetchSemanticsNodes().size,
            )

            val textSize = compose.onNodeWithContentDescription("Terminal text size")
            textSize.assertIsDisplayed().assertHasClickAction().assertIsNotEnabled()
            val textSizeNode = textSize.fetchSemanticsNode()
            assertEquals(
                "the text-size control must announce Role.Button under its spoken name; " +
                    "semantics=${textSizeNode.config}",
                Role.Button,
                textSizeNode.config.getOrNull(SemanticsProperties.Role),
            )

            val killNodes = compose.onAllNodesWithText("Kill").fetchSemanticsNodes()
            assertEquals(
                "the terminal header must retain exactly one visible Kill control beside " +
                    "Detach and the machine/session identity; matching semantics nodes=$killNodes",
                1,
                killNodes.size,
            )
            val kill = compose.onNodeWithText("Kill")
            kill.assertIsDisplayed()
            val killNode = kill.fetchSemanticsNode()
            assertEquals(
                "the trailing visible Kill label must belong to Role.Button; " +
                    "semantics=${killNode.config}",
                Role.Button,
                killNode.config.getOrNull(SemanticsProperties.Role),
            )
            assertHeaderGeometry("header-device-scale", identity)

            val retired = compose.onAllNodesWithText(
                "session keeps running",
                substring = true,
            ).fetchSemanticsNodes()
            assertTrue(
                "the lifetime promise is hard-cut from terminal chrome; retired nodes=$retired",
                retired.isEmpty(),
            )

            // The attached header speaks a count-only presence and unlocks text size.
            compose.runOnIdle {
                terminalState = terminalState.copy(
                    connection = connected,
                    viewport = TerminalViewport.Fitted(40, 18),
                    textSize = TerminalTextSizeState.Ready(
                        TERMINAL_TEXT_SIZE_DEFAULT_SP,
                        TerminalTextSizeWrite.Idle,
                        sheetOpen = false,
                    ),
                )
            }
            compose.mainClock.advanceTimeByFrame()
            assertEquals(
                "an attached terminal must expose only the client count as presence",
                "1 client",
                compose.onNodeWithContentDescription(identity).fetchSemanticsNode()
                    .config.getOrNull(SemanticsProperties.StateDescription),
            )
            compose.onNodeWithText("${machine.label.text} · 1 client").assertIsDisplayed()
            compose.onNodeWithContentDescription("Terminal text size").assertIsEnabled()

            // A fitted attachment admits user ingress into the real page and key deck.
            val pageBounds = compose.onNodeWithTag("terminal-page").fetchSemanticsNode().boundsInRoot
            compose.onNodeWithContentDescription("Escape").assertIsEnabled()
            val terminal = compose.runOnIdle {
                requireNotNull(findLockedTerminal(requireNotNull(hostView))) {
                    "the terminal screen must host the production locked WebView"
                }
            }
            assertTrue(
                "a fitted attachment must admit touch, focus, typing and paste at the hosted page",
                compose.runOnIdle { terminal.isEnabled },
            )

            // At the maximum system text scale every control stays reachable and unclipped;
            // only the identity may ellipsize, and it keeps its whole accessible name.
            compose.runOnIdle { headerFontScale = MAXIMUM_SYSTEM_TEXT_SCALE }
            compose.mainClock.advanceTimeByFrame()
            assertHeaderGeometry("header-text-scale-$MAXIMUM_SYSTEM_TEXT_SCALE", identity)
            compose.onNodeWithContentDescription(identity).assertContentDescriptionEquals(identity)
            compose.runOnIdle { headerFontScale = 1f }
            compose.mainClock.advanceTimeByFrame()

            // Insufficient room is a screen-level overlay: it blocks user ingress and
            // keeps its own recovery and Detach reachable without moving the page.
            compose.runOnIdle { terminalState = terminalState.copy(viewport = TerminalViewport.TooSmall) }
            compose.mainClock.advanceTimeByFrame()
            compose.onNodeWithText("Terminal needs more room").assertIsDisplayed()
            compose.onNodeWithText("Hide the keyboard, reduce text size, or make the app window larger.")
                .assertIsDisplayed()
            compose.onNodeWithText("Text size").assertIsDisplayed().assertHasClickAction()
            compose.onNodeWithText("Detach").assertIsDisplayed().assertHasClickAction()
            compose.onNodeWithContentDescription("Escape").assertIsNotEnabled()
            assertTrue(
                "a too-small viewport must gate touch, focus, typing and paste at the hosted page",
                compose.runOnIdle { !terminal.isEnabled },
            )
            assertEquals(
                "the recovery overlay must leave the page's measured bounds untouched; " +
                    "fittedPx=$pageBounds",
                pageBounds,
                compose.onNodeWithTag("terminal-page").fetchSemanticsNode().boundsInRoot,
            )

            // Recovery at the unchanged last-valid geometry admits user ingress again.
            compose.runOnIdle {
                terminalState = terminalState.copy(viewport = TerminalViewport.Fitted(40, 18))
            }
            compose.mainClock.advanceTimeByFrame()
            compose.onNodeWithText("Terminal needs more room").assertDoesNotExist()
            compose.onNodeWithContentDescription("Escape").assertIsEnabled()
            assertTrue(
                "recovered room must re-admit user ingress at the hosted page",
                compose.runOnIdle { terminal.isEnabled },
            )

            // A failed preference read blocks the page behind its own reread action.
            compose.runOnIdle {
                terminalState = terminalState.copy(textSize = TerminalTextSizeState.Unavailable)
            }
            compose.mainClock.advanceTimeByFrame()
            compose.onNodeWithText("Text size unavailable.").assertIsDisplayed()
            compose.onNodeWithText("Retry").assertIsDisplayed().assertHasClickAction()
            compose.onNodeWithTag("terminal-page").assertDoesNotExist()
            compose.onNodeWithContentDescription("Terminal text size").assertIsNotEnabled()

            // The first half proves TerminalScreen owns the real production control and
            // chrome geometry. Switch to the same production control and sheet with a
            // reducer-backed state holder so the test can drive every form phase without
            // injecting state into SkidbladnirController or adding a production test seam.
            compose.runOnIdle { showRenameHarness = true }
            compose.mainClock.advanceTimeByFrame()
            compose.mainClock.autoAdvance = true

            compose.onNodeWithContentDescription(identity).assertIsEnabled().performClick()

            compose.onNodeWithText("Rename tmux session").assertIsDisplayed()
            compose.onNodeWithText("${session.tmuxName} on ${machine.label.text}").assertIsDisplayed()
            compose.onNodeWithText("1–64 letters, numbers, underscores, or hyphens").assertIsDisplayed()
            fun field() = compose.onNodeWithText("Tmux name")
            fun submit() = compose.onAllNodes(literalButton("Rename"))[0]
            field().assertIsDisplayed().assertIsEnabled()
            assertEquals(
                "the field must be prefilled with the exact authoritative tmux name",
                session.tmuxName,
                field().fetchSemanticsNode().config
                    .getOrNull(SemanticsProperties.EditableText)?.text,
            )

            submit().assertIsNotEnabled()
            field().performTextReplacement("not a tmux name")
            submit().assertIsNotEnabled()
            field().performTextReplacement("skidbladnir-personal-renamed")
            submit().assertIsEnabled()
            field().performImeAction()

            field().assertIsNotEnabled()
            compose.onNodeWithText("Cancel").assertIsNotEnabled()
            submit().assertIsNotEnabled()

            compose.runOnIdle {
                renameState = completeRenameHttp(
                    checkNotNull(renameState),
                    GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.SessionNameConflict)),
                ).state
            }
            field().assertIsEnabled()
            assertEquals(
                "a definite rejection must preserve the desired draft",
                "skidbladnir-personal-renamed",
                field().fetchSemanticsNode().config
                    .getOrNull(SemanticsProperties.EditableText)?.text,
            )
            compose.onNodeWithText("A session with that name already exists.").assertIsDisplayed()
            submit().assertIsEnabled()

            compose.runOnIdle {
                val sending = beginRenameSending(
                    checkNotNull(renameState),
                    target,
                    terminalActionsAdmissible = connectedActionsAdmissible,
                )
                renameState = completeRenameHttp(
                    checkNotNull(sending),
                    GatewayResult.Failure(GatewayFailure.Transport),
                ).state
            }
            compose.onNodeWithText(RENAME_OUTCOME_UNKNOWN).assertIsDisplayed()
            field().assertIsNotEnabled()
            compose.onNodeWithText("Cancel").assertIsEnabled()
            submit().assertIsNotEnabled()
        } finally {
            controller.close()
        }
    }

    // The one real DataStore adapter proof, and with it the attachment gate: the
    // real controller, the real production screen, and the real preference file
    // under filesDir. The attachment upgrade is held open by the external
    // boundary so the terminal stays Connecting, where text-size mutation is
    // admissible.
    @Test
    fun terminalTextSizeIsSavedOnThePhoneAndSurvivesRecreationAndStorageFailure() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext.applicationContext
        val storage = MachineStorage(TEST_PREFERENCES, TEST_KEY_ALIAS)
        val preferenceFile = File(context.filesDir, "datastore/terminal-text-size")
        val savedPreference = preferenceFile.takeIf(File::isFile)?.readBytes()
        val boundary = ExternalTerminalBoundary(FLEET)
        resetFleetFixture(context, storage)
        var activeController by mutableStateOf(
            SkidbladnirController(context, DashboardEntryState(), storage, boundary.client()),
        )
        var screenHeight by mutableStateOf<Dp?>(null)
        try {
            compose.mainClock.autoAdvance = true
            preferenceFile.deleteRecursively()
            check(!preferenceFile.exists()) { "the saved text size could not be cleared" }
            assertEquals(FleetInstallation.Installed, MachineStore(context, storage).installFixedFleet(FLEET))
            compose.setContent {
                NidavellirTheme {
                    val state = activeController.state
                    if (state is SkidbladnirUiState.Terminal) {
                        Box(
                            modifier = screenHeight
                                ?.let { Modifier.fillMaxWidth().height(it) }
                                ?: Modifier.fillMaxSize(),
                        ) {
                            TerminalScreen(state = state, controller = activeController, onDetach = {})
                        }
                    }
                }
            }

            // A fitted page attaches; the same page in a screen too short for five
            // whole rows reaches the gateway only once the room comes back.
            openDevboxTerminal(activeController)
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
                terminalViewport(activeController) is TerminalViewport.Fitted
            }
            val fitted = terminalViewport(activeController) as TerminalViewport.Fitted
            val fittedPageHeight = compose.onNodeWithTag("terminal-page").fetchSemanticsNode().boundsInRoot.height
            val rootHeight = compose.onRoot().fetchSemanticsNode().size.height
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) { boundary.upgradeAttempts() > 0 }
            assertEquals(
                "a fitted terminal attaches exactly once; fittedGrid=${fitted.columns}x${fitted.rows}",
                1,
                boundary.upgradeAttempts(),
            )

            // Three of the fitted rows is below the twenty-by-five floor at every text size.
            val tooShortScreen = with(compose.density) {
                (rootHeight - fittedPageHeight * (1f - 3f / fitted.rows)).toDp()
            }
            compose.runOnIdle {
                activeController.close()
                activeController = SkidbladnirController(context, DashboardEntryState(), storage, boundary.client())
                screenHeight = tooShortScreen
            }
            openDevboxTerminal(activeController)
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
                terminalViewport(activeController) is TerminalViewport.TooSmall
            }
            compose.onNodeWithText("Terminal needs more room").assertIsDisplayed()
            assertEquals(
                "a terminal that has never fitted must not reach the gateway; " +
                    "screenHeightDp=$tooShortScreen fittedRows=${fitted.rows}",
                1,
                boundary.upgradeAttempts(),
            )
            compose.runOnIdle { screenHeight = null }
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
                terminalViewport(activeController) is TerminalViewport.Fitted
            }
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) { boundary.upgradeAttempts() > 1 }
            assertEquals(
                "recovered room must attach the terminal that had been held back; " +
                    "recoveredGrid=${terminalViewport(activeController)}",
                2,
                boundary.upgradeAttempts(),
            )

            // An attached terminal survives a too-small episode: the recovery surface
            // replaces the waiting notice, the attachment is neither dropped nor asked to
            // reconnect, and recovered room reuses that same connection.
            compose.runOnIdle { screenHeight = tooShortScreen }
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
                terminalViewport(activeController) is TerminalViewport.TooSmall
            }
            compose.onNodeWithText("Terminal needs more room").assertIsDisplayed()
            compose.onNodeWithText("Reattach to ${DEVBOX.machine.label.text}").assertDoesNotExist()
            compose.runOnIdle { screenHeight = null }
            compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
                terminalViewport(activeController) is TerminalViewport.Fitted
            }
            waitUntilGone(hasText("Terminal needs more room"))
            assertEquals(
                "a too-small episode over a live attachment must reuse it, never reattach; " +
                    "recoveredGrid=${terminalViewport(activeController)}",
                2,
                boundary.upgradeAttempts(),
            )

            waitUntilEnabled(hasContentDescription("Terminal text size"))
            compose.onNodeWithContentDescription("Terminal text size").performClick()
            compose.onNodeWithText("Text size").assertIsDisplayed()
            compose.onNodeWithContentDescription("Terminal text size, 16").assertIsDisplayed()
            compose.onNodeWithText("Saved on this phone. Android’s text-size setting also applies.")
                .assertIsDisplayed()
            compose.onNodeWithText("Screen size is shared with other attached terminals.").assertIsDisplayed()
            compose.onNodeWithText("Reset to 16").assertIsNotEnabled()
            waitUntilEnabled(hasContentDescription("Increase terminal text size"))
            compose.onNodeWithContentDescription("Increase terminal text size").performClick()
            waitUntilExists(hasContentDescription("Terminal text size, 17"))
            assertEquals(
                "the committed size must be durable as two ASCII digits under filesDir",
                "17",
                preferenceFile.readText(Charsets.US_ASCII),
            )
            compose.onNodeWithText("Done").performClick()
            waitUntilGone(hasText("Text size"))

            compose.runOnIdle { activeController.close() }
            activeController = SkidbladnirController(context, DashboardEntryState(), storage, boundary.client())
            openDevboxTerminal(activeController)
            waitUntilEnabled(hasContentDescription("Terminal text size"))
            compose.onNodeWithContentDescription("Terminal text size").performClick()
            compose.onNodeWithContentDescription("Terminal text size, 17").assertIsDisplayed()
            waitUntilEnabled(hasText("Reset to 16"))
            compose.onNodeWithText("Reset to 16").performClick()
            waitUntilExists(hasContentDescription("Terminal text size, 16"))
            compose.onNodeWithText("Reset to 16").assertIsNotEnabled()
            assertEquals("16", preferenceFile.readText(Charsets.US_ASCII))

            check(preferenceFile.delete() && preferenceFile.mkdir() && File(preferenceFile, "occupant").createNewFile()) {
                "the saved text size could not be replaced by a directory"
            }
            waitUntilEnabled(hasContentDescription("Increase terminal text size"))
            compose.onNodeWithContentDescription("Increase terminal text size").performClick()
            waitUntilExists(hasText("Text size could not be saved."))
            compose.onNodeWithContentDescription("Terminal text size, 16").assertIsDisplayed()
            compose.onNodeWithContentDescription("Increase terminal text size").assertIsEnabled()

            // The notice belongs to the write that failed, not to the next sheet session.
            compose.onNodeWithText("Done").performClick()
            waitUntilGone(hasText("Text size"))
            waitUntilEnabled(hasContentDescription("Terminal text size"))
            compose.onNodeWithContentDescription("Terminal text size").performClick()
            waitUntilExists(hasText("Text size"))
            compose.onNodeWithText("Text size could not be saved.").assertDoesNotExist()
            compose.onNodeWithContentDescription("Terminal text size, 16").assertIsDisplayed()
        } finally {
            boundary.release()
            compose.runOnIdle { activeController.close() }
            preferenceFile.deleteRecursively()
            savedPreference?.let { bytes ->
                preferenceFile.parentFile?.mkdirs()
                preferenceFile.writeBytes(bytes)
            }
            resetFleetFixture(context, storage)
        }
    }

    // The header's four controls keep their 48dp targets, their 8dp clearance and
    // their place inside the window at the device's own scale and at the maximum
    // system text scale, where only the weighted identity may ellipsize.
    private fun assertHeaderGeometry(caseId: String, identity: String) {
        val minimumTargetPx = with(compose.density) { 48.dp.roundToPx() }
        val minimumGapPx = with(compose.density) { 8.dp.roundToPx() }
        val root = compose.onRoot().fetchSemanticsNode().boundsInRoot
        val controls = listOf(
            "Detach" to compose.onNodeWithText("Detach"),
            "identity" to compose.onNodeWithContentDescription(identity),
            "Terminal text size" to compose.onNodeWithContentDescription("Terminal text size"),
            "Kill" to compose.onNodeWithText("Kill"),
        ).map { (name, node) -> name to node.fetchSemanticsNode().boundsInRoot }
        for ((name, bounds) in controls) {
            assertTrue(
                "case=$caseId control=$name must keep at least a 48dp touch target in both axes " +
                    "and stay inside the window without clipping; boundsPx=$bounds, " +
                    "rootPx=$root, minimumTargetPx=$minimumTargetPx, headerPx=$controls",
                bounds.width >= minimumTargetPx && bounds.height >= minimumTargetPx &&
                    bounds.left >= root.left && bounds.right <= root.right &&
                    bounds.top >= root.top && bounds.bottom <= root.bottom,
            )
        }
        val gapsPx = controls.zipWithNext { (_, left), (_, right) -> right.left - left.right }
        assertTrue(
            "case=$caseId header must remain ordered Detach, identity, Terminal text size, Kill " +
                "with at least 8dp clear between each semantic bound; headerPx=$controls, " +
                "gapsPx=$gapsPx, minimumGapPx=$minimumGapPx, density=${compose.density.density}",
            gapsPx.all { it >= minimumGapPx },
        )
    }

    private fun openDevboxTerminal(controller: SkidbladnirController) {
        compose.runOnIdle { controller.start() }
        compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) { devboxMachine(controller)?.canMutate == true }
        compose.runOnIdle {
            val machine = requireNotNull(devboxMachine(controller))
            val session = (machine.inventory as InventoryState.Fresh).snapshot.inventory.sessions.single()
            controller.openTerminal(SessionTarget(machine.machine.handle, session))
        }
    }

    private fun terminalViewport(controller: SkidbladnirController): TerminalViewport? =
        (controller.state as? SkidbladnirUiState.Terminal)?.viewport

    private fun devboxMachine(controller: SkidbladnirController): MachineState? =
        (controller.state as? SkidbladnirUiState.Dashboard)?.machines
            ?.singleOrNull { it.machine.handle == DEVBOX.machine.handle }

    private fun waitUntilExists(matcher: SemanticsMatcher) {
        compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
            compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty()
        }
    }

    private fun waitUntilGone(matcher: SemanticsMatcher) {
        compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
            compose.onAllNodes(matcher).fetchSemanticsNodes().isEmpty()
        }
    }

    private fun waitUntilEnabled(matcher: SemanticsMatcher) {
        compose.waitUntil(timeoutMillis = CHROME_WAIT_MILLIS) {
            compose.onAllNodes(matcher).fetchSemanticsNodes()
                .singleOrNull()
                ?.let { node -> node.config.getOrNull(SemanticsProperties.Disabled) == null }
                ?: false
        }
    }

    private fun resetFleetFixture(context: Context, storage: MachineStorage) {
        val preferences = storage.preferences(context)
        check(preferences.edit().clear().commit()) { "could not clear chrome fleet preferences" }
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        if (keyStore.containsAlias(TEST_KEY_ALIAS)) keyStore.deleteEntry(TEST_KEY_ALIAS)
    }

    private class ExternalTerminalBoundary(credentials: List<MachineCredential>) : Interceptor {
        private val credentialsByHost = credentials.associateBy { credential ->
            checkNotNull(credential.machine.origin.encoded.toHttpUrlOrNull()).host
        }
        private val released = CountDownLatch(1)
        private val upgrades = AtomicInteger()

        fun upgradeAttempts(): Int = upgrades.get()

        fun client(): GatewayClient = GatewayClient(
            OkHttpClient.Builder()
                .addInterceptor(this)
                .retryOnConnectionFailure(false)
                .followRedirects(false)
                .followSslRedirects(false)
                .build(),
        )

        fun release() = released.countDown()

        override fun intercept(chain: Interceptor.Chain): Response {
            val request = chain.request()
            val credential = checkNotNull(credentialsByHost[request.url.host]) { "unknown machine origin" }
            val path = request.url.encodedPath
            return when {
                request.method == "GET" && path == "/v1/sessions" ->
                    response(request, 200, sessionsResponse(credential).toResponseBody(JSON_MEDIA_TYPE))
                request.method == "GET" && path == "/v1/pressure" ->
                    response(request, 503, ByteArray(0).toResponseBody())
                request.method == "GET" && path.endsWith("/terminal") -> {
                    upgrades.incrementAndGet()
                    released.await(ATTACHMENT_HOLD_SECONDS, TimeUnit.SECONDS)
                    response(request, 503, ByteArray(0).toResponseBody())
                }
                else -> error("unexpected external route $path")
            }
        }

        private fun sessionsResponse(credential: MachineCredential): String {
            val sessions = if (credential == DEVBOX) {
                "[{\"tmuxId\":\"${'$'}1\",\"tmuxName\":\"text-size\"," +
                    "\"identityToken\":\"synthetic-lifetime\"," +
                    "\"character\":{\"key\":\"alvis\",\"displayName\":\"Alvís\"}," +
                    "\"launchProfile\":\"personal\",\"attachedClients\":1,\"activity\":\"Quiet\"}]"
            } else {
                "[]"
            }
            return "{\"machine\":{\"handle\":\"${credential.machine.handle.encoded}\"," +
                "\"platform\":\"Linux\"},\"observedAt\":\"2026-09-08T12:00:00Z\"," +
                "\"profiles\":[{\"key\":\"personal\",\"label\":\"Codex · Personal\"," +
                "\"provider\":\"Codex\"}],\"sessions\":$sessions}"
        }

        private fun response(request: Request, status: Int, body: ResponseBody): Response = Response.Builder()
            .request(request)
            .protocol(Protocol.HTTP_1_1)
            .code(status)
            .message(if (status == 200) "OK" else "Unavailable")
            .body(body)
            .build()
    }

    private companion object {
        // The terminal upgrade is answered only after the case releases it, so the
        // screen stays Connecting for the whole run.
        const val ATTACHMENT_HOLD_SECONDS = 60L
        const val MAXIMUM_SYSTEM_TEXT_SCALE = 2f
        const val CHROME_WAIT_MILLIS = 10_000L
        const val TEST_PREFERENCES = "skidbladnir.machines.terminal-chrome-test"
        const val TEST_KEY_ALIAS = "skidbladnir.machine-bearers.terminal-chrome-test"
        val JSON_MEDIA_TYPE = "application/json; charset=utf-8".toMediaType()
        val ARCH = credential(
            "mh-44444444444444444444444444444444",
            "Arch",
            "https://arch.chrome.invalid:8443/",
            "A".repeat(43),
        )
        val DEVBOX = credential(
            "mh-55555555555555555555555555555555",
            "Devbox",
            "https://devbox.chrome.invalid:8443/",
            "B" + "A".repeat(42),
        )
        val MACBOOK = credential(
            "mh-66666666666666666666666666666666",
            "MacBook",
            "https://macbook.chrome.invalid:8443/",
            "C" + "A".repeat(42),
        )
        val FLEET = listOf(ARCH, DEVBOX, MACBOOK)

        fun credential(
            handle: String,
            label: String,
            origin: String,
            bearer: String,
        ): MachineCredential = MachineCredential(
            PairedMachine(
                requireNotNull(MachineHandle.parse(handle)),
                requireNotNull(MachineLabel.parse(label)),
                requireNotNull(MachineOrigin.parse(origin)),
            ),
            requireNotNull(GatewayBearer.parse(bearer)),
        )
    }
}

private fun findLockedTerminal(root: View): LockedTerminalWebView? = when (root) {
    is LockedTerminalWebView -> root
    is ViewGroup -> (0 until root.childCount).firstNotNullOfOrNull { findLockedTerminal(root.getChildAt(it)) }
    else -> null
}

private fun literalButton(label: String): SemanticsMatcher = SemanticsMatcher(
    "Role.Button with exact visible label $label and no replacement content description",
) { node ->
    node.config.getOrNull(SemanticsProperties.Role) == Role.Button &&
        node.config.getOrNull(SemanticsProperties.Text)?.any { it.text == label } == true &&
        node.config.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty()
}

private fun PixelMap.corners(inset: Int): Map<String, Int> {
    require(inset in 0 until minOf(width, height) / 2)
    return mapOf(
        "top-start" to this[inset, inset].toArgb(),
        "top-end" to this[width - 1 - inset, inset].toArgb(),
        "bottom-start" to this[inset, height - 1 - inset].toArgb(),
        "bottom-end" to this[width - 1 - inset, height - 1 - inset].toArgb(),
    )
}
