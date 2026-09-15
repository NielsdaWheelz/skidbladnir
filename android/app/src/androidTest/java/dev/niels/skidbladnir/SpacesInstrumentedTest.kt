package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Column
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextReplacement
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleRegistry
import androidx.savedstate.SavedStateRegistry
import androidx.savedstate.SavedStateRegistryController
import androidx.savedstate.SavedStateRegistryOwner
import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Real Compose and registry acceptance; the HTTP/tmux journey remains a separate live boundary. */
@RunWith(AndroidJUnit4::class)
class SpacesInstrumentedTest {
    @get:Rule val compose = createEmptyComposeRule()

    @Test
    fun selectorEditorAndHeadingCapsuleRetainExactNavigationIntent() {
        val alpha = label("alpha")
        val beta = label("beta")
        val machines = listOf(ready(machine, (1..12).map { session(it, alpha) } + session(13, beta)))
        val initial = DashboardEntryState().apply { acceptFleet(setOf(machine.handle)) }
        var entry by mutableStateOf(initial)
        var editor by mutableStateOf<SpaceEditor?>(null)
        var terminal by mutableStateOf(false)
        var writes = 0
        lateinit var registry: DashboardRegistryOwner
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            registry = DashboardRegistryOwner().apply { restore(null) }
            initial.install(registry.savedStateRegistry)
        }
        val dashboard = SkidbladnirUiState.Dashboard(machines, false, forge = null, forgeRecovery = null, kill = null)
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                activity.setContent {
                    NidavellirTheme {
                        if (terminal) {
                            SpaceTextAction("detach", true, { terminal = false })
                        } else {
                            Column {
                                SpaceSelector(entry.space, observedSpaces(machines), entry::selectSpace)
                                DashboardDwarfCollection(dashboard, entry, {}, entry::restoreOnce,
                                    onOpen = { terminal = true }, onKill = { error("unexpected destructive action") },
                                    onSpace = { editor = SpaceEditor(it, it.session.space?.text.orEmpty()) })
                            }
                        }
                        editor?.let { current ->
                            SpaceSheet(current, machines.single(), observedSpaces(machines),
                                onChange = { editor = current.copy(draft = it, error = null) },
                                onDismiss = { editor = null },
                                onSubmit = { writes += 1; editor = current.copy(phase = SpacePhase.Checking(true)) })
                        }
                    }
                }
            }
            compose.onNodeWithTag("space-selector").performClick()
            compose.onNode(hasContentDescription("space: alpha") and hasClickAction()).performClick()
            compose.onNodeWithTag("space-selector").assertIsDisplayed()
            compose.runOnIdle {
                assertTrue("selected named intersection", entry.space.matches(alpha) && !entry.space.matches(beta))
                entry.selectScope(DashboardScope.Machine(machine.handle))
                entry.gridState.requestScrollToItem(0, 7)
            }
            compose.waitForIdle()
            lateinit var saved: Bundle
            compose.runOnIdle {
                val snapshot = entry.snapshot()
                assertTrue("actual heading is the saved anchor", snapshot.viewport.anchor == DashboardItemKey.Space(spaceFingerprint(alpha)))
                saved = registry.save()
            }
            compose.onNodeWithContentDescription("space for session1 on one: space: alpha").performClick()
            compose.onNodeWithTag("space-save").assertIsNotEnabled()
            compose.onNodeWithTag("space-draft").performTextReplacement(" bad label")
            compose.onNodeWithTag("space-save").assertIsNotEnabled()
            compose.onNodeWithText("observed spaces").performClick()
            compose.onNode(hasContentDescription("space: beta") and hasClickAction()).performClick()
            compose.runOnIdle { assertEquals(0, writes) }
            compose.onNodeWithTag("space-save").assertIsEnabled().performClick()
            compose.onNodeWithText("checking current membership").assertIsDisplayed()
            compose.onNodeWithText("cancel").performClick()
            compose.runOnIdle { assertEquals(1, writes) }
            compose.onNodeWithTag("session-card-${machine.handle.encoded}-${'$'}1").performClick()
            compose.onNodeWithText("detach").performClick()
            compose.runOnIdle { assertTrue("terminal round trip retains grid owner", entry.gridState === initial.gridState) }
            compose.runOnIdle {
                val restoredOwner = DashboardRegistryOwner().apply { restore(saved) }
                entry = DashboardEntryState().apply {
                    install(restoredOwner.savedStateRegistry)
                    acceptFleet(setOf(machine.handle))
                }
                assertTrue("restored label has no raw display text", entry.space == DashboardSpaceSelection.Named(spaceFingerprint(alpha)))
                assertTrue("unresolved creation requires a choice", entry.space.creationDraft() == SpaceDraft.Unresolved)
                entry.resolveSpace(observedSpaces(machines))
            }
            compose.waitForIdle()
            compose.runOnIdle {
                assertFalse(entry.restorationPending)
                assertTrue("heading anchor restored", entry.snapshot().viewport.anchor == DashboardItemKey.Space(spaceFingerprint(alpha)))
                entry.gridState.requestScrollToItem(5, 11)
            }
            compose.waitForIdle()
            compose.runOnIdle {
                terminal = true
                entry.followCreatedMembership(beta)
                val owner = DashboardRegistryOwner().apply { restore(null) }
                // This recreated entry was installed above; capture its actual task capsule directly.
                val next = entry.snapshot()
                val created = DashboardEntryState(next)
                created.install(owner.savedStateRegistry)
                val restoredOwner = DashboardRegistryOwner().apply { restore(owner.save()) }
                val restored = DashboardEntryState().apply { install(restoredOwner.savedStateRegistry) }
                assertTrue("confirmed creation changes saved filter", restored.space.key == DashboardSpaceKey.Named(spaceFingerprint(beta)))
                assertNull(restored.snapshot().viewport.anchor)
                assertEquals(0, restored.snapshot().viewport.fallbackIndex)
                assertEquals(0, restored.snapshot().viewport.offsetPx)
            }
        }
    }

    @Test
    fun platformNormalizationUsesTheFixedUnicode15Contract() {
        val kawi = "x\u0315" + String(Character.toChars(0x11f41))
        val ordered = "x" + String(Character.toChars(0x11f41)) + "\u0315"
        assertNull("platform enforces Unicode15 combining order", SpaceLabel.parse(kawi))
        assertTrue("platform normalizes known marks", SpaceLabel.fromDraft(kawi)?.text == ordered)
        val laterMark = "x\u0315\u0897"
        assertTrue("platform leaves post15 mark inert", SpaceLabel.fromDraft(laterMark)?.text == laterMark)
        assertTrue("platform accepts inert-boundary wire label", SpaceLabel.parse(laterMark) != null)
        val laterComposition = String(Character.toChars(0x105d2)) + "\u0307"
        assertTrue("platform leaves post15 composition inert", SpaceLabel.fromDraft(laterComposition)?.text == laterComposition)
        val combining = "x" + "\u035c".repeat(31)
        assertTrue("platform preserves long combining sequence", SpaceLabel.fromDraft(combining)?.text == combining)
        val explicitJoiner = combining + "\u034f" + "\u035c".repeat(30)
        assertTrue("platform preserves explicit combining joiner", SpaceLabel.fromDraft(explicitJoiner)?.text == explicitJoiner)
        listOf("a\ud800", "a\udc00", "a\udc00\ud800").forEachIndexed { index, candidate ->
            assertNull("platform surrogate case $index remains invalid", SpaceLabel.fromDraft(candidate))
        }
        assertTrue("platform composes older characters", SpaceLabel.fromDraft("e\u0301")?.text == "é")
    }

    @Test
    fun currentRegistrySchemaHasOnlyExactPrimitivesAndRejectsMalformedVariants() =
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            val fingerprint = spaceFingerprint(label("alpha"))
            val snapshot = DashboardEntrySnapshot(2, DashboardScope.Machine(machine.handle),
                DashboardViewport(DashboardItemKey.Space(fingerprint), 4, 9), DashboardSpaceKey.Named(fingerprint))
            val owner = DashboardRegistryOwner().apply { restore(null) }
            DashboardEntryState(snapshot).install(owner.savedStateRegistry)
            val reader = DashboardRegistryOwner().apply { restore(owner.save()) }
            val payload = requireNotNull(reader.savedStateRegistry.consumeRestoredStateForKey(REGISTRY_KEY))
            assertTrue("exact content-free primitive keys", payload.keySet() == setOf("version", "scopeKind", "scopeMachine",
                "spaceKind", "spaceLabelSha256", "anchorKind", "anchorSha256", "fallbackIndex", "offsetPx"))
            assertEquals(2, payload.getInt("version"))
            assertTrue("named selection contains only comparison hash", payload.getString("spaceLabelSha256") == fingerprint)
            assertTrue("heading anchor contains only comparison hash", payload.getString("anchorSha256") == fingerprint)
            val malformed = listOf(
                Bundle(payload).apply { putString("spaceLabelSha256", "not-a-fingerprint") },
                Bundle(payload).apply { putString("spaceKind", "all") },
                Bundle(payload).apply { putString("anchorKind", "none") },
                Bundle(payload).apply { putString("offsetPx", "9") },
                Bundle(payload).apply { putString("space", "alpha") },
            )
            malformed.forEachIndexed { index, capsule ->
                val writer = DashboardRegistryOwner().apply { restore(null) }
                writer.savedStateRegistry.registerSavedStateProvider(REGISTRY_KEY) { capsule }
                val broken = DashboardRegistryOwner().apply { restore(writer.save()) }
                assertThrows("invalid schema case $index", IllegalStateException::class.java) {
                    DashboardEntryState().install(broken.savedStateRegistry)
                }
            }
        }

    private fun label(text: String) = requireNotNull(SpaceLabel.parse(text))
    private fun session(id: Int, space: SpaceLabel?) = TmuxSession("${'$'}$id", "session$id", "identity$id",
        CharacterSummary("norse.durinn", "Durinn"), attachedClients = 0, space = space)
    private fun ready(machine: PairedMachine, sessions: List<TmuxSession>) = MachineState(machine, MachineAccess.Ready,
        InventoryState.Fresh(InventorySnapshot(SessionsResponse(MachineSummary(machine.handle, MachinePlatform.Linux),
            Instant.parse("2026-09-15T12:00:00Z"), emptyList(), sessions), 10)), PressureState.Reading)
    private val machine = PairedMachine(requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef")),
        requireNotNull(MachineLabel.parse("one")), requireNotNull(MachineOrigin.parse("https://one.example:8443/")))
    private companion object { const val REGISTRY_KEY = "dev.niels.skidbladnir.dashboard-entry" }
}

internal class DashboardRegistryOwner : SavedStateRegistryOwner {
    private val lifecycleRegistry = LifecycleRegistry(this)
    private val controller = SavedStateRegistryController.create(this)
    override val lifecycle: Lifecycle get() = lifecycleRegistry
    override val savedStateRegistry: SavedStateRegistry get() = controller.savedStateRegistry
    init { controller.performAttach() }
    fun restore(savedState: Bundle?) {
        controller.performRestore(savedState)
        lifecycleRegistry.currentState = Lifecycle.State.CREATED
    }
    fun save(): Bundle = Bundle().also(controller::performSave)
}
