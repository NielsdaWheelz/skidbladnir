package dev.niels.skidbladnir

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
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
        compose.setContent {
            MaterialTheme {
                AgentCard(
                    session = session(kind),
                    profiles = PROFILES,
                    observedAt = OBSERVED_AT,
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
        compose.setContent {
            MaterialTheme {
                AgentCard(
                    session = session(SessionStatusKind.Working, attention = true),
                    profiles = PROFILES,
                    observedAt = OBSERVED_AT,
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
        compose.setContent {
            MaterialTheme {
                AgentCard(
                    session = session(SessionStatusKind.Idle, attention = true),
                    profiles = PROFILES,
                    observedAt = OBSERVED_AT,
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
    fun theForgeOpensLitInForgeGlow() {
        compose.mainClock.autoAdvance = false
        compose.setContent {
            MaterialTheme {
                ForgeSheet(
                    state = ForgeState(
                        draft = ForgeDraft(cwd = "~", profile = "codex", optionalTmuxName = "", objective = ""),
                        pending = false,
                        error = null,
                    ),
                    profiles = PROFILES,
                    onDismiss = {},
                    onDraftChange = {},
                    onSubmit = {},
                )
            }
        }
        compose.mainClock.advanceTimeBy(5_000)
        val pixels = compose.onNodeWithText("New agent").captureToImage().toPixelMap()
        val corner = pixels[1, 1]
        assertEquals(
            "after the warm-in window the sheet container must be exactly ForgeGlow — " +
                "DeepSurface here means the warm-in stranded or never targeted the lit color",
            ForgeGlow.toArgb(),
            corner.toArgb(),
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

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val PROFILES = listOf(ProfileChoice(key = "codex", label = "Codex"))
        val MINIMUM_TARGET = 48.dp
        val LOZENGE_SIDE = 8.dp
        const val SIGNAL_AT = "2026-08-26T11:57:00Z"
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
