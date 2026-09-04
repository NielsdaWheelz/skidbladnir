package dev.niels.skidbladnir

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import java.time.Duration
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

private val MACHINE_POLL_CADENCE: Duration = Duration.ofSeconds(5)

internal enum class FleetResetReason { InviteIdentityMismatch, StoredFleetUnusable }

internal sealed interface SkidbladnirUiState {
    data object Booting : SkidbladnirUiState

    sealed interface Workspace : SkidbladnirUiState

    data class FleetConnect(
        val mode: FleetConnectMode,
        val phase: FleetConnectPhase,
        val resetReason: FleetResetReason? = null,
    ) : SkidbladnirUiState {
        init {
            require((phase == FleetConnectPhase.ResetRequired) == (resetReason != null))
        }
    }

    data class Dashboard(
        val machines: List<MachineState>,
        val refreshing: Boolean,
        val notice: String? = null,
        val forge: ForgeState?,
        val forgeRecovery: ForgeRecovery?,
        val kill: KillState?,
        val unreadableMachines: List<UnreadableStoredMachine> = emptyList(),
    ) : Workspace

    data class Terminal(
        val machine: MachineState,
        val target: SessionTarget,
        val attempt: Int,
        val connection: TerminalUiStatus,
        val kill: KillState?,
        val rename: RenameState? = null,
    ) : Workspace
}

internal fun dashboardRestorationReady(
    scope: DashboardScope,
    machines: Collection<MachineState>,
    livePollers: Set<MachineHandle>,
    foreground: Boolean,
): Boolean {
    if (!foreground) return false
    // justify-defect: acceptFleet validates restored scope before Dashboard publication;
    // absence here means controller and entry ownership have diverged.
    val scopedMachines = when (scope) {
        DashboardScope.All -> machines
        is DashboardScope.Machine -> listOf(
            checkNotNull(machines.singleOrNull { it.machine.handle == scope.handle }),
        )
    }
    return scopedMachines.all { machine ->
        when (machine.access) {
            MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> true
            MachineAccess.Ready -> when (machine.inventory) {
                is InventoryState.Fresh,
                is InventoryState.Superseded,
                is InventoryState.Stale,
                is InventoryState.Unreachable,
                -> true
                InventoryState.Reading -> {
                    // justify-defect: foreground Ready/Reading is maintained only while
                    // this controller owns that machine's live poller.
                    check(machine.machine.handle in livePollers) {
                        "foreground inventory reading lost its poller"
                    }
                    false
                }
            }
        }
    }
}

internal fun fleetReconnectCanCancel(state: SkidbladnirUiState.FleetConnect): Boolean =
    state.mode == FleetConnectMode.Reconnect &&
        state.phase != FleetConnectPhase.Connecting &&
        (state.phase != FleetConnectPhase.ResetRequired ||
            state.resetReason == FleetResetReason.InviteIdentityMismatch)

internal data class ForgeState(
    val form: ForgeForm,
    val pending: Boolean,
    val failure: ForgeFailure,
    val surface: ForgeSurface,
)

internal fun ForgeState.admissibleSubmission(): ForgeDraft? =
    if (!pending && surface == ForgeSurface.Form) form.submission() else null

private data class PendingFleetPersistence(
    val mode: FleetConnectMode,
    val credentials: List<MachineCredential>,
)
internal sealed interface ForgeRecovery {
    val draft: ForgeDraft
    data class RefreshRequired(override val draft: ForgeDraft) : ForgeRecovery
    data class ReviewReady(override val draft: ForgeDraft) : ForgeRecovery
}
internal data class KillState(
    val machine: PairedMachine,
    val target: SessionTarget,
    val pending: Boolean,
)
internal sealed interface TerminalUiStatus {
    data object Preparing : TerminalUiStatus
    data object Verifying : TerminalUiStatus
    data object Connecting : TerminalUiStatus
    data class Connected(val attachedClients: Int, val geometry: TerminalGeometry) : TerminalUiStatus
    data class ReconnectRequired(val message: String) : TerminalUiStatus
}

internal fun terminalActionAdmissible(machineCanMutate: Boolean, connection: TerminalUiStatus): Boolean =
    machineCanMutate && when (connection) {
        is TerminalUiStatus.Connected, is TerminalUiStatus.ReconnectRequired -> true
        TerminalUiStatus.Preparing, TerminalUiStatus.Verifying, TerminalUiStatus.Connecting -> false
    }

internal data class ForgeCarry(val forge: ForgeState?, val recovery: ForgeRecovery?)
internal fun forgeCarry(state: SkidbladnirUiState): ForgeCarry {
    val dashboard = state as? SkidbladnirUiState.Dashboard ?: return ForgeCarry(null, null)
    val forge = dashboard.forge
    return if (forge?.pending == true) {
        ForgeCarry(
            null,
            ForgeRecovery.RefreshRequired(
                checkNotNull(forge.form.submission()),
            ),
        )
    }
    else ForgeCarry(forge, dashboard.forgeRecovery)
}

internal fun resumeForgeRecovery(
    dashboard: SkidbladnirUiState.Dashboard,
    dashboardEntry: DashboardEntryState,
): SkidbladnirUiState.Dashboard {
    val recovery = dashboard.forgeRecovery as? ForgeRecovery.ReviewReady ?: return dashboard
    val target = dashboard.machines.singleOrNull {
        it.machine.handle == recovery.draft.machineHandle
    } ?: return dashboard
    if (!target.canMutate) return dashboard
    dashboardEntry.selectScope(DashboardScope.Machine(target.machine.handle))
    return dashboard.copy(
        forge = ForgeState(
            ForgeForm(recovery.draft),
            pending = false,
            failure = ForgeFailure.None,
            surface = ForgeSurface.Form,
        ),
        forgeRecovery = null,
    )
}

internal fun advanceForgeRecovery(
    recovery: ForgeRecovery?,
    machines: Collection<MachineState>,
): ForgeRecovery? {
    if (recovery !is ForgeRecovery.RefreshRequired) return recovery
    val machine = machines.singleOrNull { it.machine.handle == recovery.draft.machineHandle }
    return if (machine?.inventory is InventoryState.Fresh) {
        ForgeRecovery.ReviewReady(recovery.draft)
    } else {
        recovery
    }
}

internal fun createdTerminalAdmissionStatus(
    terminal: SkidbladnirUiState.Terminal,
    completedMutationFence: Long,
    requiredMutationFence: Long,
): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Verifying)
    require(completedMutationFence >= 0 && requiredMutationFence > 0)
    if (completedMutationFence < requiredMutationFence) return TerminalUiStatus.Verifying
    return availableTerminalStatus(terminal, TerminalUiStatus.Preparing)
}

internal fun terminalPageAdmissionStatus(terminal: SkidbladnirUiState.Terminal): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Preparing)
    return availableTerminalStatus(terminal, TerminalUiStatus.Connecting)
}

internal fun terminalReadAdmissionStatus(
    terminal: SkidbladnirUiState.Terminal,
    exactLifetimeAvailable: Boolean,
): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Verifying)
    unavailableTerminalStatus(terminal)?.let { return it }
    return if (exactLifetimeAvailable) TerminalUiStatus.Preparing else TerminalUiStatus.ReconnectRequired(
        "${terminal.machine.machine.label.text}: that session lifetime is no longer available.",
    )
}

private fun availableTerminalStatus(
    terminal: SkidbladnirUiState.Terminal,
    available: TerminalUiStatus,
): TerminalUiStatus {
    unavailableTerminalStatus(terminal)?.let { return it }
    val inventory = terminal.machine.inventory
    // justify-service-invariant-check: canMutate is a derived property, so the compiler cannot
    // narrow the Fresh inventory variant that an admitted machine already guarantees.
    check(inventory is InventoryState.Fresh)
    val response = inventory.snapshot.inventory
    val exact = terminal.machine.machine.handle == terminal.target.machineHandle &&
        response.machine.handle == terminal.target.machineHandle &&
        response.sessions.any {
            it.tmuxId == terminal.target.session.tmuxId &&
                it.tmuxName == terminal.target.session.tmuxName &&
                it.identityToken == terminal.target.session.identityToken
        }
    return if (exact) available else TerminalUiStatus.ReconnectRequired(
        "${terminal.machine.machine.label.text}: that session lifetime is no longer available.",
    )
}

private fun unavailableTerminalStatus(
    terminal: SkidbladnirUiState.Terminal,
): TerminalUiStatus.ReconnectRequired? {
    if (terminal.kill?.pending != true && terminal.machine.canMutate) return null
    return TerminalUiStatus.ReconnectRequired(machineAccessMessage(terminal.machine))
}

internal fun dashboardAfterTerminalAccessLoss(
    terminal: SkidbladnirUiState.Terminal,
    machines: List<MachineState>,
    refreshing: Boolean,
    dashboardEntry: DashboardEntryState,
): SkidbladnirUiState.Dashboard {
    val machine = machines.single { it.machine.handle == terminal.target.machineHandle }
    require(machine.access != MachineAccess.Ready)
    dashboardEntry.selectTerminalAccessLoss(machine.machine.handle)
    return SkidbladnirUiState.Dashboard(
        machines = machines,
        refreshing = refreshing,
        notice = machineAccessMessage(machine),
        forge = null,
        forgeRecovery = null,
        kill = null,
    )
}

internal fun dashboardAfterMachineAccessLoss(
    dashboard: SkidbladnirUiState.Dashboard,
    machines: List<MachineState>,
    handle: MachineHandle,
    refreshing: Boolean,
    dashboardEntry: DashboardEntryState,
    failure: GatewayFailure.Api,
): SkidbladnirUiState.Dashboard {
    val machine = machines.single { it.machine.handle == handle }
    require(machine.access != MachineAccess.Ready)
    require(
        (machine.access == MachineAccess.AuthRequired && failure.code == ApiErrorCode.Unauthenticated) ||
            (machine.access == MachineAccess.IdentityChanged &&
                failure.code == ApiErrorCode.MachineIdentityMismatch),
    )
    val message = machineAccessMessage(machine)
    val affectedForge = dashboard.forge?.takeIf { it.form.machineHandle == handle }
    val affectedKill = dashboard.kill?.takeIf { it.target.machineHandle == handle }
    val recoveryOwnsScope =
        affectedForge?.pending == true ||
            affectedForge?.surface is ForgeSurface.DirectoryPicker ||
            affectedKill != null
    if (recoveryOwnsScope) {
        dashboardEntry.selectScope(DashboardScope.Machine(handle))
    }
    return dashboard.copy(
        machines = machines,
        refreshing = refreshing,
        notice = message,
        forge = if (affectedForge != null) {
            affectedForge.copy(
                pending = false,
                failure = ForgeFailure.Definite(failure),
                surface = ForgeSurface.Form,
            )
        } else {
            dashboard.forge
        },
        kill = dashboard.kill?.takeUnless { it.target.machineHandle == handle },
    )
}

private fun machineAccessMessage(machine: MachineState): String = when (machine.access) {
    MachineAccess.Ready -> "${machine.machine.label.text}: reconnect required."
    MachineAccess.AuthRequired -> "${machine.machine.label.text}: authentication required."
    MachineAccess.IdentityChanged ->
        "${machine.machine.label.text}: machine identity changed. Fleet reset is required."
}

private data class PollRuntime(
    val inventory: CoalescingPollLane = CoalescingPollLane(),
    val pressure: CoalescingPollLane = CoalescingPollLane(),
    val inventoryOperation: InventoryOperationLane,
    var inventoryFuture: ScheduledFuture<*>? = null,
    var pressureFuture: ScheduledFuture<*>? = null,
)

private data class CreatedTerminalAdmission(
    val attempt: Int,
    val requiredMutationFence: Long,
)

internal data class PollRun(val sequence: Long, val startsNow: Boolean)

internal class CoalescingPollLane {
    private var activeSequence: Long? = null
    private var trailingSequence: Long? = null
    private var nextSequence = 1L

    @Synchronized
    fun request(requireTrailing: Boolean = false): PollRun? {
        if (activeSequence == null) {
            val sequence = nextSequence++
            activeSequence = sequence
            return PollRun(sequence, startsNow = true)
        }
        if (!requireTrailing) return null
        val sequence = trailingSequence ?: nextSequence++.also { trailingSequence = it }
        return PollRun(sequence, startsNow = false)
    }

    @Synchronized
    fun finish(completedSequence: Long): PollRun? {
        check(activeSequence == completedSequence)
        val trailing = trailingSequence
        if (trailing != null) {
            activeSequence = trailing
            trailingSequence = null
            return PollRun(trailing, startsNow = true)
        }
        activeSequence = null
        return null
    }

    @Synchronized
    fun abort() {
        activeSequence = null
        trailingSequence = null
    }
}

internal class AwaitedInventoryReads {
    private val requiredSequenceByMachine = mutableMapOf<MachineHandle, Long>()

    val isActive: Boolean get() = requiredSequenceByMachine.isNotEmpty()

    fun requireRead(handle: MachineHandle, sequence: Long) {
        val current = requiredSequenceByMachine[handle]
        check(current == null || sequence >= current)
        requiredSequenceByMachine[handle] = sequence
    }

    fun readLanded(handle: MachineHandle, completedSequence: Long) {
        val required = requiredSequenceByMachine[handle] ?: return
        if (completedSequence >= required) requiredSequenceByMachine.remove(handle)
    }

    fun stop(handle: MachineHandle) { requiredSequenceByMachine.remove(handle) }

    fun clear() { requiredSequenceByMachine.clear() }
}

internal class InventoryOperationLane(
    private val executor: Executor,
    private val onDefect: (RuntimeException) -> Unit,
) {
    private val queued = ArrayDeque<() -> Unit>()
    private var draining = false
    private var submittedMutationFence = 0L
    private var completedMutationFence = 0L

    @Synchronized
    fun submitRead(action: (Long) -> Unit) {
        enqueue { action(completedMutationFence) }
    }

    @Synchronized
    fun submitMutation(onReserved: (Long) -> Unit, action: (Long) -> Unit) {
        val fence = ++submittedMutationFence
        onReserved(fence)
        enqueue {
            try {
                action(fence)
            } finally {
                synchronized(this) { completedMutationFence = fence }
            }
        }
    }

    private fun enqueue(action: () -> Unit) {
        queued.addLast(action)
        if (draining) return
        draining = true
        executor.execute(::drain)
    }

    private fun drain() {
        while (true) {
            val action = synchronized(this) {
                if (queued.isEmpty()) {
                    draining = false
                    return
                }
                queued.removeFirst()
            }
            try {
                action()
            } catch (defect: RuntimeException) {
                onDefect(defect)
            }
        }
    }
}

internal fun submitCoalescedInventoryRead(
    operations: InventoryOperationLane,
    polls: CoalescingPollLane,
    initialRun: PollRun,
    read: (PollRun, Long) -> Unit,
) {
    operations.submitRead { completedMutationFence ->
        try {
            read(initialRun, completedMutationFence)
            val trailing = polls.finish(initialRun.sequence)
            if (trailing != null) {
                submitCoalescedInventoryRead(operations, polls, trailing, read)
            }
        } catch (defect: RuntimeException) {
            polls.abort()
            throw defect
        }
    }
}

internal class MachineInventoryOperations(
    private val executor: Executor,
    private val onDefect: (RuntimeException) -> Unit,
) {
    private val lanes = ConcurrentHashMap<MachineHandle, InventoryOperationLane>()

    fun forMachine(handle: MachineHandle): InventoryOperationLane =
        lanes.computeIfAbsent(handle) { InventoryOperationLane(executor, onDefect) }
}

internal data class ReconciledStoredMachine(
    val credential: MachineCredential,
    val machine: MachineState,
)

internal data class ReconciledStoredControllerState(
    val machines: List<ReconciledStoredMachine>,
    val forgeCarry: ForgeCarry,
)

internal class StoredMachineRead internal constructor(
    val readingState: SkidbladnirUiState,
    private val currentCredentials: List<MachineCredential>,
    private val currentMachines: List<MachineState>,
    val retainedForgeCarry: ForgeCarry?,
) {
    fun reconcileIfCurrent(
        isCurrent: Boolean,
        storedCredentials: List<MachineCredential>,
    ): ReconciledStoredControllerState? {
        if (!isCurrent) return null
        return ReconciledStoredControllerState(
            machines = reconcileStoredMachines(
                currentCredentials = currentCredentials,
                currentMachines = currentMachines,
                storedCredentials = storedCredentials,
            ),
            forgeCarry = retainedForgeCarry?.takeIf {
                forgeAuthoritySurvives(it, currentCredentials, storedCredentials)
            } ?: ForgeCarry(null, null),
        )
    }
}

internal fun beginStoredMachineRead(
    state: SkidbladnirUiState,
    currentCredentials: Collection<MachineCredential>,
    currentMachines: Collection<MachineState>,
    retainedForgeCarry: ForgeCarry? = null,
): StoredMachineRead = StoredMachineRead(
    readingState = SkidbladnirUiState.Booting,
    currentCredentials = currentCredentials.toList(),
    currentMachines = currentMachines.toList(),
    retainedForgeCarry = when (state) {
        is SkidbladnirUiState.Dashboard -> forgeCarry(state).takeUnless { it == ForgeCarry(null, null) }
        SkidbladnirUiState.Booting -> retainedForgeCarry
        is SkidbladnirUiState.FleetConnect, is SkidbladnirUiState.Terminal -> null
    },
)

private fun forgeAuthoritySurvives(
    carry: ForgeCarry,
    currentCredentials: Collection<MachineCredential>,
    storedCredentials: Collection<MachineCredential>,
): Boolean {
    val handles = listOfNotNull(
        carry.forge?.form?.machineHandle,
        carry.recovery?.draft?.machineHandle,
    ).distinct()
    if (handles.isEmpty()) return false
    val currentByHandle = currentCredentials.associateBy { it.machine.handle }
    val storedByHandle = storedCredentials.associateBy { it.machine.handle }
    return handles.all { handle ->
        val current = currentByHandle[handle]
        val stored = storedByHandle[handle]
        current != null && stored != null &&
            current.machine.origin == stored.machine.origin &&
            current.bearer == stored.bearer
    }
}

internal fun reconcileStoredMachines(
    currentCredentials: Collection<MachineCredential>,
    currentMachines: Collection<MachineState>,
    storedCredentials: List<MachineCredential>,
): List<ReconciledStoredMachine> {
    require(storedCredentials.distinctBy { it.machine.handle }.size == storedCredentials.size)
    val currentCredentialsByHandle = currentCredentials.associateBy { it.machine.handle }
    val currentMachinesByHandle = currentMachines.associateBy { it.machine.handle }
    return storedCredentials.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) }.map { stored ->
        val currentCredential = currentCredentialsByHandle[stored.machine.handle]
        val currentMachine = currentMachinesByHandle[stored.machine.handle]
        val unchangedAuthority = currentCredential != null &&
            currentCredential.bearer == stored.bearer &&
            currentCredential.machine.origin == stored.machine.origin &&
            currentMachine != null
        ReconciledStoredMachine(
            credential = stored,
            machine = if (unchangedAuthority) {
                checkNotNull(currentMachine).copy(machine = stored.machine)
            } else {
                MachineState(
                    machine = stored.machine,
                    access = MachineAccess.Ready,
                    inventory = InventoryState.Reading,
                    pressure = PressureState.Reading,
                )
            },
        )
    }
}

internal class SkidbladnirController(
    context: Context,
    private val dashboardEntry: DashboardEntryState,
    storage: MachineStorage = MachineStorage.production,
    private val client: GatewayClient = GatewayClient(),
) {
    var state: SkidbladnirUiState by mutableStateOf(SkidbladnirUiState.Booting)
        private set

    private val main = Handler(Looper.getMainLooper())
    private val scheduler = Executors.newSingleThreadScheduledExecutor { task ->
        Thread(task, "skidbladnir-poll-ticks").apply { isDaemon = true }
    }
    private val network = Executors.newCachedThreadPool { task ->
        Thread(task, "skidbladnir-machine-client").apply { isDaemon = true }
    }
    private val credentialOperations = Executors.newSingleThreadExecutor { task ->
        Thread(task, "skidbladnir-machine-store").apply { isDaemon = true }
    }
    private val store = MachineStore(context.applicationContext, storage)
    private val credentials = ConcurrentHashMap<MachineHandle, MachineCredential>()
    private val machineStates = linkedMapOf<MachineHandle, MachineState>()
    private val unreadableMachines = mutableListOf<UnreadableStoredMachine>()
    private val polling = ConcurrentHashMap<MachineHandle, PollRuntime>()
    private val inventoryOperations = MachineInventoryOperations(network, ::surfaceDefect)
    private val pendingRenameFences = mutableMapOf<MachineHandle, Long>()

    /** Main-thread owner of the app-wide inventory-read indicator. */
    private val awaitedInventoryReads = AwaitedInventoryReads()
    private var pendingDashboardNotice: String? = null
    private var retainedStoredMachineForgeCarry: ForgeCarry? = null
    @Volatile private var foreground = false
    @Volatile private var generation = 0L
    private var terminalConnection: TerminalConnection? = null
    private var terminalPage: TerminalPage? = null
    private var terminalOwner: Any? = null
    private var createdTerminalAdmission: CreatedTerminalAdmission? = null
    private var nextTerminalAttempt = 1
    private var nextWorkingDirectoryPickerInstance = 1L
    private var pendingFleetScan: String? = null
    @Volatile private var pendingFleetPersistence: PendingFleetPersistence? = null

    fun start() {
        if (foreground) return
        foreground = true
        val connectState = state as? SkidbladnirUiState.FleetConnect
        val interruptedPersistence = pendingFleetPersistence?.takeIf {
            connectState?.phase == FleetConnectPhase.Connecting && connectState.mode == it.mode
        }
        ++generation
        if (connectState?.phase == FleetConnectPhase.Scanning) {
            machineStates.values.filter { it.access == MachineAccess.Ready }.forEach {
                startPolling(it.machine.handle, generation)
            }
            pendingFleetScan?.let { encoded ->
                pendingFleetScan = null
                acceptFleetScan(encoded)
            }
            return
        }
        val activeGeneration = generation
        awaitedInventoryReads.clear()
        val storedMachineRead = beginStoredMachineRead(
            state = state,
            currentCredentials = credentials.values,
            currentMachines = machineStates.values,
            retainedForgeCarry = retainedStoredMachineForgeCarry,
        )
        retainedStoredMachineForgeCarry = storedMachineRead.retainedForgeCarry
        state = storedMachineRead.readingState
        executeCredentialOperation {
            val stored = store.read()
            main.post {
                val reconciliation = storedMachineRead.reconcileIfCurrent(
                    isCurrent = isActiveGeneration(activeGeneration),
                    storedCredentials = stored.credentials,
                ) ?: return@post
                retainedStoredMachineForgeCarry = null
                val reconciled = reconciliation.machines
                credentials.clear()
                machineStates.clear()
                reconciled.forEach { entry ->
                    credentials[entry.credential.machine.handle] = entry.credential
                    machineStates[entry.machine.machine.handle] = entry.machine
                }
                unreadableMachines.clear()
                unreadableMachines += stored.unreadable
                dashboardEntry.acceptFleet(machineStates.keys.toSet())
                val interruptedDisposition = interruptedPersistence?.let {
                    resumedFleetPersistenceDisposition(it.mode, it.credentials, stored)
                }
                if (pendingFleetPersistence == interruptedPersistence) pendingFleetPersistence = null
                val storageNotice = stored.unreadable.takeIf { it.isNotEmpty() }?.let {
                    "Saved fleet credentials are unreadable. Fleet reset is required outside this app."
                }
                if (machineStates.isEmpty() && unreadableMachines.isNotEmpty()) {
                    state = SkidbladnirUiState.FleetConnect(
                        mode = interruptedPersistence?.mode ?: connectState?.mode ?: FleetConnectMode.Install,
                        phase = FleetConnectPhase.ResetRequired,
                        resetReason = FleetResetReason.StoredFleetUnusable,
                    )
                    return@post
                }
                if (machineStates.isEmpty() && unreadableMachines.isEmpty()) {
                    state = SkidbladnirUiState.FleetConnect(
                        mode = FleetConnectMode.Install,
                        phase = if (
                            connectState?.phase == FleetConnectPhase.Connecting ||
                            connectState?.phase == FleetConnectPhase.Failed
                        ) {
                            FleetConnectPhase.Failed
                        } else {
                            FleetConnectPhase.Ready
                        },
                    )
                    return@post
                }
                reconciled.filter { it.machine.access == MachineAccess.Ready }.forEach {
                    startPolling(it.credential.machine.handle, activeGeneration)
                }
                if (interruptedDisposition == FleetPersistenceDisposition.Connected) {
                    publishDashboard(notice = storageNotice, carry = reconciliation.forgeCarry)
                    return@post
                }
                if (interruptedDisposition == FleetPersistenceDisposition.ResetRequired) {
                    state = SkidbladnirUiState.FleetConnect(
                        mode = checkNotNull(interruptedPersistence).mode,
                        phase = FleetConnectPhase.ResetRequired,
                        resetReason = FleetResetReason.StoredFleetUnusable,
                    )
                    return@post
                }
                if (connectState?.mode == FleetConnectMode.Reconnect) {
                    state = connectState.copy(
                        phase = if (connectState.phase == FleetConnectPhase.Connecting) {
                            FleetConnectPhase.Failed
                        } else {
                            connectState.phase
                        },
                    )
                    return@post
                }
                publishDashboard(notice = storageNotice, carry = reconciliation.forgeCarry)
            }
        }
    }

    fun stopForBackground() {
        if (!foreground) return
        foreground = false
        generation += 1
        polling.values.forEach { runtime ->
            runtime.inventoryFuture?.cancel(false)
            runtime.pressureFuture?.cancel(false)
        }
        polling.clear()
        awaitedInventoryReads.clear()
        leaveTerminal()
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> state = current.copy(
                refreshing = awaitedInventoryReads.isActive,
                forge = current.forge?.let(::workingDirectoryPickerAfterForegroundInvalidation),
            )
            is SkidbladnirUiState.Terminal -> publishDashboard()
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> Unit
        }
    }

    fun close() {
        stopForBackground()
        pendingFleetScan = null
        pendingFleetPersistence = null
        scheduler.shutdownNow()
        credentialOperations.shutdownNow()
        network.shutdownNow()
        client.closeAsync()
    }

    fun requestFleetScan() {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.phase != FleetConnectPhase.Ready && current.phase != FleetConnectPhase.Failed) return
        state = current.copy(phase = FleetConnectPhase.Scanning)
    }

    fun requestFleetReconnect() {
        if (state !is SkidbladnirUiState.Dashboard) return
        if (unreadableMachines.isNotEmpty() || credentials.size != 3 || machineStates.size != 3) {
            publishDashboard(notice = "The installed fleet is incomplete. Fleet reset is required outside this app.")
            return
        }
        state = SkidbladnirUiState.FleetConnect(FleetConnectMode.Reconnect, FleetConnectPhase.Scanning)
    }

    fun cancelFleetReconnect() {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (!fleetReconnectCanCancel(current)) return
        publishDashboard()
    }

    fun cancelFleetScan() {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.phase == FleetConnectPhase.Scanning) state = current.copy(phase = FleetConnectPhase.Ready)
    }

    fun failFleetScan() {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.phase == FleetConnectPhase.Scanning) state = current.copy(phase = FleetConnectPhase.Failed)
    }

    fun acceptFleetScan(encoded: String) {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.phase != FleetConnectPhase.Scanning) return
        if (!foreground) {
            pendingFleetScan = encoded
            return
        }
        val invite = parseFleetInvite(encoded)
        if (invite == null) {
            state = current.copy(phase = FleetConnectPhase.Failed)
            return
        }
        if (current.mode == FleetConnectMode.Reconnect &&
            !reconnectInviteMatchesInstalled(invite, credentials.values)
        ) {
            state = current.copy(
                phase = FleetConnectPhase.ResetRequired,
                resetReason = FleetResetReason.InviteIdentityMismatch,
            )
            return
        }
        val activeGeneration = generation
        val mode = current.mode
        state = current.copy(phase = FleetConnectPhase.Connecting)
        redeemFleetInvite(invite, network, client::redeemPairing).whenComplete { connected, completionFailure ->
            if (completionFailure != null) {
                val cause = completionFailure.cause ?: completionFailure
                surfaceDefect(
                    cause as? RuntimeException
                        ?: IllegalStateException("fleet redemption failed exceptionally"),
                )
                return@whenComplete
            }
            if (connected == null) {
                main.post { failFleetConnectionIfCurrent(activeGeneration, mode) }
                return@whenComplete
            }
            pendingFleetPersistence = PendingFleetPersistence(mode, connected)
            executeCredentialOperation {
                if (!isActiveGeneration(activeGeneration)) return@executeCredentialOperation
                val disposition = persistConnectedFleet(mode, connected)
                main.post {
                    if (!isActiveGeneration(activeGeneration)) return@post
                    val active = state as? SkidbladnirUiState.FleetConnect ?: return@post
                    if (active.mode != mode || active.phase != FleetConnectPhase.Connecting) return@post
                    pendingFleetPersistence = null
                    when (disposition) {
                        FleetPersistenceDisposition.Connected -> acceptConnectedFleet(connected, activeGeneration)
                        FleetPersistenceDisposition.RetryWithFreshInvite ->
                            state = active.copy(phase = FleetConnectPhase.Failed)
                        FleetPersistenceDisposition.ResetRequired ->
                            state = active.copy(
                                phase = FleetConnectPhase.ResetRequired,
                                resetReason = FleetResetReason.StoredFleetUnusable,
                            )
                    }
                }
            }
        }
    }

    private fun persistConnectedFleet(
        mode: FleetConnectMode,
        connected: List<MachineCredential>,
    ): FleetPersistenceDisposition = when (mode) {
        FleetConnectMode.Install -> {
            val result = store.installFixedFleet(connected)
            val durable = if (result == FleetInstallation.StorageUnavailable) {
                store.read()
            } else {
                MachineStoreRead(emptyList(), emptyList())
            }
            fleetInstallationDisposition(result, durable, connected)
        }
        FleetConnectMode.Reconnect -> {
            val result = store.reconnectFixedFleet(connected)
            val durable = if (result == FleetReconnection.StorageUnavailable) {
                store.read()
            } else {
                MachineStoreRead(emptyList(), emptyList())
            }
            fleetReconnectionDisposition(result, durable, connected)
        }
    }

    private fun failFleetConnectionIfCurrent(activeGeneration: Long, mode: FleetConnectMode) {
        if (!isActiveGeneration(activeGeneration)) return
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.mode == mode && current.phase == FleetConnectPhase.Connecting) {
            state = current.copy(phase = FleetConnectPhase.Failed)
        }
    }

    private fun acceptConnectedFleet(connected: List<MachineCredential>, activeGeneration: Long) {
        polling.keys.toList().forEach(::stopPolling)
        credentials.clear()
        machineStates.clear()
        unreadableMachines.clear()
        connected.forEach { credential ->
            credentials[credential.machine.handle] = credential
            machineStates[credential.machine.handle] = MachineState(
                credential.machine,
                MachineAccess.Ready,
                InventoryState.Reading,
                PressureState.Reading,
            )
            startPolling(credential.machine.handle, activeGeneration)
        }
        dashboardEntry.acceptFleet(machineStates.keys.toSet())
        publishDashboard()
    }

    fun verifyVisibleInventory() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val activeGeneration = generation
        // Only machines that still own live polling work can be refreshed; a machine whose access
        // failed says so instead of silently dropping the request.
        val targets = visibleInventoryTargets(polling.keys, dashboardEntry.scope)
        var requested = false
        targets.forEach { handle -> if (awaitInventory(handle, activeGeneration)) requested = true }
        state = current.copy(
            refreshing = awaitedInventoryReads.isActive,
            notice = if (requested) {
                null
            } else {
                unrefreshableNotice(dashboardEntry.scope)
            },
        )
    }

    fun restoreDashboardOnce(keys: List<DashboardCardKey>) {
        if (!dashboardEntry.restorationPending) return
        if (dashboardRestorationReady(
            scope = dashboardEntry.scope,
            machines = machineStates.values,
            livePollers = polling.keys,
            foreground = foreground,
        )) dashboardEntry.restoreOnce(keys)
    }

    private fun unrefreshableNotice(scope: DashboardScope): String {
        val visible = machineStates.values.filter { machine ->
            when (scope) {
                DashboardScope.All -> true
                is DashboardScope.Machine -> machine.machine.handle == scope.handle
            }
        }
        if (visible.isEmpty()) {
            return "No fleet is connected."
        }
        return visible.joinToString(" ", transform = ::machineAccessMessage)
    }

    fun openForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val scope = dashboardEntry.scope
        val handle = when (scope) {
            DashboardScope.All -> null
            is DashboardScope.Machine -> scope.handle
        }
        val admissible = when (scope) {
            DashboardScope.All -> machineStates.values.any { it.canForge }
            is DashboardScope.Machine -> machineStates[scope.handle]?.canForge == true
        }
        if (!admissible) return
        state = current.copy(
            forge = ForgeState(
                ForgeForm(handle, "", null, "", ""),
                pending = false,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
        )
    }

    fun openWorkingDirectoryPicker() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val handle = forge.form.machineHandle ?: return
        val machine = machineStates[handle] ?: return
        val opened = openWorkingDirectoryPicker(
            forge,
            machine,
            nextWorkingDirectoryPickerInstance,
        ) ?: return
        nextWorkingDirectoryPickerInstance += 1
        state = current.copy(forge = opened)
    }

    fun openExactWorkingDirectoryPicker() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val handle = forge.form.machineHandle ?: return
        val machine = machineStates[handle] ?: return
        val opened = openExactWorkingDirectoryPicker(
            forge,
            machine,
            nextWorkingDirectoryPickerInstance,
        ) ?: return
        nextWorkingDirectoryPickerInstance += 1
        state = current.copy(forge = opened)
    }

    fun browseWorkingDirectoryHome() {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(
            browseWorkingDirectoryHome(picker, generation) ?: return,
        )
    }

    fun openWorkingDirectoryChild(directory: HomeDirectory) {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(
            openWorkingDirectoryChild(picker, directory, generation) ?: return,
        )
    }

    fun openWorkingDirectoryParent() {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(
            openWorkingDirectoryParent(picker, generation) ?: return,
        )
    }

    fun retryWorkingDirectory() {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(
            retryWorkingDirectory(picker, generation) ?: return,
        )
    }

    fun updateWorkingDirectoryFilter(filter: String) {
        updateWorkingDirectoryPicker { picker -> updateWorkingDirectoryFilter(picker, filter) }
    }

    fun setWorkingDirectoryHidden(showHidden: Boolean) {
        updateWorkingDirectoryPicker { picker -> setWorkingDirectoryHidden(picker, showHidden) }
    }

    fun updateWorkingDirectoryViewport(viewport: DirectoryViewport) {
        updateWorkingDirectoryPicker { picker -> updateWorkingDirectoryViewport(picker, viewport) }
    }

    fun showExactWorkingDirectory() {
        updateForge(::showExactWorkingDirectory)
    }

    fun updateExactWorkingDirectory(draft: String) {
        updateForge { forge -> updateExactWorkingDirectory(forge, draft) }
    }

    fun chooseActiveWorkingDirectory(directory: WorkingDirectoryPath) {
        updateForge { forge -> chooseActiveWorkingDirectory(forge, directory) }
    }

    fun useCurrentWorkingDirectory() {
        updateForge(::useCurrentWorkingDirectory)
    }

    fun useExactWorkingDirectory() {
        updateForge(::useExactWorkingDirectory)
    }

    fun workingDirectoryBack() {
        updateForge(::workingDirectoryBack)
    }

    fun cancelWorkingDirectoryPicker() {
        updateForge(::cancelWorkingDirectoryPicker)
    }

    private fun updateForge(transform: (ForgeState) -> ForgeState) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val updated = transform(forge)
        if (updated != forge) state = current.copy(forge = updated)
    }

    private fun updateWorkingDirectoryPicker(
        transform: (WorkingDirectoryPickerState) -> WorkingDirectoryPickerState,
    ) {
        updateForge { forge ->
            val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return@updateForge forge
            val picker = transform(surface.picker)
            if (picker == surface.picker) forge else forge.copy(
                surface = surface.copy(picker = picker),
            )
        }
    }

    private fun activeWorkingDirectoryPicker(): WorkingDirectoryPickerState? {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return null
        return (dashboard.forge?.surface as? ForgeSurface.DirectoryPicker)?.picker
    }

    private fun startWorkingDirectoryRequest(start: WorkingDirectoryRequestStart) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val active = forge.surface as? ForgeSurface.DirectoryPicker ?: return
        if (active.picker.instance != start.request.pickerInstance) return
        val credential = credentials[start.request.machine.handle] ?: return
        if (credential.machine.handle != start.request.machine.handle) return
        state = current.copy(
            forge = forge.copy(surface = active.copy(picker = start.picker)),
        )
        executeNetwork {
            val result = client.listDirectory(
                credential,
                start.request.directory,
                start.request.machine,
            )
            main.post {
                if (credentials[start.request.machine.handle] != credential) return@post
                val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                val currentForge = dashboard.forge ?: return@post
                val surface = currentForge.surface as? ForgeSurface.DirectoryPicker ?: return@post
                when (val completion = completeWorkingDirectoryRequest(
                    picker = surface.picker,
                    request = start.request,
                    foregroundGeneration = generation.takeIf {
                        isActiveGeneration(start.request.generation)
                    },
                    result = result,
                )) {
                    WorkingDirectoryCompletion.Ignored -> Unit
                    is WorkingDirectoryCompletion.Updated -> state = dashboard.copy(
                        forge = currentForge.copy(
                            surface = surface.copy(picker = completion.picker),
                        ),
                    )
                    is WorkingDirectoryCompletion.AccessLost ->
                        acceptAccessFailure(start.request.machine.handle, completion.failure)
                }
            }
        }
    }

    fun resumeForgeRecovery() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        state = dev.niels.skidbladnir.resumeForgeRecovery(current, dashboardEntry)
    }

    fun dismissForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.forge?.pending != true) state = current.copy(forge = null)
    }

    fun discardForgeRecovery() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.forgeRecovery !is ForgeRecovery.RefreshRequired) state = current.copy(forgeRecovery = null)
    }

    fun updateForgeDraft(transform: (ForgeForm) -> ForgeForm) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        if (forge.pending) return
        val proposed = transform(forge.form)
        if (proposed.machineHandle != null && machineStates[proposed.machineHandle] == null) return
        state = current.copy(forge = updateForgeState(forge, proposed))
    }

    fun forge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val draft = forge.admissibleSubmission() ?: return
        val credential = credentials[draft.machineHandle] ?: return
        val machine = machineStates[draft.machineHandle] ?: return
        val runtime = polling[draft.machineHandle] ?: return
        if (!machine.canMutate) return
        state = current.copy(
            forge = forge.copy(
                pending = true,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
        )
        val activeGeneration = generation
        runtime.inventoryOperation.submitMutation(
            onReserved = { fence -> requireInventoryRefresh(credential.machine.handle, fence) },
        ) { mutationFence ->
            when (val result = client.createSession(credential, draft)) {
                is GatewayResult.Success -> main.post {
                    if (!isCredentialActive(activeGeneration, credential)) return@post
                    if (machineStates[credential.machine.handle]?.access != MachineAccess.Ready ||
                        polling[credential.machine.handle] !== runtime) return@post
                    enterCreatedTerminal(
                        SessionTarget(credential.machine.handle, result.value),
                        mutationFence,
                    )
                    awaitInventory(credential.machine.handle, activeGeneration)
                }
                is GatewayResult.Failure -> main.post {
                    if (!isCredentialActive(activeGeneration, credential)) return@post
                    if (acceptAccessFailure(credential.machine.handle, result.failure)) return@post
                    val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                    val activeForge = dashboard.forge ?: return@post
                    awaitInventory(credential.machine.handle, activeGeneration)
                    if (createFailureIsDefinitive(result.failure)) {
                        val definiteFailure = when (val failure = result.failure) {
                            GatewayFailure.Transport ->
                                error("transport failure cannot be a definite Create rejection")
                            is GatewayFailure.Api -> ForgeFailure.Definite(failure)
                        }
                        clearInventoryRefresh(credential.machine.handle)
                        state = dashboard.copy(
                            machines = sortedMachineStates(),
                            refreshing = awaitedInventoryReads.isActive,
                            forge = activeForge.copy(
                                pending = false,
                                failure = definiteFailure,
                                surface = ForgeSurface.Form,
                            ),
                        )
                    } else {
                        markInventoryFailed(credential.machine.handle, result.failure)
                        state = dashboard.copy(
                            machines = sortedMachineStates(),
                            refreshing = awaitedInventoryReads.isActive,
                            forge = null,
                            forgeRecovery = ForgeRecovery.RefreshRequired(
                                checkNotNull(activeForge.form.submission()),
                            ),
                        )
                    }
                }
            }
        }
    }

    fun openTerminal(target: SessionTarget) {
        val machine = machineStates[target.machineHandle] ?: return
        if (!machine.canMutate) return
        enterTerminal(machine, target)
    }

    private fun enterTerminal(machine: MachineState, target: SessionTarget) {
        leaveTerminal()
        state = SkidbladnirUiState.Terminal(
            machine = machine,
            target = target,
            attempt = nextTerminalAttempt++,
            connection = TerminalUiStatus.Preparing,
            kill = null,
        )
    }

    private fun enterCreatedTerminal(target: SessionTarget, requiredMutationFence: Long) {
        val machine = machineStates[target.machineHandle] ?: return
        leaveTerminal()
        val attempt = nextTerminalAttempt++
        state = SkidbladnirUiState.Terminal(
            machine = machine,
            target = target,
            attempt = attempt,
            connection = TerminalUiStatus.Verifying,
            kill = null,
        )
        createdTerminalAdmission = CreatedTerminalAdmission(attempt, requiredMutationFence)
    }

    fun terminalPageReady(attempt: Int, page: TerminalPage) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection != TerminalUiStatus.Preparing) return
        when (val admission = terminalPageAdmissionStatus(current)) {
            TerminalUiStatus.Connecting -> Unit
            is TerminalUiStatus.ReconnectRequired -> {
                leaveTerminal()
                state = current.copy(connection = admission)
                return
            }
            TerminalUiStatus.Preparing, TerminalUiStatus.Verifying, is TerminalUiStatus.Connected ->
                // justify-defect: terminalPageAdmissionStatus answers a Preparing terminal with
                // exactly Connecting or ReconnectRequired, so any other value is a same-system
                // contract violation in owned code.
                error("terminal page admission produced an impossible state")
        }
        val credential = credentials[current.target.machineHandle]
        if (credential == null) {
            leaveTerminal()
            publishDashboard(
                notice = "${current.machine.machine.label.text}: the machine pairing is no longer available.",
            )
            return
        }
        terminalPage = page
        state = current.copy(connection = TerminalUiStatus.Connecting)
        val owner = Any()
        terminalOwner = owner
        val connection = TerminalConnection(client, credential, current.target, page, object : TerminalConnectionObserver {
            override fun onPresence(attachedClients: Int, geometry: TerminalGeometry) {
                main.post {
                    val terminal = state as? SkidbladnirUiState.Terminal ?: return@post
                    if (terminal.attempt == attempt && terminalOwner === owner) {
                        state = terminal.copy(connection = TerminalUiStatus.Connected(attachedClients, geometry))
                    }
                }
            }
            override fun onFailure(code: ApiErrorCode) {
                main.post {
                    val terminal = state as? SkidbladnirUiState.Terminal ?: return@post
                    if (terminal.attempt != attempt || terminalOwner !== owner) return@post
                    if (terminalAccessLoss(code) != null) {
                        acceptAccessFailure(credential.machine.handle, GatewayFailure.Api(code))
                        return@post
                    }
                    leaveTerminal()
                    state = terminal.copy(connection = TerminalUiStatus.ReconnectRequired(apiErrorMessage(code)))
                }
            }
        })
        terminalConnection = connection
        connection.start()
    }

    fun resizeTerminal(attempt: Int, columns: Int, rows: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt == attempt) terminalConnection?.resize(columns, rows)
    }
    fun terminalPageFailed(attempt: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt) return
        terminalConnection?.terminalUnavailable()
        leaveTerminal()
        state = current.copy(connection = TerminalUiStatus.ReconnectRequired("Reconnect required."))
    }
    fun sendTerminal(attempt: Int, bytes: ByteArray) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection !is TerminalUiStatus.Connected) return
        terminalConnection?.send(bytes)
        terminalPage?.focus()
    }
    fun sendTerminalAccessory(attempt: Int, accessory: TerminalAccessory) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt == attempt && current.connection is TerminalUiStatus.Connected) terminalPage?.sendAccessory(accessory)
    }

    fun openRename() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.rename != null || current.kill != null ||
            !terminalActionAdmissible(current.machine.canMutate, current.connection)
        ) return
        terminalPage?.resetInputState()
        state = current.copy(rename = beginRename(current.target))
    }

    fun updateRenameDraft(draft: String) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        val updated = updateRenameDraft(rename, draft)
        if (updated != rename) state = current.copy(rename = updated)
    }

    fun dismissRename() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        val updated = dismissRename(rename)
        if (updated != rename) state = current.copy(rename = updated)
    }

    fun submitRename() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        val sending = beginRenameSending(
            state = rename,
            terminalTarget = current.target,
            terminalActionsAdmissible = terminalActionAdmissible(current.machine.canMutate, current.connection),
        ) ?: return
        val credential = credentials[current.target.machineHandle] ?: return
        val runtime = polling[current.target.machineHandle] ?: return
        val attempt = current.attempt
        val activeGeneration = generation
        state = current.copy(rename = sending)
        runtime.inventoryOperation.submitMutation(
            onReserved = { fence -> requireRenameInventoryRefresh(current.target.machineHandle, fence) },
        ) { mutationFence ->
            val result = client.renameSession(credential, sending.target, sending.draft)
            main.post {
                if (!isCredentialActive(activeGeneration, credential)) return@post
                if (result is GatewayResult.Failure &&
                    acceptAccessFailure(sending.target.machineHandle, result.failure)
                ) return@post
                val transition = completeRenameHttp(sending, result)
                if (transition.clearMutationFence) {
                    clearRenameInventoryRefresh(sending.target.machineHandle, mutationFence)
                }
                val terminal = state as? SkidbladnirUiState.Terminal
                val activeRename = terminal?.rename
                if (terminal?.attempt == attempt && activeRename?.phase == RenamePhase.Sending &&
                    activeRename.target == sending.target && activeRename.draft == sending.draft
                ) {
                    state = terminal.copy(rename = transition.state)
                }
                if (transition.requireInventoryRead) {
                    awaitInventory(sending.target.machineHandle, activeGeneration)
                }
            }
        }
    }

    fun reattachTerminal() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val handle = current.target.machineHandle
        if (!terminalActionAdmissible(current.machine.canMutate, current.connection)) return
        unavailableTerminalStatus(current)?.let { unavailable ->
            leaveTerminal()
            state = current.copy(connection = unavailable)
            return
        }
        val credential = credentials[handle]
        val runtime = polling[handle]
        if (credential == null || runtime == null) {
            leaveTerminal()
            publishDashboard(
                notice = "${current.machine.machine.label.text}: the machine pairing is no longer available.",
            )
            return
        }
        leaveTerminal()
        val attempt = nextTerminalAttempt++
        state = current.copy(attempt = attempt, connection = TerminalUiStatus.Verifying, kill = null)
        val activeGeneration = generation
        runtime.inventoryOperation.submitRead {
            val result = client.listSessions(credential)
            main.post {
                val terminal = state as? SkidbladnirUiState.Terminal ?: return@post
                if (!isCredentialActive(activeGeneration, credential) || terminal.attempt != attempt) return@post
                when (result) {
                    is GatewayResult.Failure -> {
                        if (!acceptAccessFailure(credential.machine.handle, result.failure)) {
                            updateMachine(credential.machine.handle) { it.inventoryFailed(result.failure) }
                        }
                        val active = state as? SkidbladnirUiState.Terminal ?: return@post
                        state = active.copy(connection = TerminalUiStatus.ReconnectRequired(machineError(credential.machine, result.failure)))
                    }
                    is GatewayResult.Success -> {
                        if (!acceptMachineIdentity(credential, result.value)) return@post
                        val exact = result.value.sessions.any {
                            it.tmuxId == current.target.session.tmuxId &&
                                it.tmuxName == current.target.session.tmuxName &&
                                it.identityToken == current.target.session.identityToken
                        }
                        val active = state as? SkidbladnirUiState.Terminal ?: return@post
                        val connection = terminalReadAdmissionStatus(active, exact)
                        if (connection is TerminalUiStatus.ReconnectRequired) leaveTerminal()
                        state = active.copy(connection = connection)
                    }
                }
            }
        }
    }

    fun detachToSessions() {
        leaveTerminal()
        publishDashboard()
        verifyVisibleInventory()
    }

    fun requestKill(target: SessionTarget) {
        val machine = machineStates[target.machineHandle] ?: return
        val kill = KillState(machine.machine, target, false)
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (machine.canMutate) current.copy(kill = kill) else return
            is SkidbladnirUiState.Terminal ->
                if (current.rename == null && terminalActionAdmissible(machine.canMutate, current.connection)) {
                    terminalPage?.resetInputState()
                    current.copy(kill = kill)
                } else {
                    return
                }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> return
        }
    }

    fun dismissKill() {
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (current.kill?.pending == true) current else current.copy(kill = null)
            is SkidbladnirUiState.Terminal -> if (current.kill?.pending == true) current else current.copy(kill = null)
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> current
        }
    }

    fun confirmKill() {
        val current = state
        val kill = when (current) {
            is SkidbladnirUiState.Dashboard -> current.kill
            is SkidbladnirUiState.Terminal -> current.kill
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> null
        } ?: return
        if (kill.pending) return
        val machine = machineStates[kill.target.machineHandle] ?: return
        val credential = credentials[kill.target.machineHandle] ?: return
        val runtime = polling[kill.target.machineHandle] ?: return
        val pending = kill.copy(pending = true)
        state = when (current) {
            is SkidbladnirUiState.Dashboard -> if (machine.canMutate) current.copy(kill = pending) else return
            is SkidbladnirUiState.Terminal ->
                if (terminalActionAdmissible(machine.canMutate, current.connection)) {
                    leaveTerminal()
                    current.copy(kill = pending)
                } else {
                    return
                }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> return
        }
        val activeGeneration = generation
        runtime.inventoryOperation.submitMutation(
            onReserved = { fence -> requireInventoryRefresh(kill.target.machineHandle, fence) },
        ) { _ ->
            val result = client.killSession(credential, kill.target)
            main.post {
                if (!isCredentialActive(activeGeneration, credential)) return@post
                when (result) {
                    is GatewayResult.Success -> {
                        leaveTerminal()
                        removeTargetFromSnapshot(kill.target)
                        awaitInventory(kill.target.machineHandle, activeGeneration)
                        publishDashboard()
                    }
                    is GatewayResult.Failure -> {
                        if (acceptAccessFailure(kill.target.machineHandle, result.failure)) return@post
                        leaveTerminal()
                        val message = when {
                            killFailureIsDefinitive(result.failure) -> machineError(kill.machine, result.failure)
                            else -> "${kill.machine.label.text}: kill outcome unknown. Sessions are refreshing."
                        }
                        markInventoryFailed(kill.target.machineHandle, result.failure)
                        awaitInventory(kill.target.machineHandle, activeGeneration)
                        publishDashboard(notice = message)
                    }
                }
            }
        }
    }

    private fun startPolling(handle: MachineHandle, activeGeneration: Long) {
        stopPolling(handle)
        val runtime = PollRuntime(
            inventoryOperation = inventoryOperations.forMachine(handle),
        )
        polling[handle] = runtime
        awaitInventory(handle, activeGeneration)
        // justify-polling: tmux and host pressure expose no push inventory; the product fixes a five-second
        // foreground cadence, coalesces overlaps, and stopPolling cancels both schedules on loss/background.
        runtime.inventoryFuture = scheduler.scheduleAtFixedRate(
            { requestInventory(handle, activeGeneration) },
            MACHINE_POLL_CADENCE.toMillis(),
            MACHINE_POLL_CADENCE.toMillis(),
            TimeUnit.MILLISECONDS,
        )
        runtime.pressureFuture = scheduler.scheduleAtFixedRate(
            { requestPressure(handle, activeGeneration) },
            0,
            MACHINE_POLL_CADENCE.toMillis(),
            TimeUnit.MILLISECONDS,
        )
    }

    private fun stopPolling(handle: MachineHandle) {
        awaitedInventoryReads.stop(handle)
        pendingRenameFences.remove(handle)
        polling.remove(handle)?.let { runtime ->
            runtime.inventoryFuture?.cancel(false)
            runtime.pressureFuture?.cancel(false)
        }
    }

    /** Requests an inventory read and keeps its post-request sequence visible until it lands. */
    private fun awaitInventory(handle: MachineHandle, activeGeneration: Long): Boolean {
        val requiredSequence = requestInventory(handle, activeGeneration, requireTrailing = true) ?: return false
        awaitedInventoryReads.requireRead(handle, requiredSequence)
        return true
    }

    /** Returns the admitted or coalesced sequence that will publish a result for this machine. */
    private fun requestInventory(
        handle: MachineHandle,
        activeGeneration: Long,
        requireTrailing: Boolean = false,
    ): Long? {
        val runtime = polling[handle] ?: return null
        val credential = credentials[handle] ?: return null
        val requested = runtime.inventory.request(requireTrailing) ?: return null
        if (requested.startsNow) {
            submitCoalescedInventoryRead(runtime.inventoryOperation, runtime.inventory, requested) { run, completedMutationFence ->
                pollInventory(
                    credential,
                    activeGeneration,
                    completedMutationFence,
                    runtime,
                    run.sequence,
                )
            }
        }
        return requested.sequence
    }

    private fun requestPressure(handle: MachineHandle, activeGeneration: Long) {
        val runtime = polling[handle] ?: return
        val credential = credentials[handle] ?: return
        val run = runtime.pressure.request() ?: return
        executeNetwork {
            try {
                pollPressure(credential, activeGeneration)
            } finally {
                runtime.pressure.finish(run.sequence)
            }
        }
    }

    private fun pollInventory(
        credential: MachineCredential,
        activeGeneration: Long,
        completedMutationFence: Long,
        runtime: PollRuntime,
        completedReadSequence: Long,
    ) {
        val result = client.listSessions(credential)
        val receivedAt = SystemClock.elapsedRealtime()
        main.post {
            if (!isCredentialActive(activeGeneration, credential)) return@post
            val handle = credential.machine.handle
            if (polling[handle] !== runtime) return@post
            awaitedInventoryReads.readLanded(handle, completedReadSequence)
            val machine = machineStates[handle]
            if (machine != null && machine.access == MachineAccess.Ready &&
                mutationFenceSatisfied(machine.inventory, completedMutationFence)
            ) {
                when (result) {
                    is GatewayResult.Failure -> if (!acceptAccessFailure(handle, result.failure) &&
                        pendingRenameFences[handle]?.let { completedMutationFence >= it } != true
                    ) markInventoryFailed(handle, result.failure)
                    is GatewayResult.Success -> if (acceptMachineIdentity(credential, result.value)) {
                        updateMachine(handle) {
                            it.copy(
                                access = MachineAccess.Ready,
                                inventory = InventoryState.Fresh(InventorySnapshot(result.value, receivedAt)),
                            )
                        }
                        pendingRenameFences[handle]?.let { fence ->
                            if (completedMutationFence >= fence) pendingRenameFences.remove(handle)
                        }
                        advanceTerminalRename(handle)
                    }
                }
                advanceCreatedTerminalAdmission(handle, completedMutationFence)
            }
            publishDashboardIfVisible()
        }
    }

    private fun mutationFenceSatisfied(inventory: InventoryState, completedMutationFence: Long): Boolean =
        inventory !is InventoryState.Superseded || completedMutationFence >= inventory.requiredMutationFence

    private fun advanceCreatedTerminalAdmission(handle: MachineHandle, completedMutationFence: Long) {
        val admission = createdTerminalAdmission ?: return
        val terminal = state as? SkidbladnirUiState.Terminal ?: run {
            createdTerminalAdmission = null
            return
        }
        if (terminal.target.machineHandle != handle) return
        if (terminal.attempt != admission.attempt) {
            createdTerminalAdmission = null
            return
        }
        val connection = createdTerminalAdmissionStatus(
            terminal,
            completedMutationFence,
            admission.requiredMutationFence,
        )
        if (connection == TerminalUiStatus.Verifying) return
        createdTerminalAdmission = null
        if (connection is TerminalUiStatus.ReconnectRequired) leaveTerminal()
        state = terminal.copy(connection = connection)
    }

    private fun advanceTerminalRename(handle: MachineHandle) {
        val terminal = state as? SkidbladnirUiState.Terminal ?: return
        if (terminal.target.machineHandle != handle) return
        val reconciled = reconcileTerminalRename(terminal)
        if (reconciled.detachTransport) leaveTerminal()
        state = reconciled.terminal
    }

    private fun pollPressure(credential: MachineCredential, activeGeneration: Long) {
        val result = client.readPressure(credential)
        main.post {
            if (!isCredentialActive(activeGeneration, credential)) return@post
            if (machineStates[credential.machine.handle]?.access != MachineAccess.Ready) return@post
            when (result) {
                is GatewayResult.Success -> updateMachine(credential.machine.handle) {
                    it.copy(pressure = PressureState.Fresh(result.value))
                }
                is GatewayResult.Failure -> if (!acceptAccessFailure(credential.machine.handle, result.failure)) {
                    updateMachine(credential.machine.handle) {
                        it.copy(pressure = it.pressure.downgraded(result.failure))
                    }
                }
            }
            publishDashboardIfVisible()
        }
    }

    private fun acceptMachineIdentity(credential: MachineCredential, inventory: SessionsResponse): Boolean {
        if (inventory.machine.handle == credential.machine.handle) return true
        markIdentityChanged(credential.machine.handle)
        return false
    }

    private fun acceptAccessFailure(handle: MachineHandle, failure: GatewayFailure): Boolean {
        val apiFailure = failure as? GatewayFailure.Api ?: return false
        val code = apiFailure.code
        val access = when (code) {
            ApiErrorCode.Unauthenticated -> MachineAccess.AuthRequired
            ApiErrorCode.MachineIdentityMismatch -> MachineAccess.IdentityChanged
            else -> return false
        }
        stopPolling(handle)
        updateMachine(handle) { machine ->
            machine.copy(
                access = access,
                inventory = machine.inventory.downgraded(failure),
                pressure = machine.pressure.downgraded(failure),
            )
        }
        val machines = sortedMachineStates()
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> state = dashboardAfterMachineAccessLoss(
                current,
                machines,
                handle,
                refreshing = awaitedInventoryReads.isActive,
                dashboardEntry = dashboardEntry,
                failure = apiFailure,
            )
            is SkidbladnirUiState.Terminal -> if (current.target.machineHandle == handle) {
                leaveTerminal()
                state = dashboardAfterTerminalAccessLoss(
                    current,
                    machines,
                    refreshing = awaitedInventoryReads.isActive,
                    dashboardEntry = dashboardEntry,
                )
            } else {
                pendingDashboardNotice = machineAccessMessage(machineStates.getValue(handle))
            }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect ->
                pendingDashboardNotice = machineAccessMessage(machineStates.getValue(handle))
        }
        return true
    }

    private fun markIdentityChanged(handle: MachineHandle) {
        acceptAccessFailure(handle, GatewayFailure.Api(ApiErrorCode.MachineIdentityMismatch))
    }

    private fun markInventoryFailed(handle: MachineHandle, failure: GatewayFailure) {
        updateMachine(handle) { it.inventoryFailed(failure) }
    }

    private fun requireInventoryRefresh(handle: MachineHandle, fence: Long) {
        updateMachine(handle) { machine ->
            when (val inventory = machine.inventory) {
                is InventoryState.Fresh ->
                    machine.copy(inventory = InventoryState.Superseded(inventory.snapshot, fence))
                is InventoryState.Superseded, is InventoryState.Stale,
                is InventoryState.Unreachable, InventoryState.Reading,
                -> machine
            }
        }
        val dashboard = state as? SkidbladnirUiState.Dashboard
        if (dashboard != null) state = dashboard.copy(machines = sortedMachineStates())
    }

    private fun requireRenameInventoryRefresh(handle: MachineHandle, fence: Long) {
        check(pendingRenameFences.put(handle, fence) == null)
        requireInventoryRefresh(handle, fence)
    }

    private fun clearRenameInventoryRefresh(handle: MachineHandle, fence: Long) {
        if (pendingRenameFences[handle] != fence) return
        pendingRenameFences.remove(handle)
        updateMachine(handle) { machine ->
            machine.copy(inventory = clearRenameMutationFence(machine.inventory, fence))
        }
        publishDashboardIfVisible()
    }

    private fun clearInventoryRefresh(handle: MachineHandle) {
        updateMachine(handle) { machine ->
            when (val inventory = machine.inventory) {
                is InventoryState.Superseded ->
                    machine.copy(inventory = InventoryState.Fresh(inventory.snapshot))
                is InventoryState.Fresh, is InventoryState.Stale,
                is InventoryState.Unreachable, InventoryState.Reading,
                -> machine
            }
        }
    }

    private fun removeTargetFromSnapshot(target: SessionTarget) {
        updateMachine(target.machineHandle) { machine ->
            val trimmed = machine.inventory.lastSnapshot()?.let { snapshot ->
                snapshot.copy(
                    inventory = snapshot.inventory.copy(
                        sessions = snapshot.inventory.sessions.filterNot {
                            it.tmuxId == target.session.tmuxId &&
                                it.identityToken == target.session.identityToken
                        },
                    ),
                )
            } ?: return@updateMachine machine
            machine.copy(inventory = when (val inventory = machine.inventory) {
                is InventoryState.Fresh -> InventoryState.Fresh(trimmed)
                is InventoryState.Superseded -> InventoryState.Superseded(trimmed, inventory.requiredMutationFence)
                is InventoryState.Stale -> InventoryState.Stale(trimmed, inventory.cause)
                InventoryState.Reading, is InventoryState.Unreachable -> inventory
            })
        }
    }

    private fun updateMachine(handle: MachineHandle, transform: (MachineState) -> MachineState) {
        val machine = machineStates[handle] ?: return
        val updated = transform(machine)
        machineStates[handle] = updated
        val terminal = state as? SkidbladnirUiState.Terminal ?: return
        if (terminal.target.machineHandle == handle) state = terminal.copy(machine = updated)
    }

    private fun publishDashboardIfVisible() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val carry = forgeCarry(current)
        state = current.copy(
            machines = sortedMachineStates(),
            refreshing = awaitedInventoryReads.isActive,
            forge = carry.forge,
            forgeRecovery = advanceForgeRecovery(carry.recovery, machineStates.values),
        )
    }

    private fun publishDashboard(
        notice: String? = null,
        carry: ForgeCarry = forgeCarry(state),
    ) {
        val dashboardNotice = notice ?: pendingDashboardNotice
        pendingDashboardNotice = null
        state = SkidbladnirUiState.Dashboard(
            machines = sortedMachineStates(),
            refreshing = awaitedInventoryReads.isActive,
            notice = dashboardNotice,
            forge = carry.forge,
            forgeRecovery = carry.recovery,
            kill = null,
            unreadableMachines = unreadableMachines.toList(),
        )
    }

    private fun sortedMachineStates(): List<MachineState> = machineStates.values.sortedBy {
        it.machine.label.text.lowercase(Locale.ROOT)
    }

    private fun leaveTerminal() {
        terminalOwner = null
        createdTerminalAdmission = null
        val connection = terminalConnection
        val page = terminalPage
        terminalConnection = null
        terminalPage = null
        page?.resetInputState()
        connection?.detach()
    }

    private fun machineError(machine: PairedMachine, failure: GatewayFailure): String =
        "${machine.label.text}: ${gatewayFailureMessage(failure)}"

    // justify-defect: worker executors and the inventory lanes otherwise swallow same-system
    // invariant failures; rethrowing on the main looper makes them fatal where they are visible.
    private fun surfaceDefect(defect: RuntimeException) {
        main.post { throw defect }
    }

    private fun executeNetwork(action: () -> Unit) {
        network.execute {
            try { action() } catch (defect: RuntimeException) { surfaceDefect(defect) }
        }
    }

    private fun executeCredentialOperation(action: () -> Unit) {
        credentialOperations.execute {
            try { action() } catch (defect: RuntimeException) { surfaceDefect(defect) }
        }
    }

    private fun isActiveGeneration(activeGeneration: Long): Boolean = foreground && generation == activeGeneration
    private fun isCredentialActive(activeGeneration: Long, credential: MachineCredential): Boolean =
        isActiveGeneration(activeGeneration) && credentials[credential.machine.handle] == credential
}

internal fun terminalAccessLoss(code: ApiErrorCode): MachineAccess? = when (code) {
    ApiErrorCode.Unauthenticated -> MachineAccess.AuthRequired
    ApiErrorCode.MachineIdentityMismatch -> MachineAccess.IdentityChanged
    else -> null
}
