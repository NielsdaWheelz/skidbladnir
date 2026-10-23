package dev.niels.skidbladnir

import androidx.compose.foundation.background
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.waitForUpOrCancellation
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
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
import androidx.compose.material3.pulltorefresh.pullToRefresh
import androidx.compose.material3.pulltorefresh.rememberPullToRefreshState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

@Composable
internal fun DashboardScreen(
    state: SkidbladnirUiState.Dashboard,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
    onNotificationSetup: () -> Unit,
) {
    val context = LocalContext.current
    val openNotificationSettings = { context.startActivity(context.notificationOwner().settingsIntent()) }
    var machineSelection by remember { mutableStateOf<DashboardMachineSelection?>(null) }
    val machines = state.machines
    Box(modifier = Modifier.fillMaxSize().background(Ink).systemBarsPadding()) {
        Column(modifier = Modifier.fillMaxSize()) {
            DashboardTopBar(
                onMachines = { machineSelection = DashboardMachineSelection.Machines },
                onNotificationSettings = openNotificationSettings,
            )

            DashboardViewStrip(dashboardViews(machines, entry.view), entry.view, entry::selectView)
            machines.forEach { machine ->
                key(machine.machine.handle) {
                    machineNotice(machine)?.let { MachineNoticeText(it, Modifier.padding(horizontal = 28.dp, vertical = 2.dp)) }
                }
            }
            if (controller.notificationHealth !in setOf(NotificationHealth.Ready, NotificationHealth.Disabled)) NoticePanel(
                tone = NoticeTone.Degraded, body = notificationHealthMessage(controller.notificationHealth),
                actions = {
                    TextButton(onClick = onNotificationSetup) { Text("set up notifications") }
                },
            )

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

            DwarfCollection(state, entry, controller)
        }

        // The margin is the wrapper's, so the seal's semantics bounds stay the 56dp the grid's clearance measures.
        Box(modifier = Modifier.align(Alignment.BottomEnd).padding(16.dp)) {
            ForgeSeal(canForge = machines.any(MachineState::canMutate), onClick = controller::openForge)
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
    state.forge?.let { ForgeSheet(it, machines, controller) }
    state.groupEditor?.let { edit ->
        GroupSheet(edit, machines.single { it.machine.handle == edit.target.machineHandle }, observedGroups(machines), controller)
    }
    state.close?.let { close ->
        CloseConfirmation(
            state = close,
            actionAdmissible = machines.singleOrNull { it.machine.handle == close.target.machineHandle }?.canMutate == true,
            onDismiss = controller::dismissClose,
            onConfirm = controller::confirmClose,
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DwarfCollection(
    state: SkidbladnirUiState.Dashboard,
    entry: DashboardEntryState,
    controller: SkidbladnirController,
) {
    val machines = state.machines
    val items = dashboardItems(machines, entry.view)
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
        if (entry.restorationPending) controller.restoreDashboardOnce(keys, recoveryNotices.size)
    }
    if (entry.restorationPending || needsInputView && items.isEmpty() && !state.needsInputSettled) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            CircularProgressIndicator()
        }
        return
    }
    val motionEnabled = rememberMotionEnabled()
    val pull = rememberPullToRefreshState()
    // A gesture that starts on a recovery notice cannot pull; the next down re-enables it, so the notice's fling stays excluded.
    var pullEnabled by remember { mutableStateOf(true) }
    // M3 calls onRefresh on release even when disabled, so a release after the last Ready machine left must not refresh.
    val pullable = machines.any { it.access == MachineAccess.Ready }
    Box(
        modifier = Modifier.fillMaxSize()
            .pointerInput(Unit) {
                awaitEachGesture {
                    awaitFirstDown(requireUnconsumed = false, pass = PointerEventPass.Initial)
                    pullEnabled = true
                    waitForUpOrCancellation(pass = PointerEventPass.Initial)
                }
            }
            .pullToRefresh(
                isRefreshing = state.refreshing,
                state = pull,
                enabled = pullEnabled && pullable,
                onRefresh = { if (pullEnabled && pullable && !state.refreshing) controller.verifyVisibleInventory() },
            ),
    ) {
        val topPadding = 12.dp
        // The trailing clearance (16dp margin + 56dp seal + 12dp gap) lets the last row scroll clear of the seal.
        val sealClearance = 84.dp
        BoxWithConstraints(Modifier.fillMaxSize()) {
            val emptyItemHeight = (maxHeight - topPadding - sealClearance).coerceAtLeast(0.dp)
            // One card per row on a phone; wider windows take further columns of the same card.
            LazyVerticalGrid(
                columns = GridCells.Adaptive(300.dp),
                modifier = Modifier.fillMaxSize(),
                state = entry.gridState,
                contentPadding = PaddingValues(
                    start = 12.dp,
                    top = topPadding,
                    end = 12.dp,
                    bottom = sealClearance,
                ),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp),
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
                                    pullEnabled = false
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
                                    style = MaterialTheme.typography.labelMedium,
                                    modifier = Modifier.fillMaxWidth(), maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                            is DashboardItem.Session -> {
                                val visible = item.visible
                                val target = visible.target
                                val machine = machines.single { it.machine.handle == target.machineHandle }
                                SessionCard(
                                    visible,
                                    machine,
                                    needsInputView = needsInputView,
                                    motionEnabled = motionEnabled,
                                    actions = listOf(
                                        MenuAction("change group", machine.canMutate, destructive = false) {
                                            controller.openGroupEditor(target)
                                        },
                                    ) + terminalLifetimeActions(
                                        machine.canMutate && !state.terminalControlPending,
                                        onStop = { controller.stopTerminal(target) },
                                        onTerminalClose = { controller.requestTerminalClose(target) },
                                        onClose = { controller.requestClose(target) },
                                    ),
                                    onOpen = { controller.openTerminal(target) },
                                )
                            }
                        }
                    }
                }
            }
        }
        val bar = Modifier.align(Alignment.TopCenter).fillMaxWidth().padding(horizontal = 12.dp).height(2.dp)
        if (state.refreshing && motionEnabled) {
            LinearProgressIndicator(
                modifier = bar,
                color = Gold,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Butt,
                gapSize = 0.dp,
            )
        } else if (state.refreshing || pull.distanceFraction > 0f) {
            // Without motion a refresh draws the complete static band.
            LinearProgressIndicator(
                progress = { if (state.refreshing) 1f else pull.distanceFraction.coerceIn(0f, 1f) },
                modifier = bar,
                color = Gold,
                trackColor = Color.Transparent,
                strokeCap = StrokeCap.Butt,
                gapSize = 0.dp,
                drawStopIndicator = {},
            )
        }
    }
}

@Composable
private fun DashboardTopBar(
    onMachines: () -> Unit,
    onNotificationSettings: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        HlidskjalfMark(color = Gold, markSize = 24.dp)
        Text(
            "Dwarves",
            modifier = Modifier.weight(1f),
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        ActionMenuButton(
            description = "dashboard options",
            actions = listOf(
                MenuAction("machines", enabled = true, destructive = false, onMachines),
                MenuAction("notification settings", enabled = true, destructive = false, onNotificationSettings),
            ),
            markCenterY = 24.dp,
        )
    }
}

@Composable
private fun DashboardViewStrip(
    views: List<DashboardViewSelection>,
    view: DashboardViewSelection,
    onSelect: (DashboardViewSelection) -> Unit,
) {
    val selectedChip = remember { BringIntoViewRequester() }
    BoxWithConstraints(Modifier.fillMaxWidth()) {
        // Only selection, its resolved name or the viewport width reveals the chip; inventory polls never move the strip.
        LaunchedEffect(view.key, view.displayLabel(), maxWidth) { selectedChip.bringIntoView() }
        val maximumWidth = (maxWidth - 32.dp).coerceAtLeast(48.dp)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            views.forEach { choice ->
                key(choice.key) {
                    val selected = choice.key == view.key
                    val label = choice.displayLabel()
                    Box(
                        modifier = Modifier
                            .widthIn(min = 48.dp, max = maximumWidth)
                            .heightIn(min = 48.dp)
                            .then(if (selected) Modifier.bringIntoViewRequester(selectedChip) else Modifier)
                            .selectable(
                                selected = selected,
                                interactionSource = null,
                                indication = AngularIndication(NidavellirShapes.Chip),
                                onClick = { onSelect(choice) },
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Surface(
                            color = if (selected) RaisedSurface else Color.Transparent,
                            shape = NidavellirShapes.Chip,
                            border = if (selected) BorderStroke(1.dp, Gold) else null,
                            modifier = Modifier.widthIn(min = 48.dp).heightIn(min = 32.dp),
                        ) {
                            Text(
                                label, color = if (selected) Gold else Bone,
                                style = MaterialTheme.typography.labelLarge,
                                textAlign = TextAlign.Center,
                                maxLines = 1, overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 6.dp),
                            )
                        }
                    }
                }
            }
        }
    }
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

private sealed interface DashboardMachineSelection {
    data object Machines : DashboardMachineSelection
    data class Details(val handle: MachineHandle) : DashboardMachineSelection
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DashboardMachineSheet(
    machines: List<MachineState>,
    selection: DashboardMachineSelection,
    onSelect: (DashboardMachineSelection) -> Unit,
    onDismiss: () -> Unit,
    onReconnect: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        shape = NidavellirShapes.Sheet,
        properties = ModalBottomSheetProperties(
            shouldDismissOnBackPress = selection == DashboardMachineSelection.Machines,
        ),
    ) {
        when (selection) {
            DashboardMachineSelection.Machines -> SheetForm {
                SheetTitle("machines")
                machines.forEach { machine ->
                    key(machine.machine.handle) {
                        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                            MachinePressureRail(
                                machine.machine, machine.pressure,
                                onOpenDetails = { onSelect(DashboardMachineSelection.Details(machine.machine.handle)) },
                                modifier = Modifier.fillMaxWidth(),
                            )
                            machineNotice(machine)?.let { MachineNoticeText(it, Modifier.padding(horizontal = 12.dp)) }
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
    // No ornament near destructive surfaces: the cut-corner shape only.
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
    )
}

@Composable
private fun EmptyState(
    title: String,
    body: MachineNotice?,
    ornament: Boolean = false,
) {
    Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            // The top bar's mark, only for a genuinely empty hall, never beside degraded or repair states.
            if (ornament) {
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

private fun forgeRecoveryMessage(
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

/** The empty state's wait copy: one Failure among the machines makes the whole notice Failure, so it never whispers what the strip above shouts. */
private fun dashboardInventoryWaitCopy(machines: List<MachineState>): MachineNotice? {
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
