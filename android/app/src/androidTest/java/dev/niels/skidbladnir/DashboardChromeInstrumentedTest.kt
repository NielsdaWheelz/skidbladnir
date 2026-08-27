package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
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
