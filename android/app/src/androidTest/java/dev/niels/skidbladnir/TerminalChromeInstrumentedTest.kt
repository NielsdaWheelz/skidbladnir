package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.PixelMap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
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
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.time.Instant
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
    fun terminalHeaderRendersOneQuietDetachControlBesideItsMachineSessionIdentity() {
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
        val connected = TerminalUiStatus.Connected(1, TerminalGeometry.Owner)
        val connectedActionsAdmissible = terminalActionAdmissible(machineState.canMutate, connected)
        var showRenameHarness by mutableStateOf(false)
        var renameState by mutableStateOf<RenameState?>(null)

        try {
            compose.setContent {
                MaterialTheme {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = Ink,
                        contentColor = Bone,
                    ) {
                        if (!showRenameHarness) {
                            TerminalScreen(
                                state = SkidbladnirUiState.Terminal(
                                    machine = machineState,
                                    target = target,
                                    attempt = 1,
                                    connection = TerminalUiStatus.Verifying,
                                    kill = null,
                                ),
                                controller = controller,
                                onDetach = {},
                            )
                        } else {
                            Column(modifier = Modifier.fillMaxSize()) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 12.dp, vertical = 8.dp),
                                    verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                ) {
                                    DetachButton(onClick = {})
                                    TerminalRenameControl(
                                        machine = machine,
                                        target = target,
                                        presence = "1 client · OWNER",
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

            val detachBounds = detachNode.boundsInRoot
            val minimumTargetPx = with(compose.density) { 48.dp.roundToPx() }
            assertTrue(
                "Detach must keep at least a 48dp touch target in both axes; " +
                    "boundsPx=$detachBounds, widthPx=${detachBounds.width}, " +
                    "heightPx=${detachBounds.height}, minimumPx=$minimumTargetPx, " +
                    "density=${compose.density.density}",
                detachBounds.width >= minimumTargetPx && detachBounds.height >= minimumTargetPx,
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
            val rename = compose.onNodeWithContentDescription(
                "Rename ${session.tmuxName} on ${machine.label.text}",
            )
            rename.assertIsDisplayed().assertHasClickAction().assertContentDescriptionEquals(
                "Rename ${session.tmuxName} on ${machine.label.text}",
            ).assertIsNotEnabled()
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
            assertTrue(
                "Rename must keep at least a 48dp touch target in both axes; " +
                    "boundsPx=${renameNode.boundsInRoot}, minimumPx=$minimumTargetPx",
                renameNode.boundsInRoot.width >= minimumTargetPx &&
                    renameNode.boundsInRoot.height >= minimumTargetPx,
            )
            val presence = "${machine.label.text} · Verifying machine and session"
            assertEquals(
                "the existing machine/presence line must remain literal below Rename",
                1,
                compose.onAllNodesWithText(presence).fetchSemanticsNodes().size,
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
            val killBounds = killNode.boundsInRoot
            val minimumGapPx = with(compose.density) { 8.dp.roundToPx() }
            val detachRenameGapPx = renameNode.boundsInRoot.left - detachBounds.right
            val renameKillGapPx = killBounds.left - renameNode.boundsInRoot.right
            assertTrue(
                "terminal chrome must remain ordered Detach, Rename identity control, Kill " +
                    "with at least 8dp clear between each semantic bound; boundsPx: " +
                    "detach=$detachBounds, rename=${renameNode.boundsInRoot}, kill=$killBounds; " +
                    "gapsPx=[$detachRenameGapPx, $renameKillGapPx], " +
                    "minimumPx=$minimumGapPx, density=${compose.density.density}",
                detachRenameGapPx >= minimumGapPx && renameKillGapPx >= minimumGapPx,
            )

            val retired = compose.onAllNodesWithText(
                "session keeps running",
                substring = true,
            ).fetchSemanticsNodes()
            assertTrue(
                "the lifetime promise is hard-cut from terminal chrome; retired nodes=$retired",
                retired.isEmpty(),
            )

            // The first half proves TerminalScreen owns the real production control and
            // chrome geometry. Switch to the same production control and sheet with a
            // reducer-backed state holder so the test can drive every form phase without
            // injecting state into SkidbladnirController or adding a production test seam.
            compose.runOnIdle { showRenameHarness = true }
            compose.mainClock.advanceTimeByFrame()
            compose.mainClock.autoAdvance = true

            compose.onNodeWithContentDescription(
                "Rename ${session.tmuxName} on ${machine.label.text}",
            ).assertIsEnabled().performClick()

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
