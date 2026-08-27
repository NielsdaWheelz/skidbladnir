package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PixelMap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.test.assertHasClickAction
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
import androidx.compose.ui.unit.IntRect
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
    fun theDashboardTopBarMarkLeadsTheLiteralTitleAndStaysSemanticsSilent() {
        compose.setContent {
            MaterialTheme {
                DashboardTopBar(
                    summary = "4 tmux sessions across 2 machines",
                    refreshing = false,
                    canForge = true,
                    onRefresh = {},
                    onNewAgent = { error("rendering the top bar opened the Forge") },
                )
            }
        }

        compose.onNodeWithText("Dwarves").assertIsDisplayed()

        val mark = compose.onNodeWithTag("dashboard-mark")
        mark.assertHasNoClickAction()
        val config = mark.fetchSemanticsNode().config
        assertTrue(
            "the Hlíðskjálf mark is decoration (design-language.md §8): the literal title " +
                "beside it carries the whole meaning, so the mark must add nothing spoken, " +
                "but it offered ${config.getOrNull(SemanticsProperties.ContentDescription)} " +
                "and ${config.getOrNull(SemanticsProperties.Text)}",
            config.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty() &&
                config.getOrNull(SemanticsProperties.Text).isNullOrEmpty(),
        )

        val title = compose.onNodeWithTag("dashboard-title", useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val markBounds = mark.getUnclippedBoundsInRoot()
        assertTrue(
            "the mark must lead the title on the one compact row, not stack above it",
            markBounds.right <= title.left && markBounds.top < title.bottom && title.top < markBounds.bottom,
        )
    }

    @Test
    fun theTopBarStaysOneRowWithTheMarkLeadingAndCreateStillTrailing() {
        // The mark took 32dp (24dp glyph + 8dp arrangement spacing) out of a row that is a
        // fixed 64dp tall and already carried title, Refresh and the create action. The
        // equivalent assertions in MultiMachineUiInstrumentedTest sit behind an assumeTrue
        // for provisioned machines, so they do not run on an unprovisioned device — this
        // composes the bar directly so the crowding is proved on every run.
        compose.setContent {
            MaterialTheme {
                DashboardTopBar(
                    summary = "4 tmux sessions across 2 machines",
                    refreshing = false,
                    canForge = true,
                    onRefresh = {},
                    onNewAgent = { error("rendering the top bar opened the Forge") },
                )
            }
        }

        compose.onNodeWithText("Refresh").assertIsDisplayed()
        compose.onNodeWithTag("new-agent").assertIsDisplayed()

        val bar = compose.onNodeWithTag("dashboard-top-bar").getUnclippedBoundsInRoot()
        val mark = compose.onNodeWithTag("dashboard-mark", useUnmergedTree = true).getUnclippedBoundsInRoot()
        val title = compose.onNodeWithTag("dashboard-title", useUnmergedTree = true).getUnclippedBoundsInRoot()
        val create = compose.onNodeWithTag("new-agent").getUnclippedBoundsInRoot()

        assertTrue(
            "the create action must stay inside the one 64dp row, not be pushed out of it by " +
                "the mark: bar=$bar create=$create",
            create.top >= bar.top && create.bottom <= bar.bottom,
        )
        assertTrue(
            "mark, title and create action must share one row rather than stack: " +
                "mark=$mark title=$title create=$create",
            create.top < title.bottom && title.top < create.bottom &&
                mark.top < title.bottom && title.top < mark.bottom,
        )
        assertTrue(
            "reading order across the row is mark, then title, then create action: " +
                "mark=$mark title=$title create=$create",
            mark.right <= title.left && title.right <= create.left,
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

    // The destructive control is the only shape in the product whose corners
    // disagree: one corner is cleaved away while the rest keep the chip facet.
    // That mark is geometry, not colour, so it survives greyscale and a
    // colour-blind reading where a warm tint does not. Ground truth is the Ink
    // the control is rendered on, so the proof is "equals the ground" against
    // "does not", never a blended fill constant — and it reads the asymmetry
    // itself, not the cleft's depth or which corner carries it, both of which
    // are free to be retuned.
    @Test
    fun theKillControlRendersAsAnAsymmetricallyCleavedChip() {
        compose.mainClock.autoAdvance = false
        compose.setContent {
            MaterialTheme {
                Box(Modifier.background(Ink).padding(GROUND_MARGIN).testTag(KILL_GROUND)) {
                    KillButton(
                        machineLabel = MACHINE.label,
                        target = killTarget(),
                        enabled = true,
                        onClick = { error("measuring the kill control killed a session") },
                    )
                }
            }
        }

        compose.mainClock.advanceTimeByFrame()
        val pixels = compose.onNodeWithTag(KILL_GROUND).captureToImage().toPixelMap()
        val material = pixels.materialBounds()
        val inset = with(compose.density) { CORNER_SAMPLE.roundToPx() }
        val ground = Ink.argbHex()
        val corners = mapOf(
            "top-start" to pixels.block(material.left + inset, material.top + inset),
            "top-end" to pixels.block(material.right - 1 - inset, material.top + inset),
            "bottom-start" to pixels.block(material.left + inset, material.bottom - 1 - inset),
            "bottom-end" to pixels.block(material.right - 1 - inset, material.bottom - 1 - inset),
        )
        val cleaved = corners.filterValues { corner -> corner.all { it == ground } }.keys

        assertEquals(
            "the kill control is the only shape in the product whose corners disagree, and that " +
                "mark survives greyscale where colour does not: sampled $CORNER_SAMPLE in, exactly " +
                "one corner must read bare ground and the other three must read material. Four " +
                "corners reading ground is an evenly cut box and none is the plain chip every " +
                "other control wears; neither one marks kill apart. Corners over bare ground: " +
                "$cleaved, sampled over $material as $corners",
            1,
            cleaved.size,
        )
    }

    // Every kill button on the grid speaks an identical bare "Kill" today, so a
    // screen-reader user cannot tell one session's kill from another's. The
    // disabled control keeps its OnClick semantics — Modifier.clickable adds
    // the action alongside disabled() even when enabled = false — so the real
    // contract is not-enabled plus a tap that reaches no handler, and the
    // dropped hairline is the non-opacity cue design-language.md §12 requires.
    @Test
    fun theKillControlSpeaksItsTargetAndDropsItsHairlineWhenDisabled() {
        compose.mainClock.autoAdvance = false
        val killed = mutableListOf<String>()
        compose.setContent {
            MaterialTheme {
                Column {
                    Box(Modifier.background(Ink).padding(GROUND_MARGIN).testTag(KILL_GROUND)) {
                        KillButton(
                            machineLabel = MACHINE.label,
                            target = killTarget(),
                            enabled = true,
                            onClick = { killed += "enabled" },
                            modifier = Modifier.testTag(ENABLED_KILL),
                        )
                    }
                    Box(Modifier.background(Ink).padding(GROUND_MARGIN).testTag(DISABLED_GROUND)) {
                        KillButton(
                            machineLabel = MACHINE.label,
                            target = killTarget(),
                            enabled = false,
                            onClick = { killed += "disabled" },
                            modifier = Modifier.testTag(DISABLED_KILL),
                        )
                    }
                }
            }
        }

        // Captured before any press: AngularIndication would flash Bone over
        // the material and the hairline is what this half of the proof reads.
        compose.mainClock.advanceTimeByFrame()
        // noticeToneColor is severity's sole owner, so the hairline is read
        // against the tone it is drawn from: naming Ember here would turn this
        // device-gated proof red for a retune of the Failure hue alone.
        val failure = noticeToneColor(NoticeTone.Failure).argbHex()
        val armed = compose.onNodeWithTag(KILL_GROUND).captureToImage().toPixelMap().startEdge()
        val inert = compose.onNodeWithTag(DISABLED_GROUND).captureToImage().toPixelMap().startEdge()
        assertTrue(
            "an enabled kill control carries a 1dp hairline in the Failure tone; " +
                "its start edge read $armed",
            armed.any { it == failure },
        )
        assertTrue(
            "disabled must be readable without relying on opacity (design-language.md §12): " +
                "dropping the hairline is that cue, but the start edge still read the Failure " +
                "tone in $inert",
            inert.none { it == failure },
        )

        val enabled = compose.onNodeWithTag(ENABLED_KILL)
        enabled.assertIsEnabled().assertHasClickAction()
        assertEquals(
            "TalkBack must announce the app's only destructive control as a button: the " +
                "TextButton this replaces carried Role.Button for free, and a hand-built " +
                "Surface without it reports android.view.View rather than android.widget.Button",
            Role.Button,
            enabled.fetchSemanticsNode().config.getOrNull(SemanticsProperties.Role),
        )
        assertEquals(
            "a grid of kill buttons that all speak a bare \"Kill\" is indistinguishable to a " +
                "screen reader: each control must speak its own tmux session and machine",
            listOf(KILL_DESCRIPTION),
            enabled.fetchSemanticsNode().config.getOrNull(SemanticsProperties.ContentDescription).orEmpty(),
        )
        enabled.performClick()
        compose.runOnIdle {
            assertEquals(
                "tapping the enabled kill control must request exactly that kill once",
                listOf("enabled"),
                killed,
            )
        }

        val disabled = compose.onNodeWithTag(DISABLED_KILL)
        disabled.assertIsNotEnabled()
        disabled.performClick()
        compose.runOnIdle {
            assertEquals(
                "a kill control on a machine that cannot mutate must not fire when tapped",
                listOf("enabled"),
                killed,
            )
        }
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

    private fun killTarget() = AgentTarget(MACHINE.handle, session(SessionStatusKind.Working))

    // KillButton's minimumInteractiveComponentSize() pads its layout node out
    // to the 48dp touch target, so the captured image is wider and taller than
    // the material drawn inside it. Ink is the known ground, so the drawn
    // rectangle is the bounding box of everything that is not Ink — a cut
    // corner only shortens the two edges meeting there, it never moves them.
    private fun PixelMap.materialBounds(): IntRect {
        val ground = Ink.toArgb()
        var left = width
        var top = height
        var right = -1
        var bottom = -1
        for (y in 0 until height) {
            for (x in 0 until width) {
                if (this[x, y].toArgb() == ground) continue
                if (x < left) left = x
                if (y < top) top = y
                if (x > right) right = x
                if (y > bottom) bottom = y
            }
        }
        assertTrue("the kill control drew nothing on the ${width}x$height ground", right >= left)
        return IntRect(left, top, right + 1, bottom + 1)
    }

    // Every corner sample sits at least 1.4dp clear of a cut edge, so a 3x3
    // block around the target point stays off the diagonal's antialiasing
    // while still failing if the cut lands a pixel out.
    private fun PixelMap.block(x: Int, y: Int): List<String> =
        (-1..1).flatMap { dy -> (-1..1).map { dx -> this[x + dx, y + dy].argbHex() } }

    // The hairline is 1dp of the Failure tone on the shape's straight start
    // edge, between the two cut corners; the outermost column can be blended
    // by the capture, so the sample is the first three columns of material at
    // mid-height.
    private fun PixelMap.startEdge(): List<String> {
        val material = materialBounds()
        val y = (material.top + material.bottom) / 2
        return (0..2).map { this[material.left + it, y].argbHex() }
    }

    // Samples are compared and reported as ARGB hex: a raw channel Int prints
    // as a negative decimal and tells a failing run nothing.
    private fun Color.argbHex(): String = Integer.toHexString(toArgb())

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val MACHINE = PairedMachine(
            handle = MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")!!,
            label = MachineLabel.parse("Devbox")!!,
            origin = MachineOrigin.parse("https://devbox.example:8443/")!!,
        )
        val PROFILES = listOf(ProfileChoice(key = ProfileKey.parse("codex")!!, label = "Codex"))
        val MINIMUM_TARGET = 48.dp
        val GROUND_MARGIN = 8.dp
        val CORNER_SAMPLE = 3.dp
        val LOZENGE_SIDE = 8.dp
        val SIGNAL_AT: Instant = Instant.parse("2026-08-26T11:57:00Z")
        const val SESSION_ID = "session-durinn"
        const val PORTRAIT_DESCRIPTION = "Portrait of Durinn"
        const val ATTENTION_DESCRIPTION = "Needs attention"
        const val KILL_DESCRIPTION = "Kill ga-durinn on Devbox"
        const val KILL_GROUND = "kill-ground"
        const val DISABLED_GROUND = "disabled-kill-ground"
        const val ENABLED_KILL = "enabled-kill"
        const val DISABLED_KILL = "disabled-kill"
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
