package dev.niels.skidbladnir

import java.util.Locale

private const val MAXIMUM_WORKING_DIRECTORY_FILTER_SCALARS = 256
private const val MAXIMUM_WORKING_DIRECTORY_HISTORY = 32

internal class WorkingDirectoryPath private constructor(val encoded: String) {
    companion object {
        fun parse(candidate: String): WorkingDirectoryPath? {
            if (candidate.utf8ByteCountWithin(MAXIMUM_WORKING_DIRECTORY_BYTES) == null) return null
            if (candidate.hasDisplayUnsafeCodePoint()) return null
            if (candidate != "~" && !candidate.startsWith("~/") && !candidate.startsWith('/')) return null
            return WorkingDirectoryPath(candidate)
        }
    }

    override fun equals(other: Any?): Boolean = other is WorkingDirectoryPath && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

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
    val activeDirectories: List<WorkingDirectoryPath>,
    val exactDraft: String,
    val page: WorkingDirectoryPage,
    val history: List<DirectoryView>,
    val nextSequence: Long,
)

internal sealed interface WorkingDirectoryPage {
    data object Places : WorkingDirectoryPage
    data class Browsing(val load: DirectoryLoad) : WorkingDirectoryPage
    data class ExactPath(
        val origin: ExactPathOrigin,
        val validation: ExactPathValidation,
    ) : WorkingDirectoryPage
}

internal sealed interface ExactPathOrigin {
    data object Places : ExactPathOrigin
    data class Browse(val load: DirectoryLoad) : ExactPathOrigin {
        init {
            require(load !is DirectoryLoad.Loading)
        }
    }
}

internal enum class ExactPathValidation { Pristine, Valid, Invalid }

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

internal fun openWorkingDirectoryPicker(
    forge: ForgeState,
    machine: MachineState,
    pickerInstance: Long,
): ForgeState? {
    if (forge.pending || forge.surface !is ForgeSurface.Form || pickerInstance <= 0) return null
    if (forge.form.machineHandle != machine.machine.handle || machine.access != MachineAccess.Ready) return null
    val inventory = machine.inventory as? InventoryState.Fresh ?: return null
    val activeDirectories = inventory.snapshot.inventory.sessions.asSequence()
        .mapNotNull(TmuxSession::cwd)
        .map { cwd ->
            WorkingDirectoryPath.parse(cwd)
                ?: throw ProtocolDecodeException("inventory working-directory value")
        }
        .distinct()
        .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.encoded, second.encoded) }
        .toList()
    return forge.copy(
        surface = ForgeSurface.DirectoryPicker(
            WorkingDirectoryPickerState(
                instance = pickerInstance,
                machine = machine.machine,
                machineSummary = inventory.snapshot.inventory.machine,
                activeDirectories = activeDirectories,
                exactDraft = forge.form.cwd,
                page = WorkingDirectoryPage.Places,
                history = emptyList(),
                nextSequence = 1,
            ),
        ),
    )
}

internal fun openExactWorkingDirectoryPicker(
    forge: ForgeState,
    machine: MachineState,
    pickerInstance: Long,
): ForgeState? = openWorkingDirectoryPicker(forge, machine, pickerInstance)?.let(::showExactWorkingDirectory)

internal fun browseWorkingDirectoryHome(
    picker: WorkingDirectoryPickerState,
    generation: Long,
): WorkingDirectoryRequestStart? = if (picker.page == WorkingDirectoryPage.Places) {
    beginWorkingDirectoryRequest(picker, HomeDirectory.Home, RetainedDirectoryView.None, generation)
} else {
    null
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
    val failed = (picker.page as? WorkingDirectoryPage.Browsing)?.load as? DirectoryLoad.Failed
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
            page = WorkingDirectoryPage.Browsing(
                DirectoryLoad.Loading(sequence, directory, retained),
            ),
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
    val loading = ((picker.page as? WorkingDirectoryPage.Browsing)?.load as? DirectoryLoad.Loading)
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
                    page = WorkingDirectoryPage.Browsing(
                        DirectoryLoad.Loaded(
                            DirectoryView(
                                listing = listing,
                                filter = "",
                                showHidden = false,
                                viewport = DirectoryViewport.Top,
                            ),
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
                ApiErrorCode.ProfileUnknown,
                ApiErrorCode.SessionNameInvalid,
                ApiErrorCode.ObjectiveInvalid,
                ApiErrorCode.SessionNameConflict,
                ApiErrorCode.SessionNotFound,
                ApiErrorCode.SessionIdentityMismatch,
                ApiErrorCode.SessionGroupedConflict,
                ApiErrorCode.PairingInviteRejected,
                ApiErrorCode.ReconnectRequired,
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
        page = WorkingDirectoryPage.Browsing(
            DirectoryLoad.Failed(loading.candidate, loading.retained, failure),
        ),
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
    val browsing = page as? WorkingDirectoryPage.Browsing ?: return this
    val updated = when (val load = browsing.load) {
        is DirectoryLoad.Loaded -> load.copy(view = transform(load.view))
        is DirectoryLoad.Loading -> load.copy(retained = load.retained.map(transform))
        is DirectoryLoad.Failed -> load.copy(retained = load.retained.map(transform))
    }
    return copy(page = browsing.copy(load = updated))
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

internal fun workingDirectoryPickerAfterForegroundInvalidation(
    forge: ForgeState,
): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val picker = surface.picker
    val page = when (val current = picker.page) {
        WorkingDirectoryPage.Places,
        is WorkingDirectoryPage.ExactPath,
        -> return forge
        is WorkingDirectoryPage.Browsing -> when (val load = current.load) {
            is DirectoryLoad.Loaded,
            is DirectoryLoad.Failed,
            -> return forge
            is DirectoryLoad.Loading -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> WorkingDirectoryPage.Places
                is RetainedDirectoryView.Present ->
                    WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained.view))
            }
        }
    }
    return forge.copy(
        surface = surface.copy(picker = picker.copy(page = page)),
    )
}

internal fun workingDirectoryBack(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val picker = surface.picker
    val updated = when (val page = picker.page) {
        WorkingDirectoryPage.Places -> return forge.copy(surface = ForgeSurface.Form)
        is WorkingDirectoryPage.ExactPath -> picker.copy(
            page = when (val origin = page.origin) {
                ExactPathOrigin.Places -> WorkingDirectoryPage.Places
                is ExactPathOrigin.Browse -> WorkingDirectoryPage.Browsing(origin.load)
            },
        )
        is WorkingDirectoryPage.Browsing -> when (val load = page.load) {
            is DirectoryLoad.Loading -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> picker.copy(page = WorkingDirectoryPage.Places)
                is RetainedDirectoryView.Present -> picker.copy(
                    page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained.view)),
                )
            }
            is DirectoryLoad.Loaded -> picker.restorePreviousDirectory()
            is DirectoryLoad.Failed -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> picker.copy(page = WorkingDirectoryPage.Places)
                is RetainedDirectoryView.Present -> picker.copy(
                    page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(retained.view)),
                )
            }
        }
    }
    return forge.copy(surface = surface.copy(picker = updated))
}

private fun WorkingDirectoryPickerState.restorePreviousDirectory(): WorkingDirectoryPickerState =
    if (history.isEmpty()) {
        copy(page = WorkingDirectoryPage.Places)
    } else {
        copy(
            page = WorkingDirectoryPage.Browsing(DirectoryLoad.Loaded(history.last())),
            history = history.dropLast(1),
        )
    }

internal fun cancelWorkingDirectoryPicker(forge: ForgeState): ForgeState =
    if (forge.surface is ForgeSurface.DirectoryPicker) forge.copy(surface = ForgeSurface.Form) else forge

internal fun showExactWorkingDirectory(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val picker = surface.picker
    val origin = when (val page = picker.page) {
        WorkingDirectoryPage.Places -> ExactPathOrigin.Places
        is WorkingDirectoryPage.ExactPath -> return forge
        is WorkingDirectoryPage.Browsing -> when (val load = page.load) {
            is DirectoryLoad.Loading -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> ExactPathOrigin.Places
                is RetainedDirectoryView.Present ->
                    ExactPathOrigin.Browse(DirectoryLoad.Loaded(retained.view))
            }
            is DirectoryLoad.Loaded -> ExactPathOrigin.Browse(load)
            is DirectoryLoad.Failed -> ExactPathOrigin.Browse(load)
        }
    }
    val updated = picker.copy(
        page = WorkingDirectoryPage.ExactPath(origin, exactPathValidation(picker.exactDraft)),
    )
    return forge.copy(surface = surface.copy(picker = updated))
}

internal fun updateExactWorkingDirectory(forge: ForgeState, draft: String): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val page = surface.picker.page as? WorkingDirectoryPage.ExactPath ?: return forge
    if (draft.utf8ByteCountWithin(MAXIMUM_WORKING_DIRECTORY_BYTES) == null) return forge
    val picker = surface.picker.copy(
        exactDraft = draft,
        page = page.copy(validation = exactPathValidation(draft)),
    )
    return forge.copy(surface = surface.copy(picker = picker))
}

internal fun useExactWorkingDirectory(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val page = surface.picker.page as? WorkingDirectoryPage.ExactPath ?: return forge
    val path = WorkingDirectoryPath.parse(surface.picker.exactDraft)
    if (path == null) {
        return forge.copy(
            surface = surface.copy(
                picker = surface.picker.copy(
                    page = page.copy(validation = ExactPathValidation.Invalid),
                ),
            ),
        )
    }
    return selectWorkingDirectory(forge, surface.picker, path)
}

internal fun chooseActiveWorkingDirectory(
    forge: ForgeState,
    directory: WorkingDirectoryPath,
): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    if (surface.picker.page != WorkingDirectoryPage.Places ||
        directory !in surface.picker.activeDirectories
    ) return forge
    return selectWorkingDirectory(forge, surface.picker, directory)
}

internal fun useCurrentWorkingDirectory(forge: ForgeState): ForgeState {
    val surface = forge.surface as? ForgeSurface.DirectoryPicker ?: return forge
    val view = surface.picker.actionableDirectoryView() ?: return forge
    val path = checkNotNull(WorkingDirectoryPath.parse(view.listing.directory.encoded))
    return selectWorkingDirectory(forge, surface.picker, path)
}

private fun selectWorkingDirectory(
    forge: ForgeState,
    picker: WorkingDirectoryPickerState,
    directory: WorkingDirectoryPath,
): ForgeState {
    if (forge.form.machineHandle != picker.machine.handle) return forge
    return forge.copy(
        form = forge.form.copy(cwd = directory.encoded),
        failure = forge.failure.afterWorkingDirectoryChoice(),
        surface = ForgeSurface.Form,
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
        ApiErrorCode.ProfileUnknown,
        ApiErrorCode.SessionNameInvalid,
        ApiErrorCode.ObjectiveInvalid,
        ApiErrorCode.SessionNameConflict,
        ApiErrorCode.SessionNotFound,
        ApiErrorCode.SessionIdentityMismatch,
        ApiErrorCode.SessionGroupedConflict,
        ApiErrorCode.PairingInviteRejected,
        ApiErrorCode.MachineIdentityMismatch,
        ApiErrorCode.InternalError,
        ApiErrorCode.ReconnectRequired,
        -> false
    }
}

private fun ForgeFailure.afterWorkingDirectoryChoice(): ForgeFailure =
    if (isWorkingDirectoryRejection()) ForgeFailure.None else this

internal fun updateForgeState(forge: ForgeState, proposed: ForgeForm): ForgeState {
    val machineChanged = proposed.machineHandle != forge.form.machineHandle
    return forge.copy(
        form = changeForgeDraft(forge.form, proposed),
        failure = ForgeFailure.None,
        surface = if (machineChanged) ForgeSurface.Form else forge.surface,
    )
}

private fun exactPathValidation(draft: String): ExactPathValidation = when {
    draft.isEmpty() -> ExactPathValidation.Pristine
    WorkingDirectoryPath.parse(draft) != null -> ExactPathValidation.Valid
    else -> ExactPathValidation.Invalid
}

private fun WorkingDirectoryPickerState.actionableDirectoryView(): DirectoryView? {
    val load = (page as? WorkingDirectoryPage.Browsing)?.load ?: return null
    return when (load) {
        is DirectoryLoad.Loading -> null
        is DirectoryLoad.Loaded -> load.view
        is DirectoryLoad.Failed -> when (val retained = load.retained) {
            RetainedDirectoryView.None -> null
            is RetainedDirectoryView.Present -> retained.view
        }
    }
}

internal fun retainedWorkingDirectoryView(picker: WorkingDirectoryPickerState): DirectoryView? {
    val load = (picker.page as? WorkingDirectoryPage.Browsing)?.load ?: return null
    return when (load) {
        is DirectoryLoad.Loaded -> load.view
        is DirectoryLoad.Loading -> when (val retained = load.retained) {
            RetainedDirectoryView.None -> null
            is RetainedDirectoryView.Present -> retained.view
        }
        is DirectoryLoad.Failed -> when (val retained = load.retained) {
            RetainedDirectoryView.None -> null
            is RetainedDirectoryView.Present -> retained.view
        }
    }
}
