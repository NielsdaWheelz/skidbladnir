package dev.niels.skidbladnir

import java.time.Instant
import java.util.Locale
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class MultiMachineContractTest {
    private val devboxHandle = requireNotNull(MachineHandle.parse("mh-0123456789abcdef0123456789abcdef"))
    private val macBookHandle = requireNotNull(MachineHandle.parse("mh-fedcba9876543210fedcba9876543210"))
    private val devbox = PairedMachine(
        handle = devboxHandle,
        label = requireNotNull(MachineLabel.parse("Devbox")),
        origin = requireNotNull(MachineOrigin.parse("https://devbox.example.ts.net:8443")),
    )
    private val macBook = PairedMachine(
        handle = macBookHandle,
        label = requireNotNull(MachineLabel.parse("MacBook")),
        origin = requireNotNull(MachineOrigin.parse("https://macbook.example.ts.net:8443")),
    )
    private val personal = requireNotNull(ProfileKey.parse("personal"))

    @Test
    fun `machine boundary values are canonical and origins cannot widen authority`() {
        assertEquals(devboxHandle, MachineHandle.parse(devboxHandle.encoded))
        assertEquals("Devbox", MachineLabel.parse("Devbox")?.text)
        assertTrue(devbox.origin.encoded == "https://devbox.example.ts.net:8443/")

        listOf(
            "mh-0123456789ABCDEF0123456789abcdef",
            "mh-0123456789abcdef",
            "0123456789abcdef0123456789abcdef",
        ).forEach { assertEquals(null, MachineHandle.parse(it)) }
        listOf("", " Devbox", "Devbox ", "Dev\nbox").forEach {
            assertEquals(null, MachineLabel.parse(it))
        }
        listOf(
            "http://devbox.example.ts.net:8443",
            "https://devbox.example.ts.net",
            "https://user@devbox.example.ts.net:8443",
            "https://devbox.example.ts.net:8443/v1",
            "https://devbox.example.ts.net:8443?machine=devbox",
            "https://devbox.example.ts.net:8443/#fragment",
            "https://[fd7a:115c:a1e0::1]:443",
            "https://:8443",
        ).forEach { assertEquals("accepted invalid origin $it", null, MachineOrigin.parse(it)) }

        val mixedCase = requireNotNull(MachineOrigin.parse("https://DevBox.Example.TS.NET:8443"))
        assertTrue(devbox.origin == mixedCase)
        assertTrue(mixedCase.encoded == "https://devbox.example.ts.net:8443/")
    }

    @Test
    fun `canonical origins round-trip for hostname, IPv4, and IPv6 authorities`() {
        val cases = mapOf(
            "https://DevBox.Example.TS.NET:8443" to "https://devbox.example.ts.net:8443/",
            "https://100.64.0.1:8443" to "https://100.64.0.1:8443/",
            "https://[FD7A:115C:A1E0::1]:8443" to "https://[fd7a:115c:a1e0::1]:8443/",
            "https://[fd7a:115c:a1e0::1]:8443/" to "https://[fd7a:115c:a1e0::1]:8443/",
        )
        cases.forEach { (candidate, canonical) ->
            val origin = requireNotNull(MachineOrigin.parse(candidate)) { "rejected $candidate" }
            assertEquals(canonical, origin.encoded)
            assertEquals(
                "canonical origin $canonical did not survive its own parser",
                origin,
                MachineOrigin.parse(origin.encoded),
            )
        }
    }

    @Test
    fun `inventory requires a strict machine envelope and closed platform`() {
        val inventory = decodeSessionsResponse(inventoryJson(devboxHandle, "Linux"))
        assertEquals(devboxHandle, inventory.machine.handle)
        assertEquals(MachinePlatform.Linux, inventory.machine.platform)
        assertEquals(Instant.parse("2026-08-26T12:00:00Z"), inventory.observedAt)
        assertEquals(listOf(personal), inventory.profiles.map(ProfileChoice::key))

        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(inventoryJson(devboxHandle, "Windows"))
        }
        assertThrows(ProtocolDecodeException::class.java) {
            decodeSessionsResponse(inventoryJson(devboxHandle, "Linux").replace(
                "\"platform\":\"Linux\"",
                "\"platform\":\"Linux\",\"hostname\":\"devbox\"",
            ))
        }
    }

    @Test
    fun `equal local sessions remain distinct and one failure cannot stale the other`() {
        val duplicate = session()
        val initial = listOf(readyMachine(macBook, duplicate), readyMachine(devbox, duplicate))

        val sessions = visibleSessions(initial, scope = DashboardScope.All)
        assertEquals(listOf("Devbox", "MacBook"), sessions.map { it.machine.label.text })
        assertTrue(sessions[0].target != sessions[1].target)
        assertEquals(2, sessions.map(VisibleSession::target).distinct().size)

        val failed = initial.map {
            if (it.machine.handle == devboxHandle) it.inventoryFailed(GatewayFailure.Transport) else it
        }
        assertTrue(failed.single { it.machine.handle == devboxHandle }.inventory is InventoryState.Stale)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.inventory is InventoryState.Fresh)
        assertFalse(failed.single { it.machine.handle == devboxHandle }.canMutate)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.canMutate)
    }

    @Test
    fun `pressure rails require an explicit machine filter`() {
        assertFalse(
            "All must omit machine pressure rails",
            pressureRailsVisible(DashboardScope.All),
        )
        assertTrue(
            "an explicit machine filter must show its pressure rail",
            pressureRailsVisible(DashboardScope.Machine(macBookHandle)),
        )
    }

    @Test
    fun `sessions sort fresh quiet then fresh active then retained and use total identity order`() {
        fun machine(handle: String, label: String) = PairedMachine(
            handle = requireNotNull(MachineHandle.parse(handle)),
            label = requireNotNull(MachineLabel.parse(label)),
            origin = requireNotNull(MachineOrigin.parse("https://${handle.takeLast(4)}.example.ts.net:8443/")),
        )
        val exactAlpha = machine("mh-00000000000000000000000000000001", "Alpha")
        val laterHandle = machine("mh-00000000000000000000000000000002", "Alpha")
        val caseAlpha = machine("mh-00000000000000000000000000000003", "alpha")
        val stale = machine("mh-00000000000000000000000000000004", "Aardvark")
        val authRequired = machine("mh-00000000000000000000000000000005", "Aardvark")
        val exactAlphaState = readyMachine(
            exactAlpha,
            activitySession(tmuxId(3), "aardvark", "Active"),
            activitySession(tmuxId(2), "work", "Quiet"),
            activitySession(tmuxId(1), "Work", "Quiet"),
            activitySession(tmuxId(0), "Work", "Quiet"),
        )
        val laterHandleState = readyMachine(
            laterHandle,
            activitySession(tmuxId(4), "Work", "Quiet"),
        )
        val caseAlphaState = readyMachine(
            caseAlpha,
            activitySession(tmuxId(5), "Work", "Quiet"),
        )
        val freshStale = readyMachine(stale, activitySession(tmuxId(6), "z-retained", "Active"))
        val staleState = freshStale.copy(
            inventory = InventoryState.Stale(
                (freshStale.inventory as InventoryState.Fresh).snapshot,
                GatewayFailure.Transport,
            ),
        )
        val authRequiredState = readyMachine(
            authRequired,
            activitySession(tmuxId(7), "a-retained", "Quiet"),
        ).copy(access = MachineAccess.AuthRequired)

        assertEquals(
            listOf(
                "Alpha/${exactAlpha.handle.encoded}/Work/${tmuxId(0)}",
                "Alpha/${exactAlpha.handle.encoded}/Work/${tmuxId(1)}",
                "Alpha/${exactAlpha.handle.encoded}/work/${tmuxId(2)}",
                "Alpha/${laterHandle.handle.encoded}/Work/${tmuxId(4)}",
                "alpha/${caseAlpha.handle.encoded}/Work/${tmuxId(5)}",
                "Alpha/${exactAlpha.handle.encoded}/aardvark/${tmuxId(3)}",
                "Aardvark/${stale.handle.encoded}/z-retained/${tmuxId(6)}",
                "Aardvark/${authRequired.handle.encoded}/a-retained/${tmuxId(7)}",
            ),
            visibleSessions(
                listOf(authRequiredState, caseAlphaState, staleState, laterHandleState, exactAlphaState),
                scope = DashboardScope.All,
            ).map {
                "${it.machine.label.text}/${it.machine.handle.encoded}/" +
                    "${it.target.session.tmuxName}/${it.target.session.tmuxId}"
            },
        )
    }

    @Test
    fun `machine and session ordering is invariant under Turkish locale`() {
        val original = Locale.getDefault()
        Locale.setDefault(Locale.forLanguageTag("tr-TR"))
        try {
            val iota = devbox.copy(label = requireNotNull(MachineLabel.parse("Iota")))
            val zeta = macBook.copy(label = requireNotNull(MachineLabel.parse("Zeta")))
            assertEquals(
                listOf("Iota", "Zeta"),
                visibleSessions(
                    listOf(readyMachine(zeta, session()), readyMachine(iota, session())),
                    scope = DashboardScope.All,
                ).map { it.machine.label.text },
            )

            val sessions = readyMachine(
                devbox,
                session(tmuxId = tmuxId(2), tmuxName = "Zeta", identityToken = "token-zeta"),
                session(tmuxId = tmuxId(1), tmuxName = "Iota", identityToken = "token-iota"),
            )
            assertEquals(
                listOf("Iota", "Zeta"),
                visibleSessions(listOf(sessions), scope = DashboardScope.All).map { it.target.session.tmuxName },
            )
        } finally {
            Locale.setDefault(original)
        }
    }

    @Test
    fun `inventory work serializes per machine while other machines progress independently`() {
        val executor = Executors.newFixedThreadPool(2)
        val operations = MachineInventoryOperations(executor) { defect -> throw defect }
        val mutationStarted = CountDownLatch(1)
        val releaseMutation = CountDownLatch(1)
        val devboxRead = CountDownLatch(1)
        val macBookRead = CountDownLatch(1)
        val mutationCompleted = AtomicBoolean(false)
        val mutationCompletedBeforeRead = AtomicBoolean(false)
        val reservedFence = AtomicLong()
        val observedFence = AtomicLong()

        try {
            operations.forMachine(devboxHandle).submitMutation(onReserved = reservedFence::set) {
                mutationStarted.countDown()
                check(releaseMutation.await(5, TimeUnit.SECONDS))
                mutationCompleted.set(true)
            }
            assertTrue("same-machine mutation did not start", mutationStarted.await(5, TimeUnit.SECONDS))
            operations.forMachine(devboxHandle).submitRead { fence ->
                mutationCompletedBeforeRead.set(mutationCompleted.get())
                observedFence.set(fence)
                devboxRead.countDown()
            }
            operations.forMachine(macBookHandle).submitRead { macBookRead.countDown() }

            assertTrue("other machine did not progress independently", macBookRead.await(5, TimeUnit.SECONDS))
            assertFalse(
                "same-machine read overtook an in-flight mutation",
                devboxRead.await(250, TimeUnit.MILLISECONDS),
            )
            releaseMutation.countDown()
            assertTrue("same-machine read did not follow the mutation", devboxRead.await(5, TimeUnit.SECONDS))
            assertTrue("the read ran before its machine's mutation completed", mutationCompletedBeforeRead.get())
            assertEquals("the read did not observe the reserved mutation fence", reservedFence.get(), observedFence.get())
        } finally {
            releaseMutation.countDown()
            executor.shutdownNow()
            assertTrue("inventory executor did not terminate", executor.awaitTermination(5, TimeUnit.SECONDS))
        }
    }

    @Test
    fun `overlapping poll ticks coalesce into exactly one trailing run`() {
        val lane = CoalescingPollLane()

        val leading = checkNotNull(lane.request())
        assertTrue("the leading tick must start immediately", leading.startsNow)

        val verification = checkNotNull(lane.request(requireTrailing = true))
        assertFalse("verification must not run beside the leading read", verification.startsNow)
        assertTrue(
            "a read that began before verification must not satisfy it",
            leading.sequence < verification.sequence,
        )
        assertEquals(
            "further requests must share the same one trailing read",
            verification,
            lane.request(requireTrailing = true),
        )

        val awaited = AwaitedInventoryReads()
        awaited.requireRead(devboxHandle, verification.sequence)
        awaited.requireRead(macBookHandle, sequence = 1L)
        assertTrue("a required read must make the shared indicator active", awaited.isActive)
        awaited.readLanded(macBookHandle, completedSequence = 1L)
        assertTrue("one machine landing must not finish another machine's indicator", awaited.isActive)
        awaited.readLanded(devboxHandle, leading.sequence)
        assertTrue("the pre-request result cleared the newer verification", awaited.isActive)

        val trailing = checkNotNull(lane.finish(leading.sequence))
        assertEquals("the required trailing read did not start", verification.sequence, trailing.sequence)
        assertTrue("the trailing read must start after the leading result", trailing.startsNow)
        val laterVerification = checkNotNull(lane.request(requireTrailing = true))
        awaited.requireRead(devboxHandle, laterVerification.sequence)
        awaited.readLanded(devboxHandle, trailing.sequence)
        assertTrue("an active trailing read cleared a later programmatic verification", awaited.isActive)
        val laterTrailing = checkNotNull(lane.finish(trailing.sequence))
        assertEquals(
            "the later verification did not reserve one new trailing read",
            laterVerification.sequence,
            laterTrailing.sequence,
        )
        assertTrue("the later trailing read did not start after the active read", laterTrailing.startsNow)
        awaited.readLanded(devboxHandle, laterTrailing.sequence)
        assertFalse("the later post-request result did not satisfy verification", awaited.isActive)
        assertNull("exactly one later trailing result must release the lane", lane.finish(laterTrailing.sequence))

        val ordinary = checkNotNull(lane.request())
        assertTrue("an idle lane must admit the next tick", ordinary.startsNow)
        assertNull("an overlapping ordinary tick must be dropped", lane.request())
        assertNull("a dropped tick must not add a trailing run", lane.finish(ordinary.sequence))

        checkNotNull(lane.request())
        checkNotNull(lane.request(requireTrailing = true))
        lane.abort()
        val afterAbort = checkNotNull(lane.request())
        assertTrue("an aborted lane must admit a new run without stale trailing work", afterAbort.startsNow)
        awaited.requireRead(devboxHandle, afterAbort.sequence)
        awaited.stop(devboxHandle)
        assertFalse("stopping the poller must finish its pending indicator", awaited.isActive)
        assertNull(lane.finish(afterAbort.sequence))
    }

    @Test
    fun `promoted verification read follows a queued mutation and observes its fence`() {
        val executor = Executors.newSingleThreadExecutor()
        val operations = InventoryOperationLane(executor) { defect -> throw defect }
        val lane = CoalescingPollLane()
        val leadingStarted = CountDownLatch(1)
        val releaseLeading = CountDownLatch(1)
        val trailingLanded = CountDownLatch(1)
        val mutationCompleted = AtomicBoolean(false)
        val trailingFollowedMutation = AtomicBoolean(false)
        val reservedMutationFence = AtomicLong()
        val trailingMutationFence = AtomicLong(-1L)
        val trailingReadSequence = AtomicLong(-1L)
        val leading = checkNotNull(lane.request())
        val awaited = AwaitedInventoryReads()

        try {
            submitCoalescedInventoryRead(operations, lane, leading) { run, completedMutationFence ->
                if (run.sequence == leading.sequence) {
                    leadingStarted.countDown()
                    check(releaseLeading.await(5, TimeUnit.SECONDS))
                } else {
                    trailingFollowedMutation.set(mutationCompleted.get())
                    trailingMutationFence.set(completedMutationFence)
                    trailingReadSequence.set(run.sequence)
                    trailingLanded.countDown()
                }
            }
            assertTrue("the leading inventory read did not start", leadingStarted.await(5, TimeUnit.SECONDS))

            operations.submitMutation(onReserved = reservedMutationFence::set) {
                mutationCompleted.set(true)
            }
            val verification = checkNotNull(lane.request(requireTrailing = true))
            awaited.requireRead(devboxHandle, verification.sequence)

            releaseLeading.countDown()
            assertTrue("the promoted verification read did not land", trailingLanded.await(5, TimeUnit.SECONDS))
            assertTrue("the promoted verification overtook the queued mutation", trailingFollowedMutation.get())
            assertEquals(
                "the promoted verification reused the leading read's stale mutation fence",
                reservedMutationFence.get(),
                trailingMutationFence.get(),
            )
            assertEquals("the required trailing sequence did not run", verification.sequence, trailingReadSequence.get())
            awaited.readLanded(devboxHandle, leading.sequence)
            assertTrue("the pre-mutation leading result cleared the awaited verification", awaited.isActive)
            awaited.readLanded(devboxHandle, trailingReadSequence.get())
            assertFalse("the post-mutation trailing result did not satisfy verification", awaited.isActive)
        } finally {
            releaseLeading.countDown()
            executor.shutdownNow()
            assertTrue("inventory executor did not terminate", executor.awaitTermination(5, TimeUnit.SECONDS))
        }
    }

    @Test
    fun `changing Forge machine clears local path and profile but preserves intent`() {
        val form = ForgeForm(
            machineHandle = devboxHandle,
            cwd = "/home/niels/src/skidbladnir",
            profile = personal,
            optionalTmuxName = "forge-review",
            objective = "Review the federation",
        )

        val changed = changeForgeDraft(form, form.copy(machineHandle = macBookHandle))
        assertEquals(macBookHandle, changed.machineHandle)
        assertTrue(changed.cwd.isEmpty())
        assertNull(changed.profile)
        assertEquals("forge-review", changed.optionalTmuxName)
        assertTrue(changed.objective == "Review the federation")
        assertEquals("Create on MacBook", forgeActionLabel(macBook.label))

        val typed = changeForgeDraft(form, form.copy(cwd = "/src/other"))
        assertEquals("/src/other", typed.cwd)
        assertEquals(personal, typed.profile)
    }

    @Test
    fun `Forge requires an explicitly chosen machine, profile, and working directory`() {
        val empty = ForgeForm(
            machineHandle = null,
            cwd = "",
            profile = null,
            optionalTmuxName = "preserved-name",
            objective = "preserved objective",
        )
        assertNull(empty.submission())

        val machineChosen = changeForgeDraft(empty, empty.copy(machineHandle = devboxHandle))
        assertEquals(devboxHandle, machineChosen.machineHandle)
        assertNull("choosing a machine must never arm a profile", machineChosen.profile)
        assertNull("a machine alone cannot submit", machineChosen.submission())

        val profileChosen = changeForgeDraft(machineChosen, machineChosen.copy(profile = personal))
        assertNull("a profile without a working directory cannot submit", profileChosen.submission())

        val ready = changeForgeDraft(profileChosen, profileChosen.copy(cwd = "/src"))
        assertEquals(
            ForgeDraft(devboxHandle, "/src", personal, "preserved-name", "preserved objective"),
            ready.submission(),
        )
    }

    @Test
    fun `resuming Forge recovery follows its machine instead of the selected filter`() {
        val recoveryDraft = ForgeDraft(
            machineHandle = devboxHandle,
            cwd = "/home/niels/src/skidbladnir",
            profile = personal,
            optionalTmuxName = "recovered-agent",
            objective = "Preserve the explicit target",
        )
        val dashboard = SkidbladnirUiState.Dashboard(
            machines = listOf(readyMachine(devbox, session()), readyMachine(macBook, session())),
            refreshing = false,
            forge = null,
            forgeRecovery = ForgeRecovery.ReviewReady(recoveryDraft),
            kill = null,
        )
        val entry = DashboardEntryState().apply {
            acceptFleet(setOf(devboxHandle, macBookHandle))
            selectScope(DashboardScope.Machine(macBookHandle))
        }

        val resumed = resumeForgeRecovery(dashboard, entry)
        assertEquals(DashboardScope.Machine(devboxHandle), entry.scope)
        assertTrue(resumed.forge?.form?.submission() == recoveryDraft)
        assertEquals(null, resumed.forgeRecovery)

        val staleDevbox = dashboard.machines.map {
            if (it.machine.handle == devboxHandle) it.inventoryFailed(GatewayFailure.Transport) else it
        }
        val refusedEntry = DashboardEntryState().apply {
            acceptFleet(setOf(devboxHandle, macBookHandle))
            selectScope(DashboardScope.Machine(macBookHandle))
        }
        val refused = resumeForgeRecovery(dashboard.copy(machines = staleDevbox), refusedEntry)
        assertEquals(DashboardScope.Machine(macBookHandle), refusedEntry.scope)
        assertEquals(null, refused.forge)
        assertTrue(refused.forgeRecovery?.draft == recoveryDraft)
    }

    @Test
    fun `a superseded snapshot disables mutations until the mutation fence is observed`() {
        val fresh = readyMachine(devbox, session())
        val snapshot = (fresh.inventory as InventoryState.Fresh).snapshot
        val superseded = fresh.copy(inventory = InventoryState.Superseded(snapshot, requiredMutationFence = 4))

        assertFalse("a machine awaiting its own mutation cannot mutate again", superseded.canMutate)
        assertFalse(superseded.canForge)
        assertEquals(
            "a superseded machine still shows its last sessions",
            snapshot.inventory.sessions.map(TmuxSession::tmuxId),
            visibleSessions(listOf(superseded), DashboardScope.All).map { it.target.session.tmuxId },
        )
        assertEquals(
            "a failed read downgrades a superseded snapshot to stale rather than losing it",
            InventoryState.Stale(snapshot, GatewayFailure.Transport),
            superseded.inventoryFailed(GatewayFailure.Transport).inventory,
        )
        assertEquals(
            InventoryState.Unreachable(GatewayFailure.Transport),
            InventoryState.Reading.downgraded(GatewayFailure.Transport),
        )
    }

    @Test
    fun `terminal admission waits for the exact post-Forge lifetime and rejects lost readiness explicitly`() {
        val target = SessionTarget(devboxHandle, session())
        val exact = readyMachine(devbox, target.session)
        val verifying = SkidbladnirUiState.Terminal(
            machine = exact,
            target = target,
            attempt = 1,
            connection = TerminalUiStatus.Verifying,
            kill = null,
        )

        assertEquals(
            TerminalUiStatus.Verifying,
            createdTerminalAdmissionStatus(verifying, completedMutationFence = 0, requiredMutationFence = 1),
        )
        assertEquals(
            TerminalUiStatus.Preparing,
            createdTerminalAdmissionStatus(verifying, completedMutationFence = 1, requiredMutationFence = 1),
        )

        val replacedLifetime = verifying.copy(
            machine = readyMachine(devbox, target.session.copy(identityToken = "replacement-lifetime")),
        )
        assertTrue(
            createdTerminalAdmissionStatus(
                replacedLifetime,
                completedMutationFence = 1,
                requiredMutationFence = 1,
            ) is TerminalUiStatus.ReconnectRequired,
        )

        val renamedSession = verifying.copy(
            machine = readyMachine(devbox, target.session.copy(tmuxName = "renamed")),
        )
        assertTrue(
            createdTerminalAdmissionStatus(
                renamedSession,
                completedMutationFence = 1,
                requiredMutationFence = 1,
            ) is TerminalUiStatus.ReconnectRequired,
        )

        val stalePage = verifying.copy(
            machine = exact.inventoryFailed(GatewayFailure.Transport),
            connection = TerminalUiStatus.Preparing,
        )
        val rejectedPage = terminalPageAdmissionStatus(stalePage)
        assertTrue(rejectedPage is TerminalUiStatus.ReconnectRequired)
        assertEquals("Devbox: reconnect required.", (rejectedPage as TerminalUiStatus.ReconnectRequired).message)

        assertEquals(TerminalUiStatus.Preparing, terminalReadAdmissionStatus(verifying, exactLifetimeAvailable = true))
        assertTrue(
            terminalReadAdmissionStatus(verifying, exactLifetimeAvailable = false) is TerminalUiStatus.ReconnectRequired,
        )
    }

    @Test
    fun `failed terminal admission read stales only its machine and fences terminal actions`() {
        val target = SessionTarget(devboxHandle, session())
        val healthy = readyMachine(macBook, session())
        val failed = readyMachine(devbox, target.session).inventoryFailed(GatewayFailure.Transport)
        val terminal = SkidbladnirUiState.Terminal(
            machine = failed,
            target = target,
            attempt = 2,
            connection = TerminalUiStatus.ReconnectRequired("Devbox: reconnect required."),
            kill = null,
        )

        assertTrue(failed.inventory is InventoryState.Stale)
        assertFalse(failed.canMutate)
        assertTrue(healthy.inventory is InventoryState.Fresh)
        assertTrue(healthy.canMutate)
        assertFalse(terminalActionAdmissible(terminal.machine.canMutate, terminal.connection))
    }

    @Test
    fun `terminal access loss returns to the affected machine dashboard with an actionable notice`() {
        val target = SessionTarget(devboxHandle, session())
        listOf(
            MachineAccess.AuthRequired to "Devbox: authentication required.",
            MachineAccess.IdentityChanged to
                "Devbox: machine identity changed. Fleet reset is required.",
        ).forEach { (access, expectedNotice) ->
            val lost = readyMachine(devbox, target.session).copy(access = access)
            val dashboardEntry = DashboardEntryState(
                DashboardEntrySnapshot(
                    schemaVersion = 1,
                    scope = DashboardScope.Machine(macBookHandle),
                    viewport = DashboardViewport(
                        anchor = DashboardCardKey("4".repeat(64)),
                        fallbackIndex = 1,
                        offsetPx = 11,
                    ),
                ),
            )
            dashboardEntry.acceptFleet(setOf(devboxHandle, macBookHandle))
            val terminal = SkidbladnirUiState.Terminal(
                machine = lost,
                target = target,
                attempt = 1,
                connection = TerminalUiStatus.Connecting,
                kill = KillState(devbox, target, pending = true),
            )
            val dashboard = dashboardAfterTerminalAccessLoss(
                terminal,
                listOf(lost, readyMachine(macBook, session())),
                refreshing = false,
                dashboardEntry = dashboardEntry,
            )

            assertEquals(
                "terminal access loss must make the affected machine the retained Dashboard scope",
                DashboardScope.Machine(devboxHandle),
                dashboardEntry.scope,
            )
            assertFalse(dashboardEntry.restorationPending)
            assertEquals(0, dashboardEntry.gridState.firstVisibleItemIndex)
            assertEquals(0, dashboardEntry.gridState.firstVisibleItemScrollOffset)
            assertEquals(expectedNotice, dashboard.notice)
            assertEquals(null, dashboard.kill)
            assertEquals(null, dashboard.forge)
            assertEquals(null, dashboard.forgeRecovery)
            assertFalse(dashboard.refreshing)
        }
    }

    @Test
    fun `Dashboard mutation access loss clears pending controls and focuses recovery`() {
        val target = SessionTarget(devboxHandle, session())
        val draft = ForgeDraft(devboxHandle, "/src", personal, "name", "objective")
        val base = SkidbladnirUiState.Dashboard(
            machines = listOf(readyMachine(devbox, target.session), readyMachine(macBook, session())),
            refreshing = false,
            forge = null,
            forgeRecovery = null,
            kill = null,
        )
        val pendingForge = base.copy(
            forge = ForgeState(
                ForgeForm(draft),
                pending = true,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
        )
        val pendingKill = base.copy(
            kill = KillState(devbox, target, pending = true),
        )

        listOf(
            MachineAccess.AuthRequired to "Devbox: authentication required.",
            MachineAccess.IdentityChanged to
                "Devbox: machine identity changed. Fleet reset is required.",
        ).forEach { (access, expectedNotice) ->
            val machines = listOf(
                readyMachine(devbox, target.session).copy(access = access),
                readyMachine(macBook, session()),
            )

            val accessFailure = GatewayFailure.Api(
                when (access) {
                    MachineAccess.AuthRequired -> ApiErrorCode.Unauthenticated
                    MachineAccess.IdentityChanged -> ApiErrorCode.MachineIdentityMismatch
                    MachineAccess.Ready -> error("test access loss cannot be ready")
                },
            )
            val createEntry = DashboardEntryState().apply {
                acceptFleet(setOf(devboxHandle, macBookHandle))
                gridState.requestScrollToItem(3, 17)
            }
            assertFalse(createEntry.restorationPending)
            val createFailed = dashboardAfterMachineAccessLoss(
                pendingForge,
                machines,
                devboxHandle,
                refreshing = true,
                dashboardEntry = createEntry,
                failure = accessFailure,
            )
            assertEquals(DashboardScope.Machine(devboxHandle), createEntry.scope)
            assertEquals(
                "Dashboard Forge access loss must not use Terminal's top reset",
                3,
                createEntry.gridState.firstVisibleItemIndex,
            )
            assertEquals(17, createEntry.gridState.firstVisibleItemScrollOffset)
            assertEquals(expectedNotice, createFailed.notice)
            assertEquals(false, createFailed.forge?.pending)
            assertEquals(ForgeFailure.Definite(accessFailure), createFailed.forge?.failure)
            assertTrue(createFailed.forge?.form?.submission() == draft)
            assertTrue("the read indicator has one owner", createFailed.refreshing)

            val killEntry = DashboardEntryState().apply {
                acceptFleet(setOf(devboxHandle, macBookHandle))
                gridState.requestScrollToItem(4, 23)
            }
            assertFalse(killEntry.restorationPending)
            val killFailed = dashboardAfterMachineAccessLoss(
                pendingKill,
                machines,
                devboxHandle,
                refreshing = false,
                dashboardEntry = killEntry,
                failure = accessFailure,
            )
            assertEquals(DashboardScope.Machine(devboxHandle), killEntry.scope)
            assertEquals(
                "Dashboard kill access loss must not use Terminal's top reset",
                4,
                killEntry.gridState.firstVisibleItemIndex,
            )
            assertEquals(23, killEntry.gridState.firstVisibleItemScrollOffset)
            assertEquals(expectedNotice, killFailed.notice)
            assertEquals(null, killFailed.kill)
            assertFalse(killFailed.refreshing)
        }
    }

    @Test
    fun `durable credential reconciliation recovers a backgrounded pairing and bearer rotation`() {
        val originalBearer = requireNotNull(GatewayBearer.parse("A".repeat(43)))
        val rotatedBearer = requireNotNull(GatewayBearer.parse("B".repeat(42) + "E"))
        val macBookBearer = requireNotNull(GatewayBearer.parse("C".repeat(42) + "I"))
        val original = MachineCredential(devbox, originalBearer)
        val rotated = MachineCredential(devbox, rotatedBearer)
        val added = MachineCredential(macBook, macBookBearer)
        val fresh = readyMachine(devbox, session())
        val current = fresh.copy(
            inventory = InventoryState.Superseded(
                (fresh.inventory as InventoryState.Fresh).snapshot,
                requiredMutationFence = 3,
            ),
        )

        val afterAdd = reconcileStoredMachines(
            currentCredentials = listOf(original),
            currentMachines = listOf(current),
            storedCredentials = listOf(original, added),
        )
        assertEquals(listOf(devboxHandle, macBookHandle), afterAdd.map { it.credential.machine.handle })
        assertTrue(current == afterAdd.single { it.credential.machine.handle == devboxHandle }.machine)
        assertEquals(
            InventoryState.Reading,
            afterAdd.single { it.credential.machine.handle == macBookHandle }.machine.inventory,
        )

        val afterRotation = reconcileStoredMachines(
            currentCredentials = listOf(original),
            currentMachines = listOf(current.copy(access = MachineAccess.AuthRequired)),
            storedCredentials = listOf(rotated),
        ).single()
        assertEquals(rotatedBearer, afterRotation.credential.bearer)
        assertEquals(MachineAccess.Ready, afterRotation.machine.access)
        assertEquals(
            "a rotated authority must not inherit the old mutation fence",
            InventoryState.Reading,
            afterRotation.machine.inventory,
        )
        assertEquals(PressureState.Reading, afterRotation.machine.pressure)
    }

    @Test
    fun `a paired credential keeps durable authority out of generated text`() {
        assertFalse(
            "the credential printed itself",
            MachineCredential(devbox, requireNotNull(GatewayBearer.parse("A".repeat(43)))).toString()
                .contains("A".repeat(43)),
        )
    }

    @Test
    fun `every request binds the exact machine origin, handle, and target`() {
        val bearer = requireNotNull(GatewayBearer.parse("A".repeat(43)))
        val credential = MachineCredential(macBook, bearer)
        val target = SessionTarget(macBookHandle, session())
        val client = GatewayClient()

        val terminal = client.terminalRequest(credential, target)
        val kill = client.killRequest(credential, target)

        assertTrue(terminal.url.host == "macbook.example.ts.net")
        assertTrue(kill.url.host == "macbook.example.ts.net")
        assertEquals(macBookHandle.encoded, terminal.header("Skidbladnir-Machine"))
        assertTrue(target.session.identityToken == terminal.header("Skidbladnir-Session-Identity"))
        assertEquals(macBookHandle.encoded, kill.header("Skidbladnir-Machine"))
        assertEquals("Kill ga-durinn on MacBook?", killConfirmationTitle(macBook.label, target))

        val ipv6 = macBook.copy(origin = requireNotNull(MachineOrigin.parse("https://[FD7A:115C:A1E0::1]:8443")))
        val ipv6Request = client.terminalRequest(MachineCredential(ipv6, bearer), target)
        assertTrue(
            "a canonical IPv6 origin was not usable as a request destination",
            ipv6Request.url.toString().startsWith("https://[fd7a:115c:a1e0::1]:8443/v1/sessions/"),
        )
    }

    @Test
    fun `rename is one unreplayed mutation followed by authoritative terminal reconciliation`() {
        val credential = MachineCredential(
            devbox,
            requireNotNull(GatewayBearer.parse("A".repeat(43))),
        )
        val original = session()
        val target = SessionTarget(devboxHandle, original)
        val client = GatewayClient()
        val request = client.renameRequest(credential, target, "review_ready")
        val body = Buffer().also { buffer -> checkNotNull(request.body).writeTo(buffer) }.readUtf8()

        assertEquals("PATCH", request.method)
        assertEquals(
            "https://devbox.example.ts.net:8443/v1/sessions/${original.tmuxId}",
            request.url.toString(),
        )
        assertEquals(
            "{\"tmuxName\":\"ga-durinn\",\"newTmuxName\":\"review_ready\"," +
                "\"identityToken\":\"${original.identityToken}\"}",
            body,
        )
        assertFalse("an unreplayable rename must keep OkHttp retry disabled", client.http.retryOnConnectionFailure)

        assertEquals(
            GatewayResult.Success(Unit),
            decodeGatewayResponse(gatewayResponse(204, ""), 204, { Unit }, ::decodeRenameHttpFailure),
        )
        assertEquals(
            GatewayResult.Failure(GatewayFailure.Transport),
            decodeGatewayResponse(gatewayResponse(204, "unexpected"), 204, { Unit }, ::decodeRenameHttpFailure),
        )
        assertThrows(ProtocolDecodeException::class.java) {
            decodeRenameHttpFailure(
                422,
                "{\"code\":\"ProfileUnknown\",\"message\":\"Choose an available profile.\"}",
            )
        }

        val editing = beginRename(target)
        assertEquals(original.tmuxName, editing.draft)
        assertFalse(renameSubmissionAdmissible(editing, target, terminalActionsAdmissible = true))
        assertFalse(
            renameSubmissionAdmissible(
                updateRenameDraft(editing, "bad name"),
                target,
                terminalActionsAdmissible = true,
            ),
        )
        val sending = checkNotNull(
            beginRenameSending(
                updateRenameDraft(editing, "review_ready"),
                target,
                terminalActionsAdmissible = true,
            ),
        )
        assertEquals(RenamePhase.Sending, sending.phase)

        val success = completeRenameHttp(sending, GatewayResult.Success(Unit))
        assertTrue(success.state.phase is RenamePhase.Reconciling)
        assertEquals(original, success.state.target.session)
        assertFalse("204 must not optimistically replace the target", success.clearMutationFence)
        assertTrue("204 requires a later authoritative inventory", success.requireInventoryRead)

        val unknown = completeRenameHttp(sending, GatewayResult.Failure(GatewayFailure.Transport))
        assertTrue("transport outcome must reconcile instead of returning to editing", unknown.state.phase is RenamePhase.Reconciling)
        assertEquals(RENAME_OUTCOME_UNKNOWN, unknown.state.error)
        assertFalse(unknown.clearMutationFence)
        assertTrue("outcome-unknown must schedule inventory even after Terminal leaves", unknown.requireInventoryRead)

        val invalid = completeRenameHttp(
            sending,
            GatewayResult.Failure(GatewayFailure.Api(ApiErrorCode.SessionNameInvalid)),
        )
        assertEquals(RenamePhase.Editing(), invalid.state.phase)
        assertEquals("review_ready", invalid.state.draft)
        assertEquals(apiErrorMessage(ApiErrorCode.SessionNameInvalid), invalid.state.error)
        assertTrue(invalid.clearMutationFence)
        assertFalse(invalid.requireInventoryRead)
        for (code in listOf(
            ApiErrorCode.InvalidRequest,
            ApiErrorCode.RequestTooLarge,
            ApiErrorCode.SessionNameInvalid,
            ApiErrorCode.SessionNameConflict,
        )) {
            val definiteInvalid = completeRenameHttp(
                sending,
                GatewayResult.Failure(GatewayFailure.Api(code)),
            )
            assertTrue("only a definite invalid/conflict result clears its exact fence", definiteInvalid.clearMutationFence)
            assertFalse(definiteInvalid.requireInventoryRead)
        }

        for (code in listOf(ApiErrorCode.SessionNotFound, ApiErrorCode.SessionIdentityMismatch)) {
            val stale = completeRenameHttp(sending, GatewayResult.Failure(GatewayFailure.Api(code)))
            assertTrue("$code requires authoritative reconciliation", stale.state.phase is RenamePhase.Reconciling)
            assertFalse(stale.clearMutationFence)
            assertTrue(stale.requireInventoryRead)
        }

        val fresh = readyMachine(devbox, original)
        val snapshot = (fresh.inventory as InventoryState.Fresh).snapshot
        val superseded = InventoryState.Superseded(snapshot, requiredMutationFence = 4)
        assertEquals(
            "an older callback must not clear a newer mutation fence",
            superseded,
            clearRenameMutationFence(superseded, fence = 3),
        )
        assertEquals(InventoryState.Fresh(snapshot), clearRenameMutationFence(superseded, fence = 4))

        val laneEvents = mutableListOf<String>()
        val lane = InventoryOperationLane(Executor(Runnable::run)) { throw it }
        lane.submitMutation(
            onReserved = { fence -> laneEvents += "reserved:$fence" },
        ) {
            laneEvents += "patch"
            lane.submitRead { completedFence -> laneEvents += "inventory:$completedFence" }
        }
        assertEquals(
            "the callback-owned inventory read must remain queued after the mutation and observe its fence",
            listOf("reserved:1", "patch", "inventory:1"),
            laneEvents,
        )

        val connection = TerminalUiStatus.Connected(2, TerminalGeometry.Constrained)
        val reconcilingTerminal = SkidbladnirUiState.Terminal(
            machine = fresh,
            target = target,
            attempt = 77,
            connection = connection,
            kill = null,
            rename = success.state,
        )
        val authoritative = original.copy(
            tmuxName = "review_ready",
            attachedClients = 2,
            activity = SessionActivity.Active,
        )
        val adopted = reconcileTerminalRename(
            reconcilingTerminal.copy(machine = readyMachine(devbox, authoritative)),
        )
        assertEquals(authoritative, adopted.terminal.target.session)
        assertEquals(77, adopted.terminal.attempt)
        assertEquals(connection, adopted.terminal.connection)
        assertNull(adopted.terminal.rename)
        assertFalse("same lifetime rename must retain page, WSS, and terminal attempt", adopted.detachTransport)

        val laterWriter = authoritative.copy(tmuxName = "later-writer")
        val staleEdit = reconcileTerminalRename(
            reconcilingTerminal.copy(machine = readyMachine(devbox, laterWriter)),
        )
        assertEquals(laterWriter, staleEdit.terminal.target.session)
        assertEquals("review_ready", staleEdit.terminal.rename?.draft)
        assertEquals(RenamePhase.Editing(stale = true), staleEdit.terminal.rename?.phase)
        assertEquals(RENAME_STALE_EDIT, staleEdit.terminal.rename?.error)
        assertFalse(staleEdit.detachTransport)

        val hidden = checkNotNull(dismissRename(success.state))
        val hiddenResolution = reconcileTerminalRename(
            reconcilingTerminal.copy(
                machine = readyMachine(devbox, laterWriter),
                rename = hidden,
            ),
        )
        assertEquals(laterWriter, hiddenResolution.terminal.target.session)
        assertNull(hiddenResolution.terminal.rename)
        assertFalse(hiddenResolution.detachTransport)

        val missing = reconcileTerminalRename(
            reconcilingTerminal.copy(machine = readyMachine(devbox)),
        )
        assertTrue(missing.terminal.connection is TerminalUiStatus.ReconnectRequired)
        assertNull(missing.terminal.rename)
        assertTrue("an absent or replaced lifetime must use the reconnect path", missing.detachTransport)

        val externalRename = reconcileTerminalRename(
            reconcilingTerminal.copy(
                machine = readyMachine(devbox, laterWriter),
                rename = null,
            ),
        )
        assertEquals(laterWriter, externalRename.terminal.target.session)
        assertEquals(77, externalRename.terminal.attempt)
        assertEquals(connection, externalRename.terminal.connection)
        assertFalse(externalRename.detachTransport)
    }

    @Test
    fun `machine notices tone absence as degraded and only trust events as failure`() {
        val pressure = decodePressureResponse(pressureJson("[\"memoryPressure\"]", linuxMetrics))
        val ready = readyMachine(devbox, session())
        val snapshot = (ready.inventory as InventoryState.Fresh).snapshot
        val stalePressure = ready.copy(pressure = PressureState.Stale(pressure, GatewayFailure.Transport))
        val unreachable = "Could not reach this machine over your Tailnet."

        assertEquals(
            "stale pressure is absent knowledge, not an alarm: a Ready machine whose pressure read " +
                "aged must not borrow the failure tone, or a routinely stale host makes the alarm " +
                "colour the dashboard's resting state",
            NoticeTone.Degraded,
            machineNotice(stalePressure)?.tone,
        )

        // The retained-card marker and machine notice share the availability classifier. This
        // table prevents an auth or identity fence from being mislabeled as stale.
        val brokenTrustOnFreshInventory = ready.copy(access = MachineAccess.AuthRequired)
        assertTrue(
            "the fixture must hold a Fresh inventory for this to prove anything",
            brokenTrustOnFreshInventory.inventory is InventoryState.Fresh &&
                !brokenTrustOnFreshInventory.canMutate,
        )
        assertEquals(
            "a machine that cannot be trusted is loud even when its sessions are current",
            NoticeTone.Failure,
            availabilityTone(machineAvailability(brokenTrustOnFreshInventory)),
        )
        val retainedCardCases = listOf(
            ready to null,
            ready.copy(inventory = InventoryState.Superseded(snapshot, requiredMutationFence = 4)) to
                SessionAvailabilityContent("REFRESHING · actions disabled", NoticeTone.Degraded),
            ready.copy(inventory = InventoryState.Stale(snapshot, GatewayFailure.Transport)) to
                SessionAvailabilityContent("STALE · actions disabled", NoticeTone.Degraded),
            brokenTrustOnFreshInventory to
                SessionAvailabilityContent("AUTH REQUIRED · actions disabled", NoticeTone.Failure),
            ready.copy(access = MachineAccess.IdentityChanged) to
                SessionAvailabilityContent("IDENTITY CHANGED · actions disabled", NoticeTone.Failure),
            ready.copy(inventory = InventoryState.Reading) to null,
            ready.copy(inventory = InventoryState.Unreachable(GatewayFailure.Transport)) to null,
        )
        retainedCardCases.forEach { (machine, expected) ->
            assertEquals(machineAvailability(machine).toString(), expected, sessionAvailabilityContent(machine))
        }

        // The table below is machineNotice's coverage record: a new MachineAvailability
        // variant must be given a row here.
        val cases: List<Pair<MachineState, MachineNotice?>> = listOf(
            ready to null,
            ready.copy(pressure = PressureState.Fresh(pressure)) to null,
            stalePressure to
                MachineNotice("Devbox: pressure is STALE. Sessions remain current.", NoticeTone.Degraded),
            ready.copy(pressure = PressureState.Unavailable(GatewayFailure.Transport)) to
                MachineNotice("Devbox: pressure unavailable. Sessions remain current.", NoticeTone.Degraded),
            ready.copy(inventory = InventoryState.Reading) to
                MachineNotice("Devbox: reading tmux sessions.", NoticeTone.Degraded),
            ready.copy(inventory = InventoryState.Superseded(snapshot, requiredMutationFence = 4)) to
                MachineNotice("Devbox: confirming the latest tmux inventory. Actions disabled.", NoticeTone.Degraded),
            ready.copy(inventory = InventoryState.Stale(snapshot, GatewayFailure.Transport)) to
                MachineNotice(
                    "Devbox: $unreachable Prior sessions are STALE; actions disabled. Pull down to check again.",
                    NoticeTone.Degraded,
                ),
            ready.copy(inventory = InventoryState.Unreachable(GatewayFailure.Transport)) to
                MachineNotice("Devbox: $unreachable Pull down to check again.", NoticeTone.Degraded),
            ready.copy(access = MachineAccess.AuthRequired) to
                MachineNotice("Devbox: authentication required. Actions disabled.", NoticeTone.Failure),
            ready.copy(access = MachineAccess.IdentityChanged) to
                MachineNotice("Devbox: identity changed. Fleet reset is required.", NoticeTone.Failure),
        )
        cases.forEach { (machine, expected) ->
            assertEquals(
                "${machineAvailability(machine)} with ${machine.pressure::class.simpleName} pressure",
                expected,
                machineNotice(machine),
            )
        }
        assertEquals(
            "Devbox · IDENTITY CHANGED",
            forgeMachineChoiceLabel(ready.copy(access = MachineAccess.IdentityChanged)),
        )
    }

    private fun readyMachine(machine: PairedMachine, vararg sessions: TmuxSession): MachineState = MachineState(
        machine = machine,
        access = MachineAccess.Ready,
        inventory = InventoryState.Fresh(
            InventorySnapshot(
                SessionsResponse(
                    machine = MachineSummary(machine.handle, MachinePlatform.Linux),
                    observedAt = OBSERVED_AT,
                    profiles = listOf(
                        ProfileChoice(personal, "Codex · Personal", AgentProvider.Codex),
                    ),
                    sessions = sessions.toList(),
                ),
                receivedAtElapsedMillis = 1_000,
            ),
        ),
        pressure = PressureState.Reading,
    )

    private fun gatewayResponse(code: Int, body: String): Response = Response.Builder()
        .request(Request.Builder().url("https://gateway.example.ts.net:8443/v1/sessions/session-1").build())
        .protocol(Protocol.HTTP_1_1)
        .code(code)
        .message(if (code == 204) "No Content" else "Error")
        .body(body.toResponseBody())
        .build()

    private fun inventoryJson(handle: MachineHandle, platform: String): String =
        """{"machine":{"handle":"${handle.encoded}","platform":"$platform"},"observedAt":"2026-08-26T12:00:00Z","profiles":[{"key":"personal","label":"Codex · Personal","provider":"Codex"}],"sessions":[]}"""

    private fun activitySession(
        tmuxId: String,
        tmuxName: String,
        activity: String,
    ): TmuxSession {
        val session =
            """{"tmuxId":"$tmuxId","tmuxName":"$tmuxName","identityToken":"token-$tmuxId","character":{"key":"norse.durinn","displayName":"Durinn"},"attachedClients":0,"activity":"$activity"}"""
        return decodeSessionsResponse(
            """{"machine":{"handle":"${devboxHandle.encoded}","platform":"Linux"},"observedAt":"2026-08-26T12:00:00Z","profiles":[{"key":"personal","label":"Codex · Personal","provider":"Codex"}],"sessions":[$session]}""",
        ).sessions.single()
    }

    private fun session(
        tmuxId: String = tmuxId(1),
        tmuxName: String = "ga-durinn",
        identityToken: String = "v1-0123456789abcdef0123456789abcdef.100.200.1",
    ): TmuxSession = TmuxSession(
        tmuxId = tmuxId,
        tmuxName = tmuxName,
        identityToken = identityToken,
        launchProfile = personal,
        objective = null,
        character = CharacterSummary("norse.durinn", "Durinn"),
        cwd = "/src/skidbladnir",
        activeCommand = "codex",
        attachedClients = 1,
        activity = SessionActivity.Quiet,
        agent = AgentRuntime(AgentProvider.Codex, pid = 1234, profile = personal),
    )

    private fun pressureJson(unsupported: String, metrics: String): String {
        val sample =
            """{"sampledAt":"2026-08-26T12:00:00Z","level":"Normal","phase":"Steady","reasons":[],"signals":$metrics,"missing":[]}"""
        val history = """{"sampledAt":"2026-08-26T12:00:00Z","level":"Normal"}"""
        return """{"unsupported":$unsupported,"current":$sample,"history":[$history]}"""
    }

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        fun tmuxId(index: Int): String = "${'$'}$index"

        const val linuxMetrics =
            """{"cpuPercent":{"value":12.5,"state":"Informational"},"normalizedLoad":{"value":0.4,"state":"Normal"},"memoryAvailablePercent":{"value":42.0,"state":"Normal"},"swapUsedPercent":{"value":0.0,"state":"Informational"},"diskAvailablePercent":{"value":60.0,"state":"Normal"},"cpuPsiSomeAvg60Percent":{"value":0.0,"state":"Normal"},"memoryPsiFullAvg60Percent":{"value":0.0,"state":"Normal"},"ioPsiFullAvg60Percent":{"value":0.0,"state":"Normal"}}"""
    }
}
