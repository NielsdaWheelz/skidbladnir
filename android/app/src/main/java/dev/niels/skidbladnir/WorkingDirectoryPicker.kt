package dev.niels.skidbladnir

import java.util.Locale

private const val MAXIMUM_WORKING_DIRECTORY_FILTER_SCALARS = 256
private const val MAXIMUM_WORKING_DIRECTORY_HISTORY = 32

internal sealed interface ForgeFailure {
    data object None : ForgeFailure
    data class Definite(val rejection: GatewayFailure.Api) : ForgeFailure
}

internal sealed interface ForgeSurface {
    data object Form : ForgeSurface
    data class DirectoryPicker(val picker: WorkingDirectoryPickerState) : ForgeSurface
}

internal data class WorkingDirectoryPickerState(
    val instance: Long,
    val machine: PairedMachine,
    val machineSummary: MachineSummary,
    val load: DirectoryLoad,
    val history: List<DirectoryView>,
    val nextSequence: Long,
)

internal data class DirectoryView(
    val listing: DirectoryListing,
    val filter: String,
    val showHidden: Boolean,
    val viewport: DirectoryViewport,
)

internal sealed interface DirectoryViewport {
    data object Top : DirectoryViewport
    data class Anchor(val directory: HomeDirectory, val offset: Int) : DirectoryViewport {
        init {
            require(offset >= 0)
        }
    }
}

internal sealed interface RetainedDirectoryView {
    data object None : RetainedDirectoryView
    data class Present(val view: DirectoryView) : RetainedDirectoryView
}

internal sealed interface DirectoryLoad {
    data class Loading(
        val sequence: Long,
        val candidate: HomeDirectory,
        val retained: RetainedDirectoryView,
    ) : DirectoryLoad

    data class Loaded(val view: DirectoryView) : DirectoryLoad

    data class Failed(
        val candidate: HomeDirectory,
        val retained: RetainedDirectoryView,
        val failure: DirectoryBrowseFailure,
    ) : DirectoryLoad
}

internal enum class DirectoryBrowseFailure { Transport, Unavailable, TooLarge, Internal }

internal data class WorkingDirectoryRequest(
    val generation: Long,
    val machine: MachineSummary,
    val pickerInstance: Long,
    val sequence: Long,
    val directory: HomeDirectory,
)

internal data class WorkingDirectoryRequestStart(
    val picker: WorkingDirectoryPickerState,
    val request: WorkingDirectoryRequest,
)

internal sealed interface WorkingDirectoryCompletion {
    data object Ignored : WorkingDirectoryCompletion
    data class Updated(val picker: WorkingDirectoryPickerState) : WorkingDirectoryCompletion
    data class AccessLost(val failure: GatewayFailure.Api) : WorkingDirectoryCompletion
}

internal fun browseWorkingDirectoryHome(
    forge: ForgeState,
    machine: MachineState,
    pickerInstance: Long,
    generation: Long,
): WorkingDirectoryRequestStart? {
    if (forge.pending || forge.surface != ForgeSurface.Form || !machine.canForge ||
        forge.form.machineHandle != machine.machine.handle) return null
    // canForge guarantees Fresh; Kotlin cannot narrow through the derived property.
    val inventory = (machine.inventory as InventoryState.Fresh).snapshot.inventory
    return WorkingDirectoryRequestStart(
        picker = WorkingDirectoryPickerState(
            instance = pickerInstance,
            machine = machine.machine,
            machineSummary = inventory.machine,
            load = DirectoryLoad.Loading(1, HomeDirectory.Home, RetainedDirectoryView.None),
            history = emptyList(),
            nextSequence = 2,
        ),
        request = WorkingDirectoryRequest(
            generation = generation,
            machine = inventory.machine,
            pickerInstance = pickerInstance,
            sequence = 1,
            directory = HomeDirectory.Home,
        ),
    )
}

internal fun openWorkingDirectoryChild(
    picker: WorkingDirectoryPickerState,
    directory: HomeDirectory,
    generation: Long,
): WorkingDirectoryRequestStart? {
    val view = picker.actionableDirectoryView() ?: return null
    if (view.listing.children.none { entry -> entry.directory == directory }) return null
    return beginWorkingDirectoryRequest(
        picker,
        directory,
        RetainedDirectoryView.Present(view),
        generation,
    )
}

internal fun openWorkingDirectoryParent(
    picker: WorkingDirectoryPickerState,
    generation: Long,
): WorkingDirectoryRequestStart? {
    val view = picker.actionableDirectoryView() ?: return null
    val parent = view.listing.parent as? ParentDirectory.Available ?: return null
    return beginWorkingDirectoryRequest(
        picker,
        parent.directory,
        RetainedDirectoryView.Present(view),
        generation,
    )
}

internal fun retryWorkingDirectory(
    picker: WorkingDirectoryPickerState,
    generation: Long,
): WorkingDirectoryRequestStart? {
    val failed = picker.load as? DirectoryLoad.Failed
        ?: return null
    return beginWorkingDirectoryRequest(picker, failed.candidate, failed.retained, generation)
}

private fun beginWorkingDirectoryRequest(
    picker: WorkingDirectoryPickerState,
    directory: HomeDirectory,
    retained: RetainedDirectoryView,
    generation: Long,
): WorkingDirectoryRequestStart {
    check(picker.nextSequence < Long.MAX_VALUE)
    val sequence = picker.nextSequence
    return WorkingDirectoryRequestStart(
        picker = picker.copy(
            load = DirectoryLoad.Loading(sequence, directory, retained),
            nextSequence = sequence + 1,
        ),
        request = WorkingDirectoryRequest(
            generation = generation,
            machine = picker.machineSummary,
            pickerInstance = picker.instance,
            sequence = sequence,
            directory = directory,
        ),
    )
}

internal fun completeWorkingDirectoryRequest(
    picker: WorkingDirectoryPickerState,
    request: WorkingDirectoryRequest,
    foregroundGeneration: Long?,
    result: GatewayResult<DirectoryListing>,
): WorkingDirectoryCompletion {
    val loading = picker.load as? DirectoryLoad.Loading
    if (
        foregroundGeneration != request.generation ||
        picker.machineSummary != request.machine ||
        picker.instance != request.pickerInstance ||
        picker.nextSequence != request.sequence + 1 ||
        loading?.sequence != request.sequence ||
        loading.candidate != request.directory
    ) {
        return WorkingDirectoryCompletion.Ignored
    }
    return when (result) {
        is GatewayResult.Success -> {
            val listing = result.value
            if (listing.machine != request.machine || listing.directory != request.directory) {
                throw ProtocolDecodeException("directory-listing response identity")
            }
            val history = when (val retained = loading.retained) {
                RetainedDirectoryView.None -> picker.history
                is RetainedDirectoryView.Present ->
                    (picker.history + retained.view).takeLast(MAXIMUM_WORKING_DIRECTORY_HISTORY)
            }
            WorkingDirectoryCompletion.Updated(
                picker.copy(
                    load = DirectoryLoad.Loaded(
                        DirectoryView(
                            listing = listing,
                            filter = "",
                            showHidden = false,
                            viewport = DirectoryViewport.Top,
                        ),
                    ),
                    history = history,
                ),
            )
        }
        is GatewayResult.Failure -> when (val failure = result.failure) {
            GatewayFailure.Transport -> failedWorkingDirectoryCompletion(
                picker,
                loading,
                DirectoryBrowseFailure.Transport,
            )
            is GatewayFailure.Api -> when (failure.code) {
                ApiErrorCode.Unauthenticated,
                ApiErrorCode.MachineIdentityMismatch,
                -> WorkingDirectoryCompletion.AccessLost(failure)
                ApiErrorCode.DirectoryListingUnavailable -> failedWorkingDirectoryCompletion(
                    picker,
                    loading,
                    DirectoryBrowseFailure.Unavailable,
                )
                ApiErrorCode.DirectoryListingTooLarge -> failedWorkingDirectoryCompletion(
                    picker,
                    loading,
                    DirectoryBrowseFailure.TooLarge,
                )
                ApiErrorCode.InternalError -> failedWorkingDirectoryCompletion(
                    picker,
                    loading,
                    DirectoryBrowseFailure.Internal,
                )
                ApiErrorCode.InvalidRequest,
                ApiErrorCode.RequestTooLarge,
                ApiErrorCode.WorkingDirectoryInvalid,
                ApiErrorCode.WorkingDirectoryUnavailable,
                ApiErrorCode.DirectorySearchUnavailable,
                ApiErrorCode.DirectorySearchTooLarge,
                ApiErrorCode.TerminalContextUnavailable,
        ApiErrorCode.TerminalTargetChanged, ApiErrorCode.TerminalUnavailable, ApiErrorCode.TerminalInputBlocked,
                ApiErrorCode.ProfileUnknown,
                ApiErrorCode.SessionNameInvalid,
                ApiErrorCode.ObjectiveInvalid, ApiErrorCode.GroupInvalid,
                ApiErrorCode.SessionNameConflict, ApiErrorCode.SessionNameChanged,
                ApiErrorCode.SessionNotFound,
                ApiErrorCode.SessionIdentityMismatch,
                ApiErrorCode.PairingInviteRejected,
                ApiErrorCode.ReconnectRequired,
                ApiErrorCode.TerminalConfigurationUnsupported,
                ApiErrorCode.AgentTargetStale,
                ApiErrorCode.AgentUnavailable, ApiErrorCode.AgentInputInvalid, ApiErrorCode.HistoryChanged,
                -> throw ProtocolDecodeException("directory-listing completion error set")
            }
        }
    }
}

private fun failedWorkingDirectoryCompletion(
    picker: WorkingDirectoryPickerState,
    loading: DirectoryLoad.Loading,
    failure: DirectoryBrowseFailure,
): WorkingDirectoryCompletion = WorkingDirectoryCompletion.Updated(
    picker.copy(
        load = DirectoryLoad.Failed(loading.candidate, loading.retained, failure),
    ),
)

internal fun updateWorkingDirectoryFilter(
    picker: WorkingDirectoryPickerState,
    filter: String,
): WorkingDirectoryPickerState {
    if (filter.codePointCount(0, filter.length) > MAXIMUM_WORKING_DIRECTORY_FILTER_SCALARS) return picker
    return picker.updateVisibleDirectoryView { view ->
        view.copy(filter = filter).withVisibleViewport()
    }
}

internal fun setWorkingDirectoryHidden(
    picker: WorkingDirectoryPickerState,
    showHidden: Boolean,
): WorkingDirectoryPickerState = picker.updateVisibleDirectoryView { view ->
    view.copy(showHidden = showHidden).withVisibleViewport()
}

private fun DirectoryView.withVisibleViewport(): DirectoryView {
    val anchor = viewport as? DirectoryViewport.Anchor ?: return this
    return if (visibleWorkingDirectoryEntries(this).any { it.directory == anchor.directory }) {
        this
    } else {
        copy(viewport = DirectoryViewport.Top)
    }
}

internal fun updateWorkingDirectoryViewport(
    picker: WorkingDirectoryPickerState,
    viewport: DirectoryViewport,
): WorkingDirectoryPickerState = picker.updateVisibleDirectoryView { view ->
    if (viewport is DirectoryViewport.Anchor &&
        view.listing.children.none { entry -> entry.directory == viewport.directory }
    ) {
        view
    } else {
        view.copy(viewport = viewport)
    }
}

private fun WorkingDirectoryPickerState.updateVisibleDirectoryView(
    transform: (DirectoryView) -> DirectoryView,
): WorkingDirectoryPickerState {
    val updated = when (val load = load) {
        is DirectoryLoad.Loaded -> load.copy(view = transform(load.view))
        is DirectoryLoad.Loading -> load.copy(retained = load.retained.map(transform))
        is DirectoryLoad.Failed -> load.copy(retained = load.retained.map(transform))
    }
    return copy(load = updated)
}

private fun RetainedDirectoryView.map(
    transform: (DirectoryView) -> DirectoryView,
): RetainedDirectoryView = when (this) {
    RetainedDirectoryView.None -> this
    is RetainedDirectoryView.Present -> copy(view = transform(view))
}

internal fun visibleWorkingDirectoryEntries(view: DirectoryView): List<DirectoryEntry> {
    val visible = view.listing.children.withIndex().filter { indexed ->
        view.showHidden || !indexed.value.directory.hidden
    }
    if (view.filter.isEmpty()) return visible.map { indexed -> indexed.value }
    val query = view.filter.lowercase(Locale.ROOT)
    return visible.mapNotNull { indexed ->
        val basename = indexed.value.directory.basename.lowercase(Locale.ROOT)
        directoryMatchRank(basename, query)?.let { rank ->
            RankedDirectoryEntry(
                entry = indexed.value,
                rank = rank,
                basenameScalars = indexed.value.directory.basename.codePointCount(
                    0,
                    indexed.value.directory.basename.length,
                ),
                serverIndex = indexed.index,
            )
        }
    }.sortedWith(
        compareBy<RankedDirectoryEntry> { it.rank }
            .thenBy { it.basenameScalars }
            .thenBy { it.serverIndex },
    ).map(RankedDirectoryEntry::entry)
}

internal fun workingDirectoryHasHiddenEntries(view: DirectoryView): Boolean =
    view.listing.children.any { entry -> entry.directory.hidden }

private data class RankedDirectoryEntry(
    val entry: DirectoryEntry,
    val rank: Int,
    val basenameScalars: Int,
    val serverIndex: Int,
)

private fun directoryMatchRank(candidate: String, query: String): Int? = when {
    candidate == query -> 0
    candidate.startsWith(query) -> 1
    candidate.contains(query) -> 2
    candidate.containsOrderedSubsequence(query) -> 3
    else -> null
}

private fun String.containsOrderedSubsequence(query: String): Boolean {
    val candidatePoints = codePoints().toArray()
    val queryPoints = query.codePoints().toArray()
    var candidateIndex = 0
    for (queryPoint in queryPoints) {
        while (candidateIndex < candidatePoints.size && candidatePoints[candidateIndex] != queryPoint) {
            candidateIndex++
        }
        if (candidateIndex == candidatePoints.size) return false
        candidateIndex++
    }
    return true
}

internal fun workingDirectoryPickerAfterForegroundInvalidation(forge: ForgeState): ForgeState {
    val cleared = forge.copy(directorySearch = DirectorySearchState.Idle)
    val surface = cleared.surface as? ForgeSurface.DirectoryPicker ?: return cleared
    val loading = surface.picker.load as? DirectoryLoad.Loading ?: return cleared
    return when (val retained = loading.retained) {
        RetainedDirectoryView.None -> cleared.copy(surface = ForgeSurface.Form)
        is RetainedDirectoryView.Present -> cleared.copy(surface = surface.copy(
            picker = surface.picker.copy(load = DirectoryLoad.Loaded(retained.view)),
        ))
    }
}

internal fun workingDirectoryBack(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val picker = surface.picker
    val retained = when (val load = picker.load) {
        is DirectoryLoad.Loading -> load.retained
        is DirectoryLoad.Failed -> load.retained
        is DirectoryLoad.Loaded -> {
            if (picker.history.isEmpty()) return forge.copy(surface = ForgeSurface.Form)
            return forge.copy(surface = surface.copy(picker = picker.copy(
                load = DirectoryLoad.Loaded(picker.history.last()),
                history = picker.history.dropLast(1),
            )))
        }
    }
    return when (retained) {
        RetainedDirectoryView.None -> forge.copy(surface = ForgeSurface.Form)
        is RetainedDirectoryView.Present -> forge.copy(surface = surface.copy(
            picker = picker.copy(load = DirectoryLoad.Loaded(retained.view)),
        ))
    }
}

internal fun cancelWorkingDirectoryPicker(forge: ForgeState): ForgeState =
    if (forge.surface is ForgeSurface.DirectoryPicker) forge.copy(surface = ForgeSurface.Form) else forge

internal fun useCurrentWorkingDirectory(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val view = surface.picker.actionableDirectoryView() ?: return forge
    val path = checkNotNull(WorkingDirectoryPath.parse(view.listing.directory.encoded))
    if (forge.form.machineHandle != surface.picker.machine.handle) return forge
    return forge.copy(
        form = forge.form.copy(cwd = path.encoded),
        failure = forge.failure.afterWorkingDirectoryChoice(),
        surface = ForgeSurface.Form,
        directorySearch = DirectorySearchState.Idle,
    )
}

internal fun ForgeFailure.isWorkingDirectoryRejection(): Boolean = when (this) {
    ForgeFailure.None -> false
    is ForgeFailure.Definite -> when (rejection.code) {
        ApiErrorCode.WorkingDirectoryInvalid,
        ApiErrorCode.WorkingDirectoryUnavailable,
        -> true
        ApiErrorCode.Unauthenticated,
        ApiErrorCode.InvalidRequest,
        ApiErrorCode.RequestTooLarge,
        ApiErrorCode.DirectoryListingUnavailable,
        ApiErrorCode.DirectoryListingTooLarge,
        ApiErrorCode.DirectorySearchUnavailable,
        ApiErrorCode.DirectorySearchTooLarge,
        ApiErrorCode.TerminalContextUnavailable,
        ApiErrorCode.TerminalTargetChanged, ApiErrorCode.TerminalUnavailable, ApiErrorCode.TerminalInputBlocked,
        ApiErrorCode.ProfileUnknown,
        ApiErrorCode.SessionNameInvalid,
        ApiErrorCode.ObjectiveInvalid, ApiErrorCode.GroupInvalid,
        ApiErrorCode.SessionNameConflict, ApiErrorCode.SessionNameChanged,
        ApiErrorCode.SessionNotFound,
        ApiErrorCode.SessionIdentityMismatch,
        ApiErrorCode.PairingInviteRejected,
        ApiErrorCode.MachineIdentityMismatch,
        ApiErrorCode.InternalError,
        ApiErrorCode.ReconnectRequired,
        ApiErrorCode.TerminalConfigurationUnsupported,
        ApiErrorCode.AgentTargetStale,
        ApiErrorCode.AgentUnavailable, ApiErrorCode.AgentInputInvalid, ApiErrorCode.HistoryChanged,
        -> false
    }
}

internal fun ForgeFailure.afterWorkingDirectoryChoice(): ForgeFailure =
    if (isWorkingDirectoryRejection()) ForgeFailure.None else this

internal fun updateForgeState(forge: ForgeState, proposed: ForgeForm): ForgeState {
    val machineChanged = proposed.machineHandle != forge.form.machineHandle
    val form = changeForgeDraft(forge.form, proposed)
    return forge.copy(
        form = form,
        failure = ForgeFailure.None,
        surface = if (machineChanged) ForgeSurface.Form else forge.surface,
        directorySearch = if (machineChanged || form.cwd != forge.form.cwd) DirectorySearchState.Idle else forge.directorySearch,
    )
}

private fun WorkingDirectoryPickerState.actionableDirectoryView(): DirectoryView? = when (val load = load) {
    is DirectoryLoad.Loading -> null
    is DirectoryLoad.Loaded -> load.view
    is DirectoryLoad.Failed -> when (val retained = load.retained) {
        RetainedDirectoryView.None -> null
        is RetainedDirectoryView.Present -> retained.view
    }
}
