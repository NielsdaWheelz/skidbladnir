package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.dp

private const val MAXIMUM_FILTER_SCALARS = 256

/** A folder row's key: its server ordinal, so a filter or hidden change keeps the row Compose anchors on. */
private fun folderKey(ordinal: Int) = "folder:$ordinal"

/** The forge's folder browser; every way back to the form ends at the directory field. */
@Composable
internal fun WorkingDirectoryPickerScreen(
    picker: WorkingDirectoryPickerState,
    controller: SkidbladnirController,
    enabled: Boolean,
    onReturnToField: () -> Unit,
) {
    val load = picker.load
    // The folder on screen, if any, and the folder last asked for.
    val (view, candidate) = when (load) {
        is DirectoryLoad.Loaded -> load.view to load.view.listing.directory
        is DirectoryLoad.Loading -> load.retained to load.candidate
        is DirectoryLoad.Failed -> load.retained to load.candidate
    }
    // Parent, folders and Use act on the folder on screen, so they wait while another loads.
    val browse = enabled && load !is DirectoryLoad.Loading
    val visible = view?.let(::visibleWorkingDirectoryEntries).orEmpty()
    val scroll = view?.scroll ?: rememberLazyListState()
    // Compose keeps the first visible row by key; return to the top unless that row is a folder that stays.
    fun refine(next: DirectoryView) {
        val first = scroll.layoutInfo.visibleItemsInfo.firstOrNull { it.index == scroll.firstVisibleItemIndex }?.key
        if (visibleWorkingDirectoryEntries(next).none { folderKey(it.index) == first }) scroll.requestScrollToItem(0)
    }
    val back = { if (controller.workingDirectoryBack()) onReturnToField() }
    val cancel = { if (controller.cancelWorkingDirectoryPicker()) onReturnToField() }
    ModalBackHandler(back)
    Column(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                TextButton(onClick = back, modifier = Modifier.heightIn(min = 48.dp)) { Text("Back") }
                Spacer(Modifier.weight(1f))
                TextButton(onClick = cancel, modifier = Modifier.heightIn(min = 48.dp)) { Text("Cancel") }
            }
            SheetTitle("Browse Home", Modifier.padding(horizontal = 8.dp))
            Text("On ${picker.machine.label.text}", color = Muted, style = MaterialTheme.typography.labelLarge,
                modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp))
        }
        Column(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(if (view == null) "Requested folder" else "Current folder", color = Muted,
                style = MaterialTheme.typography.labelLarge)
            WorkingDirectoryPathLine((view?.listing?.directory ?: candidate).encoded, Modifier.fillMaxWidth())
            when (load) {
                is DirectoryLoad.Loaded -> if (visible.isEmpty()) Text("No visible folders here.", color = Muted)
                is DirectoryLoad.Loading -> Text("Opening “${load.candidate.displayName}”…", color = Muted)
                is DirectoryLoad.Failed -> {
                    Text("Could not open “${load.candidate.displayName}”.", color = Muted)
                    val (body, retry) = when (load.failure) {
                        DirectoryBrowseFailure.Transport -> "Could not reach this machine over your Tailnet." to true
                        DirectoryBrowseFailure.Unavailable ->
                            "This directory cannot be browsed. Enter the path instead." to false
                        DirectoryBrowseFailure.TooLarge ->
                            "This directory has too many folders to show. Enter the path instead." to false
                        DirectoryBrowseFailure.Internal -> "Skíðblaðnir could not complete the request." to true
                    }
                    NoticePanel(tone = NoticeTone.Failure, body = body, actions = if (!retry) null else {
                        {
                            TextButton(onClick = controller::retryWorkingDirectory, enabled = enabled,
                                modifier = Modifier.heightIn(min = 48.dp)) { Text("Try again") }
                        }
                    })
                }
            }
        }
        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f),
            state = scroll,
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            if (view?.listing?.parent is ParentDirectory.Available) {
                item(key = "parent") { PickerActionRow("Parent folder", browse, controller::openWorkingDirectoryParent) }
            }
            item(key = "enter-path") { PickerActionRow("Enter path", true, cancel) }
            if (view == null) return@LazyColumn
            item(key = "filter") {
                OutlinedTextField(
                    value = view.filter,
                    onValueChange = { filter ->
                        if (filter.codePointCount(0, filter.length) > MAXIMUM_FILTER_SCALARS) return@OutlinedTextField
                        refine(view.copy(filter = filter))
                        controller.updateWorkingDirectoryFilter(filter)
                    },
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text("Filter folders") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(autoCorrectEnabled = false),
                )
            }
            if (view.listing.children.any { it.directory.hidden }) {
                item(key = "hidden") {
                    FilterChip(
                        selected = view.showHidden,
                        onClick = {
                            refine(view.copy(showHidden = !view.showHidden))
                            controller.setWorkingDirectoryHidden(!view.showHidden)
                        },
                        label = { Text(if (view.showHidden) "Hide hidden folders" else "Show hidden folders") },
                        modifier = Modifier.heightIn(min = 48.dp),
                    )
                }
            }
            if (view.listing.omissions == DirectoryOmissions.Present) {
                item(key = "omission") { NoticePanel(tone = NoticeTone.Degraded, body = "Some folders cannot be shown.") }
            }
            items(visible, key = { folderKey(it.index) }) { entry ->
                PickerActionRow(entry.value.directory.basename, browse) {
                    controller.openWorkingDirectoryChild(entry.value.directory)
                }
            }
        }
        if (view != null) {
            Button(
                onClick = { if (controller.useCurrentWorkingDirectory()) onReturnToField() },
                enabled = browse,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp).fillMaxWidth().heightIn(min = 48.dp),
                shape = NidavellirShapes.Chip,
            ) { Text(if (view.listing.directory == HomeDirectory.Home) "Use Home" else "Use this folder") }
        }
    }
}

@Composable
private fun PickerActionRow(label: String, enabled: Boolean, onClick: () -> Unit) {
    Surface(
        color = RaisedSurface,
        border = BorderStroke(1.dp, Gold.copy(alpha = if (enabled) 0.40f else 0.18f)),
        shape = NidavellirShapes.Chip,
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clickable(
                interactionSource = null,
                indication = AngularIndication(NidavellirShapes.Chip),
                enabled = enabled,
                onClick = onClick,
            ),
    ) {
        Box(Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 10.dp), contentAlignment = Alignment.CenterStart) {
            Text(label, style = MaterialTheme.typography.bodyLarge, color = if (enabled) Bone else Muted)
        }
    }
}

/** One line of path, scrolled to its end so the deepest folder shows. */
@Composable
internal fun WorkingDirectoryPathLine(path: String, modifier: Modifier = Modifier) {
    val scrollState = rememberScrollState()
    LaunchedEffect(path, scrollState.maxValue) {
        scrollState.scrollTo(scrollState.maxValue)
    }
    Row(
        modifier.heightIn(min = 48.dp).horizontalScroll(scrollState),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            path,
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

private val HomeDirectory.displayName: String
    get() = if (this == HomeDirectory.Home) "Home" else basename
