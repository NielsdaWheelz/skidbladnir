package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.distinctUntilChanged

internal class WorkingDirectoryPickerActions(
    val openChild: (HomeDirectory) -> Unit,
    val openParent: () -> Unit,
    val retry: () -> Unit,
    val updateFilter: (String) -> Unit,
    val setHidden: (Boolean) -> Unit,
    val updateViewport: (DirectoryViewport) -> Unit,
    val enterPath: () -> Boolean,
    val useCurrent: () -> Boolean,
    val back: () -> Boolean,
    val cancel: () -> Boolean,
)

private data class BrowseContent(
    val context: PickerBrowseContext,
    val view: DirectoryView?,
    val actionsEnabled: Boolean,
)

private class BrowseViewportScope(
    val listing: DirectoryListing,
    val filter: String,
    val showHidden: Boolean,
) {
    override fun equals(other: Any?): Boolean = other is BrowseViewportScope &&
        listing === other.listing && filter == other.filter && showHidden == other.showHidden

    override fun hashCode(): Int =
        31 * (31 * System.identityHashCode(listing) + filter.hashCode()) + showHidden.hashCode()
}

private sealed interface PickerAction {
    data object EnterPath : PickerAction
    data object Parent : PickerAction
    data class Folder(val directory: HomeDirectory) : PickerAction
}

private sealed interface BrowseStatus {
    data object Ready : BrowseStatus
    data class Supporting(val visual: String, val spoken: String) : BrowseStatus
    data class Failed(
        val headingVisual: String,
        val headingSpoken: String,
        val body: String,
        val tone: NoticeTone,
        val retryLabel: String?,
    ) : BrowseStatus
}

private data class PickerBrowseContext(
    val locationLabel: String,
    val directory: HomeDirectory,
    val locationSpoken: String,
    val status: BrowseStatus,
)

private sealed interface PickerRow {
    val key: PickerRowKey
    data class Action(
        override val key: PickerRowKey,
        val label: String,
        val contentDescription: String,
        val action: PickerAction,
    ) : PickerRow
    data class Filter(val value: String) : PickerRow { override val key = PickerRowKey.Filter }
    data class Hidden(val shown: Boolean) : PickerRow { override val key = PickerRowKey.Hidden }
    data object Omission : PickerRow { override val key = PickerRowKey.Omission }
}

private sealed interface PickerRowKey {
    data object EnterPath : PickerRowKey
    data object Parent : PickerRowKey
    data object Filter : PickerRowKey
    data object Hidden : PickerRowKey
    data object Omission : PickerRowKey
    data class Folder(val directory: HomeDirectory, val ordinal: Int) : PickerRowKey
}

private fun PickerRowKey.saveableKey(): String = when (this) {
    PickerRowKey.EnterPath -> "enter-path"
    PickerRowKey.Parent -> "parent"
    PickerRowKey.Filter -> "filter"
    PickerRowKey.Hidden -> "hidden"
    PickerRowKey.Omission -> "omission"
    is PickerRowKey.Folder -> "folder:$ordinal"
}

@Composable
internal fun WorkingDirectoryPickerScreen(
    picker: WorkingDirectoryPickerState,
    actions: WorkingDirectoryPickerActions,
    enabled: Boolean,
    onReturnToField: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val content = workingDirectoryBrowseContent(picker.load)
    val view = content.view
    val rows = workingDirectoryBrowseRows(picker.machine, view)
    val listState = rememberLazyListState()
    // A new reply snapshot or folder subset must restore before scroll capture resumes.
    val viewportScope = view?.let { BrowseViewportScope(it.listing, it.filter, it.showHidden) }
    var restoredScope by remember(picker.instance) { mutableStateOf<BrowseViewportScope?>(null) }
    val restorationReady = view == null || restoredScope == viewportScope
    RestoreWorkingDirectoryViewport(picker.instance, viewportScope, view, rows, listState) { restoredScope = it }
    CaptureWorkingDirectoryViewport(picker.instance, view, rows, listState, restorationReady, actions.updateViewport)
    val back = { if (actions.back()) onReturnToField() }
    ModalBackHandler(back)
    Column(modifier.fillMaxSize().imePadding()) {
        PickerHeader(picker.machine, back, { if (actions.cancel()) onReturnToField() })
        BrowseContext(content.context, actions.retry, enabled)
        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f),
            state = listState,
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(rows, key = { it.key.saveableKey() }) { row ->
                PickerRow(row, enabled && content.actionsEnabled && restorationReady, actions, onReturnToField)
            }
        }
        view?.listing?.directory?.let { directory ->
            val home = directory == HomeDirectory.Home
            Button(
                onClick = { if (actions.useCurrent()) onReturnToField() },
                enabled = enabled && restorationReady && content.actionsEnabled,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
                    .fillMaxWidth().heightIn(min = 48.dp).minimumInteractiveComponentSize()
                    .semantics {
                        contentDescription = "Use ${directory.encoded} as working directory on ${picker.machine.label.text}."
                    },
                shape = NidavellirShapes.Chip,
            ) { Text(if (home) "Use Home" else "Use this folder") }
        }
    }
}

@Composable
private fun PickerHeader(machine: PairedMachine, onBack: () -> Unit, onCancel: () -> Unit) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = onBack, modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()) {
                Text("Back")
            }
            Spacer(Modifier.weight(1f))
            TextButton(onClick = onCancel, modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()) {
                Text("Cancel")
            }
        }
        Text("Browse Home", style = MaterialTheme.typography.headlineSmall,
            fontFamily = NidavellirType.Display, fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(horizontal = 8.dp))
        Text("On ${bidiIsolate(machine.label.text)}", color = Muted,
            style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp).semantics {
                contentDescription = "On ${machine.label.text}"
            })
    }
}

@Composable
private fun PickerRow(
    row: PickerRow,
    browseActionsEnabled: Boolean,
    actions: WorkingDirectoryPickerActions,
    onReturnToField: () -> Unit,
) {
    when (row) {
        is PickerRow.Action -> PickerActionRow(
            label = row.label,
            description = row.contentDescription,
            enabled = when (row.action) {
                PickerAction.EnterPath -> true
                PickerAction.Parent, is PickerAction.Folder -> browseActionsEnabled
            },
            onClick = when (val action = row.action) {
                PickerAction.EnterPath -> ({ if (actions.enterPath()) onReturnToField() })
                PickerAction.Parent -> actions.openParent
                is PickerAction.Folder -> ({ actions.openChild(action.directory) })
            },
        )
        is PickerRow.Filter -> OutlinedTextField(
            value = row.value, onValueChange = actions.updateFilter, modifier = Modifier.fillMaxWidth(),
            label = { Text("Filter folders") }, singleLine = true,
            keyboardOptions = KeyboardOptions(autoCorrectEnabled = false),
        )
        is PickerRow.Hidden -> FilterChip(
            selected = row.shown, onClick = { actions.setHidden(!row.shown) },
            label = { Text(if (row.shown) "Hide hidden folders" else "Show hidden folders") },
            shape = NidavellirShapes.Chip,
            modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize(),
        )
        PickerRow.Omission -> NoticePanel(tone = NoticeTone.Degraded, body = "Some folders cannot be shown.")
    }
}

@Composable
private fun PickerActionRow(
    label: String,
    description: String,
    onClick: () -> Unit,
    enabled: Boolean = true,
) {
    Surface(
        color = RaisedSurface,
        border = BorderStroke(1.dp, Gold.copy(alpha = if (enabled) 0.40f else 0.18f)),
        shape = NidavellirShapes.Chip,
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Chip),
                enabled = enabled,
                role = Role.Button,
                onClick = onClick,
            )
            .semantics(mergeDescendants = true) {
                contentDescription = description
                role = Role.Button
            },
    ) {
        Box(
            Modifier.fillMaxWidth().minimumInteractiveComponentSize()
                .padding(horizontal = 12.dp, vertical = 10.dp),
            contentAlignment = Alignment.CenterStart,
        ) {
            Text(label, style = MaterialTheme.typography.bodyLarge,
                color = if (enabled) Bone else Muted)
        }
    }
}

@Composable
private fun BrowseContext(
    content: PickerBrowseContext,
    onRetry: () -> Unit,
    enabled: Boolean,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Text(
            content.locationLabel,
            color = Muted,
            style = MaterialTheme.typography.labelLarge,
        )
        WorkingDirectoryPathLine(
            path = content.directory.encoded,
            contentDescription = content.locationSpoken,
            modifier = Modifier.fillMaxWidth(),
        )
        when (val status = content.status) {
            BrowseStatus.Ready -> Unit
            is BrowseStatus.Supporting -> Text(
                status.visual,
                color = Muted,
                modifier = Modifier.semantics { contentDescription = status.spoken },
            )
            is BrowseStatus.Failed -> Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Text(
                    status.headingVisual,
                    color = Muted,
                    modifier = Modifier.semantics {
                        contentDescription = status.headingSpoken
                    },
                )
                NoticePanel(
                    tone = status.tone,
                    body = status.body,
                    actions = status.retryLabel?.let { label ->
                        {
                            TextButton(
                                onClick = onRetry,
                                enabled = enabled,
                                modifier = Modifier.heightIn(min = 48.dp)
                                    .minimumInteractiveComponentSize(),
                            ) { Text(label) }
                        }
                    },
                )
            }
        }
    }
}

@Composable
internal fun WorkingDirectoryPathLine(
    path: String,
    modifier: Modifier = Modifier,
    contentDescription: String? = path,
) {
    val scrollState = rememberScrollState()
    LaunchedEffect(path, scrollState.maxValue) {
        scrollState.scrollTo(scrollState.maxValue)
    }
    Row(
        modifier.heightIn(min = 48.dp).horizontalScroll(scrollState).then(
            contentDescription?.let { description ->
                Modifier.semantics(mergeDescendants = true) {
                    this.contentDescription = description
                }
            } ?: Modifier,
        ),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            ltrIsolate(path),
            color = Bone,
            fontFamily = NidavellirType.Data,
            maxLines = 1,
            softWrap = false,
            style = MaterialTheme.typography.bodyMedium.merge(
                TextStyle(textDirection = TextDirection.Ltr),
            ),
        )
    }
}

@Composable
private fun RestoreWorkingDirectoryViewport(
    pickerInstance: Long,
    scope: BrowseViewportScope?,
    view: DirectoryView?,
    rows: List<PickerRow>,
    state: LazyListState,
    onRestored: (BrowseViewportScope?) -> Unit,
) {
    LaunchedEffect(pickerInstance, scope) {
        onRestored(null)
        if (view == null) {
            state.scrollToItem(0)
            return@LaunchedEffect
        }
        when (val viewport = view.viewport) {
            DirectoryViewport.Top -> state.scrollToItem(0)
            is DirectoryViewport.Anchor -> {
                val index = rows.indexOfFirst { row ->
                    (row.key as? PickerRowKey.Folder)?.directory == viewport.directory
                }
                if (index < 0) {
                    state.scrollToItem(0)
                } else {
                    state.scrollToItem(index, viewport.offset)
                }
            }
        }
        withFrameNanos { }
        onRestored(scope)
    }
}

@Composable
private fun CaptureWorkingDirectoryViewport(
    pickerInstance: Long,
    view: DirectoryView?,
    rows: List<PickerRow>,
    state: LazyListState,
    enabled: Boolean,
    onViewport: (DirectoryViewport) -> Unit,
) {
    val directory = view?.listing?.directory
    val authoritativeViewport by rememberUpdatedState(view?.viewport)
    LaunchedEffect(pickerInstance, directory, rows, enabled) {
        if (view == null || !enabled) return@LaunchedEffect
        val firstFolderIndex = rows.indexOfFirst { it.key is PickerRowKey.Folder }
        snapshotFlow {
            val visible = state.layoutInfo.visibleItemsInfo
            val first = visible.firstOrNull() ?: return@snapshotFlow null
            if (firstFolderIndex < 0 || first.index < firstFolderIndex) {
                return@snapshotFlow DirectoryViewport.Top
            }
            val folder = visible.firstOrNull { item ->
                rows.getOrNull(item.index)?.key is PickerRowKey.Folder
            }
                ?: return@snapshotFlow null
            val key = rows[folder.index].key as PickerRowKey.Folder
            DirectoryViewport.Anchor(key.directory, (-folder.offset).coerceAtLeast(0))
        }.distinctUntilChanged().collect { viewport ->
            if (viewport != null && viewport != authoritativeViewport) onViewport(viewport)
        }
    }
}

private fun workingDirectoryBrowseContent(load: DirectoryLoad): BrowseContent = when (load) {
    is DirectoryLoad.Loaded -> browseContent(
        locationIsCurrent = true, location = load.view.listing.directory, view = load.view,
        actionsEnabled = true,
        status = if (visibleWorkingDirectoryEntries(load.view).isEmpty()) {
            BrowseStatus.Supporting("No visible folders here.", "No visible folders here.")
        } else BrowseStatus.Ready,
    )
    is DirectoryLoad.Loading -> when (val retained = load.retained) {
        RetainedDirectoryView.None -> browseContent(false, load.candidate, null, false, openingStatus(load.candidate))
        is RetainedDirectoryView.Present -> browseContent(true, retained.view.listing.directory,
            retained.view, false, openingStatus(load.candidate))
    }
    is DirectoryLoad.Failed -> when (val retained = load.retained) {
        RetainedDirectoryView.None -> browseContent(false, load.candidate, null, false,
            browseFailureStatus(load.candidate, load.failure))
        is RetainedDirectoryView.Present -> browseContent(true, retained.view.listing.directory,
            retained.view, true, browseFailureStatus(load.candidate, load.failure))
    }
}

private fun openingStatus(directory: HomeDirectory): BrowseStatus.Supporting {
    val name = directory.displayName
    return BrowseStatus.Supporting(
        visual = "Opening “${bidiIsolate(name)}”…",
        spoken = "Opening “$name”…",
    )
}

private fun browseFailureStatus(
    requested: HomeDirectory,
    failure: DirectoryBrowseFailure,
): BrowseStatus.Failed {
    val name = requested.displayName
    val bodyAndRetry = when (failure) {
        DirectoryBrowseFailure.Transport ->
            "Could not reach this machine over your Tailnet." to "Try again"
        DirectoryBrowseFailure.Unavailable ->
            "This directory cannot be browsed. Enter the path instead." to null
        DirectoryBrowseFailure.TooLarge ->
            "This directory has too many folders to show. Enter the path instead." to null
        DirectoryBrowseFailure.Internal ->
            "Skíðblaðnir could not complete the request." to "Try again"
    }
    return BrowseStatus.Failed(
        headingVisual = "Could not open “${bidiIsolate(name)}”.",
        headingSpoken = "Could not open “$name”.",
        body = bodyAndRetry.first,
        tone = NoticeTone.Failure,
        retryLabel = bodyAndRetry.second,
    )
}

private fun browseContent(
    locationIsCurrent: Boolean,
    location: HomeDirectory,
    view: DirectoryView?,
    actionsEnabled: Boolean,
    status: BrowseStatus,
): BrowseContent {
    val label = if (locationIsCurrent) "Current folder" else "Requested folder"
    return BrowseContent(PickerBrowseContext(label, location, "$label ${location.encoded}", status), view, actionsEnabled)
}

private fun workingDirectoryBrowseRows(machine: PairedMachine, view: DirectoryView?): List<PickerRow> = buildList {
    if (view?.listing?.parent is ParentDirectory.Available) {
        add(PickerRow.Action(PickerRowKey.Parent, "Parent folder",
            "Parent folder. Opens the parent folder.", PickerAction.Parent))
    }
    add(PickerRow.Action(PickerRowKey.EnterPath, "Enter path",
        "Enter a working directory on ${machine.label.text}.", PickerAction.EnterPath))
    if (view == null) return@buildList
    add(PickerRow.Filter(view.filter))
    if (workingDirectoryHasHiddenEntries(view)) add(PickerRow.Hidden(view.showHidden))
    if (view.listing.omissions == DirectoryOmissions.Present) add(PickerRow.Omission)
    val ordinals = view.listing.children.withIndex().associate { it.value.directory to it.index }
    visibleWorkingDirectoryEntries(view).forEach { entry ->
        val name = entry.directory.basename
        add(PickerRow.Action(
            PickerRowKey.Folder(entry.directory, checkNotNull(ordinals[entry.directory])),
            bidiIsolate(name),
            when (entry.kind) {
                DirectoryEntryKind.Directory -> "Folder $name. Opens folder."
                DirectoryEntryKind.SymbolicLink -> "Linked folder $name. Opens folder."
            },
            PickerAction.Folder(entry.directory),
        ))
    }
}

private val HomeDirectory.displayName: String
    get() = if (this == HomeDirectory.Home) "Home" else basename

internal fun bidiIsolate(value: String): String = "\u2068$value\u2069"
private fun ltrIsolate(value: String): String = "\u2066$value\u2069"
