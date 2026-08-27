package dev.niels.skidbladnir

import java.time.Instant
import java.util.Locale
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
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
    fun `pressure capability is strict for Linux and Darwin`() {
        val linux = decodePressureResponse(pressureJson("[\"memoryPressure\"]", linuxMetrics))
        assertEquals(listOf(PressureMetric.MemoryPressure), linux.unsupported)
        assertEquals(null, linux.current.metrics.memoryPressure)

        val darwinUnsupported =
            "[\"cpuPsiSomeAvg60Percent\",\"ioPsiFullAvg60Percent\",\"memoryAvailablePercent\",\"memoryPsiFullAvg60Percent\"]"
        val darwin = decodePressureResponse(pressureJson(darwinUnsupported, darwinMetrics))
        assertEquals(SystemMemoryPressure.Normal, darwin.current.metrics.memoryPressure)
        assertEquals(4, darwin.unsupported.size)

        assertThrows(ProtocolDecodeException::class.java) {
            decodePressureResponse(
                pressureJson(
                    "[\"memoryAvailablePercent\",\"cpuPsiSomeAvg60Percent\",\"ioPsiFullAvg60Percent\",\"memoryPsiFullAvg60Percent\"]",
                    darwinMetrics,
                ),
            )
        }
    }

    @Test
    fun `equal local sessions remain distinct and one failure cannot stale the other`() {
        val duplicate = session()
        val initial = listOf(readyMachine(macBook, duplicate), readyMachine(devbox, duplicate))

        val agents = visibleAgents(initial, selectedMachine = null)
        assertEquals(listOf("Devbox", "MacBook"), agents.map { it.machine.label.text })
        assertTrue(agents[0].target != agents[1].target)
        assertEquals(2, agents.map(VisibleAgent::target).distinct().size)

        val failed = initial.map {
            if (it.machine.handle == devboxHandle) it.inventoryFailed(GatewayFailure.Transport) else it
        }
        assertTrue(failed.single { it.machine.handle == devboxHandle }.inventory is InventoryState.Stale)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.inventory is InventoryState.Fresh)
        assertFalse(failed.single { it.machine.handle == devboxHandle }.canMutate)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.canMutate)
    }

    @Test
    fun `agents sort by attention, then status, machine label, session name, and local tmux ID`() {
        val alpha = devbox.copy(label = requireNotNull(MachineLabel.parse("Alpha")))
        val beta = macBook.copy(label = requireNotNull(MachineLabel.parse("Beta")))
        val alphaMachine = readyMachine(
            alpha,
            session(id = tmuxId(1), name = "zeta", status = status(SessionStatusKind.Idle)),
            session(id = tmuxId(3), name = "beta"),
            session(id = tmuxId(5), name = "Alpha"),
            session(id = tmuxId(0), name = "Alpha"),
        )
        val betaMachine = readyMachine(
            beta,
            session(id = tmuxId(2), name = "aaa", attention = true, status = status(SessionStatusKind.Unknown)),
            session(id = tmuxId(4), name = "alpha"),
        )

        assertEquals(
            listOf(
                "Beta/aaa/${tmuxId(2)}",
                "Alpha/Alpha/${tmuxId(0)}",
                "Alpha/Alpha/${tmuxId(5)}",
                "Alpha/beta/${tmuxId(3)}",
                "Beta/alpha/${tmuxId(4)}",
                "Alpha/zeta/${tmuxId(1)}",
            ),
            visibleAgents(listOf(betaMachine, alphaMachine), selectedMachine = null).map {
                "${it.machine.label.text}/${it.target.session.name}/${it.target.session.id}"
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
                visibleAgents(
                    listOf(readyMachine(zeta, session()), readyMachine(iota, session())),
                    selectedMachine = null,
                ).map { it.machine.label.text },
            )

            val sessions = readyMachine(
                devbox,
                session(id = tmuxId(2), name = "Zeta", identityToken = "token-zeta"),
                session(id = tmuxId(1), name = "Iota", identityToken = "token-iota"),
            )
            assertEquals(
                listOf("Iota", "Zeta"),
                visibleAgents(listOf(sessions), selectedMachine = null).map { it.target.session.name },
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

        assertTrue("the leading tick must be admitted", lane.tryStart())
        assertFalse("a tick must never run beside the leading one", lane.tryStart(requireTrailing = true))
        assertFalse("further ticks must coalesce into that same trailing run", lane.tryStart(requireTrailing = true))
        assertTrue("coalesced ticks owe exactly one trailing run", lane.finish())
        assertFalse("the trailing run releases the lane", lane.finish())

        assertTrue("an idle lane admits the next tick", lane.tryStart())
        assertFalse("a scheduled tick with no trailing requirement is dropped", lane.tryStart())
        assertFalse("a dropped tick owes no trailing run", lane.finish())

        assertTrue(lane.tryStart())
        assertFalse(lane.tryStart(requireTrailing = true))
        lane.abort()
        assertTrue("an aborted lane admits the next tick without a stale trailing run", lane.tryStart())
        assertFalse(lane.finish())
    }

    @Test
    fun `changing Forge machine clears local path and profile but preserves intent`() {
        val form = ForgeForm(
            machineHandle = devboxHandle,
            cwd = "/home/niels/src/skidbladnir",
            profile = personal,
            optionalName = "forge-review",
            objective = "Review the federation",
        )

        val changed = changeForgeDraft(form, form.copy(machineHandle = macBookHandle))
        assertEquals(macBookHandle, changed.machineHandle)
        assertTrue(changed.cwd.isEmpty())
        assertNull(changed.profile)
        assertEquals("forge-review", changed.optionalName)
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
            optionalName = "preserved-name",
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
            optionalName = "recovered-agent",
            objective = "Preserve the explicit target",
        )
        val dashboard = SkidbladnirUiState.Dashboard(
            machines = listOf(readyMachine(devbox, session()), readyMachine(macBook, session())),
            selectedMachine = macBookHandle,
            refreshing = false,
            forge = null,
            forgeRecovery = ForgeRecovery.ReviewReady(recoveryDraft),
            kill = null,
        )

        val resumed = resumeForgeRecovery(dashboard)
        assertEquals(devboxHandle, resumed.selectedMachine)
        assertTrue(resumed.forge?.form?.submission() == recoveryDraft)
        assertEquals(null, resumed.forgeRecovery)

        val staleDevbox = dashboard.machines.map {
            if (it.machine.handle == devboxHandle) it.inventoryFailed(GatewayFailure.Transport) else it
        }
        val refused = resumeForgeRecovery(dashboard.copy(machines = staleDevbox))
        assertEquals(macBookHandle, refused.selectedMachine)
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
            snapshot.inventory.sessions.map(AgentSession::id),
            visibleAgents(listOf(superseded), selectedMachine = null).map { it.target.session.id },
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
        val target = AgentTarget(devboxHandle, session())
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
        val target = AgentTarget(devboxHandle, session())
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
        val target = AgentTarget(devboxHandle, session())
        listOf(
            MachineAccess.AuthRequired to "Devbox: authentication required.",
            MachineAccess.IdentityChanged to
                "Devbox: machine identity changed. Provisioning repair is required.",
        ).forEach { (access, expectedNotice) ->
            val lost = readyMachine(devbox, target.session).copy(access = access)
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
            )

            assertEquals(devboxHandle, dashboard.selectedMachine)
            assertEquals(expectedNotice, dashboard.notice)
            assertEquals(null, dashboard.kill)
            assertEquals(null, dashboard.forge)
            assertEquals(null, dashboard.forgeRecovery)
            assertFalse(dashboard.refreshing)
        }
    }

    @Test
    fun `Dashboard mutation access loss clears pending controls and focuses recovery`() {
        val target = AgentTarget(devboxHandle, session())
        val draft = ForgeDraft(devboxHandle, "/src", personal, "name", "objective")
        val base = SkidbladnirUiState.Dashboard(
            machines = listOf(readyMachine(devbox, target.session), readyMachine(macBook, session())),
            selectedMachine = null,
            refreshing = false,
            forge = null,
            forgeRecovery = null,
            kill = null,
        )
        val pendingForge = base.copy(
            forge = ForgeState(ForgeForm(draft), pending = true, error = null),
        )
        val pendingKill = base.copy(
            kill = KillState(devbox, target, pending = true),
        )

        listOf(
            MachineAccess.AuthRequired to "Devbox: authentication required.",
            MachineAccess.IdentityChanged to
                "Devbox: machine identity changed. Provisioning repair is required.",
        ).forEach { (access, expectedNotice) ->
            val machines = listOf(
                readyMachine(devbox, target.session).copy(access = access),
                readyMachine(macBook, session()),
            )

            val createFailed = dashboardAfterMachineAccessLoss(pendingForge, machines, devboxHandle, refreshing = true)
            assertEquals(devboxHandle, createFailed.selectedMachine)
            assertEquals(expectedNotice, createFailed.notice)
            assertEquals(false, createFailed.forge?.pending)
            assertEquals(expectedNotice, createFailed.forge?.error)
            assertTrue(createFailed.forge?.form?.submission() == draft)
            assertTrue("the read indicator has one owner", createFailed.refreshing)

            val killFailed = dashboardAfterMachineAccessLoss(pendingKill, machines, devboxHandle, refreshing = false)
            assertEquals(devboxHandle, killFailed.selectedMachine)
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
    fun `repair rejects another machine bearer before contacting any gateway`() {
        val devboxBearer = requireNotNull(GatewayBearer.parse("A".repeat(43)))
        val macBookBearer = requireNotNull(GatewayBearer.parse("B".repeat(42) + "E"))
        val credentials = listOf(
            MachineCredential(devbox, devboxBearer),
            MachineCredential(macBook, macBookBearer),
        )

        assertTrue(
            bearerRepairConflict(
                credentials,
                storageComplete = true,
                targetHandle = macBookHandle,
                bearer = devboxBearer,
            ),
        )
        assertFalse(
            bearerRepairConflict(
                credentials,
                storageComplete = true,
                targetHandle = macBookHandle,
                bearer = macBookBearer,
            ),
        )
        assertTrue(
            bearerRepairConflict(
                credentials,
                storageComplete = false,
                targetHandle = macBookHandle,
                bearer = macBookBearer,
            ),
        )
    }

    @Test
    fun `a redacting bearer draft keeps credential material out of generated text`() {
        val repair = SkidbladnirUiState.BearerRepair(
            machine = devbox,
            bearer = BearerDraft("A".repeat(43)),
            pending = false,
            error = null,
        )
        assertFalse("the UI state printed the bearer draft", repair.toString().contains("A".repeat(43)))
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
        val target = AgentTarget(macBookHandle, session())
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

    private fun readyMachine(machine: PairedMachine, vararg sessions: AgentSession): MachineState = MachineState(
        machine = machine,
        access = MachineAccess.Ready,
        inventory = InventoryState.Fresh(
            InventorySnapshot(
                SessionsResponse(
                    machine = MachineSummary(machine.handle, MachinePlatform.Linux),
                    observedAt = OBSERVED_AT,
                    profiles = listOf(ProfileChoice(personal, "Personal")),
                    sessions = sessions.toList(),
                ),
                receivedAtElapsedMillis = 1_000,
            ),
        ),
        pressure = PressureState.Reading,
    )

    private fun inventoryJson(handle: MachineHandle, platform: String): String =
        """{"machine":{"handle":"${handle.encoded}","platform":"$platform"},"observedAt":"2026-08-26T12:00:00Z","profiles":[{"key":"personal","label":"Personal"}],"sessions":[]}"""

    private fun status(kind: SessionStatusKind): SessionStatus = SessionStatus(
        kind,
        when (kind) {
            SessionStatusKind.Working, SessionStatusKind.Idle -> SessionStatusSignal.Lifecycle
            SessionStatusKind.Running, SessionStatusKind.Shell -> SessionStatusSignal.Process
            SessionStatusKind.Unknown -> SessionStatusSignal.PollFailure
        },
        SIGNAL_AT,
    )

    private fun session(
        id: String = tmuxId(1),
        name: String = "ga-durinn",
        identityToken: String = "v1-0123456789abcdef0123456789abcdef.100.200.1",
        attention: Boolean = false,
        status: SessionStatus = status(SessionStatusKind.Working),
    ): AgentSession = AgentSession(
        id = id,
        name = name,
        identityToken = identityToken,
        profile = "personal",
        objective = null,
        character = CharacterSummary("norse.durinn", "Durinn"),
        cwd = "/src/skidbladnir",
        activeCommand = "codex",
        attachedClients = 1,
        attention = attention,
        status = status,
    )

    private fun pressureJson(unsupported: String, metrics: String): String {
        val sample =
            """{"sampledAt":"2026-08-26T12:00:00Z","level":"Normal","reasons":[],"metrics":$metrics,"missing":[]}"""
        return """{"unsupported":$unsupported,"current":$sample,"history":[$sample]}"""
    }

    private companion object {
        val OBSERVED_AT: Instant = Instant.parse("2026-08-26T12:00:00Z")
        val SIGNAL_AT: Instant = Instant.parse("2026-08-26T11:59:55Z")

        fun tmuxId(index: Int): String = "${'$'}$index"

        const val linuxMetrics =
            """{"cpuPercent":12.5,"normalizedLoad":0.4,"memoryAvailablePercent":42.0,"swapUsedPercent":0.0,"diskAvailablePercent":60.0,"cpuPsiSomeAvg60Percent":0.0,"memoryPsiFullAvg60Percent":0.0,"ioPsiFullAvg60Percent":0.0}"""
        const val darwinMetrics =
            """{"cpuPercent":12.5,"normalizedLoad":0.4,"swapUsedPercent":0.0,"diskAvailablePercent":60.0,"memoryPressure":"Normal"}"""
    }
}
