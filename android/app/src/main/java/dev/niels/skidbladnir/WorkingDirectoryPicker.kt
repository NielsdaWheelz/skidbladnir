package dev.niels.skidbladnir

import androidx.compose.foundation.lazy.LazyListState
import java.util.Locale

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
    val load: DirectoryLoad,
    val history: List<DirectoryView>,
    val nextSequence: Long,
)

internal data class DirectoryView(
    val listing: DirectoryListing,
    val filter: String = "",
    val showHidden: Boolean = false,
    /** This view's list position: new with each listing, kept in history and the forge for Back and app switches. */
    val scroll: LazyListState = LazyListState(),
)

internal sealed interface DirectoryLoad {
    data class Loading(val sequence: Long, val candidate: HomeDirectory, val retained: DirectoryView?) : DirectoryLoad
    data class Loaded(val view: DirectoryView) : DirectoryLoad
    data class Failed(val candidate: HomeDirectory, val retained: DirectoryView?, val failure: DirectoryBrowseFailure) : DirectoryLoad
}

internal enum class DirectoryBrowseFailure { Transport, Unavailable, TooLarge, Internal }

internal data class WorkingDirectoryRequest(
    val pickerInstance: Long,
    val sequence: Long,
    val directory: HomeDirectory,
)

internal data class WorkingDirectoryRequestStart(
    val picker: WorkingDirectoryPickerState,
    val request: WorkingDirectoryRequest,
)

internal fun browseWorkingDirectoryHome(
    forge: ForgeState,
    machine: MachineState,
    pickerInstance: Long,
): WorkingDirectoryRequestStart? {
    if (forge.pending || forge.surface != ForgeSurface.Form || !machine.canMutate ||
        forge.form.machineHandle != machine.machine.handle) return null
    return WorkingDirectoryRequestStart(
        picker = WorkingDirectoryPickerState(
            instance = pickerInstance,
            machine = machine.machine,
            load = DirectoryLoad.Loading(1, HomeDirectory.Home, null),
            history = emptyList(),
            nextSequence = 2,
        ),
        request = WorkingDirectoryRequest(pickerInstance, sequence = 1, directory = HomeDirectory.Home),
    )
}

internal fun openWorkingDirectoryChild(
    picker: WorkingDirectoryPickerState,
    directory: HomeDirectory,
): WorkingDirectoryRequestStart? {
    val view = picker.actionableDirectoryView() ?: return null
    if (view.listing.children.none { entry -> entry.directory == directory }) return null
    return beginWorkingDirectoryRequest(picker, directory, view)
}

internal fun openWorkingDirectoryParent(picker: WorkingDirectoryPickerState): WorkingDirectoryRequestStart? {
    val view = picker.actionableDirectoryView() ?: return null
    val parent = view.listing.parent as? ParentDirectory.Available ?: return null
    return beginWorkingDirectoryRequest(picker, parent.directory, view)
}

internal fun retryWorkingDirectory(picker: WorkingDirectoryPickerState): WorkingDirectoryRequestStart? {
    val failed = picker.load as? DirectoryLoad.Failed ?: return null
    return beginWorkingDirectoryRequest(picker, failed.candidate, failed.retained)
}

private fun beginWorkingDirectoryRequest(
    picker: WorkingDirectoryPickerState,
    directory: HomeDirectory,
    retained: DirectoryView?,
): WorkingDirectoryRequestStart {
    val sequence = picker.nextSequence
    return WorkingDirectoryRequestStart(
        picker = picker.copy(
            load = DirectoryLoad.Loading(sequence, directory, retained),
            nextSequence = sequence + 1,
        ),
        request = WorkingDirectoryRequest(picker.instance, sequence, directory),
    )
}

/** null: the reply is stale (another request or picker instance owns the load). */
internal fun completeWorkingDirectoryRequest(
    picker: WorkingDirectoryPickerState,
    request: WorkingDirectoryRequest,
    result: GatewayResult<DirectoryListing>,
): WorkingDirectoryPickerState? {
    val loading = picker.load as? DirectoryLoad.Loading ?: return null
    if (picker.instance != request.pickerInstance || loading.sequence != request.sequence) return null
    return when (result) {
        is GatewayResult.Success -> picker.copy(
            load = DirectoryLoad.Loaded(DirectoryView(result.value)),
            history = loading.retained?.let { (picker.history + it).takeLast(MAXIMUM_WORKING_DIRECTORY_HISTORY) }
                ?: picker.history,
        )
        is GatewayResult.Failure -> {
            val browse = when (val failure = result.failure) {
                GatewayFailure.Transport -> DirectoryBrowseFailure.Transport
                is GatewayFailure.Api -> when (failure.code) {
                    ApiErrorCode.DirectoryListingUnavailable -> DirectoryBrowseFailure.Unavailable
                    ApiErrorCode.DirectoryListingTooLarge -> DirectoryBrowseFailure.TooLarge
                    else -> DirectoryBrowseFailure.Internal
                }
            }
            picker.copy(load = DirectoryLoad.Failed(loading.candidate, loading.retained, browse))
        }
    }
}

internal fun updateWorkingDirectoryFilter(
    picker: WorkingDirectoryPickerState,
    filter: String,
): WorkingDirectoryPickerState = picker.updateVisibleDirectoryView { view -> view.copy(filter = filter) }

internal fun setWorkingDirectoryHidden(
    picker: WorkingDirectoryPickerState,
    showHidden: Boolean,
): WorkingDirectoryPickerState = picker.updateVisibleDirectoryView { view -> view.copy(showHidden = showHidden) }

private fun WorkingDirectoryPickerState.updateVisibleDirectoryView(
    transform: (DirectoryView) -> DirectoryView,
): WorkingDirectoryPickerState = copy(load = when (val load = load) {
    is DirectoryLoad.Loaded -> load.copy(view = transform(load.view))
    is DirectoryLoad.Loading -> load.copy(retained = load.retained?.let(transform))
    is DirectoryLoad.Failed -> load.copy(retained = load.retained?.let(transform))
})

/** The folders the view shows, each with its server ordinal as index. */
internal fun visibleWorkingDirectoryEntries(view: DirectoryView): List<IndexedValue<DirectoryEntry>> {
    val visible = view.listing.children.withIndex().filter { view.showHidden || !it.value.directory.hidden }
    if (view.filter.isEmpty()) return visible
    val query = view.filter.lowercase(Locale.ROOT)
    // Rank, then fewer scalars in the basename; the sort is stable, so remaining ties keep server order.
    return visible
        .mapNotNull { entry ->
            directoryMatchRank(entry.value.directory.basename.lowercase(Locale.ROOT), query)?.let { rank -> rank to entry }
        }
        .sortedWith(compareBy({ it.first }, { it.second.value.directory.basename.run { codePointCount(0, length) } }))
        .map { it.second }
}

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
    val retained = loading.retained ?: return cleared.copy(surface = ForgeSurface.Form)
    return cleared.copy(surface = surface.copy(picker = surface.picker.copy(load = DirectoryLoad.Loaded(retained))))
}

internal fun workingDirectoryBack(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val picker = surface.picker
    val previous = when (val load = picker.load) {
        is DirectoryLoad.Loading -> load.retained?.let { picker.copy(load = DirectoryLoad.Loaded(it)) }
        is DirectoryLoad.Failed -> load.retained?.let { picker.copy(load = DirectoryLoad.Loaded(it)) }
        is DirectoryLoad.Loaded -> picker.history.lastOrNull()?.let {
            picker.copy(load = DirectoryLoad.Loaded(it), history = picker.history.dropLast(1))
        }
    }
    return forge.copy(surface = previous?.let { surface.copy(picker = it) } ?: ForgeSurface.Form)
}

internal fun cancelWorkingDirectoryPicker(forge: ForgeState): ForgeState =
    if (forge.surface is ForgeSurface.DirectoryPicker) forge.copy(surface = ForgeSurface.Form) else forge

internal fun useCurrentWorkingDirectory(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val view = surface.picker.actionableDirectoryView() ?: return forge
    return forge.copy(
        form = forge.form.copy(cwd = view.listing.directory.encoded),
        failure = forge.failure.afterWorkingDirectoryChoice(),
        surface = ForgeSurface.Form,
        directorySearch = DirectorySearchState.Idle,
    )
}

/** Choosing a folder clears a rejection of the previous one. */
internal fun ForgeFailure.afterWorkingDirectoryChoice(): ForgeFailure =
    if (this is ForgeFailure.Definite &&
        rejection.code in setOf(ApiErrorCode.WorkingDirectoryInvalid, ApiErrorCode.WorkingDirectoryUnavailable)
    ) ForgeFailure.None else this

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
    is DirectoryLoad.Failed -> load.retained
}
