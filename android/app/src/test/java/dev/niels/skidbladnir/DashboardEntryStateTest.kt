package dev.niels.skidbladnir

import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class DashboardEntryStateTest {
    private val devboxHandle = requireNotNull(
        MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"),
    )
    private val macBookHandle = requireNotNull(
        MachineHandle.parse("mh-fedcba9876543210fedcba9876543210"),
    )
    private val devbox = machine(devboxHandle, "Devbox", "https://devbox.example.ts.net:8443")
    private val macBook = machine(macBookHandle, "MacBook", "https://macbook.example.ts.net:8443")
    private val devboxKey = DashboardCardKey("1".repeat(64))
    private val macBookKey = DashboardCardKey("2".repeat(64))
    private val laterKey = DashboardCardKey("3".repeat(64))

    @Test
    fun `Dashboard entry preserves typed scope semantic place and readiness policy`() {
        val macBookCapsule = snapshot(
            scope = DashboardScope.Machine(macBookHandle),
            anchor = macBookKey,
            fallbackIndex = 2,
            offsetPx = 37,
        )
        val readyMachines = listOf(
            readyMachine(devbox, session("devbox-session", "devbox", "devbox-token")),
            readyMachine(macBook, session("macbook-session", "macbook", "macbook-token")),
        )
        val paired = setOf(devboxHandle, macBookHandle)

        val machineEntry = DashboardEntryState(macBookCapsule)
        machineEntry.acceptFleet(paired)
        assertEquals(
            "the restored MacBook intent must be accepted before inventory readiness",
            DashboardScope.Machine(macBookHandle),
            machineEntry.scope,
        )
        assertTrue("the accepted capsule must wait for a scoped outcome", machineEntry.restorationPending)
        assertEquals(
            "post-return verification must target only the restored MacBook poller",
            setOf(macBookHandle),
            visibleInventoryTargets(
                liveMachineHandles = paired,
                scope = machineEntry.scope,
            ),
        )
        assertTrue(
            "a restored machine without a live poller must add no verification work",
            visibleInventoryTargets(
                liveMachineHandles = setOf(devboxHandle),
                scope = machineEntry.scope,
            ).isEmpty(),
        )
        assertTrue(
            "fresh scoped inventory must admit one semantic restore",
            dashboardRestorationReady(
                scope = machineEntry.scope,
                machines = readyMachines,
                livePollers = paired,
                foreground = true,
            ),
        )

        machineEntry.restoreOnce(listOf(laterKey, macBookKey, devboxKey))
        assertFalse("the surviving anchor must be consumed exactly once", machineEntry.restorationPending)
        assertEquals("reorder must follow the same lifetime", 1, machineEntry.gridState.firstVisibleItemIndex)
        assertEquals("reorder must preserve its pixel offset", 37, machineEntry.gridState.firstVisibleItemScrollOffset)

        val deletedEntry = DashboardEntryState(macBookCapsule)
        deletedEntry.acceptFleet(paired)
        deletedEntry.restoreOnce(listOf(devboxKey, laterKey))
        assertEquals(
            "a deleted anchor must clamp its former index into the current collection",
            1,
            deletedEntry.gridState.firstVisibleItemIndex,
        )
        assertEquals(37, deletedEntry.gridState.firstVisibleItemScrollOffset)

        val absentPairing = DashboardEntryState(macBookCapsule)
        absentPairing.acceptFleet(setOf(devboxHandle))
        assertEquals("an absent restored pairing must reset atomically", DashboardScope.All, absentPairing.scope)
        assertFalse(absentPairing.restorationPending)
        assertEquals(0, absentPairing.gridState.firstVisibleItemIndex)
        assertEquals(0, absentPairing.gridState.firstVisibleItemScrollOffset)
        assertEquals(freshSnapshot, absentPairing.snapshot())

        listOf(
            MachineState(
                machine = macBook,
                access = MachineAccess.Ready,
                inventory = InventoryState.Unreachable(GatewayFailure.Transport),
                pressure = PressureState.Reading,
            ),
            MachineState(
                machine = macBook,
                access = MachineAccess.AuthRequired,
                inventory = InventoryState.Reading,
                pressure = PressureState.Reading,
            ),
            MachineState(
                machine = macBook,
                access = MachineAccess.IdentityChanged,
                inventory = InventoryState.Reading,
                pressure = PressureState.Reading,
            ),
        ).forEach { unavailable ->
            val unavailableEntry = DashboardEntryState(macBookCapsule)
            unavailableEntry.acceptFleet(paired)
            assertTrue(
                "${unavailable.access}/${unavailable.inventory} must resolve without inventing a snapshot",
                dashboardRestorationReady(
                    scope = unavailableEntry.scope,
                    machines = listOf(unavailable),
                    livePollers = emptySet(),
                    foreground = true,
                ),
            )
            unavailableEntry.restoreOnce(emptyList())
            assertEquals(DashboardScope.Machine(macBookHandle), unavailableEntry.scope)
            assertFalse(unavailableEntry.restorationPending)
            assertEquals(0, unavailableEntry.gridState.firstVisibleItemIndex)
            assertEquals(0, unavailableEntry.gridState.firstVisibleItemScrollOffset)
        }

        val retainedSession = session("retained-session", "retained", "retained-token")
        val precedingSession = session("preceding-session", "preceding", "preceding-token")
        val retainedFresh = readyMachine(macBook, precedingSession, retainedSession)
        val retainedStale = retainedFresh.copy(
            inventory = InventoryState.Stale(
                snapshot = (retainedFresh.inventory as InventoryState.Fresh).snapshot,
                cause = GatewayFailure.Transport,
            ),
        )
        val retainedKey = dashboardCardKey(SessionTarget(macBookHandle, retainedSession))
        val retainedEntry = DashboardEntryState(
            snapshot(
                scope = DashboardScope.Machine(macBookHandle),
                anchor = retainedKey,
                fallbackIndex = 0,
                offsetPx = 25,
            ),
        )
        retainedEntry.acceptFleet(paired)
        assertTrue(
            "a retained stale snapshot is a settled machine-scoped restoration outcome",
            dashboardRestorationReady(
                scope = retainedEntry.scope,
                machines = listOf(retainedStale),
                livePollers = emptySet(),
                foreground = true,
            ),
        )
        retainedEntry.restoreOnce(
            visibleSessions(listOf(retainedStale), retainedEntry.scope).map(VisibleSession::cardKey),
        )
        assertEquals(1, retainedEntry.gridState.firstVisibleItemIndex)
        assertEquals(25, retainedEntry.gridState.firstVisibleItemScrollOffset)

        val stoppedEntry = DashboardEntryState(macBookCapsule)
        stoppedEntry.acceptFleet(paired)
        val readingMacBook = MachineState(
            machine = macBook,
            access = MachineAccess.Ready,
            inventory = InventoryState.Reading,
            pressure = PressureState.Reading,
        )
        assertFalse(
            "background stop must leave pending restoration intact",
            dashboardRestorationReady(
                scope = stoppedEntry.scope,
                machines = listOf(readingMacBook),
                livePollers = emptySet(),
                foreground = false,
            ),
        )
        assertEquals("save-again must not erase pending place", macBookCapsule, stoppedEntry.snapshot())
        assertFalse(
            "a live foreground poller still reading must remain pending",
            dashboardRestorationReady(
                scope = stoppedEntry.scope,
                machines = listOf(readingMacBook),
                livePollers = setOf(macBookHandle),
                foreground = true,
            ),
        )
        assertThrows(
            "a foreground Ready/Reading machine without its poller is a controller defect",
            IllegalStateException::class.java,
        ) {
            dashboardRestorationReady(
                scope = stoppedEntry.scope,
                machines = listOf(readingMacBook),
                livePollers = setOf(devboxHandle),
                foreground = true,
            )
        }

        stoppedEntry.gridState.requestScrollToItem(3, 41)
        stoppedEntry.selectScope(DashboardScope.Machine(macBookHandle))
        assertTrue("selecting the active scope must not consume restoration", stoppedEntry.restorationPending)
        assertEquals(3, stoppedEntry.gridState.firstVisibleItemIndex)
        assertEquals(41, stoppedEntry.gridState.firstVisibleItemScrollOffset)
        stoppedEntry.selectScope(DashboardScope.All)
        assertEquals(DashboardScope.All, stoppedEntry.scope)
        assertFalse("selecting a different scope must cancel restoration", stoppedEntry.restorationPending)
        assertEquals(3, stoppedEntry.gridState.firstVisibleItemIndex)
        assertEquals(41, stoppedEntry.gridState.firstVisibleItemScrollOffset)

        val accessLossEntry = DashboardEntryState(macBookCapsule)
        accessLossEntry.acceptFleet(paired)
        assertTrue(accessLossEntry.restorationPending)
        accessLossEntry.gridState.requestScrollToItem(2, 31)
        assertEquals(2, accessLossEntry.gridState.firstVisibleItemIndex)
        assertEquals(31, accessLossEntry.gridState.firstVisibleItemScrollOffset)
        accessLossEntry.selectTerminalAccessLoss(devboxHandle)
        assertEquals(DashboardScope.Machine(devboxHandle), accessLossEntry.scope)
        assertFalse(accessLossEntry.restorationPending)
        assertEquals(0, accessLossEntry.gridState.firstVisibleItemIndex)
        assertEquals(0, accessLossEntry.gridState.firstVisibleItemScrollOffset)
        assertEquals(
            DashboardEntrySnapshot(
                schemaVersion = 1,
                scope = DashboardScope.Machine(devboxHandle),
                viewport = DashboardViewport(anchor = null, fallbackIndex = 0, offsetPx = 0),
            ),
            accessLossEntry.snapshot(),
        )

        val allCapsule = snapshot(
            scope = DashboardScope.All,
            anchor = devboxKey,
            fallbackIndex = 0,
            offsetPx = 19,
        )
        val allEntry = DashboardEntryState(allCapsule)
        allEntry.acceptFleet(paired)
        assertEquals(
            "All must retain every live inventory target",
            paired,
            visibleInventoryTargets(
                liveMachineHandles = paired,
                scope = allEntry.scope,
            ),
        )
        assertTrue(
            "All with no live pollers must add no verification work",
            visibleInventoryTargets(emptySet(), allEntry.scope).isEmpty(),
        )
        assertFalse(
            "All must not expose a partial grid while one live machine is still Reading",
            dashboardRestorationReady(
                scope = allEntry.scope,
                machines = readyMachines.map { machine ->
                    if (machine.machine.handle == macBookHandle) readingMacBook else machine
                },
                livePollers = paired,
                foreground = true,
            ),
        )
        assertTrue(
            dashboardRestorationReady(
                scope = allEntry.scope,
                machines = readyMachines,
                livePollers = paired,
                foreground = true,
            ),
        )
        allEntry.restoreOnce(listOf(macBookKey, devboxKey))
        assertEquals(1, allEntry.gridState.firstVisibleItemIndex)
        assertEquals(19, allEntry.gridState.firstVisibleItemScrollOffset)

        val canonicalTarget = SessionTarget(
            machineHandle = macBookHandle,
            session = session("session-17", "canonical", "identity-17"),
        )
        val canonicalKey = dashboardCardKey(canonicalTarget)
        assertEquals(
            "the comparison key must use the reviewed domain and 32-bit big-endian frames",
            "fdc952207a9b5abd2dc67b8aa11dc6648733f3b07a5590f8d74cf147f8da94cd",
            canonicalKey.lifetimeFingerprint,
        )
        assertEquals(
            "an operator rename must not change one tmux lifetime key",
            canonicalKey,
            dashboardCardKey(
                canonicalTarget.copy(session = canonicalTarget.session.copy(tmuxName = "renamed")),
            ),
        )
        assertTrue(
            "a replacement identity token must produce a different card key",
            canonicalKey != dashboardCardKey(
                canonicalTarget.copy(
                    session = canonicalTarget.session.copy(identityToken = "identity-18"),
                ),
            ),
        )
        val visible = VisibleSession(macBook, canonicalTarget)
        val replacement = canonicalTarget.copy(
            session = canonicalTarget.session.copy(identityToken = "identity-18"),
        )
        assertTrue(
            "copying a visible card to a replacement target must rederive its lifetime key",
            visible.cardKey != visible.copy(target = replacement).cardKey,
        )
    }

    private fun snapshot(
        scope: DashboardScope,
        anchor: DashboardCardKey,
        fallbackIndex: Int,
        offsetPx: Int,
    ) = DashboardEntrySnapshot(
        schemaVersion = 1,
        scope = scope,
        viewport = DashboardViewport(anchor, fallbackIndex, offsetPx),
    )

    private fun machine(
        handle: MachineHandle,
        label: String,
        origin: String,
    ) = PairedMachine(
        handle = handle,
        label = requireNotNull(MachineLabel.parse(label)),
        origin = requireNotNull(MachineOrigin.parse(origin)),
    )

    private fun readyMachine(machine: PairedMachine, vararg sessions: TmuxSession) = MachineState(
        machine = machine,
        access = MachineAccess.Ready,
        inventory = InventoryState.Fresh(
            InventorySnapshot(
                inventory = SessionsResponse(
                    machine = MachineSummary(machine.handle, MachinePlatform.Linux),
                    observedAt = OBSERVED_AT,
                    profiles = emptyList(),
                    sessions = sessions.toList(),
                ),
                receivedAtElapsedMillis = 1_000,
            ),
        ),
        pressure = PressureState.Reading,
    )

    private fun session(tmuxId: String, tmuxName: String, identityToken: String) = TmuxSession(
        tmuxId = tmuxId,
        tmuxName = tmuxName,
        identityToken = identityToken,
        character = CharacterSummary("norse.durinn", "Durinn"),
        attachedClients = 0,
        activity = SessionActivity.Quiet,
    )

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-31T12:00:00Z")
        val freshSnapshot = DashboardEntrySnapshot(
            schemaVersion = 1,
            scope = DashboardScope.All,
            viewport = DashboardViewport(anchor = null, fallbackIndex = 0, offsetPx = 0),
        )
    }
}
