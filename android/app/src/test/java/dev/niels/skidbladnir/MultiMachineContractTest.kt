package dev.niels.skidbladnir

import java.util.Locale
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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
        ).forEach { assertEquals(null, MachineOrigin.parse(it)) }

        val mixedCase = requireNotNull(MachineOrigin.parse("https://DevBox.Example.TS.NET:8443"))
        assertTrue(devbox.origin == mixedCase)
        assertTrue(mixedCase.encoded == "https://devbox.example.ts.net:8443/")
    }

    @Test
    fun `machine label rename preserves authority and rejects case insensitive collisions`() {
        val initial = listOf(readyMachine(devbox, session()), readyMachine(macBook, session()))
        val renamed = renameMachineLabel(
            machines = initial,
            handle = devboxHandle,
            label = requireNotNull(MachineLabel.parse("Build Mac")),
        )

        val updated = renamed.single { it.machine.handle == devboxHandle }.machine
        assertEquals("Build Mac", updated.label.text)
        assertEquals(devbox.handle, updated.handle)
        assertTrue(devbox.origin == updated.origin)
        assertEquals("Devbox", initial.single { it.machine.handle == devboxHandle }.machine.label.text)
        assertThrows(IllegalArgumentException::class.java) {
            renameMachineLabel(
                initial,
                devboxHandle,
                requireNotNull(MachineLabel.parse("macbook")),
            )
        }
    }

    @Test
    fun `inventory requires a strict machine envelope and closed platform`() {
        val inventory = decodeSessionsResponse(inventoryJson(devboxHandle, "Linux"))
        assertEquals(devboxHandle, inventory.machine.handle)
        assertEquals(MachinePlatform.Linux, inventory.machine.platform)

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
        val devboxState = readyMachine(devbox, duplicate)
        val macBookState = readyMachine(macBook, duplicate)
        val initial = listOf(macBookState, devboxState)

        val agents = visibleAgents(initial, selectedMachine = null)
        assertEquals(listOf("Devbox", "MacBook"), agents.map { it.machine.label.text })
        assertTrue(agents[0].target != agents[1].target)
        assertEquals(2, agents.map(VisibleAgent::target).distinct().size)

        val failed = markInventoryFailure(initial, devboxHandle, GatewayFailure.Transport)
        assertTrue(failed.single { it.machine.handle == devboxHandle }.inventory is InventoryState.Stale)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.inventory is InventoryState.Fresh)
        assertFalse(failed.single { it.machine.handle == devboxHandle }.canMutate)
        assertTrue(failed.single { it.machine.handle == macBookHandle }.canMutate)
    }

    @Test
    fun `inventory work serializes per machine while other machines and one trailing poll progress`() {
        val executor = Executors.newFixedThreadPool(2)
        val operations = MachineInventoryOperations(executor)
        val devboxStarted = CountDownLatch(1)
        val releaseDevbox = CountDownLatch(1)
        val devboxRead = CountDownLatch(1)
        val macBookRead = CountDownLatch(1)
        val devboxOrder = CopyOnWriteArrayList<String>()
        val observedFence = AtomicLong()
        val pollLane = CoalescingPollLane()
        val pollStarted = CountDownLatch(1)
        val releasePoll = CountDownLatch(1)
        val pollRuns = AtomicInteger()

        try {
            val reservedFence = operations.forMachine(devboxHandle).submitMutation { fence ->
                devboxOrder += "mutation-start:$fence"
                devboxStarted.countDown()
                check(releaseDevbox.await(5, TimeUnit.SECONDS))
                devboxOrder += "mutation-finish:$fence"
            }
            assertTrue("same-machine mutation did not start", devboxStarted.await(5, TimeUnit.SECONDS))
            operations.forMachine(devboxHandle).submitRead { fence ->
                observedFence.set(fence)
                devboxOrder += "read:$fence"
                devboxRead.countDown()
            }
            operations.forMachine(macBookHandle).submitRead { macBookRead.countDown() }
            assertTrue("other machine did not progress independently", macBookRead.await(5, TimeUnit.SECONDS))
            releaseDevbox.countDown()
            assertTrue("same-machine read did not follow mutation", devboxRead.await(5, TimeUnit.SECONDS))
            assertEquals(listOf("mutation-start:1", "mutation-finish:1", "read:1"), devboxOrder)
            assertEquals(reservedFence, observedFence.get())

            assertTrue(pollLane.tryStart())
            val poll = executor.submit {
                do {
                    pollRuns.incrementAndGet()
                    if (pollRuns.get() == 1) {
                        pollStarted.countDown()
                        check(releasePoll.await(5, TimeUnit.SECONDS))
                    }
                } while (pollLane.finish())
            }
            assertTrue("leading poll did not start", pollStarted.await(5, TimeUnit.SECONDS))
            assertFalse(pollLane.tryStart(requireTrailing = true))
            assertFalse(pollLane.tryStart(requireTrailing = true))
            releasePoll.countDown()
            poll.get(5, TimeUnit.SECONDS)
            assertEquals("multiple ticks must coalesce to one trailing poll", 2, pollRuns.get())
        } finally {
            releaseDevbox.countDown()
            releasePoll.countDown()
            executor.shutdownNow()
            assertTrue("inventory executor did not terminate", executor.awaitTermination(5, TimeUnit.SECONDS))
        }
    }

    @Test
    fun `changing Forge machine clears local path and profile but preserves intent`() {
        val changed = changeForgeMachine(
            ForgeForm(
                machineHandle = devboxHandle,
                cwd = "/home/niels/src/skidbladnir",
                profile = "personal",
                optionalName = "forge-review",
                objective = "Review the federation",
            ),
            macBookHandle,
        )

        assertEquals(macBookHandle, changed.machineHandle)
        assertTrue(changed.cwd.isEmpty())
        assertEquals("", changed.profile)
        assertEquals("forge-review", changed.optionalName)
        assertTrue(changed.objective == "Review the federation")
        assertEquals("Create on MacBook", forgeActionLabel(macBook.label))
    }

    @Test
    fun `Forge opened from All requires an explicit machine before submission`() {
        val form = ForgeForm(
            machineHandle = null,
            cwd = "",
            profile = "",
            optionalName = "preserved-name",
            objective = "preserved objective",
        )
        assertEquals(null, form.submission())

        val selected = changeForgeMachine(form, devboxHandle)
        assertEquals(devboxHandle, selected.machineHandle)
        assertTrue(selected.cwd.isEmpty())
        assertEquals("", selected.profile)
        assertEquals("preserved-name", selected.optionalName)
        assertTrue(selected.objective == "preserved objective")
    }

    @Test
    fun `resuming Forge recovery follows its machine instead of the selected filter`() {
        val recoveryDraft = ForgeDraft(
            machineHandle = devboxHandle,
            cwd = "/home/niels/src/skidbladnir",
            profile = "personal",
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

        val staleDevbox = markInventoryFailure(dashboard.machines, devboxHandle, GatewayFailure.Transport)
        val refused = resumeForgeRecovery(dashboard.copy(machines = staleDevbox))
        assertEquals(macBookHandle, refused.selectedMachine)
        assertEquals(null, refused.forge)
        assertTrue(refused.forgeRecovery?.draft == recoveryDraft)
    }

    @Test
    fun `terminal admission waits for the exact post-Forge lifetime and rejects lost readiness explicitly`() {
        val target = AgentTarget(devboxHandle, session())
        val verifying = SkidbladnirUiState.Terminal(
            machine = devbox,
            target = target,
            machineCanMutate = true,
            attempt = 1,
            connection = TerminalUiStatus.Verifying,
            kill = null,
        )
        val exact = readyMachine(devbox, target.session)

        assertEquals(
            TerminalUiStatus.Verifying,
            createdTerminalAdmissionStatus(verifying, exact, completedMutationFence = 0, requiredMutationFence = 1),
        )
        assertEquals(
            TerminalUiStatus.Preparing,
            createdTerminalAdmissionStatus(verifying, exact, completedMutationFence = 1, requiredMutationFence = 1),
        )

        assertTrue(
            createdTerminalAdmissionStatus(
                verifying,
                readyMachine(devbox, target.session.copy(identityToken = "replacement-lifetime")),
                completedMutationFence = 1,
                requiredMutationFence = 1,
            ) is TerminalUiStatus.ReconnectRequired,
        )

        val preparing = verifying.copy(connection = TerminalUiStatus.Preparing)
        val stale = exact.copy(
            inventory = InventoryState.Stale(
                (exact.inventory as InventoryState.Fresh).snapshot,
                GatewayFailure.Transport,
            ),
        )
        val rejectedPage = terminalPageAdmissionStatus(preparing, stale)
        assertTrue(rejectedPage is TerminalUiStatus.ReconnectRequired)
        assertEquals("Devbox: reconnect required.", (rejectedPage as TerminalUiStatus.ReconnectRequired).message)

    }

    @Test
    fun `failed terminal admission read stales only its machine and fences terminal actions`() {
        val target = AgentTarget(devboxHandle, session())
        val terminal = SkidbladnirUiState.Terminal(
            machine = devbox,
            target = target,
            machineCanMutate = true,
            attempt = 2,
            connection = TerminalUiStatus.Verifying,
            kill = null,
        )
        val healthy = readyMachine(macBook, session())
        val failed = terminalInventoryReadFailure(
            readyMachine(devbox, target.session),
            GatewayFailure.Transport,
        )
        val synchronized = synchronizeTerminalMachineState(terminal, failed).copy(
            connection = TerminalUiStatus.ReconnectRequired("Devbox: reconnect required."),
        )

        assertTrue(failed.inventory is InventoryState.Stale)
        assertFalse(failed.canMutate)
        assertTrue(healthy.inventory is InventoryState.Fresh)
        assertTrue(healthy.canMutate)
        assertFalse(terminalActionAdmissible(synchronized.machineCanMutate, synchronized.connection))
    }

    @Test
    fun `terminal access loss returns to the affected machine dashboard with an actionable notice`() {
        val target = AgentTarget(devboxHandle, session())
        val terminal = SkidbladnirUiState.Terminal(
            machine = devbox,
            target = target,
            machineCanMutate = true,
            attempt = 1,
            connection = TerminalUiStatus.Connecting,
            kill = KillState(devbox, target, pending = true),
        )
        listOf(
            MachineAccess.AuthRequired to "Devbox: authentication required.",
            MachineAccess.IdentityChanged to
                "Devbox: machine identity changed. Remove and pair it again.",
        ).forEach { (access, expectedNotice) ->
            val lost = readyMachine(devbox, target.session).copy(access = access)
            val dashboard = dashboardAfterTerminalAccessLoss(
                terminal,
                listOf(lost, readyMachine(macBook, session())),
            )

            assertEquals(devboxHandle, dashboard.selectedMachine)
            assertEquals(expectedNotice, dashboard.notice)
            assertEquals(null, dashboard.kill)
            assertEquals(null, dashboard.forge)
            assertEquals(null, dashboard.forgeRecovery)
        }
    }

    @Test
    fun `Dashboard mutation access loss clears pending controls and focuses recovery`() {
        val target = AgentTarget(devboxHandle, session())
        val draft = ForgeDraft(devboxHandle, "/src", "personal", "name", "objective")
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
                "Devbox: machine identity changed. Remove and pair it again.",
        ).forEach { (access, expectedNotice) ->
            val machines = listOf(
                readyMachine(devbox, target.session).copy(access = access),
                readyMachine(macBook, session()),
            )

            val createFailed = dashboardAfterMachineAccessLoss(pendingForge, machines, devboxHandle)
            assertEquals(devboxHandle, createFailed.selectedMachine)
            assertEquals(expectedNotice, createFailed.notice)
            assertEquals(false, createFailed.forge?.pending)
            assertEquals(expectedNotice, createFailed.forge?.error)
            assertTrue(createFailed.forge?.form?.submission() == draft)

            val killFailed = dashboardAfterMachineAccessLoss(pendingKill, machines, devboxHandle)
            assertEquals(devboxHandle, killFailed.selectedMachine)
            assertEquals(expectedNotice, killFailed.notice)
            assertEquals(null, killFailed.kill)
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
        val current = readyMachine(devbox, session()).copy(inventoryRefreshRequired = true)

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
        assertEquals(InventoryState.Reading, afterRotation.machine.inventory)
        assertEquals(PressureState.Reading, afterRotation.machine.pressure)
        assertFalse(afterRotation.machine.inventoryRefreshRequired)
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
            pairingAuthorityConflict(
                credentials,
                storageComplete = true,
                repairHandle = macBookHandle,
                label = macBook.label,
                origin = macBook.origin,
                bearer = devboxBearer,
            ),
        )
        assertFalse(
            pairingAuthorityConflict(
                credentials,
                storageComplete = true,
                repairHandle = macBookHandle,
                label = macBook.label,
                origin = macBook.origin,
                bearer = macBookBearer,
            ),
        )
        assertTrue(
            pairingAuthorityConflict(
                credentials,
                storageComplete = false,
                repairHandle = macBookHandle,
                label = macBook.label,
                origin = macBook.origin,
                bearer = macBookBearer,
            ),
        )
    }

    @Test
    fun `removing the selected machine clears every machine-owned dashboard reference`() {
        val target = AgentTarget(devboxHandle, session())
        val draft = ForgeDraft(devboxHandle, "/src", "personal", "name", "objective")
        val dashboard = SkidbladnirUiState.Dashboard(
            machines = listOf(readyMachine(devbox, session()), readyMachine(macBook, session())),
            selectedMachine = devboxHandle,
            refreshing = false,
            forge = ForgeState(ForgeForm(draft), pending = false, error = null),
            forgeRecovery = ForgeRecovery.ReviewReady(draft),
            rename = RenameState(devbox, "Devbox", pending = false, error = null),
            kill = KillState(devbox, target, pending = false),
        )

        val removed = removeMachineReferences(dashboard, devboxHandle)
        assertEquals(null, removed.selectedMachine)
        assertEquals(null, removed.forge)
        assertEquals(null, removed.forgeRecovery)
        assertEquals(null, removed.rename)
        assertEquals(null, removed.kill)
        assertEquals(2, removed.machines.size)
    }

    @Test
    fun `machine and session ordering is invariant under Turkish locale`() {
        val original = Locale.getDefault()
        Locale.setDefault(Locale.forLanguageTag("tr-TR"))
        try {
            val iota = devbox.copy(label = requireNotNull(MachineLabel.parse("Iota")))
            val zeta = macBook.copy(label = requireNotNull(MachineLabel.parse("Zeta")))
            val machineAgents = visibleAgents(
                listOf(readyMachine(zeta, session()), readyMachine(iota, session())),
                selectedMachine = null,
            )
            assertEquals(listOf("Iota", "Zeta"), machineAgents.map { it.machine.label.text })

            val first = session().copy(id = "${'$'}1", name = "Iota", identityToken = "token-iota")
            val second = session().copy(id = "${'$'}2", name = "Zeta", identityToken = "token-zeta")
            val base = readyMachine(devbox, first)
            val snapshot = (base.inventory as InventoryState.Fresh).snapshot
            val sessions = base.copy(
                inventory = InventoryState.Fresh(
                    snapshot.copy(inventory = snapshot.inventory.copy(sessions = listOf(second, first))),
                ),
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
    fun `terminal and kill requests bind the exact machine origin and target`() {
        val credential = MachineCredential(
            machine = macBook,
            bearer = requireNotNull(GatewayBearer.parse("A".repeat(43))),
        )
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
    }

    private fun readyMachine(machine: PairedMachine, session: AgentSession): MachineState {
        val inventory = decodeSessionsResponse(inventoryJson(machine.handle, "Linux", session))
        return MachineState(
            machine = machine,
            access = MachineAccess.Ready,
            inventory = InventoryState.Fresh(InventorySnapshot(inventory, receivedAtElapsedMillis = 1_000)),
            pressure = PressureState.Reading,
        )
    }

    private fun inventoryJson(
        handle: MachineHandle,
        platform: String,
        session: AgentSession? = null,
    ): String =
        """{"machine":{"handle":"${handle.encoded}","platform":"$platform"},"observedAt":"2026-08-26T12:00:00Z","profiles":[{"key":"personal","label":"Personal"}],"sessions":[${session?.let(::sessionJson).orEmpty()}]}"""

    private fun session(): AgentSession = AgentSession(
        id = "${'$'}1",
        name = "ga-durinn",
        identityToken = "v1-0123456789abcdef0123456789abcdef.100.200.1",
        profile = "personal",
        objective = null,
        character = CharacterSummary("norse.durinn", "Durinn"),
        cwd = "/src/skidbladnir",
        activeCommand = "codex",
        attachedClients = 1,
        attention = false,
        status = SessionStatus(
            SessionStatusKind.Working,
            SessionStatusSignal.Lifecycle,
            "2026-08-26T11:59:55Z",
        ),
    )

    private fun sessionJson(session: AgentSession): String =
        """{"id":"${session.id}","name":"${session.name}","identityToken":"${session.identityToken}","profile":"personal","character":{"key":"norse.durinn","displayName":"Durinn"},"cwd":"/src/skidbladnir","activeCommand":"codex","attachedClients":1,"attention":false,"status":{"kind":"Working","signal":"Lifecycle","signalAt":"2026-08-26T11:59:55Z"}}"""

    private fun pressureJson(unsupported: String, metrics: String): String {
        val sample =
            """{"sampledAt":"2026-08-26T12:00:00Z","level":"Normal","reasons":[],"metrics":$metrics,"missing":[]}"""
        return """{"unsupported":$unsupported,"current":$sample,"history":[$sample]}"""
    }

    private companion object {
        const val linuxMetrics =
            """{"cpuPercent":12.5,"normalizedLoad":0.4,"memoryAvailablePercent":42.0,"swapUsedPercent":0.0,"diskAvailablePercent":60.0,"cpuPsiSomeAvg60Percent":0.0,"memoryPsiFullAvg60Percent":0.0,"ioPsiFullAvg60Percent":0.0}"""
        const val darwinMetrics =
            """{"cpuPercent":12.5,"normalizedLoad":0.4,"swapUsedPercent":0.0,"diskAvailablePercent":60.0,"memoryPressure":"Normal"}"""
    }
}
