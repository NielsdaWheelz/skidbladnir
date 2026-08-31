package dev.niels.skidbladnir

import android.os.SystemClock
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.PixelMap
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.DpRect
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.time.Instant
import kotlin.math.absoluteValue
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class SessionCardInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun commonCardLeadsWithWorkAndOwnsExactContextInBothScopes() {
        var fixture by mutableStateOf(COMMON_FIXTURE)
        var showMachineLabel by mutableStateOf(true)
        val opened = mutableListOf<String>()
        setCardContent(
            fixture = { fixture },
            showMachineLabel = { showMachineLabel },
            onOpen = { opened += SESSION_ID },
        )

        val card = card()
        val cardBounds = card.getUnclippedBoundsInRoot()
        val tmux = compose.onNodeWithText(TMUX_NAME, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val dwarf = compose.onNodeWithText(DWARF_NAME, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val portrait = compose.onNodeWithContentDescription(PORTRAIT_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val activity = compose.onNodeWithContentDescription(QUIET_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val directory = compose.onNodeWithTag(DIRECTORY_TAG, useUnmergedTree = true)
        val directoryBounds = directory.getUnclippedBoundsInRoot()
        val context = compose.onNodeWithTag(CONTEXT_TAG, useUnmergedTree = true)
        val contextBounds = context.getUnclippedBoundsInRoot()
        val kill = compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true).getUnclippedBoundsInRoot()

        assertTrue(
            "the card must contain its complete operator footer: card=$cardBounds context=$contextBounds kill=$kill",
            cardBounds.contains(contextBounds) && cardBounds.contains(kill),
        )
        context.assertIsDisplayed()
        assertTrue(
            "the exact common card must be no taller than 200dp: card=$cardBounds",
            cardBounds.bottom - cardBounds.top <= MAX_COMMON_HEIGHT,
        )
        val text = card.textValues()
        assertTrue("tmux must precede dwarf in traversal order: $text", text.indexOf(TMUX_NAME) < text.indexOf(DWARF_NAME))
        assertTrue("literal activity disappeared: $text", text.contains("QUIET"))
        assertTrue(
            "work identity must lead the quieter persona visually: tmux=$tmux dwarf=$dwarf",
            tmux.top < dwarf.top && tmux.bottom <= dwarf.top,
        )
        assertSquare("portrait", portrait, MINIMUM_TARGET)
        assertTrue("activity bay lost its 48dp floor: $activity", activity.bottom - activity.top >= MINIMUM_TARGET)
        assertTrue(
            "portrait and named activity must occupy the same row: portrait=$portrait activity=$activity",
            portrait.top < activity.bottom && activity.top < portrait.bottom,
        )

        directory.assertTextAndDescription("/src/skidbladnir", DIRECTORY_DESCRIPTION)
        assertEquals(
            "the merged card must speak its complete directory exactly once",
            1,
            card.contentDescriptions().count { it == DIRECTORY_DESCRIPTION },
        )
        assertTrue("directory must remain one line: bounds=$directoryBounds", directoryBounds.height <= ONE_DATA_LINE)
        context.assertContext(ALL_CONTEXT, CONTEXT_DESCRIPTION)
        card.assertMergedContext(CONTEXT_DESCRIPTION)
        assertTrue("footer context must remain one line: bounds=$contextBounds", contextBounds.height <= ONE_DATA_LINE)
        assertTrue(
            "Kill must retain its 48dp target without footer overlap: context=$contextBounds kill=$kill",
            kill.width >= MINIMUM_TARGET && kill.height >= MINIMUM_TARGET && contextBounds.right <= kill.left,
        )
        card.assertIsEnabled()
        compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true).assertIsEnabled()

        compose.runOnIdle { showMachineLabel = false }
        context.assertContext(FILTERED_CONTEXT, CONTEXT_DESCRIPTION)
        compose.runOnIdle {
            fixture = fixture.copy(
                agent = AgentRuntime(
                    provider = AgentProvider.Claude,
                    pid = 2345,
                    providerSession = ProviderSessionFacts.withId("provider-id", name = "provider-name"),
                ),
            )
        }
        context.assertContext(UNKNOWN_AGENT_PROFILE, UNKNOWN_AGENT_CONTEXT_DESCRIPTION)
        assertTrue(
            "the compact card exposed raw provider identity or PID",
            card.textValues().none { it.contains("provider-id") || it.contains("provider-name") || it.contains("2345") } &&
                card.contentDescriptions().none {
                    it.contains("provider-id") || it.contains("provider-name") || it.contains("2345")
                },
        )
        compose.runOnIdle { fixture = fixture.copy(launchProfile = WORK_PROFILE, agent = null) }
        context.assertContext(FILTERED_CONTEXT, CONTEXT_DESCRIPTION)
        compose.runOnIdle { fixture = fixture.copy(launchProfile = null, agent = null) }
        context.assertContext(PROFILE_UNKNOWN, UNKNOWN_CONTEXT_DESCRIPTION)

        card.performClick()
        compose.runOnIdle {
            assertEquals("the card must open its exact session once", listOf(SESSION_ID), opened)
        }
    }

    @Test
    fun activityIsLiteralSpokenAndMotionRespectsFreshnessAndReducedMotion() {
        compose.mainClock.autoAdvance = false
        var fixture by mutableStateOf(COMMON_FIXTURE.copy(motionEnabled = true))
        setCardContent(fixture = { fixture })
        compose.mainClock.advanceTimeByFrame()

        assertActivity("QUIET", QUIET_DESCRIPTION, Muted)
        val quietBefore = facet().captureToImage().toPixelMap()
        compose.mainClock.advanceTimeBy(400)
        val quietAfter = facet().captureToImage().toPixelMap()
        assertTrue("Quiet activity facet must be static", quietBefore.samePixels(quietAfter))

        compose.runOnIdle { fixture = fixture.copy(activity = SessionActivity.Active) }
        compose.mainClock.advanceTimeByFrame()
        assertActivity("ACTIVE", ACTIVE_DESCRIPTION, Moss)
        val activeBefore = facet().captureToImage().toPixelMap()
        compose.mainClock.advanceTimeBy(300)
        val activeQuarterTurn = facet().captureToImage().toPixelMap()
        assertFalse(
            "fresh Active activity must carry the one restrained spinner",
            activeBefore.samePixels(activeQuarterTurn),
        )

        compose.runOnIdle { fixture = fixture.copy(motionEnabled = false) }
        compose.mainClock.advanceTimeByFrame()
        val reducedBefore = facet().captureToImage().toPixelMap()
        compose.mainClock.advanceTimeBy(400)
        val reducedAfter = facet().captureToImage().toPixelMap()
        assertTrue("reduced motion must make Active static", reducedBefore.samePixels(reducedAfter))

        compose.runOnIdle { fixture = fixture.copy(stale = true, motionEnabled = true) }
        compose.mainClock.advanceTimeByFrame()
        compose.onNodeWithContentDescription(RETAINED_ACTIVE_DESCRIPTION, useUnmergedTree = true).assertIsDisplayed()
        compose.onNodeWithText(STALE_MARKER, useUnmergedTree = true).assertIsDisplayed()
        val staleBefore = facet().captureToImage().toPixelMap()
        compose.mainClock.advanceTimeBy(400)
        val staleAfter = facet().captureToImage().toPixelMap()
        assertTrue("retained Active activity must never animate", staleBefore.samePixels(staleAfter))
        card().assertIsNotEnabled()
        compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true).assertIsNotEnabled()

        compose.runOnIdle { fixture = fixture.copy(activity = SessionActivity.Quiet) }
        compose.mainClock.advanceTimeByFrame()
        compose.onNodeWithContentDescription(RETAINED_QUIET_DESCRIPTION, useUnmergedTree = true).assertIsDisplayed()

        val rendered = card().textValues()
        for (retired in listOf("WORKING", "READY", "NEEDS YOU", "AGENT OPEN", "NEW RESULT")) {
            assertFalse("retired state label survived: $rendered", rendered.contains(retired))
        }
        compose.onNodeWithContentDescription("New result", useUnmergedTree = true).assertDoesNotExist()
    }

    @Test
    fun minimumWidthLongContentAndLargeTypeKeepEveryStratumInsideTheCard() {
        val fixture = LONG_FIXTURE
        var fontScale by mutableStateOf(1f)
        setCardContent(fixture = { fixture }, fontScale = { fontScale })
        assertEquals("the long tmux fixture must exercise the protocol maximum", 64, fixture.tmuxName.length)
        assertEquals("the long machine fixture must exercise the product maximum", 40, fixture.machineLabel.length)

        val tmux = compose.onNodeWithText(LONG_TMUX, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val dwarf = compose.onNodeWithText(LONG_DWARF, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val objective = compose.onNodeWithTag(OBJECTIVE_TAG, useUnmergedTree = true)
        val directory = compose.onNodeWithTag(DIRECTORY_TAG, useUnmergedTree = true)
        val context = compose.onNodeWithTag(CONTEXT_TAG, useUnmergedTree = true)
        val kill = compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true)

        assertTrue("long tmux must stop at two lines: bounds=$tmux", tmux.height <= TWO_TITLE_LINES)
        assertTrue("long dwarf name must stop at one line: bounds=$dwarf", dwarf.height <= ONE_DWARF_LINE)
        assertTrue("objective must stop at two lines", objective.getUnclippedBoundsInRoot().height <= TWO_BODY_LINES)
        assertTrue("directory must stop at one line", directory.getUnclippedBoundsInRoot().height <= ONE_DATA_LINE)
        assertTrue("footer must stop at one line", context.getUnclippedBoundsInRoot().height <= ONE_DATA_LINE)
        assertTrue(
            "the long footer must truncate before Kill",
            context.getUnclippedBoundsInRoot().right <= kill.getUnclippedBoundsInRoot().left,
        )
        assertEquals(listOf(LONG_OBJECTIVE), objective.textValues())
        directory.assertTextAndDescription(LONG_DIRECTORY_VISIBLE, LONG_DIRECTORY_DESCRIPTION)
        context.assertContext(LONG_CONTEXT, LONG_CONTEXT_DESCRIPTION)

        compose.runOnIdle { fontScale = LARGE_FONT_SCALE }
        val card = card().getUnclippedBoundsInRoot()
        val largeTmux = compose.onNodeWithText(LONG_TMUX, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val largeDwarf = compose.onNodeWithText(LONG_DWARF, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val largePortrait = compose.onNodeWithContentDescription("Portrait of $LONG_DWARF", useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val activity = compose.onNodeWithContentDescription(QUIET_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val largeObjective = objective.getUnclippedBoundsInRoot()
        val largeDirectory = directory.getUnclippedBoundsInRoot()
        val largeContext = context.getUnclippedBoundsInRoot()
        val largeKill = kill.getUnclippedBoundsInRoot()
        val children = listOf(
            largeTmux, largeDwarf, largePortrait, activity, largeObjective, largeDirectory, largeContext, largeKill,
        )
        assertTrue(
            "large type may grow the card but no stratum may clip outside it: card=$card children=$children",
            children.all { child -> card.contains(child) },
        )
        assertTrue(
            "large type must preserve activity/objective/directory/footer order: children=$children",
            largeTmux.bottom <= largeDwarf.top && largeDwarf.bottom <= activity.top &&
                activity.bottom <= largeObjective.top && largeObjective.bottom <= largeDirectory.top &&
                largeDirectory.bottom <= largeContext.top && largeContext.right <= largeKill.left,
        )
    }

    private fun assertActivity(label: String, spoken: String, tone: androidx.compose.ui.graphics.Color) {
        compose.onNodeWithText(label, useUnmergedTree = true).assertIsDisplayed()
        compose.onNodeWithContentDescription(spoken, useUnmergedTree = true).assertIsDisplayed()
        val facet = facet()
        val config = facet.fetchSemanticsNode().config
        assertTrue(
            "the redundant facet must expose no text, description, or role: $config",
            config.getOrNull(SemanticsProperties.Text).isNullOrEmpty() &&
                config.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty() &&
                config.getOrNull(SemanticsProperties.Role) == null,
        )
        assertFacetPosition(facet.getUnclippedBoundsInRoot(), card().getUnclippedBoundsInRoot())
        val pixels = facet.captureToImage().toPixelMap()
        assertEquals("the $label facet center must carry its activity tone", tone.toArgb(), pixels.center().toArgb())
    }

    private fun setCardContent(
        fixture: () -> CardFixture,
        showMachineLabel: () -> Boolean = { true },
        fontScale: () -> Float = { 1f },
        onOpen: () -> Unit = {},
    ) {
        compose.setContent {
            MaterialTheme {
                val density = LocalDensity.current
                CompositionLocalProvider(
                    LocalDensity provides Density(density.density, fontScale()),
                    LocalContentColor provides Bone,
                ) {
                    Box(Modifier.width(CARD_WIDTH).background(Ink)) {
                        val current = fixture()
                        val session = current.session()
                        SessionCard(
                            visibleSession = VisibleSession(current.machine(), SessionTarget(MACHINE_HANDLE, session)),
                            machine = current.machineState(session),
                            showMachineLabel = showMachineLabel(),
                            motionEnabled = current.motionEnabled,
                            onOpen = onOpen,
                            onKill = { error("rendering a card killed its session") },
                        )
                    }
                }
            }
        }
    }

    private fun CardFixture.machine() = PairedMachine(
        handle = MACHINE_HANDLE,
        label = requireNotNull(MachineLabel.parse(machineLabel)),
        origin = MACHINE_ORIGIN,
    )

    private fun CardFixture.session() = TmuxSession(
        tmuxId = SESSION_ID,
        tmuxName = tmuxName,
        identityToken = "identity-1",
        character = CharacterSummary(key = "durinn", displayName = dwarfName),
        launchProfile = launchProfile,
        objective = objective,
        cwd = cwd,
        activeCommand = "codex",
        attachedClients = 1,
        activity = activity,
        agent = agent,
    )

    private fun CardFixture.machineState(session: TmuxSession): MachineState {
        val snapshot = InventorySnapshot(
            SessionsResponse(
                machine = MachineSummary(MACHINE_HANDLE, MachinePlatform.Linux),
                observedAt = OBSERVED_AT,
                profiles = listOf(
                    ProfileChoice(WORK_PROFILE, profileLabel, AgentProvider.Codex),
                    ProfileChoice(PERSONAL_PROFILE, "Codex · Personal", AgentProvider.Codex),
                ),
                sessions = listOf(session),
            ),
            receivedAtElapsedMillis = SystemClock.elapsedRealtime(),
        )
        return MachineState(
            machine = machine(),
            access = MachineAccess.Ready,
            inventory = if (stale) {
                InventoryState.Stale(snapshot, GatewayFailure.Transport)
            } else {
                InventoryState.Fresh(snapshot)
            },
            pressure = PressureState.Reading,
        )
    }

    private fun card() = compose.onNodeWithTag(CARD_TAG)
    private fun facet() = compose.onNodeWithTag(FACET_TAG, useUnmergedTree = true)

    private fun SemanticsNodeInteraction.textValues(): List<String> =
        fetchSemanticsNode().config.getOrNull(SemanticsProperties.Text)?.map { it.text }.orEmpty()

    private fun SemanticsNodeInteraction.contentDescriptions(): List<String> =
        fetchSemanticsNode().config.getOrNull(SemanticsProperties.ContentDescription).orEmpty()

    private fun SemanticsNodeInteraction.assertTextAndDescription(visible: String, spoken: String) {
        assertEquals("visible text", listOf(visible), textValues())
        assertEquals("spoken description", listOf(spoken), contentDescriptions())
    }

    private fun SemanticsNodeInteraction.assertContext(visible: String, spoken: String) {
        assertEquals("footer visible context", listOf(visible), textValues())
        assertEquals("footer spoken context", listOf(spoken), contentDescriptions())
    }

    private fun SemanticsNodeInteraction.assertMergedContext(spoken: String) {
        assertEquals(
            "the merged card must contain exactly one footer-owned context description",
            1,
            contentDescriptions().count { it == spoken },
        )
    }

    private fun assertFacetPosition(facet: DpRect, card: DpRect) {
        assertSquare("activity facet", facet, FACET_SIDE)
        assertTrue(
            "facet must stay at the top-trailing 10dp inset: facet=$facet card=$card",
            ((facet.top - card.top) - CARD_PADDING).value.absoluteValue <= POSITION_TOLERANCE &&
                ((card.right - facet.right) - CARD_PADDING).value.absoluteValue <= POSITION_TOLERANCE,
        )
    }

    private fun assertSquare(label: String, bounds: DpRect, side: androidx.compose.ui.unit.Dp) {
        assertTrue(
            "$label must be exactly $side square: bounds=$bounds",
            (bounds.width - side).value.absoluteValue <= POSITION_TOLERANCE &&
                (bounds.height - side).value.absoluteValue <= POSITION_TOLERANCE,
        )
    }

    private fun DpRect.contains(child: DpRect): Boolean =
        child.left >= left && child.top >= top && child.right <= right && child.bottom <= bottom

    private data class CardFixture(
        val machineLabel: String,
        val tmuxName: String,
        val dwarfName: String,
        val launchProfile: ProfileKey?,
        val agent: AgentRuntime?,
        val profileLabel: String,
        val objective: String?,
        val cwd: String,
        val activity: SessionActivity,
        val motionEnabled: Boolean = false,
        val stale: Boolean = false,
    )

    private companion object {
        val MACHINE_HANDLE = requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"))
        val MACHINE_ORIGIN = requireNotNull(MachineOrigin.parse("https://devbox.example:8443/"))
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val CARD_WIDTH = 170.dp
        val MAX_COMMON_HEIGHT = 200.dp
        val MINIMUM_TARGET = 48.dp
        val FACET_SIDE = 12.dp
        val CARD_PADDING = 10.dp
        val ONE_DATA_LINE = 17.dp
        val ONE_DWARF_LINE = 17.dp
        val TWO_TITLE_LINES = 49.dp
        val TWO_BODY_LINES = 41.dp
        const val POSITION_TOLERANCE = 0.5f
        const val LARGE_FONT_SCALE = 2.0f
        const val SESSION_ID = "session-durinn"
        const val TMUX_NAME = "ga-durinn"
        const val DWARF_NAME = "Durinn"
        const val ALL_CONTEXT = "Devbox · Codex · Work"
        const val FILTERED_CONTEXT = "Codex · Work"
        const val PROFILE_UNKNOWN = "profile unknown"
        const val CONTEXT_DESCRIPTION = "Machine Devbox. Profile Codex · Work."
        const val UNKNOWN_AGENT_PROFILE = "Claude · profile unknown"
        const val UNKNOWN_AGENT_CONTEXT_DESCRIPTION =
            "Machine Devbox. Profile Claude · profile unknown."
        const val UNKNOWN_CONTEXT_DESCRIPTION = "Machine Devbox. Profile profile unknown."
        const val PORTRAIT_DESCRIPTION = "Portrait of Durinn"
        const val DIRECTORY_DESCRIPTION = "Directory /src/skidbladnir"
        const val ACTIVE_DESCRIPTION = "Recent tmux activity at the last check"
        const val QUIET_DESCRIPTION = "No recent tmux activity at the last check"
        const val RETAINED_ACTIVE_DESCRIPTION = "Last observed: recent tmux activity"
        const val RETAINED_QUIET_DESCRIPTION = "Last observed: no recent tmux activity"
        const val STALE_MARKER = "STALE · actions disabled"
        const val LONG_TMUX = "skidbladnir-codex-work-12345678901234567890123456789012345678900"
        const val LONG_DWARF = "Alberich of Nibelheim"
        const val LONG_OBJECTIVE = "Refactor the dashboard card layout without changing terminal behavior."
        const val LONG_DIRECTORY = "/srv/workspaces/skidbladnir/android"
        const val LONG_DIRECTORY_VISIBLE = "…/skidbladnir/android"
        const val LONG_DIRECTORY_DESCRIPTION = "Directory /srv/workspaces/skidbladnir/android"
        const val LONG_MACHINE = "MacBook Pro Across The Far Tailnet Realm"
        const val LONG_CONTEXT = "MacBook Pro Across The Far Tailnet Realm · Codex · Work"
        const val LONG_CONTEXT_DESCRIPTION =
            "Machine MacBook Pro Across The Far Tailnet Realm. Profile Codex · Work."
        const val CARD_TAG = "session-card-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val FACET_TAG = "session-activity-facet-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val DIRECTORY_TAG = "session-directory-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val OBJECTIVE_TAG = "session-objective-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val CONTEXT_TAG = "session-context-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val KILL_TAG = "session-kill-mh-0123456789abcdef0123456789abcdef-session-durinn"

        val WORK_PROFILE = requireNotNull(ProfileKey.parse("work"))
        val PERSONAL_PROFILE = requireNotNull(ProfileKey.parse("personal"))
        val COMMON_AGENT = AgentRuntime(AgentProvider.Codex, 1234, profile = WORK_PROFILE)
        val COMMON_FIXTURE = CardFixture(
            machineLabel = "Devbox",
            tmuxName = TMUX_NAME,
            dwarfName = DWARF_NAME,
            launchProfile = PERSONAL_PROFILE,
            agent = COMMON_AGENT,
            profileLabel = "Codex · Work",
            objective = null,
            cwd = "/src/skidbladnir",
            activity = SessionActivity.Quiet,
        )
        val LONG_FIXTURE = CardFixture(
            machineLabel = LONG_MACHINE,
            tmuxName = LONG_TMUX,
            dwarfName = LONG_DWARF,
            launchProfile = WORK_PROFILE,
            agent = null,
            profileLabel = "Codex · Work",
            objective = LONG_OBJECTIVE,
            cwd = LONG_DIRECTORY,
            activity = SessionActivity.Quiet,
        )
    }
}

private val DpRect.width get() = right - left
private val DpRect.height get() = bottom - top
private fun PixelMap.center() = this[width / 2, height / 2]
private fun PixelMap.samePixels(other: PixelMap): Boolean =
    width == other.width && height == other.height &&
        IntArray(width * height) { index -> this[index % width, index / width].toArgb() }
            .contentEquals(
                IntArray(other.width * other.height) { index ->
                    other[index % other.width, index / other.width].toArgb()
                },
            )
