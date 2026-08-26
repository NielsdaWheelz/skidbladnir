package dev.niels.skidbladnir

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

internal sealed interface PairingMode {
    data object Add : PairingMode
    data class Repair(val handle: MachineHandle) : PairingMode
}

internal data class PairingDraft(val label: String, val origin: String, val bearer: String)

internal sealed interface SkidbladnirUiState {
    data object Booting : SkidbladnirUiState

    data class Pairing(
        val mode: PairingMode,
        val draft: PairingDraft,
        val pending: Boolean,
        val error: String?,
        val canCancel: Boolean,
    ) : SkidbladnirUiState

    data class Dashboard(
        val machines: List<MachineState>,
        val selectedMachine: MachineHandle?,
        val refreshing: Boolean,
        val notice: String? = null,
        val forge: ForgeState?,
        val forgeRecovery: ForgeRecovery?,
        val rename: RenameState? = null,
        val kill: KillState?,
    ) : SkidbladnirUiState

    data class Terminal(
        val machine: PairedMachine,
        val target: AgentTarget,
        val machineCanMutate: Boolean,
        val attempt: Int,
        val connection: TerminalUiStatus,
        val kill: KillState?,
    ) : SkidbladnirUiState
}

internal data class ForgeState(val form: ForgeForm, val pending: Boolean, val error: String?)
internal data class RenameState(
    val machine: PairedMachine,
    val draft: String,
    val pending: Boolean,
    val error: String?,
)
internal sealed interface ForgeRecovery {
    val draft: ForgeDraft
    data class RefreshRequired(override val draft: ForgeDraft) : ForgeRecovery
    data class ReviewReady(override val draft: ForgeDraft) : ForgeRecovery
}
internal data class KillState(
    val machine: PairedMachine,
    val target: AgentTarget,
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

internal fun synchronizeTerminalMachineState(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState,
): SkidbladnirUiState.Terminal = if (terminal.target.machineHandle == machine.machine.handle) {
    terminal.copy(machine = machine.machine, machineCanMutate = machine.canMutate)
} else {
    terminal
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
): SkidbladnirUiState.Dashboard {
    val recovery = dashboard.forgeRecovery as? ForgeRecovery.ReviewReady ?: return dashboard
    val target = dashboard.machines.singleOrNull {
        it.machine.handle == recovery.draft.machineHandle
    } ?: return dashboard
    if (!target.canMutate) return dashboard
    return dashboard.copy(
        selectedMachine = target.machine.handle,
        forge = ForgeState(ForgeForm(recovery.draft), pending = false, error = null),
        forgeRecovery = null,
    )
}

internal fun advanceForgeRecovery(
    recovery: ForgeRecovery?,
    machines: Collection<MachineState>,
): ForgeRecovery? {
    if (recovery !is ForgeRecovery.RefreshRequired) return recovery
    val machine = machines.singleOrNull { it.machine.handle == recovery.draft.machineHandle }
    return if (machine?.inventory is InventoryState.Fresh && !machine.inventoryRefreshRequired) {
        ForgeRecovery.ReviewReady(recovery.draft)
    } else {
        recovery
    }
}

internal fun createdTerminalAdmissionStatus(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState?,
    completedMutationFence: Long,
    requiredMutationFence: Long,
): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Verifying)
    require(completedMutationFence >= 0 && requiredMutationFence > 0)
    if (completedMutationFence < requiredMutationFence) return TerminalUiStatus.Verifying
    return availableTerminalStatus(terminal, machine, TerminalUiStatus.Preparing)
}

internal fun terminalPageAdmissionStatus(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState?,
): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Preparing)
    return availableTerminalStatus(terminal, machine, TerminalUiStatus.Connecting)
}

internal fun terminalReadAdmissionStatus(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState?,
    exactLifetimeAvailable: Boolean,
): TerminalUiStatus {
    require(terminal.connection == TerminalUiStatus.Verifying)
    val unavailable = unavailableTerminalStatus(terminal, machine)
    if (unavailable != null) return unavailable
    return if (exactLifetimeAvailable) TerminalUiStatus.Preparing else TerminalUiStatus.ReconnectRequired(
        "${terminal.machine.label.text}: that session lifetime is no longer available.",
    )
}

private fun availableTerminalStatus(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState?,
    available: TerminalUiStatus,
): TerminalUiStatus {
    val unavailable = unavailableTerminalStatus(terminal, machine)
    if (unavailable != null) return unavailable
    requireNotNull(machine)
    val inventory = (machine.inventory as InventoryState.Fresh).snapshot.inventory
    val exact = machine.machine.handle == terminal.target.machineHandle &&
        inventory.machine.handle == terminal.target.machineHandle &&
        inventory.sessions.any {
            it.id == terminal.target.session.id &&
                it.name == terminal.target.session.name &&
                it.identityToken == terminal.target.session.identityToken
        }
    return if (exact) available else TerminalUiStatus.ReconnectRequired(
        "${terminal.machine.label.text}: that session lifetime is no longer available.",
    )
}

private fun unavailableTerminalStatus(
    terminal: SkidbladnirUiState.Terminal,
    machine: MachineState?,
): TerminalUiStatus.ReconnectRequired? {
    if (terminal.kill?.pending != true && machine?.canMutate == true) return null
    val message = when (machine?.access) {
        MachineAccess.AuthRequired -> "${terminal.machine.label.text}: authentication required."
        MachineAccess.IdentityChanged ->
            "${terminal.machine.label.text}: machine identity changed. Remove and pair it again."
        MachineAccess.Ready, null -> "${terminal.machine.label.text}: reconnect required."
    }
    return TerminalUiStatus.ReconnectRequired(message)
}

internal enum class MachineRemovalDestination { PreserveCurrent, Dashboard, Pairing }

internal fun machineRemovalDestination(
    state: SkidbladnirUiState,
    removed: MachineHandle,
    remainingMachines: Int,
): MachineRemovalDestination = when {
    remainingMachines == 0 -> MachineRemovalDestination.Pairing
    state is SkidbladnirUiState.Dashboard -> MachineRemovalDestination.Dashboard
    state is SkidbladnirUiState.Terminal && state.target.machineHandle == removed ->
        MachineRemovalDestination.Dashboard
    else -> MachineRemovalDestination.PreserveCurrent
}

internal fun removeMachineReferences(
    dashboard: SkidbladnirUiState.Dashboard,
    handle: MachineHandle,
): SkidbladnirUiState.Dashboard = dashboard.copy(
    selectedMachine = dashboard.selectedMachine.takeUnless { it == handle },
    forge = dashboard.forge?.takeUnless { it.form.machineHandle == handle },
    forgeRecovery = dashboard.forgeRecovery?.takeUnless { it.draft.machineHandle == handle },
    rename = dashboard.rename?.takeUnless { it.machine.handle == handle },
    kill = dashboard.kill?.takeUnless { it.target.machineHandle == handle },
)

internal fun dashboardAfterTerminalAccessLoss(
    terminal: SkidbladnirUiState.Terminal,
    machines: List<MachineState>,
): SkidbladnirUiState.Dashboard {
    val machine = machines.single { it.machine.handle == terminal.target.machineHandle }
    require(machine.access != MachineAccess.Ready)
    return SkidbladnirUiState.Dashboard(
        machines = machines,
        selectedMachine = machine.machine.handle,
        refreshing = machines.any { it.inventory == InventoryState.Reading },
        notice = machineAccessMessage(machine),
        forge = null,
        forgeRecovery = null,
        rename = null,
        kill = null,
    )
}

internal fun dashboardAfterMachineAccessLoss(
    dashboard: SkidbladnirUiState.Dashboard,
    machines: List<MachineState>,
    handle: MachineHandle,
): SkidbladnirUiState.Dashboard {
    val machine = machines.single { it.machine.handle == handle }
    require(machine.access != MachineAccess.Ready)
    val message = machineAccessMessage(machine)
    val affectedForge = dashboard.forge?.takeIf { it.form.machineHandle == handle }
    val affectedKill = dashboard.kill?.takeIf { it.target.machineHandle == handle }
    return dashboard.copy(
        machines = machines,
        selectedMachine = if (affectedForge?.pending == true || affectedKill != null) {
            handle
        } else {
            dashboard.selectedMachine
        },
        refreshing = machines.any { it.inventory == InventoryState.Reading },
        notice = message,
        forge = if (affectedForge?.pending == true) {
            affectedForge.copy(pending = false, error = message)
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
        "${machine.machine.label.text}: machine identity changed. Remove and pair it again."
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

internal class CoalescingPollLane {
    private var inFlight = false
    private var trailingRequired = false

    @Synchronized
    fun tryStart(requireTrailing: Boolean = false): Boolean {
        if (!inFlight) {
            inFlight = true
            return true
        }
        if (requireTrailing) trailingRequired = true
        return false
    }

    @Synchronized
    fun finish(): Boolean {
        check(inFlight)
        if (trailingRequired) {
            trailingRequired = false
            return true
        }
        inFlight = false
        return false
    }

    @Synchronized
    fun abort() {
        inFlight = false
        trailingRequired = false
    }
}

internal class InventoryOperationLane {
    private val executor: Executor
    private val onDefect: (RuntimeException) -> Unit
    private val queued = ArrayDeque<() -> Unit>()
    private var draining = false
    private var submittedMutationFence = 0L
    private var completedMutationFence = 0L

    constructor(executor: Executor, onDefect: (RuntimeException) -> Unit = { throw it }) {
        this.executor = executor
        this.onDefect = onDefect
    }

    @Synchronized
    fun submitRead(action: (Long) -> Unit) {
        enqueue { action(completedMutationFence) }
    }

    @Synchronized
    fun submitMutation(onReserved: (Long) -> Unit = {}, action: (Long) -> Unit): Long {
        val fence = ++submittedMutationFence
        onReserved(fence)
        enqueue {
            try {
                action(fence)
            } finally {
                synchronized(this) { completedMutationFence = fence }
            }
        }
        return fence
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

internal class MachineInventoryOperations(
    private val executor: Executor,
    private val onDefect: (RuntimeException) -> Unit = { throw it },
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
    fun reconcile(storedCredentials: List<MachineCredential>): ReconciledStoredControllerState =
        ReconciledStoredControllerState(
            machines = reconcileStoredMachines(
                currentCredentials = currentCredentials,
                currentMachines = currentMachines,
                storedCredentials = storedCredentials,
            ),
            forgeCarry = retainedForgeCarry?.takeIf {
                forgeAuthoritySurvives(it, currentCredentials, storedCredentials)
            } ?: ForgeCarry(null, null),
        )

    fun reconcileIfCurrent(
        isCurrent: Boolean,
        storedCredentials: List<MachineCredential>,
    ): ReconciledStoredControllerState? = if (isCurrent) reconcile(storedCredentials) else null
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
        is SkidbladnirUiState.Pairing, is SkidbladnirUiState.Terminal -> null
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

internal class SkidbladnirController(context: Context) {
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
    private val store = MachineStore(context.applicationContext)
    private val client = GatewayClient()
    private val credentials = ConcurrentHashMap<MachineHandle, MachineCredential>()
    private val machineStates = linkedMapOf<MachineHandle, MachineState>()
    private val polling = ConcurrentHashMap<MachineHandle, PollRuntime>()
    private val inventoryOperations = MachineInventoryOperations(network) { defect ->
        main.post { throw defect }
    }
    private val pendingInventoryMutationFences = linkedMapOf<MachineHandle, Long>()
    private var pendingDashboardNotice: String? = null
    private var retainedStoredMachineForgeCarry: ForgeCarry? = null
    @Volatile private var foreground = false
    @Volatile private var generation = 0L
    private var terminalConnection: TerminalConnection? = null
    private var terminalPage: TerminalPage? = null
    private var terminalOwner: Any? = null
    private var createdTerminalAdmission: CreatedTerminalAdmission? = null
    private var nextTerminalAttempt = 1

    fun start() {
        if (foreground) return
        foreground = true
        val activeGeneration = ++generation
        val storedMachineRead = beginStoredMachineRead(
            state = state,
            currentCredentials = credentials.values,
            currentMachines = machineStates.values,
            retainedForgeCarry = retainedStoredMachineForgeCarry,
        )
        retainedStoredMachineForgeCarry = storedMachineRead.retainedForgeCarry
        state = storedMachineRead.readingState
        executeCredentialOperation {
            val stored = try {
                store.readAll()
            } catch (_: Exception) {
                val reset = runCatching { store.resetAll() }.isSuccess
                main.post {
                    if (!isActiveGeneration(activeGeneration)) return@post
                    retainedStoredMachineForgeCarry = null
                    credentials.clear()
                    machineStates.clear()
                    pendingInventoryMutationFences.clear()
                    state = pairingState(
                        error = if (reset) {
                            "Stored machines could not be read and were cleared. Pair each machine again."
                        } else {
                            "Stored machines could not be read or cleared."
                        },
                    )
                }
                return@executeCredentialOperation
            }
            main.post {
                val reconciliation = storedMachineRead.reconcileIfCurrent(
                    isCurrent = isActiveGeneration(activeGeneration),
                    storedCredentials = stored,
                ) ?: return@post
                retainedStoredMachineForgeCarry = null
                val reconciled = reconciliation.machines
                val resetHandles = reconciled.mapNotNull { entry ->
                    val current = credentials[entry.credential.machine.handle]
                    entry.credential.machine.handle.takeUnless {
                        current != null &&
                            current.bearer == entry.credential.bearer &&
                            current.machine.origin == entry.credential.machine.origin
                    }
                }
                pendingInventoryMutationFences.keys.retainAll(
                    reconciled.map { it.credential.machine.handle }.toSet(),
                )
                resetHandles.forEach(pendingInventoryMutationFences::remove)
                credentials.clear()
                machineStates.clear()
                reconciled.forEach { entry ->
                    credentials[entry.credential.machine.handle] = entry.credential
                    machineStates[entry.machine.machine.handle] = entry.machine
                }
                if (reconciled.isEmpty()) state = pairingState() else {
                    publishDashboard(refreshing = true, carry = reconciliation.forgeCarry)
                    reconciled.filter { it.machine.access == MachineAccess.Ready }.forEach {
                        startPolling(it.credential.machine.handle, activeGeneration)
                    }
                }
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
        leaveTerminal()
        val current = state
        if (current is SkidbladnirUiState.Terminal) publishDashboard(refreshing = false)
    }

    fun close() {
        stopForBackground()
        scheduler.shutdownNow()
        credentialOperations.shutdownNow()
        network.shutdownNow()
        client.http.dispatcher.executorService.shutdown()
        client.http.connectionPool.evictAll()
    }

    fun addMachine() {
        state = pairingState(canCancel = credentials.isNotEmpty())
    }

    fun repairMachine(handle: MachineHandle) {
        val machine = credentials[handle]?.machine ?: return
        state = SkidbladnirUiState.Pairing(
            mode = PairingMode.Repair(handle),
            draft = PairingDraft(machine.label.text, machine.origin.encoded, ""),
            pending = false,
            error = null,
            canCancel = true,
        )
    }

    fun cancelPairing() {
        val current = state as? SkidbladnirUiState.Pairing ?: return
        if (!current.canCancel || current.pending) return
        publishDashboard(refreshing = false)
    }

    fun updatePairingLabel(value: String) = updatePairing { it.copy(label = value) }
    fun updatePairingOrigin(value: String) = updatePairing { it.copy(origin = value) }
    fun updatePairingBearer(value: String) = updatePairing { it.copy(bearer = value) }

    private fun updatePairing(transform: (PairingDraft) -> PairingDraft) {
        val current = state as? SkidbladnirUiState.Pairing ?: return
        if (!current.pending) state = current.copy(draft = transform(current.draft), error = null)
    }

    fun pair() {
        val current = state as? SkidbladnirUiState.Pairing ?: return
        if (current.pending) return
        val label = MachineLabel.parse(current.draft.label)
        val origin = MachineOrigin.parse(current.draft.origin)
        val bearer = GatewayBearer.parse(current.draft.bearer)
        val inputError = when {
            label == null -> "Enter a unique machine label without leading, trailing, or control characters."
            origin == null -> "Enter an HTTPS machine origin with port 8443 and no path, query, or fragment."
            bearer == null -> "Enter the 43-character bearer exactly as minted on this machine."
            else -> null
        }
        if (inputError != null) {
            state = current.copy(error = inputError)
            return
        }
        requireNotNull(label); requireNotNull(origin); requireNotNull(bearer)
        val repair = current.mode as? PairingMode.Repair
        if (repair == null && credentials.values.any {
                it.machine.label.text.equals(label.text, ignoreCase = true) || it.machine.origin == origin
            }
        ) {
            state = current.copy(error = "Machine labels and origins must be unique.")
            return
        }
        val activeGeneration = generation
        state = current.copy(pending = true, error = null)
        executeCredentialOperation {
            when (val result = client.pair(origin, bearer)) {
                is GatewayResult.Failure -> main.post {
                    if (!isActiveGeneration(activeGeneration)) return@post
                    val pairing = state as? SkidbladnirUiState.Pairing ?: return@post
                    state = pairing.copy(pending = false, error = gatewayFailureMessage(result.failure))
                }
                is GatewayResult.Success -> {
                    val returnedHandle = result.value.machine.handle
                    val expected = repair?.handle
                    if (expected != null && returnedHandle != expected) {
                        main.post {
                            if (!isActiveGeneration(activeGeneration)) return@post
                            markIdentityChanged(expected)
                            state = SkidbladnirUiState.Pairing(
                                current.mode,
                                current.draft.copy(bearer = ""),
                                false,
                                "The machine identity changed. Remove this machine, then pair it again.",
                                true,
                            )
                        }
                        return@executeCredentialOperation
                    }
                    if (repair == null && credentials.containsKey(returnedHandle)) {
                        main.post {
                            if (!isActiveGeneration(activeGeneration)) return@post
                            val pairing = state as? SkidbladnirUiState.Pairing ?: return@post
                            state = pairing.copy(pending = false, error = "That machine is already paired.")
                        }
                        return@executeCredentialOperation
                    }
                    val machine = PairedMachine(returnedHandle, label, origin)
                    val credential = MachineCredential(machine, bearer)
                    val stored = runCatching {
                        if (repair == null) store.add(credential) else store.rotateBearer(credential)
                    }
                    if (stored.isFailure) {
                        main.post {
                            if (!isActiveGeneration(activeGeneration)) return@post
                            val pairing = state as? SkidbladnirUiState.Pairing ?: return@post
                            state = pairing.copy(pending = false, error = "Pairing worked, but secure machine storage failed.")
                        }
                        return@executeCredentialOperation
                    }
                    val receivedAt = SystemClock.elapsedRealtime()
                    main.post {
                        if (!isActiveGeneration(activeGeneration)) return@post
                        credentials[returnedHandle] = credential
                        machineStates[returnedHandle] = MachineState(
                            machine,
                            MachineAccess.Ready,
                            InventoryState.Fresh(InventorySnapshot(result.value, receivedAt)),
                            PressureState.Reading,
                        )
                        publishDashboard(refreshing = true)
                        startPolling(returnedHandle, activeGeneration)
                    }
                }
            }
        }
    }

    fun removeMachine(handle: MachineHandle) {
        val credential = credentials[handle] ?: return
        val activeGeneration = generation
        executeCredentialOperation {
            if (runCatching { store.remove(handle) }.isFailure) {
                main.post {
                    if (!isActiveGeneration(activeGeneration)) return@post
                    if (credentials[handle] != credential) return@post
                    val notice = "${credential.machine.label.text}: secure machine removal failed. Nothing was removed."
                    if (state is SkidbladnirUiState.Dashboard) {
                        publishDashboard(refreshing = false, notice = notice)
                    } else {
                        pendingDashboardNotice = notice
                    }
                }
                return@executeCredentialOperation
            }
            main.post {
                if (!isActiveGeneration(activeGeneration)) return@post
                if (credentials[handle] != credential) return@post
                val current = state
                val destination = machineRemovalDestination(current, handle, credentials.size - 1)
                val remainingDashboard = (current as? SkidbladnirUiState.Dashboard)?.let {
                    removeMachineReferences(it, handle)
                }
                stopPolling(handle)
                pendingInventoryMutationFences.remove(handle)
                credentials.remove(handle)
                machineStates.remove(handle)
                when (destination) {
                    MachineRemovalDestination.PreserveCurrent -> Unit
                    MachineRemovalDestination.Pairing -> {
                        leaveTerminal()
                        state = pairingState()
                    }
                    MachineRemovalDestination.Dashboard -> {
                        leaveTerminal()
                        if (remainingDashboard != null) state = remainingDashboard
                        publishDashboard(refreshing = false)
                    }
                }
            }
        }
    }

    fun requestRenameMachine(handle: MachineHandle) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val machine = machineStates[handle]?.machine ?: return
        state = current.copy(rename = RenameState(machine, machine.label.text, pending = false, error = null))
    }

    fun updateRenameMachineDraft(value: String) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val rename = current.rename ?: return
        if (!rename.pending) state = current.copy(rename = rename.copy(draft = value, error = null))
    }

    fun dismissRenameMachine() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.rename?.pending != true) state = current.copy(rename = null)
    }

    fun confirmRenameMachine() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val rename = current.rename ?: return
        if (rename.pending) return
        val label = MachineLabel.parse(rename.draft)
        if (label == null) {
            state = current.copy(
                rename = rename.copy(
                    error = "Enter a label without leading, trailing, or control characters.",
                ),
            )
            return
        }
        val handle = rename.machine.handle
        if (machineStates.values.any {
                it.machine.handle != handle && it.machine.label.text.equals(label.text, ignoreCase = true)
            }
        ) {
            state = current.copy(rename = rename.copy(error = "Machine labels must be unique."))
            return
        }
        if (label == rename.machine.label) {
            state = current.copy(rename = null)
            return
        }
        if (credentials[handle] == null) return
        state = current.copy(rename = rename.copy(pending = true, error = null))
        val activeGeneration = generation
        executeCredentialOperation {
            val persisted = runCatching { store.rename(handle, label) }
            main.post {
                if (!isActiveGeneration(activeGeneration)) return@post
                if (persisted.isFailure) {
                    val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                    val activeRename = dashboard.rename?.takeIf { it.machine.handle == handle } ?: return@post
                    state = dashboard.copy(
                        rename = activeRename.copy(
                            pending = false,
                            error = "The machine label could not be saved.",
                        ),
                    )
                    return@post
                }
                val activeCredential = credentials[handle] ?: return@post
                val renamed = renameMachineLabel(machineStates.values.toList(), handle, label)
                machineStates.clear()
                renamed.forEach { machineStates[it.machine.handle] = it }
                credentials[handle] = activeCredential.copy(machine = activeCredential.machine.copy(label = label))
                val dashboard = state as? SkidbladnirUiState.Dashboard
                if (dashboard != null) {
                    state = dashboard.copy(machines = sortedMachineStates(), rename = null)
                }
            }
        }
    }

    fun selectMachine(handle: MachineHandle?) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (handle != null && machineStates[handle] == null) return
        state = current.copy(selectedMachine = handle)
    }

    fun refresh() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        state = current.copy(refreshing = true, notice = null)
        val selected = current.selectedMachine
        credentials.keys.filter { selected == null || it == selected }.forEach { handle ->
            requestInventory(handle, generation, requireTrailing = true)
            requestPressure(handle, generation)
        }
    }

    fun openForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val handle = current.selectedMachine
        if (handle == null) {
            if (machineStates.values.none(::machineCanForge)) return
            state = current.copy(forge = ForgeState(ForgeForm(null, "", "", "", ""), false, null))
            return
        }
        val machine = machineStates[handle] ?: return
        if (!machineCanForge(machine)) return
        val profiles = (machine.inventory as InventoryState.Fresh).snapshot.inventory.profiles
        state = current.copy(
            forge = ForgeState(ForgeForm(handle, "", profiles.first().key, "", ""), false, null),
        )
    }

    fun resumeForgeRecovery() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        state = dev.niels.skidbladnir.resumeForgeRecovery(current)
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
        val updated = if (proposed.machineHandle == forge.form.machineHandle) proposed else proposed.copy(
            cwd = "",
            profile = "",
        )
        if (updated.machineHandle != null && machineStates[updated.machineHandle] == null) return
        state = current.copy(forge = forge.copy(form = updated, error = null))
    }

    fun forge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val draft = forge.form.submission() ?: return
        val credential = credentials[draft.machineHandle] ?: return
        val machine = machineStates[draft.machineHandle] ?: return
        val runtime = polling[draft.machineHandle] ?: return
        if (forge.pending || !machine.canMutate) return
        state = current.copy(forge = forge.copy(pending = true, error = null))
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
                        AgentTarget(credential.machine.handle, result.value),
                        mutationFence,
                    )
                    requestInventory(credential.machine.handle, activeGeneration, requireTrailing = true)
                }
                is GatewayResult.Failure -> main.post {
                    if (!isCredentialActive(activeGeneration, credential)) return@post
                    if (acceptAccessFailure(credential.machine.handle, result.failure)) return@post
                    val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                    val activeForge = dashboard.forge ?: return@post
                    if (createFailureIsDefinitive(result.failure)) {
                        clearInventoryRefresh(credential.machine.handle)
                        state = dashboard.copy(
                            machines = sortedMachineStates(),
                            forge = activeForge.copy(pending = false, error = machineError(credential.machine, result.failure)),
                        )
                    } else {
                        markInventoryFailed(credential.machine.handle, result.failure)
                        state = dashboard.copy(
                            machines = sortedMachineStates(),
                            forge = null,
                            forgeRecovery = ForgeRecovery.RefreshRequired(
                                checkNotNull(activeForge.form.submission()),
                            ),
                        )
                    }
                    requestInventory(credential.machine.handle, activeGeneration, requireTrailing = true)
                }
            }
        }
    }

    fun openTerminal(target: AgentTarget) {
        val machine = machineStates[target.machineHandle] ?: return
        if (!machine.canMutate) return
        enterTerminal(target)
    }

    private fun enterTerminal(target: AgentTarget) {
        val machine = machineStates[target.machineHandle] ?: return
        leaveTerminal()
        state = SkidbladnirUiState.Terminal(
            machine = machine.machine,
            target = target,
            machineCanMutate = machine.canMutate,
            attempt = nextTerminalAttempt++,
            connection = TerminalUiStatus.Preparing,
            kill = null,
        )
    }

    private fun enterCreatedTerminal(target: AgentTarget, requiredMutationFence: Long) {
        val machine = machineStates[target.machineHandle] ?: return
        leaveTerminal()
        val attempt = nextTerminalAttempt++
        state = SkidbladnirUiState.Terminal(
            machine = machine.machine,
            target = target,
            machineCanMutate = machine.canMutate,
            attempt = attempt,
            connection = TerminalUiStatus.Verifying,
            kill = null,
        )
        createdTerminalAdmission = CreatedTerminalAdmission(attempt, requiredMutationFence)
    }

    fun terminalPageReady(attempt: Int, page: TerminalPage) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection != TerminalUiStatus.Preparing) return
        when (val admission = terminalPageAdmissionStatus(current, machineStates[current.target.machineHandle])) {
            TerminalUiStatus.Connecting -> Unit
            is TerminalUiStatus.ReconnectRequired -> {
                leaveTerminal()
                state = current.copy(connection = admission)
                return
            }
            TerminalUiStatus.Preparing, TerminalUiStatus.Verifying, is TerminalUiStatus.Connected ->
                error("terminal page admission produced an impossible state")
        }
        val credential = credentials[current.target.machineHandle]
        if (credential == null) {
            leaveTerminal()
            publishDashboard(
                refreshing = false,
                notice = "${current.machine.label.text}: the machine pairing is no longer available.",
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

    fun reattachTerminal() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val handle = current.target.machineHandle
        if (!terminalActionAdmissible(machineStates[handle]?.canMutate == true, current.connection)) return
        val unavailable = unavailableTerminalStatus(current, machineStates[handle])
        if (unavailable != null) {
            leaveTerminal()
            state = current.copy(connection = unavailable)
            return
        }
        val credential = credentials[handle]
        val runtime = polling[handle]
        if (credential == null || runtime == null) {
            leaveTerminal()
            publishDashboard(
                refreshing = false,
                notice = "${current.machine.label.text}: the machine pairing is no longer available.",
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
                        acceptAccessFailure(credential.machine.handle, result.failure)
                        val active = state as? SkidbladnirUiState.Terminal ?: return@post
                        state = active.copy(connection = TerminalUiStatus.ReconnectRequired(machineError(credential.machine, result.failure)))
                    }
                    is GatewayResult.Success -> {
                        if (!acceptMachineIdentity(credential, result.value)) return@post
                        val exact = result.value.sessions.any {
                            it.id == current.target.session.id &&
                                it.name == current.target.session.name &&
                                it.identityToken == current.target.session.identityToken
                        }
                        val connection = terminalReadAdmissionStatus(
                            terminal,
                            machineStates[current.target.machineHandle],
                            exact,
                        )
                        if (connection is TerminalUiStatus.ReconnectRequired) leaveTerminal()
                        state = terminal.copy(connection = connection)
                    }
                }
            }
        }
    }

    fun detachToAgents() {
        leaveTerminal()
        publishDashboard(refreshing = true)
        refresh()
    }

    fun requestKill(target: AgentTarget) {
        val machine = machineStates[target.machineHandle] ?: return
        val kill = KillState(machine.machine, target, false)
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (machine.canMutate) current.copy(kill = kill) else return
            is SkidbladnirUiState.Terminal ->
                if (terminalActionAdmissible(machine.canMutate, current.connection)) {
                    current.copy(kill = kill)
                } else {
                    return
                }
            else -> return
        }
    }

    fun dismissKill() {
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (current.kill?.pending == true) current else current.copy(kill = null)
            is SkidbladnirUiState.Terminal -> if (current.kill?.pending == true) current else current.copy(kill = null)
            else -> current
        }
    }

    fun confirmKill() {
        val current = state
        val kill = when (current) {
            is SkidbladnirUiState.Dashboard -> current.kill
            is SkidbladnirUiState.Terminal -> current.kill
            else -> null
        } ?: return
        val machine = machineStates[kill.target.machineHandle] ?: return
        val actionAdmissible = when (current) {
            is SkidbladnirUiState.Dashboard -> machine.canMutate
            is SkidbladnirUiState.Terminal ->
                terminalActionAdmissible(machine.canMutate, current.connection)
            else -> false
        }
        if (kill.pending || !actionAdmissible) return
        val credential = credentials[kill.target.machineHandle] ?: return
        val runtime = polling[kill.target.machineHandle] ?: return
        state = when (current) {
            is SkidbladnirUiState.Dashboard -> current.copy(kill = kill.copy(pending = true))
            is SkidbladnirUiState.Terminal -> {
                leaveTerminal(); current.copy(kill = kill.copy(pending = true))
            }
            else -> return
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
                        publishDashboard(refreshing = true)
                        requestInventory(kill.target.machineHandle, activeGeneration, requireTrailing = true)
                    }
                    is GatewayResult.Failure -> {
                        if (acceptAccessFailure(kill.target.machineHandle, result.failure)) return@post
                        leaveTerminal()
                        val message = if (killFailureIsDefinitive(result.failure)) {
                            machineError(kill.machine, result.failure)
                        } else {
                            "${kill.machine.label.text}: kill outcome unknown. Agents are refreshing."
                        }
                        markInventoryFailed(kill.target.machineHandle, result.failure)
                        publishDashboard(refreshing = true, notice = message)
                        requestInventory(kill.target.machineHandle, activeGeneration, requireTrailing = true)
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
        runtime.inventoryFuture = scheduler.scheduleAtFixedRate(
            { requestInventory(handle, activeGeneration) }, 0, 5, TimeUnit.SECONDS,
        )
        runtime.pressureFuture = scheduler.scheduleAtFixedRate(
            { requestPressure(handle, activeGeneration) }, 0, 5, TimeUnit.SECONDS,
        )
    }

    private fun stopPolling(handle: MachineHandle) {
        polling.remove(handle)?.let { runtime ->
            runtime.inventoryFuture?.cancel(false)
            runtime.pressureFuture?.cancel(false)
        }
    }

    private fun requestInventory(handle: MachineHandle, activeGeneration: Long, requireTrailing: Boolean = false) {
        val runtime = polling[handle] ?: return
        val credential = credentials[handle] ?: return
        if (!runtime.inventory.tryStart(requireTrailing)) return
        runtime.inventoryOperation.submitRead { completedMutationFence ->
            try {
                do {
                    pollInventory(credential, activeGeneration, completedMutationFence)
                } while (runtime.inventory.finish())
            } catch (defect: RuntimeException) {
                runtime.inventory.abort()
                throw defect
            }
        }
    }

    private fun requestPressure(handle: MachineHandle, activeGeneration: Long) {
        val runtime = polling[handle] ?: return
        val credential = credentials[handle] ?: return
        if (!runtime.pressure.tryStart()) return
        executeNetwork {
            try { pollPressure(credential, activeGeneration) } finally { runtime.pressure.finish() }
        }
    }

    private fun pollInventory(
        credential: MachineCredential,
        activeGeneration: Long,
        completedMutationFence: Long,
    ) {
        val result = client.listSessions(credential)
        val receivedAt = SystemClock.elapsedRealtime()
        main.post {
            if (!isCredentialActive(activeGeneration, credential)) return@post
            if (machineStates[credential.machine.handle]?.access != MachineAccess.Ready) return@post
            val requiredFence = pendingInventoryMutationFences[credential.machine.handle]
            if (requiredFence != null && completedMutationFence < requiredFence) return@post
            when (result) {
                is GatewayResult.Failure -> {
                    if (!acceptAccessFailure(credential.machine.handle, result.failure)) {
                        markInventoryFailed(credential.machine.handle, result.failure)
                    }
                }
                is GatewayResult.Success -> if (acceptMachineIdentity(credential, result.value)) {
                    if (requiredFence != null) pendingInventoryMutationFences.remove(credential.machine.handle)
                    updateMachine(credential.machine.handle) {
                        it.copy(
                            access = MachineAccess.Ready,
                            inventory = InventoryState.Fresh(InventorySnapshot(result.value, receivedAt)),
                            inventoryRefreshRequired = false,
                        )
                    }
                }
            }
            advanceCreatedTerminalAdmission(credential.machine.handle, completedMutationFence)
            publishDashboardIfVisible()
        }
    }

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
            machineStates[handle],
            completedMutationFence,
            admission.requiredMutationFence,
        )
        if (connection == TerminalUiStatus.Verifying) return
        createdTerminalAdmission = null
        if (connection is TerminalUiStatus.ReconnectRequired) leaveTerminal()
        state = terminal.copy(connection = connection)
    }

    private fun pollPressure(credential: MachineCredential, activeGeneration: Long) {
        val result = client.readPressure(credential)
        main.post {
            if (!isCredentialActive(activeGeneration, credential)) return@post
            if (machineStates[credential.machine.handle]?.access != MachineAccess.Ready) return@post
            when (result) {
                is GatewayResult.Success -> updateMachine(credential.machine.handle) { it.copy(pressure = PressureState.Fresh(result.value)) }
                is GatewayResult.Failure -> if (!acceptAccessFailure(credential.machine.handle, result.failure)) {
                    updateMachine(credential.machine.handle) { machine ->
                        machine.copy(pressure = when (val pressure = machine.pressure) {
                            is PressureState.Fresh -> PressureState.Stale(pressure.response, result.failure)
                            is PressureState.Stale -> PressureState.Stale(pressure.response, result.failure)
                            PressureState.Reading, is PressureState.Unavailable -> PressureState.Unavailable(result.failure)
                        })
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
        val code = (failure as? GatewayFailure.Api)?.code ?: return false
        val access = when (code) {
            ApiErrorCode.Unauthenticated -> MachineAccess.AuthRequired
            ApiErrorCode.MachineIdentityMismatch -> MachineAccess.IdentityChanged
            else -> return false
        }
        stopPolling(handle)
        pendingInventoryMutationFences.remove(handle)
        updateMachine(handle) { machine ->
            machine.copy(
                access = access,
                inventoryRefreshRequired = false,
                inventory = when (val inventory = machine.inventory) {
                    is InventoryState.Fresh -> InventoryState.Stale(inventory.snapshot, failure)
                    is InventoryState.Stale -> InventoryState.Stale(inventory.snapshot, failure)
                    InventoryState.Reading, is InventoryState.Unreachable -> InventoryState.Unreachable(failure)
                },
                pressure = when (val pressure = machine.pressure) {
                    is PressureState.Fresh -> PressureState.Stale(pressure.response, failure)
                    is PressureState.Stale -> PressureState.Stale(pressure.response, failure)
                    PressureState.Reading, is PressureState.Unavailable -> PressureState.Unavailable(failure)
                },
            )
        }
        val machines = sortedMachineStates()
        when (val current = state) {
            is SkidbladnirUiState.Dashboard ->
                state = dashboardAfterMachineAccessLoss(current, machines, handle)
            is SkidbladnirUiState.Terminal -> if (current.target.machineHandle == handle) {
                leaveTerminal()
                state = dashboardAfterTerminalAccessLoss(current, machines)
            } else {
                pendingDashboardNotice = machineAccessMessage(machineStates.getValue(handle))
            }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.Pairing ->
                pendingDashboardNotice = machineAccessMessage(machineStates.getValue(handle))
        }
        return true
    }

    private fun markIdentityChanged(handle: MachineHandle) {
        acceptAccessFailure(handle, GatewayFailure.Api(ApiErrorCode.MachineIdentityMismatch))
    }

    private fun markInventoryFailed(handle: MachineHandle, failure: GatewayFailure) {
        updateMachine(handle) { machine ->
            markInventoryFailure(listOf(machine), handle, failure).single()
        }
    }

    private fun requireInventoryRefresh(handle: MachineHandle, fence: Long) {
        pendingInventoryMutationFences[handle] = fence
        updateMachine(handle) { it.copy(inventoryRefreshRequired = true) }
        val dashboard = state as? SkidbladnirUiState.Dashboard
        if (dashboard != null) state = dashboard.copy(machines = sortedMachineStates())
    }

    private fun clearInventoryRefresh(handle: MachineHandle) {
        pendingInventoryMutationFences.remove(handle)
        updateMachine(handle) { it.copy(inventoryRefreshRequired = false) }
    }

    private fun removeTargetFromSnapshot(target: AgentTarget) {
        updateMachine(target.machineHandle) { machine ->
            val inventory = machine.inventory as? InventoryState.Fresh ?: return@updateMachine machine
            val response = inventory.snapshot.inventory
            machine.copy(inventory = InventoryState.Fresh(inventory.snapshot.copy(
                inventory = response.copy(sessions = response.sessions.filterNot {
                    it.id == target.session.id && it.identityToken == target.session.identityToken
                }),
            )))
        }
    }

    private fun updateMachine(handle: MachineHandle, transform: (MachineState) -> MachineState) {
        val machine = machineStates[handle] ?: return
        val updated = transform(machine)
        machineStates[handle] = updated
        val terminal = state as? SkidbladnirUiState.Terminal ?: return
        if (terminal.target.machineHandle == handle) state = synchronizeTerminalMachineState(terminal, updated)
    }

    private fun publishDashboardIfVisible() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val carry = forgeCarry(current)
        state = current.copy(
            machines = sortedMachineStates(),
            refreshing = machineStates.values.any { it.inventory == InventoryState.Reading },
            forge = carry.forge,
            forgeRecovery = advanceForgeRecovery(carry.recovery, machineStates.values),
        )
    }

    private fun publishDashboard(
        refreshing: Boolean,
        notice: String? = null,
        carry: ForgeCarry = forgeCarry(state),
    ) {
        val dashboardNotice = notice ?: pendingDashboardNotice
        pendingDashboardNotice = null
        state = SkidbladnirUiState.Dashboard(
            machines = sortedMachineStates(),
            selectedMachine = (state as? SkidbladnirUiState.Dashboard)?.selectedMachine,
            refreshing = refreshing,
            notice = dashboardNotice,
            forge = carry.forge,
            forgeRecovery = carry.recovery,
            rename = null,
            kill = null,
        )
    }

    private fun sortedMachineStates(): List<MachineState> = machineStates.values.sortedBy {
        it.machine.label.text.lowercase(Locale.ROOT)
    }

    private fun machineCanForge(machine: MachineState): Boolean =
        machine.canMutate &&
            (machine.inventory as? InventoryState.Fresh)?.snapshot?.inventory?.profiles?.isNotEmpty() == true

    private fun pairingState(error: String? = null, canCancel: Boolean = false): SkidbladnirUiState.Pairing =
        SkidbladnirUiState.Pairing(PairingMode.Add, PairingDraft("", "", ""), false, error, canCancel)

    private fun leaveTerminal() {
        terminalOwner = null
        createdTerminalAdmission = null
        val connection = terminalConnection
        terminalConnection = null
        terminalPage = null
        connection?.detach()
    }

    private fun machineError(machine: PairedMachine, failure: GatewayFailure): String =
        "${machine.label.text}: ${gatewayFailureMessage(failure)}"

    private fun executeNetwork(action: () -> Unit) {
        network.execute {
            try { action() } catch (defect: RuntimeException) {
                main.post { throw defect }
            }
        }
    }

    private fun executeCredentialOperation(action: () -> Unit) {
        credentialOperations.execute {
            try { action() } catch (defect: RuntimeException) {
                main.post { throw defect }
            }
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
