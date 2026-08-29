package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.PixelMap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
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
            attention = false,
            status = SessionStatus(
                SessionStatusKind.Working,
                SessionStatusSignal.Lifecycle,
                Instant.parse("2026-08-27T12:00:00Z"),
            ),
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
        )

        try {
            compose.setContent {
                MaterialTheme {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = Ink,
                        contentColor = Bone,
                    ) {
                        TerminalScreen(
                            state = SkidbladnirUiState.Terminal(
                                machine = machineState,
                                target = SessionTarget(machine.handle, session),
                                attempt = 1,
                                connection = TerminalUiStatus.Verifying,
                                kill = null,
                            ),
                            controller = controller,
                        )
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
            val headerNode = compose.onNodeWithText(header).assertIsDisplayed().fetchSemanticsNode()
            val headerBounds = headerNode.boundsInRoot

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
            val detachIdentityGapPx = headerBounds.left - detachBounds.right
            val identityKillGapPx = killBounds.left - headerBounds.right
            assertTrue(
                "terminal chrome must remain ordered Detach, machine/session identity, Kill " +
                    "with at least 8dp clear between each semantic bound; boundsPx: " +
                    "detach=$detachBounds, identity=$headerBounds, kill=$killBounds; " +
                    "gapsPx=[$detachIdentityGapPx, $identityKillGapPx], " +
                    "minimumPx=$minimumGapPx, density=${compose.density.density}",
                detachIdentityGapPx >= minimumGapPx && identityKillGapPx >= minimumGapPx,
            )

            val retired = compose.onAllNodesWithText(
                "session keeps running",
                substring = true,
            ).fetchSemanticsNodes()
            assertTrue(
                "the lifetime promise is hard-cut from terminal chrome; retired nodes=$retired",
                retired.isEmpty(),
            )
        } finally {
            controller.close()
        }
    }
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
