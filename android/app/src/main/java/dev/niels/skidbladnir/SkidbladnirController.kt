package dev.niels.skidbladnir

import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

internal sealed interface SkidbladnirUiState {
    data object Booting : SkidbladnirUiState

    data class FleetConnect(
        val mode: FleetConnectMode,
        val phase: FleetConnectPhase,
    ) : SkidbladnirUiState

    data class Dashboard(
        val machines: List<MachineState>,
        val refreshing: Boolean,
        val notice: String? = null,
        val forge: ForgeState?,
        val forgeRecovery: ForgeRecovery?,
        val close: CloseState?,
        val groupEditor: SessionEdit? = null,
        val needsInputSettled: Boolean = true,
        val terminalControlPending: Boolean = false,
    ) : SkidbladnirUiState

    data class Terminal(
        val machine: MachineState,
        val machines: List<MachineState>,
        val target: SessionTarget,
        val attempt: Int,
        val connection: TerminalUiStatus,
        val viewport: TerminalViewport,
        val textSize: TerminalTextSizeState,
        val close: CloseState?,
        val rename: SessionEdit? = null,
        val terminalControlPending: Boolean = false,
        val shellPending: Boolean = false,
    ) : SkidbladnirUiState
}

internal fun fleetReconnectCanCancel(state: SkidbladnirUiState.FleetConnect): Boolean =
    state.mode == FleetConnectMode.Reconnect &&
        state.phase != FleetConnectPhase.Connecting && state.phase != FleetConnectPhase.ResetRequired

internal data class ForgeState(
    val form: ForgeForm,
    val pending: Boolean,
    val failure: ForgeFailure,
    val surface: ForgeSurface,
    val directorySearch: DirectorySearchState = DirectorySearchState.Idle,
)

internal fun ForgeState.admissibleSubmission(): ForgeDraft? =
    if (!pending && surface == ForgeSurface.Form) form.submission() else null


internal sealed interface ForgeRecovery {
    val draft: ForgeDraft
    data class RefreshRequired(override val draft: ForgeDraft) : ForgeRecovery
    data class ReviewReady(override val draft: ForgeDraft) : ForgeRecovery
}
internal data class CloseState(
    val machine: PairedMachine,
    val target: SessionTarget,
    val pending: Boolean,
    val terminalOnly: Boolean = false,
)

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

private fun forgeAuthoritySurvives(
    carry: ForgeCarry,
    currentCredentials: Collection<MachineCredential>,
    storedCredentials: Collection<MachineCredential>,
): Boolean {
    val handles = listOfNotNull(
        carry.forge?.form?.machineHandle,
        carry.recovery?.draft?.machineHandle,
    ).distinct()
    return handles.isNotEmpty() && handles.all { handle ->
        sameAuthority(
            currentCredentials.singleOrNull { it.machine.handle == handle },
            storedCredentials.singleOrNull { it.machine.handle == handle },
        )
    }
}

private fun machineAccessMessage(machine: MachineState): String = when (machine.access) {
    MachineAccess.Ready -> "${machine.machine.label.text}: reconnect required."
    MachineAccess.AuthRequired -> "${machine.machine.label.text}: authentication required."
    MachineAccess.IdentityChanged ->
        "${machine.machine.label.text}: machine identity changed. Fleet reset is required."
}

/**
 * The ui state machine over [Machines]: lifecycle, fleet connect and watch, dashboard and terminal
 * actions, access-loss routing and one derivation of machine facts into ui state ([publish]).
 * Main-confined. Every step does its machine operations first, then re-reads [state].
 */
internal class SkidbladnirController(
    context: Context,
    private val dashboardEntry: DashboardEntryState,
) {
    var state: SkidbladnirUiState by mutableStateOf(SkidbladnirUiState.Booting)
        private set
    internal var notificationHealth by mutableStateOf(NotificationHealth.SetupRequired)
        private set

    /** Whether any machine is installed; MainActivity reads this instead of decrypting the store on main. */
    val paired: Boolean get() = machines.all.isNotEmpty()

    private val applicationContext = context.applicationContext
    /** Controller lifetime: dispatched mutations, store operations and text size. */
    private val app = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    /** Serial store and hub work: a staged join and a connect never interleave. */
    private val fleetIO = Dispatchers.IO.limitedParallelism(1)
    /** One per foreground span, a child of [app]: observations, polls and the fleet watch die with it. */
    private var foreground: CoroutineScope? = null
    private val store = MachineStore(applicationContext)
    private val fleetClient = FleetClient(store)
    private val notifications = applicationContext.notificationOwner()
    private var notificationView = notifications.view.value
    private val machines: Machines = Machines(app, object : Machines.Listener {
        override fun changed() = publish()
        override fun inventoryReadStarting() = notifications.refresh()
        override fun inventoryRead(handle: MachineHandle) {
            advanceTerminalRename(handle)
            advanceGroupEditor(handle)
            dashboardEntry.resolveGroup(observedGroups(machines.all))
        }
        override fun accessLost(handle: MachineHandle, failure: GatewayFailure.Api) = machineAccessLost(handle, failure)
    })
    private val textSize = TerminalTextSize(applicationContext, app, ::publish)
    private var attachment: TerminalAttachment? = null
    /** The machine a dashboard stop is in flight on: that machine's access loss drops the stop's completion. */
    private var dashboardStop: MachineHandle? = null
    private var nextAttempt = 1
    private var nextPickerInstance = 1L
    private var directorySearch: Job? = null
    private var fleetWatch: Job? = null
    private var pendingFleetScan: String? = null
    private var pendingNotificationReference: String? = null
    private var pendingDashboardNotice: String? = null
    private var retainedForgeCarry: ForgeCarry? = null

    init {
        app.launch {
            notifications.view.collect { view ->
                notificationView = view
                notificationHealth = view.health
                publish()
            }
        }
        textSize.read()
    }

    fun start() {
        if (foreground != null) return
        val span = CoroutineScope(app.coroutineContext + Job(app.coroutineContext.job))
        foreground = span
        notifications.refresh()
        val connectState = state as? SkidbladnirUiState.FleetConnect
        if (connectState?.phase == FleetConnectPhase.Scanning) {
            machines.start(span)
            pendingFleetScan?.let { encoded ->
                pendingFleetScan = null
                acceptFleetScan(encoded)
            }
            return
        }
        val currentCredentials = machines.credentials()
        val carry = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> forgeCarry(current).takeUnless { it == ForgeCarry(null, null) }
            SkidbladnirUiState.Booting -> retainedForgeCarry
            is SkidbladnirUiState.FleetConnect, is SkidbladnirUiState.Terminal -> null
        }
        retainedForgeCarry = carry
        state = SkidbladnirUiState.Booting
        span.launch {
            val stored = withContext(fleetIO) {
                fleetAttempt { if (store.association()?.joined == false) fleetClient.resume() }
                store.read()
            }
            retainedForgeCarry = null
            machines.install(stored.credentials)
            machines.start(span)
            notifications.fleetChanged()
            if (stored.unreadable) {
                state = SkidbladnirUiState.FleetConnect(
                    mode = connectState?.mode ?: FleetConnectMode.Install,
                    phase = FleetConnectPhase.ResetRequired,
                )
                return@launch
            }
            if (stored.credentials.isEmpty()) {
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
                return@launch
            }
            if (connectState?.mode == FleetConnectMode.Reconnect) {
                state = connectState.copy(
                    phase = if (connectState.phase == FleetConnectPhase.Connecting) {
                        FleetConnectPhase.Failed
                    } else {
                        connectState.phase
                    },
                )
                return@launch
            }
            publishDashboard(carry = carry?.takeIf {
                forgeAuthoritySurvives(it, currentCredentials, stored.credentials)
            } ?: ForgeCarry(null, null))
            watchFleet()
        }
    }

    fun stopForBackground() {
        val span = foreground ?: return
        foreground = null
        span.cancel()
        fleetClient.stopWatching()
        machines.stop()
        leaveTerminal()
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> state = current.copy(
                refreshing = false,
                forge = current.forge?.let(::workingDirectoryPickerAfterForegroundInvalidation),
            )
            is SkidbladnirUiState.Terminal -> publishDashboard()
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> Unit
        }
    }

    fun close() {
        stopForBackground()
        pendingFleetScan = null
        app.cancel()
    }

    fun requestFleetScan() {
        val current = state as? SkidbladnirUiState.FleetConnect ?: return
        if (current.phase != FleetConnectPhase.Ready && current.phase != FleetConnectPhase.Failed) return
        state = current.copy(phase = FleetConnectPhase.Scanning)
    }

    fun requestFleetReconnect() {
        if (state !is SkidbladnirUiState.Dashboard) return
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
        val span = foreground
        if (span == null) {
            pendingFleetScan = encoded
            return
        }
        val invite = parseFleetInvite(encoded)
        if (invite == null) {
            state = current.copy(phase = FleetConnectPhase.Failed)
            return
        }
        val mode = current.mode
        state = current.copy(phase = FleetConnectPhase.Connecting)
        span.launch {
            val connected = withContext(fleetIO) { fleetAttempt { fleetClient.connect(invite) } }
            val active = state as? SkidbladnirUiState.FleetConnect ?: return@launch
            if (active.mode != mode || active.phase != FleetConnectPhase.Connecting) return@launch
            if (connected == null) {
                state = active.copy(phase = FleetConnectPhase.Failed)
                return@launch
            }
            machines.replace(connected)
            notifications.fleetChanged()
            publishDashboard()
            watchFleet()
        }
    }

    /** Hub membership revisions arrive on the event stream; each wake-up re-reads the committed store. */
    private fun watchFleet() {
        val span = foreground ?: return
        fleetWatch?.cancel()
        fleetClient.stopWatching()
        fleetWatch = span.launch(Dispatchers.IO) {
            var view = store.read().view
            while (isActive) {
                fleetClient.watch {
                    val stored = store.read()
                    val notificationChanged = stored.view != view
                    view = stored.view
                    span.launch { fleetChanged(stored, notificationChanged) }
                }
                delay(2_000)
            }
        }
    }

    private fun fleetChanged(stored: MachineStoreRead, notificationChanged: Boolean) {
        if (stored.unreadable) return
        val prior = machines.credentials()
        val next = stored.credentials
        if (next.toSet() == prior.toSet()) {
            if (notificationChanged) notifications.fleetChanged()
            return
        }
        val terminal = state as? SkidbladnirUiState.Terminal
        val terminalSurvives = terminal != null && sameAuthority(
            prior.singleOrNull { it.machine.handle == terminal.target.machineHandle },
            next.singleOrNull { it.machine.handle == terminal.target.machineHandle },
        )
        if (terminal != null && !terminalSurvives) leaveTerminal()
        machines.install(next)
        notifications.fleetChanged()
        val current = state
        when {
            next.isEmpty() -> state = SkidbladnirUiState.FleetConnect(FleetConnectMode.Install, FleetConnectPhase.Ready)
            terminal != null && !terminalSurvives -> publishDashboard()
            current is SkidbladnirUiState.Dashboard && !forgeAuthoritySurvives(forgeCarry(current), prior, next) ->
                state = current.copy(forge = null, forgeRecovery = null)
        }
    }

    fun verifyVisibleInventory() {
        notifications.refresh()
        if (state !is SkidbladnirUiState.Dashboard) return
        // Only machines that still own live polling can be refreshed; a machine whose access
        // failed says so instead of silently dropping the request.
        val requested = machines.refreshAll()
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        state = current.copy(
            refreshing = machines.refreshing,
            notice = if (requested) null else unrefreshableNotice(),
        )
    }

    fun restoreDashboardOnce(keys: List<DashboardItemKey>, noticeCount: Int) {
        if (dashboardEntry.restorationPending && foreground != null && machines.settled) {
            dashboardEntry.restoreOnce(keys, noticeCount)
        }
    }

    private fun unrefreshableNotice(): String {
        val all = machines.all
        if (all.isEmpty()) return "No fleet is connected."
        return all.joinToString(" ", transform = ::machineAccessMessage)
    }

    fun openForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (machines.all.none { it.canMutate } || current.groupEditor != null) return
        cancelDirectorySearch()
        state = current.copy(
            forge = ForgeState(
                ForgeForm(null, "", null, "", "", dashboardEntry.view.creationDraft()),
                pending = false,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
        )
    }

    fun browseWorkingDirectoryHome() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val machine = forge.form.machineHandle?.let { machines[it] } ?: return
        if (foreground == null) return
        val start = browseWorkingDirectoryHome(forge, machine, nextPickerInstance) ?: return
        nextPickerInstance += 1
        cancelDirectorySearch()
        state = current.copy(forge = forge.copy(directorySearch = DirectorySearchState.Idle))
        startWorkingDirectoryRequest(start)
    }

    fun openWorkingDirectoryChild(directory: HomeDirectory) {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(openWorkingDirectoryChild(picker, directory) ?: return)
    }

    fun openWorkingDirectoryParent() {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(openWorkingDirectoryParent(picker) ?: return)
    }

    fun retryWorkingDirectory() {
        val picker = activeWorkingDirectoryPicker() ?: return
        startWorkingDirectoryRequest(retryWorkingDirectory(picker) ?: return)
    }

    fun updateWorkingDirectoryFilter(filter: String) {
        updateWorkingDirectoryPicker { picker -> updateWorkingDirectoryFilter(picker, filter) }
    }

    fun setWorkingDirectoryHidden(showHidden: Boolean) {
        updateWorkingDirectoryPicker { picker -> setWorkingDirectoryHidden(picker, showHidden) }
    }

    fun focusWorkingDirectory() {
        val forge = (state as? SkidbladnirUiState.Dashboard)?.forge ?: return
        if (forge.directorySearch is DirectorySearchState.Loading || forge.directorySearch is DirectorySearchState.Ready) return
        scheduleDirectorySearch()
    }

    private fun scheduleDirectorySearch() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val span = foreground ?: return
        if (forge.pending || forge.surface != ForgeSurface.Form) return
        val input = classifyWorkingDirectory(forge.form.cwd) as? WorkingDirectoryInput.Query ?: return
        val handle = forge.form.machineHandle ?: return
        if (!machines.canMutate(handle)) return
        // Each edit gets a distinct snapshot; reference equality binds the debounce and the reply to it.
        val loading = DirectorySearchState.Loading()
        state = current.copy(forge = forge.copy(directorySearch = loading))
        directorySearch = span.launch {
            delay(150)
            val active = (state as? SkidbladnirUiState.Dashboard)?.forge ?: return@launch
            if (active.directorySearch !== loading || active.form.machineHandle != handle) return@launch
            if (!machines.canMutate(handle)) {
                updateForge { it.copy(directorySearch = DirectorySearchState.Idle) }
                return@launch
            }
            machines.request(handle, { GatewayClient.searchDirectories(it, input.terms) }) { result ->
                val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@request
                val currentForge = dashboard.forge ?: return@request
                if (currentForge.directorySearch !== loading || currentForge.form.machineHandle != handle) return@request
                state = dashboard.copy(forge = currentForge.copy(directorySearch = if (!machines.canMutate(handle)) {
                    DirectorySearchState.Idle
                } else when (result) {
                    is GatewayResult.Success -> DirectorySearchState.Ready(result.value)
                    is GatewayResult.Failure -> DirectorySearchState.Failed(result.failure)
                }))
            }
        }
    }

    private fun cancelDirectorySearch() {
        directorySearch?.cancel()
        directorySearch = null
    }

    fun chooseWorkingDirectory(directory: WorkingDirectoryPath): Boolean {
        val current = state as? SkidbladnirUiState.Dashboard ?: return false
        val forge = current.forge ?: return false
        val machine = forge.form.machineHandle?.let { machines[it] } ?: return false
        if (foreground == null || !machine.canMutate || forge.pending || forge.surface != ForgeSurface.Form) return false
        val literal = when (val input = classifyWorkingDirectory(forge.form.cwd)) {
            WorkingDirectoryInput.Home -> directory.encoded == "~"
            is WorkingDirectoryInput.Literal -> directory == input.path
            is WorkingDirectoryInput.Query, WorkingDirectoryInput.Invalid -> false
        }
        if (!literal && directory !in workingDirectoryChoices(forge, machine)) return false
        cancelDirectorySearch()
        state = current.copy(forge = forge.copy(
            form = forge.form.copy(cwd = directory.encoded),
            failure = forge.failure.afterWorkingDirectoryChoice(),
            directorySearch = DirectorySearchState.Idle,
        ))
        return true
    }

    fun useCurrentWorkingDirectory(): Boolean {
        val picker = activeWorkingDirectoryPicker() ?: return false
        if (foreground == null || machines[picker.machine.handle]?.canMutate != true) return false
        return updateForge(::useCurrentWorkingDirectory)?.surface == ForgeSurface.Form
    }

    fun workingDirectoryBack(): Boolean {
        if (activeWorkingDirectoryPicker() == null) return false
        return updateForge(::workingDirectoryBack)?.surface == ForgeSurface.Form
    }

    fun cancelWorkingDirectoryPicker(): Boolean {
        if (activeWorkingDirectoryPicker() == null) return false
        return updateForge(::cancelWorkingDirectoryPicker)?.surface == ForgeSurface.Form
    }

    private fun updateForge(transform: (ForgeState) -> ForgeState): ForgeState? {
        val current = state as? SkidbladnirUiState.Dashboard ?: return null
        val forge = current.forge ?: return null
        val updated = transform(forge)
        if (updated != forge) state = current.copy(forge = updated)
        return updated
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
        val handle = start.picker.machine.handle
        val active = forge.surface as? ForgeSurface.DirectoryPicker
        if (foreground == null || forge.pending || forge.form.machineHandle != handle || !machines.canMutate(handle) ||
            active != null && active.picker.instance != start.request.pickerInstance) return
        state = current.copy(forge = forge.copy(surface = ForgeSurface.DirectoryPicker(start.picker)))
        machines.request(handle, { GatewayClient.listDirectory(it, start.request.directory) }) { result ->
            val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@request
            val currentForge = dashboard.forge ?: return@request
            val surface = currentForge.surface as? ForgeSurface.DirectoryPicker ?: return@request
            val picker = completeWorkingDirectoryRequest(surface.picker, start.request, result) ?: return@request
            state = dashboard.copy(forge = currentForge.copy(surface = surface.copy(picker = picker)))
        }
    }

    fun resumeForgeRecovery() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val recovery = current.forgeRecovery as? ForgeRecovery.ReviewReady ?: return
        val target = current.machines.singleOrNull { it.machine.handle == recovery.draft.machineHandle } ?: return
        if (!target.canMutate) return
        state = current.copy(
            forge = ForgeState(
                ForgeForm(recovery.draft),
                pending = false,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
            forgeRecovery = null,
        )
    }

    fun dismissForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.forge?.pending != true) {
            cancelDirectorySearch()
            state = current.copy(forge = null)
        }
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
        if (proposed.machineHandle != null && machines[proposed.machineHandle] == null) return
        val updated = updateForgeState(forge, proposed)
        val directoryChanged = updated.form.machineHandle != forge.form.machineHandle || updated.form.cwd != forge.form.cwd
        if (directoryChanged) cancelDirectorySearch()
        state = current.copy(forge = updated)
        if (directoryChanged) scheduleDirectorySearch()
    }

    fun forge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val draft = forge.admissibleSubmission() ?: return
        val handle = draft.machineHandle
        if (!machines.canMutate(handle)) return
        state = current.copy(
            forge = forge.copy(
                pending = true,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
        )
        machines.mutate(handle, Reservation.Inventory, { GatewayClient.createSession(it, draft) }) { _, result ->
            machines.refresh(handle)
            val active = (state as? SkidbladnirUiState.Dashboard)?.forge ?: return@mutate
            if (!active.pending || active.form !== forge.form) return@mutate
            when (result) {
                is GatewayResult.Success -> enterCreatedTerminal(SessionTarget(handle, result.value))
                is GatewayResult.Failure -> {
                    val failure = result.failure
                    if (failure is GatewayFailure.Api && createFailureIsDefinitive(failure)) {
                        machines.settle(handle)
                        updateForge { it.copy(pending = false, failure = ForgeFailure.Definite(failure), surface = ForgeSurface.Form) }
                    } else {
                        machines.fail(handle, failure)
                        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@mutate
                        state = dashboard.copy(
                            forge = null,
                            forgeRecovery = ForgeRecovery.RefreshRequired(checkNotNull(active.form.submission())),
                        )
                    }
                }
            }
        }
    }

    fun openSourceTerminalForge() {
        val source = state as? SkidbladnirUiState.Terminal ?: return
        if (source.target.session.connection == null || source.shellPending ||
            source.close != null || source.rename != null ||
            !terminalActionAdmissible(source.machine.canMutate, source.connection)) return
        val handle = source.target.machineHandle
        if (machines[handle]?.canMutate != true) return
        leaveTerminal()
        publishDashboard(carry = ForgeCarry(
            ForgeState(
                ForgeForm(handle, "", LaunchChoice.Terminal, "", "",
                    GroupDraft.Chosen(source.target.session.group?.text.orEmpty())),
                pending = false,
                failure = ForgeFailure.None,
                surface = ForgeSurface.Form,
            ),
            null,
        ))
        verifyVisibleInventory()
    }

    fun newTerminalHere() {
        val source = state as? SkidbladnirUiState.Terminal ?: return
        if (source.shellPending || source.close != null || source.rename != null ||
            !terminalActionAdmissible(source.machine.canMutate, source.connection)) return
        val handle = source.target.machineHandle
        if (!machines.canMutate(handle)) return
        state = source.copy(shellPending = true)
        machines.mutate(handle, Reservation.Inventory, { GatewayClient.createShell(it, source.target) }) { _, result ->
            machines.refresh(handle)
            val current = state as? SkidbladnirUiState.Terminal ?: return@mutate
            if (current.attempt != source.attempt || !current.shellPending) return@mutate
            when (result) {
                is GatewayResult.Success -> enterCreatedTerminal(SessionTarget(handle, result.value))
                is GatewayResult.Failure -> {
                    val definite = createFailureIsDefinitive(result.failure)
                    if (definite) machines.settle(handle) else machines.fail(handle, result.failure)
                    val active = state as? SkidbladnirUiState.Terminal ?: return@mutate
                    state = active.copy(shellPending = false)
                    val message = if (definite) {
                        gatewayFailureMessage(result.failure)
                    } else "Terminal creation outcome unknown. Inspect the session collection before creating another."
                    android.widget.Toast.makeText(applicationContext, message, android.widget.Toast.LENGTH_LONG).show()
                }
            }
        }
    }

    fun openGroupEditor(target: SessionTarget) {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return
        if (dashboard.forge != null || dashboard.close != null || dashboard.groupEditor != null) return
        val machine = machines[target.machineHandle] ?: return
        if (!machine.canMutate) return
        val observed = machine.inventory.observedSession(target.session) ?: return
        state = dashboard.copy(groupEditor = beginGroupEdit(target.copy(session = observed)))
    }

    fun updateGroupDraft(draft: String) {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return
        val editor = dashboard.groupEditor ?: return
        state = dashboard.copy(groupEditor = editor.withDraft(draft))
    }

    fun dismissGroupEditor() {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return
        val editor = dashboard.groupEditor ?: return
        state = dashboard.copy(groupEditor = editor.dismissed())
    }

    fun submitGroup() {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return
        val editor = dashboard.groupEditor ?: return
        val handle = editor.target.machineHandle
        val machine = machines[handle] ?: return
        if (!editor.groupAdmits(machine) || !machines.canMutate(handle)) return
        val label = groupLabelOf(editor.draft)
        val sending = editor.copy(phase = EditPhase.Sending, error = null, labelRejection = null)
        state = dashboard.copy(groupEditor = sending)
        machines.mutate(handle, Reservation.Metadata, { GatewayClient.setSessionGroup(it, editor.target, label) }) { fence, result ->
            val reply = sending.afterReply(result)
            if (!reply.needsRead) machines.settleMetadata(handle, fence)
            val current = state as? SkidbladnirUiState.Dashboard
            if (current?.groupEditor == sending) state = current.copy(groupEditor = reply.edit)
            if (reply.needsRead) machines.refresh(handle)
        }
    }

    private fun advanceGroupEditor(handle: MachineHandle) {
        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return
        val editor = dashboard.groupEditor ?: return
        if (editor.target.machineHandle != handle) return
        // The caller has just published this machine's Fresh snapshot, so absence means the lifetime ended.
        val observed = (machines[handle] ?: return).inventory.observedSession(editor.target.session)
        state = dashboard.copy(
            groupEditor = editor.afterRead(observed),
            notice = if (observed == null) "that session lifetime is no longer available." else dashboard.notice,
        )
    }

    fun openTerminal(target: SessionTarget) {
        if ((state as? SkidbladnirUiState.Dashboard)?.groupEditor != null) return
        if (machines[target.machineHandle]?.canMutate != true) return
        enterTerminal(target.machineHandle, target.session)
    }

    /** The click names a lifetime; one lane read supplies the rich session the terminal's controls need. */
    fun openNotificationReference(ref: String) {
        if (state == SkidbladnirUiState.Booting || foreground == null) {
            pendingNotificationReference = ref
            return
        }
        val key = notificationReference(ref) ?: return
        val handle = MachineHandle(key.machine)
        val machine = machines[handle]?.machine
        if (machine == null || !machines.polled(handle)) {
            publishDashboard(notice = "that session's machine is unavailable.")
            return
        }
        val attempt = nextAttempt
        machines.read(handle) { result ->
            if (nextAttempt != attempt) return@read
            when (result) {
                is GatewayResult.Failure -> publishDashboard(notice = machineError(machine, result.failure))
                is GatewayResult.Success -> {
                    val session = result.value.sessions.singleOrNull { it.tmuxId == key.tmuxId && it.identityToken == key.identityToken }
                    if (session == null) publishDashboard(notice = "that session lifetime is no longer available.")
                    else enterTerminal(handle, session)
                }
            }
        }
    }

    /** Every entry starts Preparing; the gateway checks the session lifetime at attach. */
    private fun enterTerminal(handle: MachineHandle, session: TmuxSession) {
        val machine = machines[handle] ?: return
        leaveTerminal()
        textSize.sheet(open = false)
        state = SkidbladnirUiState.Terminal(
            machine = machine,
            machines = machines.all,
            target = SessionTarget(handle, session),
            attempt = nextAttempt++,
            connection = TerminalUiStatus.Preparing,
            viewport = TerminalViewport.Pending,
            textSize = textSize.state,
            close = null,
        )
        publish()
        // The saved size is loaded before the page exists; the page is created only once it is Ready.
        if (textSize.state == TerminalTextSizeState.Unavailable) textSize.read()
    }

    private fun enterCreatedTerminal(target: SessionTarget) {
        dashboardEntry.followCreatedMembership(target.session.group)
        enterTerminal(target.machineHandle, target.session)
    }

    fun terminalPageReady(attempt: Int, page: TerminalPage) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection != TerminalUiStatus.Preparing) return
        attachment = TerminalAttachment(page, notifications)
        state = current.copy(connection = TerminalUiStatus.Connecting)
        connectTerminalIfReady()
    }

    // WSS starts only with page Ready and a fitted grid, whichever lands last.
    private fun connectTerminalIfReady() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val owned = attachment ?: return
        if (current.connection != TerminalUiStatus.Connecting || owned.connection != null) return
        val viewport = current.viewport as? TerminalViewport.Fitted ?: return
        val handle = current.target.machineHandle
        // A fleet change that removes or re-authorizes this machine leaves the terminal first.
        val credential = checkNotNull(machines.credential(handle))
        owned.connect(credential, current.target, viewport, object : TerminalConnectionObserver {
            override fun onHello(attachedClients: Int, paneId: String, foreground: ForegroundObservation?) =
                connected(attachedClients)
            override fun onPresence(attachedClients: Int) = connected(attachedClients)
            override fun onFailure(code: ApiErrorCode) {
                if (attachment !== owned) return
                val failure = GatewayFailure.Api(code = code, message = apiErrorMessage(code))
                if (machines.accessFailure(handle, failure)) return
                val terminal = state as? SkidbladnirUiState.Terminal ?: return
                leaveTerminal()
                state = terminal.copy(connection = TerminalUiStatus.ReconnectRequired(apiErrorMessage(code)))
            }
            private fun connected(attachedClients: Int) {
                if (attachment !== owned) return
                val terminal = state as? SkidbladnirUiState.Terminal ?: return
                state = terminal.copy(connection = TerminalUiStatus.Connected(attachedClients))
            }
        })
    }

    fun resizeTerminal(attempt: Int, columns: Int, rows: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt) return
        state = current.copy(viewport = TerminalViewport.Fitted(columns, rows))
        val connection = attachment?.connection
        if (connection == null) connectTerminalIfReady() else connection.resize(columns, rows)
    }

    /** Output and automatic replies keep flowing; only user ingress waits for recovery. */
    fun viewportTooSmall(attempt: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt == attempt) state = current.copy(viewport = TerminalViewport.TooSmall)
    }
    fun terminalPageFailed(attempt: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt) return
        attachment?.connection?.terminalUnavailable()
        leaveTerminal()
        state = current.copy(connection = TerminalUiStatus.ReconnectRequired("Reconnect required."))
    }
    fun terminalPageDisposed(attempt: Int, page: TerminalPage) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || attachment?.page !== page) return
        leaveTerminal()
    }
    fun sendTerminal(attempt: Int, bytes: ByteArray) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection !is TerminalUiStatus.Connected) return
        attachment?.connection?.send(bytes)
    }
    fun sendTerminalAccessory(attempt: Int, accessory: TerminalAccessory) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt == attempt && terminalInputAdmissible(current.connection, current.viewport)) {
            attachment?.page?.sendAccessory(accessory)
        }
    }

    fun terminalOutputPresented(attempt: Int, page: TerminalPage) {
        val terminal = state as? SkidbladnirUiState.Terminal ?: return
        val owned = attachment ?: return
        if (foreground == null || terminal.attempt != attempt || owned.page !== page) return
        owned.present()
    }

    fun openTextSize() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.rename != null || current.close != null || terminalTextSizeSheet(textSize.state, current.connection) != null) return
        if (textSize.state !is TerminalTextSizeState.Ready || !terminalPageLive(current.connection)) return
        attachment?.page?.resetInputState()
        textSize.sheet(open = true)
    }

    fun dismissTextSize() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (terminalTextSizeSheet(textSize.state, current.connection) != null) textSize.sheet(open = false)
    }

    fun decreaseTextSize() = writeTextSize { it - 1 }

    fun increaseTextSize() = writeTextSize { it + 1 }

    fun resetTextSize() = writeTextSize { TERMINAL_TEXT_SIZE_DEFAULT_SP }

    fun retryTextSizeRead() {
        if (state is SkidbladnirUiState.Terminal && textSize.state == TerminalTextSizeState.Unavailable) textSize.read()
    }

    private fun writeTextSize(next: (committedSp: Int) -> Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val ready = textSize.state as? TerminalTextSizeState.Ready ?: return
        val nominalSp = next(ready.nominalSp)
        if (!terminalTextSizeStepAdmissible(ready, current.connection, nominalSp)) return
        textSize.write(nominalSp) { attachment?.page?.setTextSize(it) }
    }

    fun openRename() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.rename != null || current.close != null || terminalTextSizeSheet(current.textSize, current.connection) != null ||
            !terminalActionAdmissible(current.machine.canMutate, current.connection)
        ) return
        attachment?.page?.resetInputState()
        state = current.copy(rename = beginRename(current.target))
    }

    fun updateRenameDraft(draft: String) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        val updated = rename.withDraft(draft)
        if (updated != rename) state = current.copy(rename = updated)
    }

    fun dismissRename() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        val updated = rename.dismissed()
        if (updated != rename) state = current.copy(rename = updated)
    }

    fun submitRename() {
        val rename = (state as? SkidbladnirUiState.Terminal)?.rename ?: return
        submitNaming(SessionNaming.Manual(rename.draft))
    }

    fun useAutomaticTitle() = submitNaming(SessionNaming.Automatic)

    private fun submitNaming(naming: SessionNaming) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val rename = current.rename ?: return
        if (!rename.renameAdmits(naming, current.target, terminalActionAdmissible(current.machine.canMutate, current.connection))) return
        val sending = rename.sendingRename(naming)
        val handle = current.target.machineHandle
        if (!machines.canMutate(handle)) return
        val attempt = current.attempt
        state = current.copy(rename = sending)
        machines.mutate(handle, Reservation.Metadata, {
            GatewayClient.renameSession(it, sending.target, sending.expectedNaming, naming)
        }) { fence, result ->
            val reply = sending.afterReply(result)
            if (!reply.needsRead) machines.settleMetadata(handle, fence)
            val terminal = state as? SkidbladnirUiState.Terminal
            if (terminal?.attempt == attempt && terminal.rename == sending) state = terminal.copy(rename = reply.edit)
            if (reply.needsRead) machines.refresh(handle)
        }
    }

    /** No inventory read: the gateway checks the lifetime at attach, and the adopted session is the latest observed one. */
    fun reattachTerminal() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.close?.pending == true || !terminalActionAdmissible(current.machine.canMutate, current.connection)) return
        enterTerminal(
            current.target.machineHandle,
            current.machine.inventory.observedSession(current.target.session) ?: current.target.session,
        )
    }

    fun detachToSessions() {
        val presented = attachment?.presented == true
        leaveTerminal()
        publishDashboard()
        if (!presented) verifyVisibleInventory()
    }

    fun stopTerminal(target: SessionTarget) {
        val handle = target.machineHandle
        val machine = machines[handle] ?: return
        if (!machines.canMutate(handle)) return
        val current = state
        state = when (current) {
            is SkidbladnirUiState.Dashboard -> {
                if (current.terminalControlPending || current.close != null || current.groupEditor != null || current.forge != null) return
                dashboardStop = handle
                current.copy(terminalControlPending = true)
            }
            is SkidbladnirUiState.Terminal -> {
                if (current.terminalControlPending || current.close != null || current.rename != null ||
                    !terminalActionAdmissible(machine.canMutate, current.connection) || current.target.machineHandle != target.machineHandle ||
                    current.target.session.identityToken != target.session.identityToken) return
                attachment?.page?.resetInputState()
                current.copy(terminalControlPending = true)
            }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> return
        }
        machines.mutate(handle, Reservation.None, { GatewayClient.stopTerminal(it, target) }) { _, result ->
            state = when (val latest = state) {
                is SkidbladnirUiState.Dashboard -> latest.copy(terminalControlPending = false)
                is SkidbladnirUiState.Terminal -> if (current is SkidbladnirUiState.Terminal && latest.attempt == current.attempt) latest.copy(terminalControlPending = false) else latest
                SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> latest
            }
            val message = when (result) {
                is GatewayResult.Success -> terminalStopMessage(result.value)
                is GatewayResult.Failure -> if (result.failure is GatewayFailure.Api && result.failure.dispatch == MutationDispatch.NotSent) {
                    machineError(machine.machine, result.failure)
                } else TERMINAL_INPUT_UNKNOWN
            }
            android.widget.Toast.makeText(applicationContext, message, android.widget.Toast.LENGTH_LONG).show()
            machines.refresh(handle)
        }
    }

    fun requestClose(target: SessionTarget) = prepareClose(target, terminalOnly = false)

    fun requestTerminalClose(target: SessionTarget) = prepareClose(target, terminalOnly = true)

    private fun prepareClose(target: SessionTarget, terminalOnly: Boolean) {
        val machine = machines[target.machineHandle] ?: return
        val close = CloseState(machine.machine, target, false, terminalOnly)
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (machine.canMutate && current.groupEditor == null && current.close == null && !current.terminalControlPending) current.copy(close = close) else return
            is SkidbladnirUiState.Terminal ->
                if (current.close == null && !current.terminalControlPending && current.rename == null && terminalTextSizeSheet(current.textSize, current.connection) == null &&
                    terminalActionAdmissible(machine.canMutate, current.connection)
                ) {
                    attachment?.page?.resetInputState()
                    current.copy(close = close)
                } else {
                    return
                }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> return
        }
    }

    fun dismissClose() {
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (current.close?.pending == true) current else current.copy(close = null)
            is SkidbladnirUiState.Terminal -> if (current.close?.pending == true) current else current.copy(close = null)
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> current
        }
    }

    fun confirmClose() {
        val current = state
        val close = when (current) {
            is SkidbladnirUiState.Dashboard -> current.close
            is SkidbladnirUiState.Terminal -> current.close
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> null
        } ?: return
        if (close.pending) return
        val handle = close.target.machineHandle
        val machine = machines[handle] ?: return
        if (!machines.canMutate(handle)) return
        val pending = close.copy(pending = true)
        state = when (current) {
            is SkidbladnirUiState.Dashboard -> current.copy(close = pending)
            is SkidbladnirUiState.Terminal ->
                if (terminalActionAdmissible(machine.canMutate, current.connection)) {
                    leaveTerminal()
                    current.copy(close = pending)
                } else {
                    return
                }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> return
        }
        // Terminal-only closure has no interrupt half, so its success carries no result.
        machines.mutate<TerminalCloseResult?>(handle, Reservation.Inventory, { credential ->
            if (close.terminalOnly) {
                when (val closed = GatewayClient.closeTerminal(credential, close.target)) {
                    is GatewayResult.Success -> GatewayResult.Success(null)
                    is GatewayResult.Failure -> closed
                }
            } else GatewayClient.interruptAndCloseTerminal(credential, close.target)
        }) { _, result ->
            leaveTerminal()
            when (result) {
                is GatewayResult.Success -> {
                    val closed = result.value?.terminal ?: TerminalClosure.Closed
                    val interrupt = result.value?.interrupt
                    if (closed == TerminalClosure.Closed) machines.forget(close.target)
                    machines.refresh(handle)
                    val message = result.value?.let(::terminalCloseMessage) ?: "terminal closed. $TERMINAL_SHARED_WORK"
                    val partial = closed != TerminalClosure.Closed || interrupt != null && interrupt != TerminalInterrupt.Written
                    publishDashboard(notice = if (partial) message else null)
                    if (!partial) {
                        android.widget.Toast.makeText(applicationContext, message, android.widget.Toast.LENGTH_LONG).show()
                    }
                }
                is GatewayResult.Failure -> {
                    val failure = result.failure
                    // Definite: never dispatched, or rejected before any effect. Access codes never reach here.
                    val definite = failure is GatewayFailure.Api && (failure.dispatch == MutationDispatch.NotSent ||
                        failure.code in setOf(
                            ApiErrorCode.InvalidRequest,
                            ApiErrorCode.RequestTooLarge,
                            ApiErrorCode.SessionNotFound,
                            ApiErrorCode.SessionIdentityMismatch,
                        ))
                    machines.fail(handle, failure)
                    machines.refresh(handle)
                    publishDashboard(notice = if (definite) machineError(close.machine, failure) else TERMINAL_CLOSE_UNKNOWN)
                }
            }
        }
    }

    private fun advanceTerminalRename(handle: MachineHandle) {
        val terminal = state as? SkidbladnirUiState.Terminal ?: return
        if (terminal.target.machineHandle != handle) return
        // The caller has just published this machine's Fresh snapshot, so absence means the lifetime ended.
        val observed = terminal.machine.inventory.observedSession(terminal.target.session)
        if (observed == null) {
            if (terminal.rename == null) return
            leaveTerminal()
            state = terminal.copy(rename = null, connection = TerminalUiStatus.ReconnectRequired(
                "${terminal.machine.machine.label.text}: that session lifetime is no longer available."))
            return
        }
        state = terminal.copy(target = terminal.target.copy(session = observed), rename = terminal.rename?.afterRead(observed))
    }

    /** The machine is already downgraded and unpolled; this routes the ui away from it. */
    private fun machineAccessLost(handle: MachineHandle, failure: GatewayFailure.Api) {
        val machine = machines[handle] ?: return
        val message = machineAccessMessage(machine)
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> {
                val affectedForge = current.forge?.takeIf { it.form.machineHandle == handle }
                if (affectedForge != null) cancelDirectorySearch()
                state = current.copy(
                    notice = message,
                    forge = affectedForge?.copy(
                        pending = false,
                        failure = ForgeFailure.Definite(failure),
                        surface = ForgeSurface.Form,
                        directorySearch = DirectorySearchState.Idle,
                    ) ?: current.forge,
                    close = current.close?.takeUnless { it.target.machineHandle == handle },
                    groupEditor = current.groupEditor?.takeUnless { it.target.machineHandle == handle },
                    terminalControlPending = current.terminalControlPending && dashboardStop != handle,
                )
            }
            is SkidbladnirUiState.Terminal -> if (current.target.machineHandle == handle) {
                leaveTerminal()
                dashboardEntry.resetViewportAfterAccessLoss()
                state = SkidbladnirUiState.Dashboard(
                    machines = machines.all.map(::projected),
                    refreshing = machines.refreshing,
                    notice = message,
                    needsInputSettled = machines.settled,
                    forge = null,
                    forgeRecovery = null,
                    close = null,
                )
            } else {
                pendingDashboardNotice = message
            }
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> pendingDashboardNotice = message
        }
    }

    /** The one derivation of machine facts, notification projections and text size into the visible state. */
    private fun publish() {
        val all = machines.all.map(::projected)
        state = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> {
                // A draft whose outcome was unknown becomes reviewable once its machine reads fresh.
                val recovery = current.forgeRecovery
                val recoveryMachine = recovery?.let { r -> all.singleOrNull { it.machine.handle == r.draft.machineHandle } }
                current.copy(
                    machines = all,
                    refreshing = machines.refreshing,
                    needsInputSettled = machines.settled,
                    forgeRecovery = if (recovery is ForgeRecovery.RefreshRequired && recoveryMachine?.inventory is InventoryState.Fresh) {
                        ForgeRecovery.ReviewReady(recovery.draft)
                    } else recovery,
                )
            }
            is SkidbladnirUiState.Terminal -> all.firstOrNull { it.machine.handle == current.target.machineHandle }?.let { machine ->
                // Never retarget a pane: only the same lifetime on the same pane adopts the fresh session.
                val session = (machine.inventory as? InventoryState.Fresh)?.snapshot?.inventory?.sessions?.singleOrNull {
                    it.sameLifetime(current.target.session) && it.activePaneId == current.target.session.activePaneId
                }
                current.copy(
                    machine = machine,
                    machines = all,
                    target = session?.let { current.target.copy(session = it) } ?: current.target,
                    textSize = textSize.state,
                )
            } ?: current
            SkidbladnirUiState.Booting, is SkidbladnirUiState.FleetConnect -> current
        }
    }

    /** Every installed machine projection pairs its current facts with its admitted attention outcome. */
    private fun projected(machine: MachineState): MachineState {
        val unavailable = notificationView.state == null
        return machine.copy(notifications = machine.inventory.lastSnapshot()?.inventory?.sessions.orEmpty().associate { session ->
            val key = NotificationKey(SessionTarget(machine.machine.handle, session))
            key to NotificationPresentation(
                ready = !unavailable && machine.canMutate && notificationView.presentsReady(key, session),
                unavailable = session.agent != null && unavailable,
            )
        })
    }

    private fun publishDashboard(
        notice: String? = null,
        carry: ForgeCarry = forgeCarry(state),
    ) {
        val dashboardNotice = notice ?: pendingDashboardNotice
        pendingDashboardNotice = null
        state = SkidbladnirUiState.Dashboard(
            machines = machines.all.map(::projected),
            refreshing = machines.refreshing,
            needsInputSettled = machines.settled,
            notice = dashboardNotice,
            forge = carry.forge,
            forgeRecovery = carry.recovery,
            close = null,
        )
        pendingNotificationReference?.let { ref ->
            pendingNotificationReference = null
            openNotificationReference(ref)
        }
    }

    private fun leaveTerminal() {
        attachment?.detach()
        attachment = null
    }

    private fun machineError(machine: PairedMachine, failure: GatewayFailure): String =
        "${machine.label.text}: ${gatewayFailureMessage(failure)}"
}
