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
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assertIsDisplayed
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
import kotlin.math.sqrt
import org.junit.Assert.assertEquals
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
        val evidence = compose.onNodeWithText("lifecycle · 3m", useUnmergedTree = true).getUnclippedBoundsInRoot()
        val portrait = compose.onNodeWithContentDescription(PORTRAIT_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val status = compose.onNodeWithContentDescription(WORKING_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val directory = compose.onNodeWithTag(DIRECTORY_TAG, useUnmergedTree = true)
        val directoryBounds = directory.getUnclippedBoundsInRoot()
        val context = compose.onNodeWithTag(CONTEXT_TAG, useUnmergedTree = true).assertIsDisplayed()
        val contextBounds = context.getUnclippedBoundsInRoot()
        val kill = compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true).getUnclippedBoundsInRoot()
        assertTrue(
            "the exact common card must be no taller than 200dp: card=$cardBounds " +
                "tmux=$tmux dwarf=$dwarf status=$status portrait=$portrait evidence=$evidence " +
                "directory=$directoryBounds context=$contextBounds kill=$kill",
            cardBounds.bottom - cardBounds.top <= MAX_COMMON_HEIGHT,
        )
        val text = card.textValues()
        assertTrue("tmux must precede dwarf in traversal order: $text", text.indexOf(TMUX_NAME) < text.indexOf(DWARF_NAME))
        assertTrue("literal status and evidence disappeared: $text", text.containsAll(listOf("WORKING", "lifecycle · 3m")))
        assertTrue("common status evidence must remain on one line: bounds=$evidence", evidence.bottom - evidence.top <= ONE_DATA_LINE)

        assertTrue(
            "work identity must lead the quieter persona visually: tmux=$tmux dwarf=$dwarf",
            tmux.top < dwarf.top && tmux.bottom <= dwarf.top,
        )

        assertSquare("portrait", portrait, MINIMUM_TARGET)
        assertTrue("status bay lost its 48dp floor: $status", status.bottom - status.top >= MINIMUM_TARGET)
        assertTrue(
            "portrait and named status must occupy the same row: portrait=$portrait status=$status",
            portrait.top < status.bottom && status.top < portrait.bottom,
        )

        directory.assertTextAndDescription("/src/skidbladnir", DIRECTORY_DESCRIPTION)
        assertEquals(
            "the merged card must speak its complete directory exactly once",
            1,
            card.contentDescriptions().count { it == DIRECTORY_DESCRIPTION },
        )
        assertTrue(
            "directory must remain one line: bounds=$directoryBounds",
            directoryBounds.bottom - directoryBounds.top <= ONE_DATA_LINE,
        )

        context.assertContext(ALL_CONTEXT, CONTEXT_DESCRIPTION)
        card.assertMergedContext(CONTEXT_DESCRIPTION)
        assertTrue(
            "footer context must remain one line: bounds=$contextBounds",
            contextBounds.bottom - contextBounds.top <= ONE_DATA_LINE,
        )

        assertTrue(
            "Kill must retain its 48dp target without footer overlap: context=$contextBounds kill=$kill",
            kill.right - kill.left >= MINIMUM_TARGET && kill.bottom - kill.top >= MINIMUM_TARGET &&
                contextBounds.right <= kill.left,
        )

        compose.runOnIdle { showMachineLabel = false }
        context.assertContext(FILTERED_CONTEXT, CONTEXT_DESCRIPTION)
        card.assertMergedContext(CONTEXT_DESCRIPTION)
        compose.runOnIdle {
            fixture = fixture.copy(
                agent = AgentRuntime(
                    provider = AgentProvider.Claude,
                    pid = 2345,
                    providerSession = ProviderSessionFacts.withId("provider-id", name = "provider-name"),
                ),
                statusKind = SessionStatusKind.Running,
            )
        }
        context.assertContext(UNKNOWN_RUNTIME_PROFILE, UNKNOWN_RUNTIME_CONTEXT_DESCRIPTION)
        assertTrue(
            "the compact card exposed raw provider identity or PID",
            card.textValues().none { it.contains("provider-id") || it.contains("provider-name") || it.contains("2345") } &&
                card.contentDescriptions().none {
                    it.contains("provider-id") || it.contains("provider-name") || it.contains("2345")
                },
        )
        compose.runOnIdle {
            fixture = fixture.copy(
                agent = null,
                launchProfile = WORK_PROFILE,
                statusKind = SessionStatusKind.Shell,
            )
        }
        context.assertContext(FILTERED_CONTEXT, CONTEXT_DESCRIPTION)
        compose.runOnIdle {
            fixture = fixture.copy(launchProfile = null, statusKind = SessionStatusKind.Unknown)
        }
        context.assertContext(PROFILE_UNKNOWN, UNKNOWN_CONTEXT_DESCRIPTION)

        card.performClick()
        compose.runOnIdle {
            assertEquals("the card must open its exact session once", listOf(SESSION_ID), opened)
        }
    }

    @Test
    fun statusFacetCarriesEveryToneSilentlyAndAttentionNeverMovesIt() {
        var fixture by mutableStateOf(COMMON_FIXTURE)
        setCardContent(fixture = { fixture })

        val cardBounds = card().getUnclippedBoundsInRoot()
        val quietFacet = facet().getUnclippedBoundsInRoot()
        assertFacetPosition(quietFacet, cardBounds)

        STATUS_FIXTURES.forEach { (kind, expected) ->
            compose.runOnIdle {
                fixture = fixture.copy(
                    statusKind = kind,
                    agent = when (kind) {
                        SessionStatusKind.Working, SessionStatusKind.Running, SessionStatusKind.Idle ->
                            COMMON_FIXTURE.agent
                        SessionStatusKind.Shell, SessionStatusKind.Unknown -> null
                    },
                )
            }
            val rendered = card().textValues()
            assertTrue(
                "the $kind bay lost its literal kind, named signal, or age: $rendered",
                rendered.containsAll(listOf(expected.kind, expected.evidence)),
            )
            compose.onNodeWithContentDescription(expected.spoken, useUnmergedTree = true).assertIsDisplayed()

            val facet = facet()
            val config = facet.fetchSemanticsNode().config
            assertTrue(
                "the redundant facet must expose no text, description, or role: $config",
                config.getOrNull(SemanticsProperties.Text).isNullOrEmpty() &&
                    config.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty() &&
                    config.getOrNull(SemanticsProperties.Role) == null,
            )
            val bounds = facet.getUnclippedBoundsInRoot()
            assertFacetPosition(bounds, card().getUnclippedBoundsInRoot())
            val pixels = facet.captureToImage().toPixelMap()
            assertEquals(
                "the $kind facet center must be its statusColor tone",
                statusColor(kind).toArgb(),
                pixels[pixels.width / 2, pixels.height / 2].toArgb(),
            )
        }

        compose.mainClock.autoAdvance = false
        compose.runOnIdle {
            fixture = fixture.copy(
                statusKind = SessionStatusKind.Working,
                agent = COMMON_FIXTURE.agent,
                attention = true,
            )
        }
        compose.mainClock.advanceTimeByFrame()
        val markedFacet = facet().getUnclippedBoundsInRoot()
        assertEquals("attention must not move the fixed status facet", quietFacet, markedFacet)
        val attention = compose.onNodeWithContentDescription(ATTENTION_DESCRIPTION, useUnmergedTree = true)
        val attentionBounds = attention.getUnclippedBoundsInRoot()
        val paintedClearance = markedFacet.left - attentionBounds.right - ATTENTION_ROTATION_OVERRUN
        assertTrue(
            "attention diamond must leave about 4.34dp painted clearance: " +
                "attention=$attentionBounds facet=$markedFacet clearance=$paintedClearance",
            (paintedClearance - ATTENTION_PAINTED_CLEARANCE).value.absoluteValue <= POSITION_TOLERANCE,
        )
        compose.mainClock.advanceTimeByFrame()
        val pixels = attention.captureToImage().toPixelMap()
        assertEquals(
            "disabled animation must leave attention at full Orpiment opacity",
            Orpiment.toArgb(),
            pixels[pixels.width / 2, pixels.height / 2].toArgb(),
        )
    }

    @Test
    fun longContentTruncatesInsideItsStrataAndLargeTypeMayGrowSafely() {
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

        assertTrue("long tmux must stop at two lines: bounds=$tmux", tmux.bottom - tmux.top <= TWO_TITLE_LINES)
        assertTrue("long dwarf name must stop at one line: bounds=$dwarf", dwarf.bottom - dwarf.top <= ONE_DWARF_LINE)
        assertTrue(
            "objective must stop at two lines: bounds=${objective.getUnclippedBoundsInRoot()}",
            objective.getUnclippedBoundsInRoot().let { it.bottom - it.top <= TWO_BODY_LINES },
        )
        assertTrue(
            "directory must stop at one line: bounds=${directory.getUnclippedBoundsInRoot()}",
            directory.getUnclippedBoundsInRoot().let { it.bottom - it.top <= ONE_DATA_LINE },
        )
        assertTrue(
            "footer must stop at one line: bounds=${context.getUnclippedBoundsInRoot()}",
            context.getUnclippedBoundsInRoot().let { it.bottom - it.top <= ONE_DATA_LINE },
        )
        assertTrue(
            "the long footer must truncate before Kill: context=${context.getUnclippedBoundsInRoot()} kill=${kill.getUnclippedBoundsInRoot()}",
            context.getUnclippedBoundsInRoot().right <= kill.getUnclippedBoundsInRoot().left,
        )
        assertTrue(
            "the objective source changed",
            objective.textValues() == listOf(LONG_OBJECTIVE),
        )
        assertTrue(
            "the long directory must expose one abbreviated line and speak one exact complete source",
            directory.textValues() == listOf(LONG_DIRECTORY_VISIBLE) &&
                directory.contentDescriptions() == listOf(LONG_DIRECTORY_DESCRIPTION),
        )
        context.assertContext(LONG_CONTEXT, LONG_CONTEXT_DESCRIPTION)

        compose.runOnIdle { fontScale = LARGE_FONT_SCALE }
        val card = card().getUnclippedBoundsInRoot()
        val largeTmux = compose.onNodeWithText(LONG_TMUX, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val largeDwarf = compose.onNodeWithText(LONG_DWARF, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val largePortrait = compose.onNodeWithContentDescription("Portrait of $LONG_DWARF", useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val status = compose.onNodeWithContentDescription(UNKNOWN_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val largeObjective = objective.getUnclippedBoundsInRoot()
        val largeDirectory = directory.getUnclippedBoundsInRoot()
        val largeContext = context.getUnclippedBoundsInRoot()
        val largeKill = kill.getUnclippedBoundsInRoot()
        val children = listOf(
            largeTmux,
            largeDwarf,
            largePortrait,
            status,
            largeObjective,
            largeDirectory,
            largeContext,
            largeKill,
        )
        assertTrue(
            "large type may grow the card but no stratum may clip outside it: card=$card children=$children",
            children.all { card.contains(it) },
        )
        assertTrue(
            "large type must preserve status/objective/directory/footer order: children=$children",
            largeTmux.bottom <= largeDwarf.top && largeDwarf.bottom <= status.top &&
                status.bottom <= largeObjective.top && largeObjective.bottom <= largeDirectory.top &&
                largeDirectory.bottom <= largeContext.top && largeContext.right <= largeKill.left,
        )
    }

    @Test
    fun staleRetainedSnapshotKeepsAvailabilityBetweenStatusAndOperatorContext() {
        setCardContent(fixture = { COMMON_FIXTURE.copy(stale = true, objective = STALE_OBJECTIVE) })

        val status = compose.onNodeWithContentDescription(WORKING_DESCRIPTION, useUnmergedTree = true)
            .getUnclippedBoundsInRoot()
        val marker = compose.onNodeWithText(STALE_MARKER, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val objective = compose.onNodeWithTag(OBJECTIVE_TAG, useUnmergedTree = true).getUnclippedBoundsInRoot()
        val directory = compose.onNodeWithTag(DIRECTORY_TAG, useUnmergedTree = true).getUnclippedBoundsInRoot()
        assertTrue(
            "stale availability must follow status and precede objective/operator context: " +
                "status=$status marker=$marker objective=$objective directory=$directory",
            status.bottom <= marker.top && marker.bottom <= objective.top && objective.bottom <= directory.top,
        )
        card().assertIsNotEnabled()
        compose.onNodeWithTag(KILL_TAG, useUnmergedTree = true).assertIsNotEnabled()
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
                            visibleSession =
                                VisibleSession(current.machine(), SessionTarget(MACHINE_HANDLE, session)),
                            machine = current.machineState(session),
                            showMachineLabel = showMachineLabel(),
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
        label = MachineLabel.parse(machineLabel)!!,
        origin = MACHINE_ORIGIN,
    )

    private fun CardFixture.session() = TmuxSession(
        tmuxId = SESSION_ID,
        tmuxName = tmuxName,
        identityToken = "identity-1",
        character = CharacterSummary(key = "durinn", displayName = dwarfName),
        launchProfile = launchProfile,
        agent = agent,
        objective = objective,
        cwd = cwd,
        activeCommand = "codex",
        attachedClients = 1,
        attention = attention,
        status = SessionStatus(statusKind, signalFor(statusKind), SIGNAL_AT),
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

    private fun signalFor(kind: SessionStatusKind) = when (kind) {
        SessionStatusKind.Working, SessionStatusKind.Idle -> SessionStatusSignal.Lifecycle
        SessionStatusKind.Running, SessionStatusKind.Shell -> SessionStatusSignal.Process
        SessionStatusKind.Unknown -> SessionStatusSignal.PollFailure
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
        assertSquare("status facet", facet, FACET_SIDE)
        assertTrue(
            "facet must stay at the top-trailing 10dp inset: facet=$facet card=$card",
            ((facet.top - card.top) - CARD_PADDING).value.absoluteValue <= POSITION_TOLERANCE &&
                ((card.right - facet.right) - CARD_PADDING).value.absoluteValue <= POSITION_TOLERANCE,
        )
    }

    private fun assertSquare(label: String, bounds: DpRect, side: androidx.compose.ui.unit.Dp) {
        assertTrue(
            "$label must be exactly $side square: bounds=$bounds",
            ((bounds.right - bounds.left) - side).value.absoluteValue <= POSITION_TOLERANCE &&
                ((bounds.bottom - bounds.top) - side).value.absoluteValue <= POSITION_TOLERANCE,
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
        val statusKind: SessionStatusKind = SessionStatusKind.Working,
        val attention: Boolean = false,
        val stale: Boolean = false,
    )

    private data class StatusFixture(val kind: String, val evidence: String, val spoken: String)

    private companion object {
        val MACHINE_HANDLE = MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")!!
        val MACHINE_ORIGIN = MachineOrigin.parse("https://devbox.example:8443/")!!
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val SIGNAL_AT: Instant = Instant.parse("2026-08-26T11:57:00Z")
        val CARD_WIDTH = 170.dp
        val MAX_COMMON_HEIGHT = 200.dp
        val MINIMUM_TARGET = 48.dp
        val FACET_SIDE = 12.dp
        val CARD_PADDING = 10.dp
        val ATTENTION_ROTATION_OVERRUN = (4f * (sqrt(2f) - 1f)).dp
        val ATTENTION_PAINTED_CLEARANCE = 4.34.dp
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
        const val UNKNOWN_RUNTIME_PROFILE = "Claude · profile unknown"
        const val UNKNOWN_RUNTIME_CONTEXT_DESCRIPTION =
            "Machine Devbox. Profile Claude · profile unknown."
        const val UNKNOWN_CONTEXT_DESCRIPTION = "Machine Devbox. Profile profile unknown."
        const val PORTRAIT_DESCRIPTION = "Portrait of Durinn"
        const val DIRECTORY_DESCRIPTION = "Directory /src/skidbladnir"
        const val WORKING_DESCRIPTION = "Observed working from lifecycle 3 minutes ago"
        const val UNKNOWN_DESCRIPTION = "Observed unknown from poll failure 3 minutes ago"
        const val ATTENTION_DESCRIPTION = "Needs attention"
        const val STALE_MARKER = "STALE · actions disabled"
        const val STALE_OBJECTIVE = "Review the retained snapshot before opening this session."
        const val LONG_TMUX = "skidbladnir-codex-work-12345678901234567890123456789012345678900"
        const val LONG_DWARF = "Alberich of Nibelheim"
        const val LONG_OBJECTIVE = "Refactor the dashboard card layout without changing runtime behavior."
        const val LONG_DIRECTORY = "/srv/workspaces/skidbladnir/android"
        const val LONG_DIRECTORY_VISIBLE = "…/skidbladnir/android"
        const val LONG_DIRECTORY_DESCRIPTION = "Directory /srv/workspaces/skidbladnir/android"
        const val LONG_MACHINE = "MacBook Pro Across The Far Tailnet Realm"
        const val LONG_CONTEXT = "MacBook Pro Across The Far Tailnet Realm · Codex · Work"
        const val LONG_CONTEXT_DESCRIPTION =
            "Machine MacBook Pro Across The Far Tailnet Realm. Profile Codex · Work."
        const val CARD_TAG = "session-card-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val FACET_TAG = "session-status-facet-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val DIRECTORY_TAG = "session-directory-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val OBJECTIVE_TAG = "session-objective-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val CONTEXT_TAG = "session-context-mh-0123456789abcdef0123456789abcdef-session-durinn"
        const val KILL_TAG = "session-kill-mh-0123456789abcdef0123456789abcdef-session-durinn"

        val WORK_PROFILE = requireNotNull(ProfileKey.parse("work"))
        val PERSONAL_PROFILE = requireNotNull(ProfileKey.parse("personal"))
        val COMMON_FIXTURE = CardFixture(
            machineLabel = "Devbox",
            tmuxName = TMUX_NAME,
            dwarfName = DWARF_NAME,
            launchProfile = PERSONAL_PROFILE,
            agent = AgentRuntime(AgentProvider.Codex, 1234, profile = WORK_PROFILE),
            profileLabel = "Codex · Work",
            objective = null,
            cwd = "/src/skidbladnir",
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
            statusKind = SessionStatusKind.Unknown,
        )
        val STATUS_FIXTURES = mapOf(
            SessionStatusKind.Working to StatusFixture(
                "WORKING", "lifecycle · 3m", "Observed working from lifecycle 3 minutes ago",
            ),
            SessionStatusKind.Running to StatusFixture(
                "RUNNING", "process · 3m", "Observed running from process 3 minutes ago",
            ),
            SessionStatusKind.Idle to StatusFixture(
                "IDLE", "lifecycle · 3m", "Observed idle from lifecycle 3 minutes ago",
            ),
            SessionStatusKind.Shell to StatusFixture(
                "SHELL", "process · 3m", "Observed shell from process 3 minutes ago",
            ),
            SessionStatusKind.Unknown to StatusFixture(
                "UNKNOWN", "poll failure · 3m", UNKNOWN_DESCRIPTION,
            ),
        )
    }
}
