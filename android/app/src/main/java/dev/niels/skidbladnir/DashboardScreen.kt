package dev.niels.skidbladnir

import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.waitForUpOrCancellation
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.pulltorefresh.PullToRefreshState
import androidx.compose.material3.pulltorefresh.pullToRefresh
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlin.math.abs

@Composable
internal fun DashboardScreen(
    state: SkidbladnirUiState.Dashboard,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
    onOpenTerminal: (SessionTarget) -> Unit,
) {
    DashboardMain(state, entry, controller, controller::verifyVisibleInventory, onOpenTerminal)

    state.forge?.let { forge ->
        ForgeSheet(
            state = forge,
            machines = state.machines,
            actions = ForgeSheetActions(
                dismiss = controller::dismissForge,
                updateDraft = controller::updateForgeDraft,
                submit = controller::forge,
                focusWorkingDirectory = controller::focusWorkingDirectory,
                chooseWorkingDirectory = controller::chooseWorkingDirectory,
                browseWorkingDirectoryHome = controller::browseWorkingDirectoryHome,
                workingDirectory = WorkingDirectoryPickerActions(
                    openChild = controller::openWorkingDirectoryChild,
                    openParent = controller::openWorkingDirectoryParent,
                    retry = controller::retryWorkingDirectory,
                    updateFilter = controller::updateWorkingDirectoryFilter,
                    setHidden = controller::setWorkingDirectoryHidden,
                    updateViewport = controller::updateWorkingDirectoryViewport,
                    enterPath = controller::cancelWorkingDirectoryPicker,
                    useCurrent = controller::useCurrentWorkingDirectory,
                    back = controller::workingDirectoryBack,
                    cancel = controller::cancelWorkingDirectoryPicker,
                ),
            ),
        )
    }
    state.groupEditor?.let { editor ->
        GroupSheet(
            editor = editor,
            machine = state.machines.single { it.machine.handle == editor.target.machineHandle },
            labels = observedGroups(state.machines),
            onChange = controller::updateGroupDraft,
            onDismiss = controller::dismissGroupEditor,
            onSubmit = controller::submitGroup,
        )
    }
    state.close?.let { close ->
        CloseConfirmation(
            state = close,
            actionAdmissible = state.machines.singleOrNull {
                it.machine.handle == close.target.machineHandle
            }?.canMutate == true,
            onDismiss = controller::dismissClose,
            onConfirm = controller::confirmClose,
        )
    }
}
@Composable
internal fun DashboardMain(
    state: SkidbladnirUiState.Dashboard,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
    onVerify: () -> Unit,
    onOpenTerminal: (SessionTarget) -> Unit,
) {
    var machineSelection by remember { mutableStateOf<DashboardMachineSelection?>(null) }
    val machines = state.machines
    val items = dashboardItems(machines, entry.view)
    val canForge = machines.any(MachineState::canForge)
    Box(modifier = Modifier.fillMaxSize().background(Ink).systemBarsPadding()) {
        Column(modifier = Modifier.fillMaxSize()) {
            DashboardTopBar(
                summary = dashboardSummary(items.count { it is DashboardItem.Session }, machines.size),
                onMachines = { machineSelection = DashboardMachineSelection.Machines },
            )

            DashboardViewStrip(machines, entry.view, entry::selectView)
            machines.forEach { machine ->
                key(machine.machine.handle) {
                    MachineStrip(machine)
                }
            }
            if (state.notificationsUnavailable) {
                NoticePanel(tone = NoticeTone.Degraded, body = "notifications unavailable")
            }

            state.notice?.let { NoticePanel(tone = NoticeTone.Failure, body = it) }

            state.forgeRecovery?.let { recovery ->
                NoticePanel(
                    tone = NoticeTone.Armed,
                    body = forgeRecoveryMessage(state, recovery),
                    actions = if (recovery is ForgeRecovery.ReviewReady) {
                        {
                            TextButton(onClick = controller::resumeForgeRecovery) { Text("Resume draft") }
                            TextButton(onClick = controller::discardForgeRecovery) { Text("Discard") }
                        }
                    } else {
                        null
                    },
                )
            }

            DashboardDwarfCollection(
                state = state,
                entry = entry,
                items = items,
                onVerify = onVerify,
                onRestore = controller::restoreDashboardOnce,
                onOpen = onOpenTerminal,
                onClose = controller::requestClose,
                onStop = controller::stopTerminal,
                onTerminalClose = controller::requestTerminalClose,
                onGroup = controller::openGroupEditor,
            )
        }

        // The create affordance left the header for here (forge-seal.md,
        // "Placement and semantics"): anchored over the grid, and rendered in
        // every dashboard state including zero machines, where it is cold.
        // Absence is displayed, not hidden. The 16dp margin is the wrapper's,
        // not the seal's — padding threaded into ForgeSeal would grow its
        // semantics bounds past its ink, and the grid's trailing clearance is
        // measured against those bounds.
        Box(modifier = Modifier.align(Alignment.BottomEnd).padding(16.dp)) {
            ForgeSeal(canForge = canForge, onClick = controller::openForge)
        }
    }
    machineSelection?.let { selection ->
        DashboardMachineSheet(
            machines = machines,
            selection = selection,
            onSelect = { machineSelection = it },
            onDismiss = { machineSelection = null },
            onReconnect = {
                machineSelection = null
                controller.requestFleetReconnect()
            },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun DashboardDwarfCollection(
    state: SkidbladnirUiState.Dashboard,
    entry: DashboardEntryState,
    items: List<DashboardItem>,
    onVerify: () -> Unit,
    onRestore: (List<DashboardItemKey>, Int) -> Unit,
    onOpen: (SessionTarget) -> Unit,
    onClose: (SessionTarget) -> Unit,
    onStop: (SessionTarget) -> Unit,
    onTerminalClose: (SessionTarget) -> Unit,
    onGroup: (SessionTarget) -> Unit,
) {
    val machines = state.machines
    val needsInputView = entry.view == DashboardViewSelection.NeedsInput
    val keys = items.map(DashboardItem::key)
    val recoveryNotices = machines.mapNotNull { machine ->
        workspaceRecoveryNotice(machine)?.let { machine.machine.handle to it }
    }
    val restorationOutcomes = machines.map { machine ->
        Triple(machine.machine.handle, machine.access, machine.inventory)
    }
    LaunchedEffect(
        entry.restorationPending, entry.view.key, restorationOutcomes,
        state.needsInputSettled, keys, recoveryNotices.size,
    ) {
        if (entry.restorationPending) onRestore(keys, recoveryNotices.size)
    }
    if (entry.restorationPending || needsInputView && items.isEmpty() && !state.needsInputSettled) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            CircularProgressIndicator()
        }
        return
    }
    val motionEnabled = rememberMotionEnabled()
    if (machines.any { it.access == MachineAccess.Ready }) {
        PullableDwarfCollection(state = state, motionEnabled = motionEnabled, onVerify = onVerify) { onRecoveryGesture ->
            DashboardDwarfGrid(
                state,
                items,
                recoveryNotices,
                needsInputView,
                entry.gridState,
                motionEnabled,
                onRecoveryGesture,
                onOpen,
                onClose,
                onStop,
                onTerminalClose,
                onGroup,
            )
        }
    } else {
        DashboardDwarfGrid(
            state,
            items,
            recoveryNotices,
            needsInputView,
            entry.gridState,
            motionEnabled,
            {},
            onOpen,
            onClose,
            onStop,
            onTerminalClose,
            onGroup,
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PullableDwarfCollection(
    state: SkidbladnirUiState.Dashboard,
    motionEnabled: Boolean,
    onVerify: () -> Unit,
    content: @Composable (onRecoveryGesture: () -> Unit) -> Unit,
) {
    val pullState = rememberPullToRefreshState()
    var pullEnabled by remember { mutableStateOf(true) }
    Box(
        modifier = Modifier.fillMaxSize()
            .pointerInput(Unit) {
                awaitEachGesture {
                    awaitFirstDown(requireUnconsumed = false, pass = PointerEventPass.Initial)
                    // reset only at the next down, so a notice's release/fling stays excluded.
                    pullEnabled = true
                    waitForUpOrCancellation(pass = PointerEventPass.Initial)
                }
            }
            .pullToRefresh(
                isRefreshing = state.refreshing,
                state = pullState,
                enabled = pullEnabled,
                onRefresh = { if (pullEnabled && !state.refreshing) onVerify() },
            ),
    ) {
        content { pullEnabled = false }
        DwarfCollectionPullIndicator(
            state = pullState,
            isRefreshing = state.refreshing,
            motionEnabled = motionEnabled,
            modifier = Modifier.align(Alignment.TopCenter),
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DwarfCollectionPullIndicator(
    state: PullToRefreshState,
    isRefreshing: Boolean,
    motionEnabled: Boolean,
    modifier: Modifier = Modifier,
) {
    val indicatorModifier = modifier
        .fillMaxWidth()
        .padding(horizontal = 12.dp)
        .height(2.dp)
    when {
        isRefreshing && !motionEnabled -> LinearProgressIndicator(
            progress = { 1f },
            modifier = indicatorModifier.semantics {
                contentDescription = "Checking tmux sessions"
                progressBarRangeInfo = ProgressBarRangeInfo.Indeterminate
            },
            color = Gold,
            trackColor = Color.Transparent,
            strokeCap = StrokeCap.Butt,
            gapSize = 0.dp,
            drawStopIndicator = {},
        )
        isRefreshing -> LinearProgressIndicator(
            modifier = indicatorModifier.semantics {
                contentDescription = "Checking tmux sessions"
            },
            color = Gold,
            trackColor = Color.Transparent,
            strokeCap = StrokeCap.Butt,
            gapSize = 0.dp,
        )
        state.distanceFraction > 0f -> LinearProgressIndicator(
            progress = { state.distanceFraction.coerceIn(0f, 1f) },
            modifier = indicatorModifier,
            color = Gold,
            trackColor = Color.Transparent,
            strokeCap = StrokeCap.Butt,
            gapSize = 0.dp,
            drawStopIndicator = {},
        )
    }
}

@Composable
private fun DashboardDwarfGrid(
    state: SkidbladnirUiState.Dashboard,
    items: List<DashboardItem>,
    recoveryNotices: List<Pair<MachineHandle, MachineNotice>>,
    needsInputView: Boolean,
    gridState: LazyGridState,
    motionEnabled: Boolean,
    onRecoveryGesture: () -> Unit,
    onOpen: (SessionTarget) -> Unit,
    onClose: (SessionTarget) -> Unit,
    onStop: (SessionTarget) -> Unit,
    onTerminalClose: (SessionTarget) -> Unit,
    onGroup: (SessionTarget) -> Unit,
) {
    val machines = state.machines
    val topPadding = 12.dp
    // The Forge seal floats over the grid (forge-seal.md "Placement and
    // semantics"): cards pass beneath it while scrolling, and the trailing
    // clearance (16dp margin + 56dp seal + 12dp gap) lets the last row's
    // overflow scroll clear of it.
    val sealClearance = 84.dp
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val emptyItemHeight = (maxHeight - topPadding - sealClearance).coerceAtLeast(0.dp)
        // One card per row on a phone; wider windows take further columns of the same card.
        LazyVerticalGrid(
            columns = GridCells.Adaptive(300.dp),
            modifier = Modifier.fillMaxSize(),
            state = gridState,
            contentPadding = PaddingValues(
                start = 12.dp,
                top = topPadding,
                end = 12.dp,
                bottom = sealClearance,
            ),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(
                items = recoveryNotices,
                key = { "recovery:${it.first.encoded}" },
                span = { GridItemSpan(maxLineSpan) },
            ) { (_, notice) ->
                MachineNoticeText(
                    notice,
                    Modifier.fillMaxWidth()
                        .pointerInput(Unit) {
                            awaitEachGesture {
                                awaitFirstDown(requireUnconsumed = false, pass = PointerEventPass.Initial)
                                onRecoveryGesture()
                                waitForUpOrCancellation(pass = PointerEventPass.Initial)
                            }
                        }
                        .padding(horizontal = 16.dp, vertical = 2.dp),
                )
            }
            if (items.isEmpty()) {
                item(
                    key = "dashboard-empty-state",
                    span = { GridItemSpan(maxLineSpan) },
                ) {
                    Box(Modifier.fillMaxWidth().height(emptyItemHeight)) {
                        val wait = dashboardInventoryWaitCopy(machines)
                        when {
                            needsInputView -> EmptyState("no sessions currently need input in this view", wait)
                            wait != null -> EmptyState("no matching sessions in available inventory", wait)
                            else -> EmptyState(
                                "no sessions in this view",
                                MachineNotice(
                                    "Create a dwarf here, or launch tmux on the visible " +
                                        if (machines.size == 1) "machine." else "machines.",
                                    NoticeTone.Degraded,
                                ),
                                ornament = true,
                            )
                        }
                    }
                }
            } else {
                items(
                    items = items,
                    key = { it.key.encoded },
                    span = { if (it is DashboardItem.Heading) GridItemSpan(maxLineSpan) else GridItemSpan(1) },
                    contentType = { it::class },
                ) { item ->
                    when (item) {
                        is DashboardItem.Heading -> {
                            val label = item.label?.text ?: "unassigned"
                            Text(label, fontFamily = NidavellirType.Data, color = Muted,
                                modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp).semantics {
                                    heading()
                                    contentDescription = label
                                }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                        is DashboardItem.Session -> {
                            val visible = item.visible
                            val machine = state.machines.single { it.machine.handle == visible.target.machineHandle }
                            SessionCard(
                                visible,
                                machine,
                                state.machines,
                                needsInputView = needsInputView,
                                motionEnabled = motionEnabled,
                                terminalControlPending = state.terminalControlPending,
                                onOpen = { onOpen(visible.target) },
                                onGroup = { onGroup(visible.target) },
                                onStop = { onStop(visible.target) },
                                onTerminalClose = { onTerminalClose(visible.target) },
                                onClose = { onClose(visible.target) },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
internal fun DashboardTopBar(
    summary: String,
    onMachines: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The Hlíðskjálf mark on the surface it names (design-language.md §8):
        // Gold, decorative, and silent — "Dwarves" beside it carries the label.
        HlidskjalfMark(color = Gold, markSize = 24.dp)
        Column(modifier = Modifier.weight(1f)) {
            Text(
                "Dwarves",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.SemiBold,
                // both header lines remain single-line summaries; the row grows with text scale.
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                summary,
                color = Muted,
                style = MaterialTheme.typography.labelMedium,
                fontFamily = NidavellirType.Data,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        TextButton(onClick = onMachines) {
            Text("machines", maxLines = 1)
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
internal fun DashboardViewStrip(
    machines: List<MachineState>,
    view: DashboardViewSelection,
    onSelect: (DashboardViewSelection) -> Unit,
) {
    val labels = observedGroups(machines).toMutableList()
    val selectedGroup = view as? DashboardViewSelection.Named
    selectedGroup?.label?.let { if (it !in labels) labels.add(it) }
    val views = buildList<DashboardViewSelection> {
        add(DashboardViewSelection.NeedsInput)
        add(DashboardViewSelection.All)
        labels.sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }
            .forEach { add(DashboardViewSelection.Named(groupFingerprint(it), it)) }
        if (selectedGroup != null && selectedGroup.label == null && none { it.key == selectedGroup.key }) {
            add(selectedGroup)
        }
        if (view == DashboardViewSelection.Unassigned || machines.any { machine ->
                machine.inventory.lastSnapshot()?.inventory?.sessions.orEmpty().any { it.group == null }
            }) add(DashboardViewSelection.Unassigned)
    }
    val selectedChip = remember { BringIntoViewRequester() }
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        // Inventory can change the other tabs while the operator browses the strip.
        // Only selection, its resolved name, or viewport geometry requests a reveal.
        LaunchedEffect(view.key, view.displayLabel(), maxWidth) { selectedChip.bringIntoView() }
        val maximumWidth = (maxWidth - 32.dp).coerceAtLeast(48.dp)
        CompositionLocalProvider(LocalBringIntoViewSpec provides ViewStripBringIntoViewSpec) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .selectableGroup()
                    .padding(horizontal = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                views.forEach { choice ->
                    key(choice.key) {
                        val selected = choice.key == view.key
                        val named = choice is DashboardViewSelection.Named
                        val label = choice.displayLabel()
                        Surface(
                            color = if (selected) RaisedSurface else Color.Transparent,
                            shape = NidavellirShapes.Chip,
                            border = BorderStroke(1.dp, if (selected) Gold else Muted.copy(alpha = 0.4f)),
                            modifier = Modifier
                                .widthIn(min = 48.dp, max = maximumWidth)
                                .heightIn(min = 48.dp)
                                .then(if (selected) Modifier.bringIntoViewRequester(selectedChip) else Modifier)
                                .selectable(
                                    selected = selected,
                                    role = Role.Tab,
                                    interactionSource = remember { MutableInteractionSource() },
                                    indication = AngularIndication(NidavellirShapes.Chip),
                                    onClick = { onSelect(choice) },
                                )
                                .semantics { contentDescription = if (named) "group: $label" else label },
                        ) {
                            Row(
                                Modifier.padding(horizontal = 12.dp, vertical = 8.dp).clearAndSetSemantics {},
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                val color = if (selected) Gold else Bone
                                Text(
                                    label, color = color, fontFamily = NidavellirType.Data,
                                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                                    modifier = Modifier.weight(1f, fill = false),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

private object ViewStripBringIntoViewSpec : BringIntoViewSpec {
    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float {
        if (size > containerSize) return offset
        val trailingDistance = offset + size - containerSize
        return when {
            offset >= 0f && trailingDistance <= 0f -> 0f
            abs(offset) < abs(trailingDistance) -> offset
            else -> trailingDistance
        }
    }
}

@Composable
private fun MachineStrip(
    machine: MachineState,
    modifier: Modifier = Modifier.padding(horizontal = 28.dp, vertical = 2.dp),
) {
    val notice = machineNotice(machine) ?: return
    MachineNoticeText(notice, modifier)
}

@Composable
private fun MachineNoticeText(notice: MachineNotice, modifier: Modifier) {
    Text(
        notice.message,
        color = noticeToneColor(notice.tone),
        style = MaterialTheme.typography.labelMedium,
        modifier = modifier,
    )
}

internal sealed interface DashboardMachineSelection {
    data object Machines : DashboardMachineSelection
    data class Details(val handle: MachineHandle) : DashboardMachineSelection
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun DashboardMachineSheet(
    machines: List<MachineState>,
    selection: DashboardMachineSelection,
    onSelect: (DashboardMachineSelection) -> Unit,
    onDismiss: () -> Unit,
    onReconnect: () -> Unit,
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
        properties = ModalBottomSheetProperties(
            shouldDismissOnBackPress = selection == DashboardMachineSelection.Machines,
        ),
    ) {
        when (selection) {
            DashboardMachineSelection.Machines -> Column(
                Modifier.fillMaxWidth().verticalScroll(rememberScrollState())
                    .padding(horizontal = 20.dp).padding(bottom = 28.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text(
                    "machines", style = MaterialTheme.typography.headlineSmall,
                    fontFamily = NidavellirType.Display, fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.semantics { heading() },
                )
                machines.forEach { machine ->
                    key(machine.machine.handle) {
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            MachinePressureRail(
                                machine.machine, machine.pressure,
                                onOpenDetails = { onSelect(DashboardMachineSelection.Details(machine.machine.handle)) },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            MachineStrip(machine, Modifier.padding(horizontal = 12.dp))
                        }
                    }
                }
                TextButton(onClick = onReconnect, modifier = Modifier.align(Alignment.End)) {
                    Text("reconnect fleet")
                }
                TextButton(onClick = onDismiss, modifier = Modifier.align(Alignment.End)) {
                    Text("dismiss")
                }
            }
            is DashboardMachineSelection.Details -> {
                ModalBackHandler { onSelect(DashboardMachineSelection.Machines) }
                val machine = machines.single { it.machine.handle == selection.handle }
                MachinePressureDetails(machine.machine, machine.pressure, onDismiss)
            }
        }
    }
}

@Composable
internal fun CloseConfirmation(
    state: CloseState,
    actionAdmissible: Boolean,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    val verb = if (state.terminalOnly) TERMINAL_ONLY_CLOSE_ACTION else TERMINAL_CLOSE_ACTION
    // No ornament near destructive surfaces (design-language.md §7): the close
    // dialog carries the cut-corner shape and nothing decorative.
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(closeConfirmationTitle(state.machine.label, state.target, state.terminalOnly)) },
        text = {
            Text(when {
                state.pending -> "$verb is in progress on ${state.machine.label.text}."
                !actionAdmissible ->
                    "${state.machine.label.text} inventory is not fresh. $verb is disabled. " +
                        "Cancel, return to Dwarves, then pull down to check again."
                else -> closeConfirmationBody(state.machine.label, state.target, state.terminalOnly)
            })
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                enabled = actionAdmissible && !state.pending,
                colors = ButtonDefaults.buttonColors(
                    containerColor = noticeToneColor(NoticeTone.Failure),
                    contentColor = Ink,
                ),
                shape = NidavellirShapes.Cleft,
            ) {
                Text(if (state.pending) "$verb in progress…" else verb)
            }
        },
        dismissButton = { OutlinedButton(onClick = onDismiss, enabled = !state.pending) { Text("Cancel") } },
        shape = NidavellirShapes.Card,
        containerColor = DeepSurface,
    )
}

@Composable
internal fun EmptyState(
    title: String,
    body: MachineNotice?,
    ornament: Boolean = false,
) {
    Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            if (ornament) {
                // The same mark the top bar carries, at the one size the
                // empty hall deserves. It renders only when the inventory is
                // genuinely empty, never beside degraded or repair states.
                HlidskjalfMark(
                    color = Muted.copy(alpha = 0.40f),
                    markSize = 48.dp,
                    modifier = Modifier.padding(bottom = 12.dp),
                )
            }
            Text(title, style = MaterialTheme.typography.titleLarge)
            body?.let { Text(it.message, color = noticeToneColor(it.tone), modifier = Modifier.padding(top = 8.dp)) }
        }
    }
}

internal fun forgeRecoveryMessage(
    dashboard: SkidbladnirUiState.Dashboard,
    recovery: ForgeRecovery,
): String {
    val target = dashboard.machines.singleOrNull {
        it.machine.handle == recovery.draft.machineHandle
    }
    val label = target?.machine?.label?.text ?: "Machine"
    return when (recovery) {
        is ForgeRecovery.RefreshRequired -> {
            val repair = when (target?.access) {
                null, MachineAccess.IdentityChanged ->
                    "Fleet reset is required before reviewing this draft."
                MachineAccess.AuthRequired -> "Reconnect fleet before reviewing this draft."
                MachineAccess.Ready -> "Pull down to check again before reviewing this draft."
            }
            "$label: create outcome unknown. $repair"
        }
        is ForgeRecovery.ReviewReady ->
            "$label refreshed. Review its sessions before resuming this draft."
    }
}

internal fun dashboardSummary(sessionCount: Int, machineCount: Int): String =
    "$sessionCount tmux ${if (sessionCount == 1) "session" else "sessions"} across " +
        "$machineCount ${if (machineCount == 1) "machine" else "machines"}"

// Its own prose again, and its own concatenation — but not its own tone. The strip and this
// empty state can be on screen together naming the same machine, so a bearer failure that the
// strip paints Failure cannot be whispered here; one Failure among the machines carries the
// whole notice, since the loudest unresolved state is the one the reader must act on.
internal fun dashboardInventoryWaitCopy(machines: List<MachineState>): MachineNotice? {
    val waiting = machines.mapNotNull { machine ->
        val label = machine.machine.label.text
        val availability = machineAvailability(machine)
        when (availability) {
            MachineAvailability.Ready -> null
            MachineAvailability.Refreshing -> "$label: confirming the latest tmux inventory."
            MachineAvailability.AuthRequired -> "$label: authentication required; its sessions may be out of date."
            MachineAvailability.IdentityChanged -> "$label: identity changed; fleet reset is required."
            MachineAvailability.Reading -> "$label: reading tmux sessions."
            is MachineAvailability.Stale ->
                "$label: showing its last inventory; it is STALE and actions are disabled."
            is MachineAvailability.Unavailable -> "$label: unavailable; its sessions cannot be read."
        }?.let { it to availabilityTone(availability) }
    }
    if (waiting.isEmpty()) return null
    val tone = if (waiting.any { it.second == NoticeTone.Failure }) NoticeTone.Failure else NoticeTone.Degraded
    return MachineNotice(waiting.joinToString(" ") { it.first }, tone)
}

internal fun forgeMachineChoiceLabel(machine: MachineState): String = machine.machine.label.text + when (
    machineAvailability(machine)
) {
    MachineAvailability.Ready -> ""
    MachineAvailability.Refreshing -> " · REFRESHING"
    MachineAvailability.AuthRequired -> " · AUTH REQUIRED"
    MachineAvailability.IdentityChanged -> " · IDENTITY CHANGED"
    MachineAvailability.Reading -> " · READING"
    is MachineAvailability.Stale -> " · STALE"
    is MachineAvailability.Unavailable -> " · UNAVAILABLE"
}

// Its own prose, not machineNotice's: the Forge names the disabled draft fields
// where the strip names the machine. The tone is NOT its own — it defers to
// availabilityTone, so the two surfaces cannot disagree about how loud the same
// machine state is, which is the class of drift this delta exists to end.
internal fun forgeUnavailableCopy(machine: MachineState): MachineNotice? {
    val label = machine.machine.label.text
    val availability = machineAvailability(machine)
    val tone = availabilityTone(availability)
    return when (availability) {
        MachineAvailability.Ready -> null
        MachineAvailability.Refreshing -> MachineNotice(
            "$label is confirming its latest tmux inventory. Draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.AuthRequired -> MachineNotice(
            "$label needs the fleet reconnected. Draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.IdentityChanged -> MachineNotice(
            "$label identity changed. Fleet reset is required; draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.Reading -> MachineNotice(
            "$label is reading tmux sessions. Draft fields and Create are disabled until the inventory is fresh.",
            tone,
        )
        is MachineAvailability.Stale -> MachineNotice(
            "$label inventory is STALE. Draft fields and Create are disabled until a fresh read succeeds.",
            tone,
        )
        is MachineAvailability.Unavailable -> MachineNotice(
            "$label is unavailable. Draft fields and Create are disabled until it reconnects.",
            tone,
        )
    }
}
