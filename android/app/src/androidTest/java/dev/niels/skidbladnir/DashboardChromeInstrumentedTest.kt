package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.assertContentDescriptionEquals
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.time.Instant
import kotlin.math.absoluteValue
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class DashboardChromeInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun everyStatusKindKeepsItsLiteralChipLabelAndEvidence() {
        var kind by mutableStateOf(SessionStatusKind.Working)
        val receivedAt = SystemClock.elapsedRealtime()
        compose.setContent {
            MaterialTheme {
                val session = session(kind)
                AgentCard(
                    agent = visibleAgent(session),
                    machine = machineState(session, receivedAt),
                    onOpen = {},
                    onKill = { error("rendering a card killed its session") },
                )
            }
        }

        SessionStatusKind.entries.forEach { entry ->
            compose.runOnIdle { kind = entry }
            val label = CHIP_LABELS.getValue(entry)
            val rendered = card().fetchSemanticsNode()
                .config.getOrNull(SemanticsProperties.Text)?.map { it.text }.orEmpty()
            assertTrue(
                "the $entry chip must render its literal label with its named signal and age; " +
                    "expected \"$label\" and \"$CHIP_EVIDENCE\" among the card text $rendered",
                rendered.containsAll(listOf(label, CHIP_EVIDENCE)),
            )
        }
    }

    @Test
    fun theSessionCardKeepsItsFullTargetClickActionAndSpokenMarks() {
        compose.mainClock.autoAdvance = false
        val opened = mutableListOf<String>()
        val session = session(SessionStatusKind.Working, attention = true)
        val receivedAt = SystemClock.elapsedRealtime()
        compose.setContent {
            MaterialTheme {
                AgentCard(
                    agent = visibleAgent(session),
                    machine = machineState(session, receivedAt),
                    onOpen = { opened += SESSION_ID },
                    onKill = { error("opening a session killed it") },
                )
            }
        }

        val bounds = card().getUnclippedBoundsInRoot()
        assertTrue(
            "the session card target is smaller than $MINIMUM_TARGET: bounds=$bounds",
            bounds.right - bounds.left >= MINIMUM_TARGET && bounds.bottom - bounds.top >= MINIMUM_TARGET,
        )
        val spoken = card().fetchSemanticsNode()
            .config.getOrNull(SemanticsProperties.ContentDescription).orEmpty()
        assertTrue(
            "the card must still speak its portrait and attention marks; it spoke $spoken",
            spoken.containsAll(listOf(PORTRAIT_DESCRIPTION, ATTENTION_DESCRIPTION)),
        )

        card().performClick()
        compose.runOnIdle {
            assertEquals(
                "tapping the session card must open exactly that session once",
                listOf(SESSION_ID),
                opened,
            )
        }
    }

    @Test
    fun theAttentionLozengeRendersAsAMarkWhileAnimationsAreDisabled() {
        compose.mainClock.autoAdvance = false
        val session = session(SessionStatusKind.Idle, attention = true)
        val receivedAt = SystemClock.elapsedRealtime()
        compose.setContent {
            MaterialTheme {
                AgentCard(
                    agent = visibleAgent(session),
                    machine = machineState(session, receivedAt),
                    onOpen = {},
                    onKill = { error("rendering a card killed its session") },
                )
            }
        }

        val bounds = compose
            .onNodeWithContentDescription(ATTENTION_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        assertTrue(
            "the attention lozenge must render as a mark of its own, not a zero-sized node: bounds=$bounds",
            bounds.right - bounds.left >= LOZENGE_SIDE && bounds.bottom - bounds.top >= LOZENGE_SIDE,
        )
        compose.mainClock.advanceTimeByFrame()
        val pixels = compose
            .onNodeWithContentDescription(ATTENTION_DESCRIPTION, useUnmergedTree = true)
            .captureToImage()
            .toPixelMap()
        val center = pixels[pixels.width / 2, pixels.height / 2]
        assertEquals(
            "a static attention lozenge is a designed FULL-opacity state: the center " +
                "pixel must be exact Orpiment, not a faded blend over the card surface",
            Orpiment.toArgb(),
            center.toArgb(),
        )
    }

    @Test
    fun theLitForgeSealSpeaksNewDwarfAndRequestsTheForgeExactlyOncePerTap() {
        compose.mainClock.autoAdvance = false
        var forgeRequests = 0
        compose.setContent {
            MaterialTheme {
                ForgeSeal(canForge = true, onClick = { forgeRequests += 1 })
            }
        }

        val seal = compose.onNodeWithTag("new-agent")
        seal.assertIsEnabled()
        seal.assertContentDescriptionEquals("New dwarf")
        // TalkBack must announce it as a button, not as an unlabelled image
        // (forge-seal.md, "Placement and semantics"). Nothing else gates the Role;
        // a click action would be gated twice over by the real tap below.
        assertEquals(
            "the Forge seal must carry Role.Button so it is announced as a button",
            Role.Button,
            seal.fetchSemanticsNode().config.getOrNull(SemanticsProperties.Role),
        )
        val bounds = seal.getUnclippedBoundsInRoot()
        assertTrue(
            "the Forge seal must be exactly $SEAL_SIDE square (forge-seal.md \"Placement and " +
                "semantics\"): that is what clears the 48dp target floor without " +
                "minimumInteractiveComponentSize (design-language.md §14), and it is what makes " +
                "forgeSealField()'s quarter-point sample land inside the octagon — a node padded " +
                "out around a smaller visual would sample outside the frame and fail with a " +
                "colour message that explains nothing. bounds=$bounds",
            (bounds.right - bounds.left - SEAL_SIDE).value.absoluteValue <= SIDE_TOLERANCE &&
                (bounds.bottom - bounds.top - SEAL_SIDE).value.absoluteValue <= SIDE_TOLERANCE,
        )
        // One frame, then read: the lit/cold flip carries no warm-in, so the field is
        // already its final colour (forge-seal.md, closed decision 4).
        compose.mainClock.advanceTimeByFrame()
        assertEquals(
            "the lit seal is the only ForgeGlow outside the Forge sheet (design-language.md " +
                "§13): its field must be exact ForgeGlow, so cold has something to differ from",
            ForgeGlow.toArgb(),
            forgeSealField().toArgb(),
        )

        // The pause bought a deterministic frame to capture; the seal has no
        // animation to outrun, and holding the clock past the tap would freeze
        // the press flash mid-tween and hollow out runOnIdle.
        compose.mainClock.autoAdvance = true
        seal.performClick()
        compose.runOnIdle {
            assertEquals("one tap on the lit Forge seal must request the Forge exactly once", 1, forgeRequests)
        }
    }

    @Test
    fun theColdForgeSealIsSpokenDisabledAndSwapsItsFieldRatherThanItsOpacity() {
        compose.mainClock.autoAdvance = false
        var forgeRequests = 0
        compose.setContent {
            MaterialTheme {
                ForgeSeal(canForge = false, onClick = { forgeRequests += 1 })
            }
        }

        // A disabled `clickable` keeps its `OnClick` semantics action and adds
        // `disabled()`, so an absent action is not the contract and asserting it
        // would be unreachable. What a user meets is a control spoken disabled that
        // reaches nothing when tapped, and that is what is asserted.
        val seal = compose.onNodeWithTag("new-agent")
        seal.assertIsNotEnabled()
        compose.mainClock.advanceTimeByFrame()
        assertEquals(
            "cold changes field and hue, never opacity alone (design-language.md §12): the " +
                "cold seal's field must be exact DeepSurface, not the lit ForgeGlow faded",
            DeepSurface.toArgb(),
            forgeSealField().toArgb(),
        )

        compose.mainClock.autoAdvance = true
        seal.performClick()
        compose.runOnIdle {
            assertEquals("tapping the cold Forge seal must request nothing", 0, forgeRequests)
        }
    }

    @Test
    fun theEmptyGridStateKeepsItsLiteralTextAndTheValknutStaysSemanticsSilent() {
        compose.setContent {
            MaterialTheme {
                EmptyState(
                    "No tmux sessions",
                    "Create a dwarf here, or launch tmux on the visible machine.",
                    ornament = true,
                )
            }
        }

        compose.onNodeWithText("No tmux sessions").assertIsDisplayed()

        val ornament = compose.onNodeWithTag("EmptyStateOrnament")
        ornament.assertHasNoClickAction()
        val spoken = ornament.fetchSemanticsNode()
            .config.getOrNull(SemanticsProperties.ContentDescription)
        assertTrue(
            "the valknut is decorative (design-language.md §7): it must carry no " +
                "content description, but spoke $spoken",
            spoken.isNullOrEmpty(),
        )
    }

    // The card merges its descendants, so the portrait mark, the attention
    // mark, the chip's spoken label, and every line of card text arrive on the
    // one clickable node.
    private fun card(): SemanticsNodeInteraction =
        compose.onNode(hasClickAction() and hasContentDescription(PORTRAIT_DESCRIPTION))

    // The unstruck seal's field, sampled a quarter of the side in on both axes
    // of the node's own image — which is the octagon's square only because the
    // lit proof pins the node to exactly 56dp. That point is inside the 29%
    // octagon (x + y = 0.50 against the corner cut's 0.29) and clear of every
    // stroke: 0.1485 of the side from the nearest frame edge, 0.25 from the
    // stave at x = 0.50, and 0.257 from the crossbar's near end at (0.31, 0.50)
    // — against half-strokes of 0.0134 (frame 1.5dp) and 0.0268 (mark 3dp) at
    // 56dp (forge-seal.md, "Geometry").
    private fun forgeSealField(): Color {
        val pixels = compose.onNodeWithTag("new-agent").captureToImage().toPixelMap()
        return pixels[pixels.width / 4, pixels.height / 4]
    }

    private fun session(kind: SessionStatusKind, attention: Boolean = false) = AgentSession(
        id = SESSION_ID,
        tmuxName = "ga-durinn",
        identityToken = "identity-1",
        character = CharacterSummary(key = "durinn", displayName = "Durinn"),
        profile = "codex",
        objective = "Cut the chrome to tokens",
        cwd = "~/src/skidbladnir",
        activeCommand = "codex",
        attachedClients = 1,
        attention = attention,
        status = SessionStatus(kind = kind, signal = SessionStatusSignal.Lifecycle, signalAt = SIGNAL_AT),
    )

    private fun visibleAgent(session: AgentSession) =
        VisibleAgent(MACHINE, AgentTarget(MACHINE.handle, session))

    private fun machineState(session: AgentSession, receivedAtElapsedMillis: Long) = MachineState(
        machine = MACHINE,
        access = MachineAccess.Ready,
        inventory = InventoryState.Fresh(
            InventorySnapshot(
                inventory = SessionsResponse(
                    machine = MachineSummary(MACHINE.handle, MachinePlatform.Linux),
                    observedAt = OBSERVED_AT,
                    profiles = PROFILES,
                    sessions = listOf(session),
                ),
                receivedAtElapsedMillis = receivedAtElapsedMillis,
            ),
        ),
        pressure = PressureState.Reading,
    )

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val MACHINE = PairedMachine(
            handle = MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")!!,
            label = MachineLabel.parse("Devbox")!!,
            origin = MachineOrigin.parse("https://devbox.example:8443/")!!,
        )
        val PROFILES = listOf(ProfileChoice(key = ProfileKey.parse("codex")!!, label = "Codex"))
        val MINIMUM_TARGET = 48.dp
        val SEAL_SIDE = 56.dp
        // Half a dp: the seal is laid out in whole pixels, so its measured bounds
        // round by under one pixel at any shipped density.
        const val SIDE_TOLERANCE = 0.5f
        val LOZENGE_SIDE = 8.dp
        val SIGNAL_AT: Instant = Instant.parse("2026-08-26T11:57:00Z")
        const val SESSION_ID = "session-durinn"
        const val PORTRAIT_DESCRIPTION = "Portrait of Durinn"
        const val ATTENTION_DESCRIPTION = "Needs attention"
        const val CHIP_EVIDENCE = "lifecycle · 3m"
        val CHIP_LABELS = mapOf(
            SessionStatusKind.Working to "WORKING",
            SessionStatusKind.Running to "RUNNING",
            SessionStatusKind.Idle to "IDLE",
            SessionStatusKind.Shell to "SHELL",
            SessionStatusKind.Unknown to "UNKNOWN",
        )
    }
}
