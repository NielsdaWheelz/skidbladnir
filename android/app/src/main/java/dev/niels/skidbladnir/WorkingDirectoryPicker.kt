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
            load = DirectoryLoad.Loading(1, HomeDirectory.Home, RetainedDirectoryView.None),
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
    return beginWorkingDirectoryRequest(picker, directory, RetainedDirectoryView.Present(view))
}

internal fun openWorkingDirectoryParent(picker: WorkingDirectoryPickerState): WorkingDirectoryRequestStart? {
    val view = picker.actionableDirectoryView() ?: return null
    val parent = view.listing.parent as? ParentDirectory.Available ?: return null
    return beginWorkingDirectoryRequest(picker, parent.directory, RetainedDirectoryView.Present(view))
}

internal fun retryWorkingDirectory(picker: WorkingDirectoryPickerState): WorkingDirectoryRequestStart? {
    val failed = picker.load as? DirectoryLoad.Failed ?: return null
    return beginWorkingDirectoryRequest(picker, failed.candidate, failed.retained)
}

private fun beginWorkingDirectoryRequest(
    picker: WorkingDirectoryPickerState,
    directory: HomeDirectory,
    retained: RetainedDirectoryView,
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
            load = DirectoryLoad.Loaded(
                DirectoryView(listing = result.value, filter = "", showHidden = false, viewport = DirectoryViewport.Top),
            ),
            history = when (val retained = loading.retained) {
                RetainedDirectoryView.None -> picker.history
                is RetainedDirectoryView.Present ->
                    (picker.history + retained.view).takeLast(MAXIMUM_WORKING_DIRECTORY_HISTORY)
            },
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
    return forge.copy(
        form = forge.form.copy(cwd = view.listing.directory.encoded),
        failure = forge.failure.afterWorkingDirectoryChoice(),
        surface = ForgeSurface.Form,
        directorySearch = DirectorySearchState.Idle,
    )
}

internal fun ForgeFailure.isWorkingDirectoryRejection(): Boolean =
    this is ForgeFailure.Definite &&
        rejection.code in setOf(ApiErrorCode.WorkingDirectoryInvalid, ApiErrorCode.WorkingDirectoryUnavailable)

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
