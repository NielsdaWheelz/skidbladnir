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
import androidx.compose.foundation.text.KeyboardActions
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
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.flow.distinctUntilChanged

internal class WorkingDirectoryPickerActions(
    val browseHome: () -> Unit,
    val openChild: (HomeDirectory) -> Unit,
    val openParent: () -> Unit,
    val retry: () -> Unit,
    val updateFilter: (String) -> Unit,
    val setHidden: (Boolean) -> Unit,
    val updateViewport: (DirectoryViewport) -> Unit,
    val showExact: () -> Unit,
    val updateExact: (String) -> Unit,
    val chooseActive: (WorkingDirectoryPath) -> Unit,
    val useCurrent: () -> Unit,
    val useExact: () -> Unit,
    val back: () -> Unit,
    val cancel: () -> Unit,
)

private data class PickerChrome(
    val title: String,
    val machineVisual: String,
    val machineSpoken: String,
    val backLabel: String,
    val cancelLabel: String,
)

private sealed interface PickerContent {
    val chrome: PickerChrome
    val rows: List<PickerRow>
    val useAction: PickerUseAction?

    data class Places(
        override val chrome: PickerChrome,
        override val rows: List<PickerRow>,
    ) : PickerContent {
        override val useAction: PickerUseAction? = null
    }

    data class Browse(
        override val chrome: PickerChrome,
        override val rows: List<PickerRow>,
        override val useAction: PickerUseAction?,
        val context: PickerBrowseContext,
        val view: DirectoryView?,
        val actionsEnabled: Boolean,
    ) : PickerContent

    data class Exact(
        override val chrome: PickerChrome,
        override val rows: List<PickerRow>,
        override val useAction: PickerUseAction,
    ) : PickerContent
}

private data class PickerUseAction(
    val label: String,
    val contentDescription: String?,
    val enabled: Boolean,
    val action: PickerUseKind,
)

private enum class PickerUseKind { Current, Exact }

private sealed interface PickerAction {
    data object BrowseHome : PickerAction
    data class Active(val directory: WorkingDirectoryPath) : PickerAction
    data object Exact : PickerAction
    data object Parent : PickerAction
    data class Folder(val directory: HomeDirectory) : PickerAction
}

private sealed interface PickerRowLabel {
    data class Text(val visual: String) : PickerRowLabel
    data class Path(val raw: String, val tag: String) : PickerRowLabel
}

private sealed interface BrowseStatus {
    data object Ready : BrowseStatus
    data class Supporting(
        val visual: String,
        val spoken: String,
    ) : BrowseStatus

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
        val label: PickerRowLabel,
        val contentDescription: String,
        val action: PickerAction,
        val tag: String? = null,
    ) : PickerRow

    data class Heading(
        override val key: PickerRowKey,
        val visual: String,
        val spoken: String,
    ) : PickerRow

    data class Filter(
        val value: String,
        val label: String,
    ) : PickerRow {
        override val key = PickerRowKey.Filter
    }

    data class Hidden(
        val shown: Boolean,
        val label: String,
    ) : PickerRow {
        override val key = PickerRowKey.Hidden
    }

    data class Notice(
        override val key: PickerRowKey,
        val tone: NoticeTone,
        val body: String,
    ) : PickerRow

    data class ExactField(
        val draft: String,
        val validation: ExactPathValidation,
        val label: String,
        val guidance: String,
        val invalidBody: String,
        val invalidTone: NoticeTone,
    ) : PickerRow {
        override val key = PickerRowKey.ExactField
    }
}

private sealed interface PickerRowKey {
    data object BrowseHome : PickerRowKey
    data object ActiveHeading : PickerRowKey
    data class Active(val directory: WorkingDirectoryPath, val ordinal: Int) : PickerRowKey
    data object ExactAction : PickerRowKey
    data object Parent : PickerRowKey
    data object Filter : PickerRowKey
    data object Hidden : PickerRowKey
    data object Omission : PickerRowKey
    data class Folder(val directory: HomeDirectory, val ordinal: Int) : PickerRowKey
    data object ExactField : PickerRowKey
}

private fun PickerRowKey.saveableKey(): String = when (this) {
    PickerRowKey.BrowseHome -> "browse-home"
    PickerRowKey.ActiveHeading -> "active-heading"
    is PickerRowKey.Active -> "active:$ordinal"
    PickerRowKey.ExactAction -> "exact-action"
    PickerRowKey.Parent -> "parent"
    PickerRowKey.Filter -> "filter"
    PickerRowKey.Hidden -> "hidden"
    PickerRowKey.Omission -> "omission"
    is PickerRowKey.Folder -> "folder:$ordinal"
    PickerRowKey.ExactField -> "exact-field"
}

@Composable
internal fun WorkingDirectoryPickerScreen(
    picker: WorkingDirectoryPickerState,
    actions: WorkingDirectoryPickerActions,
    modifier: Modifier = Modifier,
) {
    val content = workingDirectoryPickerContent(picker)
    val rows = content.rows
    val listState = rememberLazyListState()
    val browse = content as? PickerContent.Browse
    val view = browse?.view
    var restoredDirectory by remember(picker.instance) {
        mutableStateOf<HomeDirectory?>(null)
    }
    val restorationReady = view == null || restoredDirectory == view.listing.directory

    RestoreWorkingDirectoryViewport(
        pickerInstance = picker.instance,
        view = view,
        rows = rows,
        state = listState,
        onRestored = { restoredDirectory = it },
    )
    CaptureWorkingDirectoryViewport(
        pickerInstance = picker.instance,
        view = view,
        rows = rows,
        state = listState,
        enabled = restorationReady,
        onViewport = actions.updateViewport,
    )

    Column(
        modifier
            .fillMaxSize()
            .imePadding()
            .testTag("working-directory-picker"),
    ) {
        PickerHeader(content.chrome, actions.back, actions.cancel)
        browse?.let { BrowseContext(it.context, actions.retry) }
        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f).testTag("working-directory-list"),
            state = listState,
            contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(rows, key = { row -> row.key.saveableKey() }) { row ->
                PickerRow(
                    row = row,
                    browseActionsEnabled = browse?.actionsEnabled == true && restorationReady,
                    actions = actions,
                )
            }
        }
        PickerUseAction(
            content = content,
            enabled = restorationReady,
            onUseCurrent = actions.useCurrent,
            onUseExact = actions.useExact,
        )
    }
}

@Composable
private fun PickerHeader(
    chrome: PickerChrome,
    onBack: () -> Unit,
    onCancel: () -> Unit,
) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 12.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            TextButton(
                onClick = onBack,
                modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()
                    .testTag("working-directory-back"),
            ) {
                Text(chrome.backLabel)
            }
            Spacer(Modifier.weight(1f))
            TextButton(
                onClick = onCancel,
                modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()
                    .testTag("working-directory-cancel"),
            ) {
                Text(chrome.cancelLabel)
            }
        }
        Text(
            chrome.title,
            style = MaterialTheme.typography.headlineSmall,
            fontFamily = NidavellirType.Display,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(horizontal = 8.dp),
        )
        Text(
            chrome.machineVisual,
            color = Muted,
            style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp).semantics {
                contentDescription = chrome.machineSpoken
            },
        )
    }
}

@Composable
private fun PickerRow(
    row: PickerRow,
    browseActionsEnabled: Boolean,
    actions: WorkingDirectoryPickerActions,
) {
    when (row) {
        is PickerRow.Action -> PickerActionRow(
            label = row.label,
            description = row.contentDescription,
            enabled = when (row.action) {
                PickerAction.Parent, is PickerAction.Folder -> browseActionsEnabled
                PickerAction.BrowseHome,
                is PickerAction.Active,
                PickerAction.Exact,
                -> true
            },
            tag = row.tag,
            onClick = when (val action = row.action) {
                PickerAction.BrowseHome -> actions.browseHome
                is PickerAction.Active -> ({ actions.chooseActive(action.directory) })
                PickerAction.Exact -> actions.showExact
                PickerAction.Parent -> actions.openParent
                is PickerAction.Folder -> ({ actions.openChild(action.directory) })
            },
        )
        is PickerRow.Heading -> Text(
            row.visual,
            color = Muted,
            style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.fillMaxWidth().padding(top = 4.dp).semantics {
                contentDescription = row.spoken
            },
        )
        is PickerRow.Filter -> OutlinedTextField(
            value = row.value,
            onValueChange = actions.updateFilter,
            modifier = Modifier.fillMaxWidth().testTag("working-directory-filter"),
            label = { Text(row.label) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(autoCorrectEnabled = false),
        )
        is PickerRow.Hidden -> FilterChip(
            selected = row.shown,
            onClick = { actions.setHidden(!row.shown) },
            label = { Text(row.label) },
            shape = NidavellirShapes.Chip,
            modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()
                .testTag("working-directory-hidden"),
        )
        is PickerRow.Notice -> NoticePanel(
            tone = row.tone,
            body = row.body,
        )
        is PickerRow.ExactField -> ExactPathField(
            content = row,
            onChange = actions.updateExact,
            onUse = actions.useExact,
        )
    }
}

@Composable
private fun PickerActionRow(
    label: PickerRowLabel,
    description: String,
    onClick: () -> Unit,
    enabled: Boolean = true,
    tag: String? = null,
) {
    Surface(
        color = RaisedSurface,
        border = BorderStroke(1.dp, Gold.copy(alpha = if (enabled) 0.40f else 0.18f)),
        shape = NidavellirShapes.Chip,
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .then(if (tag == null) Modifier else Modifier.testTag(tag))
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
            when (label) {
                is PickerRowLabel.Text -> Text(
                    label.visual,
                    style = MaterialTheme.typography.bodyLarge,
                    color = if (enabled) Bone else Muted,
                )
                is PickerRowLabel.Path -> WorkingDirectoryPathLine(
                    path = label.raw,
                    contentDescription = null,
                    modifier = Modifier.fillMaxWidth().testTag(label.tag),
                )
            }
        }
    }
}

@Composable
private fun BrowseContext(
    content: PickerBrowseContext,
    onRetry: () -> Unit,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 8.dp)
            .testTag("working-directory-live-region")
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
            modifier = Modifier.fillMaxWidth().testTag("working-directory-location"),
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
private fun ExactPathField(
    content: PickerRow.ExactField,
    onChange: (String) -> Unit,
    onUse: () -> Unit,
) {
    val focusRequester = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    LaunchedEffect(Unit) { focusRequester.requestFocus() }

    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        OutlinedTextField(
            value = content.draft,
            onValueChange = onChange,
            modifier = Modifier.fillMaxWidth().focusRequester(focusRequester)
                .testTag("working-directory-exact-field"),
            label = { Text(content.label) },
            supportingText = { Text(content.guidance) },
            isError = content.validation == ExactPathValidation.Invalid,
            singleLine = true,
            textStyle = TextStyle(
                fontFamily = NidavellirType.Data,
                textDirection = TextDirection.Ltr,
            ),
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
                keyboardType = KeyboardType.Uri,
                imeAction = ImeAction.Done,
            ),
            keyboardActions = KeyboardActions(
                onDone = {
                    if (content.validation == ExactPathValidation.Valid) keyboard?.hide()
                    onUse()
                },
            ),
        )
        if (content.validation == ExactPathValidation.Invalid) {
            Box(
                Modifier.testTag("working-directory-live-region")
                    .semantics { liveRegion = LiveRegionMode.Polite },
            ) {
                NoticePanel(
                    tone = content.invalidTone,
                    body = content.invalidBody,
                )
            }
        }
    }
}

@Composable
private fun PickerUseAction(
    content: PickerContent,
    enabled: Boolean,
    onUseCurrent: () -> Unit,
    onUseExact: () -> Unit,
) {
    val use = content.useAction ?: return
    Button(
        onClick = when (use.action) {
            PickerUseKind.Current -> onUseCurrent
            PickerUseKind.Exact -> onUseExact
        },
        enabled = enabled && use.enabled,
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
            .fillMaxWidth().heightIn(min = 48.dp).minimumInteractiveComponentSize()
            .testTag("working-directory-use")
            .then(
                use.contentDescription?.let { description ->
                    Modifier.semantics { contentDescription = description }
                } ?: Modifier,
            ),
        shape = NidavellirShapes.Chip,
    ) {
        Text(use.label)
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
    view: DirectoryView?,
    rows: List<PickerRow>,
    state: LazyListState,
    onRestored: (HomeDirectory?) -> Unit,
) {
    val directory = view?.listing?.directory
    LaunchedEffect(pickerInstance, directory) {
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
        onRestored(directory)
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

private fun workingDirectoryPickerContent(picker: WorkingDirectoryPickerState): PickerContent =
    when (val page = picker.page) {
        WorkingDirectoryPage.Places -> PickerContent.Places(
            chrome = pickerChrome(picker.machine, "Choose working directory"),
            rows = buildList {
                add(
                    PickerRow.Action(
                        key = PickerRowKey.BrowseHome,
                        label = PickerRowLabel.Text("Browse Home"),
                        contentDescription = "Browse Home. Opens Home folders on " +
                            "${picker.machine.label.text}.",
                        action = PickerAction.BrowseHome,
                    ),
                )
                if (picker.activeDirectories.isNotEmpty()) {
                    add(
                        PickerRow.Heading(
                            key = PickerRowKey.ActiveHeading,
                            visual = "Active on ${bidiIsolate(picker.machine.label.text)}",
                            spoken = "Active on ${picker.machine.label.text}",
                        ),
                    )
                    picker.activeDirectories.forEachIndexed { ordinal, directory ->
                        add(
                            PickerRow.Action(
                                key = PickerRowKey.Active(directory, ordinal),
                                label = PickerRowLabel.Path(
                                    directory.encoded,
                                    "working-directory-active-path-scroll",
                                ),
                                contentDescription = "Working directory ${directory.encoded}. " +
                                    "Selects this directory on ${picker.machine.label.text}.",
                                action = PickerAction.Active(directory),
                            ),
                        )
                    }
                }
                add(exactPathAction(picker.machine))
            },
        )
        is WorkingDirectoryPage.ExactPath -> PickerContent.Exact(
            chrome = pickerChrome(picker.machine, "Enter exact path"),
            rows = listOf(
                PickerRow.ExactField(
                    draft = picker.exactDraft,
                    validation = page.validation,
                    label = "Working directory",
                    guidance = "Use an absolute path or ~/…",
                    invalidBody = "Choose a valid working directory.",
                    invalidTone = NoticeTone.Failure,
                ),
            ),
            useAction = PickerUseAction(
                label = "Use path",
                contentDescription = null,
                enabled = page.validation == ExactPathValidation.Valid,
                action = PickerUseKind.Exact,
            ),
        )
        is WorkingDirectoryPage.Browsing -> when (val load = page.load) {
            is DirectoryLoad.Loaded -> browseContent(
                machine = picker.machine,
                locationIsCurrent = true,
                location = load.view.listing.directory,
                view = load.view,
                actionsEnabled = true,
                status = if (visibleWorkingDirectoryEntries(load.view).isEmpty()) {
                    BrowseStatus.Supporting(
                        visual = "No visible folders here.",
                        spoken = "No visible folders here.",
                    )
                } else {
                    BrowseStatus.Ready
                },
            )
            is DirectoryLoad.Loading -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> browseContent(
                    machine = picker.machine,
                    locationIsCurrent = false,
                    location = load.candidate,
                    view = null,
                    actionsEnabled = false,
                    status = openingStatus(load.candidate),
                )
                is RetainedDirectoryView.Present -> browseContent(
                    machine = picker.machine,
                    locationIsCurrent = true,
                    location = retained.view.listing.directory,
                    view = retained.view,
                    actionsEnabled = false,
                    status = openingStatus(load.candidate),
                )
            }
            is DirectoryLoad.Failed -> when (val retained = load.retained) {
                RetainedDirectoryView.None -> browseContent(
                    machine = picker.machine,
                    locationIsCurrent = false,
                    location = load.candidate,
                    view = null,
                    actionsEnabled = false,
                    status = browseFailureStatus(load.candidate, load.failure),
                )
                is RetainedDirectoryView.Present -> browseContent(
                    machine = picker.machine,
                    locationIsCurrent = true,
                    location = retained.view.listing.directory,
                    view = retained.view,
                    actionsEnabled = true,
                    status = browseFailureStatus(load.candidate, load.failure),
                )
            }
        }
    }

private fun pickerChrome(machine: PairedMachine, title: String): PickerChrome = PickerChrome(
    title = title,
    machineVisual = "On ${bidiIsolate(machine.label.text)}",
    machineSpoken = "On ${machine.label.text}",
    backLabel = "Back",
    cancelLabel = "Cancel",
)

private fun exactPathAction(machine: PairedMachine): PickerRow.Action = PickerRow.Action(
    key = PickerRowKey.ExactAction,
    label = PickerRowLabel.Text("Enter exact path"),
    contentDescription = "Enter an exact working directory on ${machine.label.text}.",
    action = PickerAction.Exact,
)

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
    machine: PairedMachine,
    locationIsCurrent: Boolean,
    location: HomeDirectory,
    view: DirectoryView?,
    actionsEnabled: Boolean,
    status: BrowseStatus,
): PickerContent.Browse {
    val locationLabel = if (locationIsCurrent) "Current folder" else "Requested folder"
    val rows = buildList {
        val parent = view?.listing?.parent as? ParentDirectory.Available
        if (parent != null) {
            add(
                PickerRow.Action(
                    key = PickerRowKey.Parent,
                    label = PickerRowLabel.Text("Parent folder"),
                    contentDescription = "Parent folder. Opens the parent folder.",
                    action = PickerAction.Parent,
                    tag = "working-directory-parent",
                ),
            )
        }
        add(exactPathAction(machine))
        if (view != null) {
            add(PickerRow.Filter(view.filter, "Filter folders"))
            if (workingDirectoryHasHiddenEntries(view)) {
                add(
                    PickerRow.Hidden(
                        shown = view.showHidden,
                        label = if (view.showHidden) {
                            "Hide hidden folders"
                        } else {
                            "Show hidden folders"
                        },
                    ),
                )
            }
            if (view.listing.omissions == DirectoryOmissions.Present) {
                add(
                    PickerRow.Notice(
                        key = PickerRowKey.Omission,
                        tone = NoticeTone.Degraded,
                        body = "Some folders cannot be shown.",
                    ),
                )
            }
            val serverOrdinals = view.listing.children.withIndex().associate { indexed ->
                indexed.value.directory to indexed.index
            }
            visibleWorkingDirectoryEntries(view).forEach { entry ->
                val name = entry.directory.basename
                add(
                    PickerRow.Action(
                        key = PickerRowKey.Folder(
                            entry.directory,
                            checkNotNull(serverOrdinals[entry.directory]),
                        ),
                        label = PickerRowLabel.Text(bidiIsolate(name)),
                        contentDescription = when (entry.kind) {
                            DirectoryEntryKind.Directory -> "Folder $name. Opens folder."
                            DirectoryEntryKind.SymbolicLink -> "Linked folder $name. Opens folder."
                        },
                        action = PickerAction.Folder(entry.directory),
                        tag = "working-directory-folder-row",
                    ),
                )
            }
        }
    }
    val useAction = view?.listing?.directory?.let { directory ->
        val home = directory == HomeDirectory.Home
        PickerUseAction(
            label = if (home) "Use Home" else "Use this folder",
            contentDescription = if (home) {
                "Use Home as working directory on ${machine.label.text}."
            } else {
                "Use ${directory.encoded} as working directory on ${machine.label.text}."
            },
            enabled = actionsEnabled,
            action = PickerUseKind.Current,
        )
    }
    return PickerContent.Browse(
        chrome = pickerChrome(machine, "Choose working directory"),
        rows = rows,
        useAction = useAction,
        context = PickerBrowseContext(
            locationLabel = locationLabel,
            directory = location,
            locationSpoken = "$locationLabel ${location.encoded}",
            status = status,
        ),
        view = view,
        actionsEnabled = actionsEnabled,
    )
}

private val HomeDirectory.displayName: String
    get() = if (this == HomeDirectory.Home) "Home" else basename

internal fun bidiIsolate(value: String): String = "\u2068$value\u2069"
private fun ltrIsolate(value: String): String = "\u2066$value\u2069"
