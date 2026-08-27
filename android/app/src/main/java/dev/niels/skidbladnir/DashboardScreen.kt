package dev.niels.skidbladnir

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyGridState
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.pulltorefresh.PullToRefreshDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshState
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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.SemanticsPropertyKey
import androidx.compose.ui.semantics.SemanticsPropertyReceiver
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import java.util.Locale

/**
 * The monotonic receipt of a machine's freshest inventory, published on that machine's real strip
 * header. It lets the acceptance journey observe that reads keep landing for one machine while
 * another is out, without adding an invisible node to the layout.
 */
internal val MachineInventoryObservationKey =
    SemanticsPropertyKey<Long>("SkidbladnirMachineInventoryObservation")
internal var SemanticsPropertyReceiver.machineInventoryObservation by MachineInventoryObservationKey

@Composable
internal fun DashboardScreen(state: SkidbladnirUiState.Dashboard, controller: SkidbladnirController) {
    DashboardMain(state, controller, controller::verifyVisibleInventory)

    state.forge?.let { forge ->
        ForgeSheet(forge, state.machines, controller::dismissForge, controller::updateForgeDraft, controller::forge)
    }
    state.kill?.let { kill ->
        KillConfirmation(
            state = kill,
            actionAdmissible = state.machines.singleOrNull {
                it.machine.handle == kill.target.machineHandle
            }?.canMutate == true,
            onDismiss = controller::dismissKill,
            onConfirm = controller::confirmKill,
        )
    }
}

@Composable
internal fun DashboardMain(
    state: SkidbladnirUiState.Dashboard,
    controller: SkidbladnirController,
    onVerify: () -> Unit,
) {
    val machines = state.machines.filter {
        state.selectedMachine == null || it.machine.handle == state.selectedMachine
    }
    val agents = visibleAgents(state.machines, state.selectedMachine)
    val canForge = machines.any(MachineState::canForge)
    Box(modifier = Modifier.fillMaxSize().background(Ink).systemBarsPadding()) {
        Column(modifier = Modifier.fillMaxSize()) {
            DashboardTopBar(
                summary = dashboardSummary(agents.size, machines.size),
            )

            MachineFilters(state.machines, state.selectedMachine, controller::selectMachine)
            machines.forEach { machine ->
                key(machine.machine.handle) {
                    MachineStrip(machine, controller, credentialWritesEnabled = state.unreadableMachines.isEmpty())
                }
            }
            state.unreadableMachines.forEach { UnreadableMachineStrip(it) }

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
                onVerify = onVerify,
                onOpen = controller::openTerminal,
                onKill = controller::requestKill,
            )
        }

        // The create affordance left the header for here (forge-seal.md,
        // "Placement and semantics"): anchored over the grid, and rendered in
        // every dashboard state including zero machines, where it is cold.
        // Absence is displayed, not hidden. The 16dp margin is the wrapper's,
        // not the seal's — padding threaded into ForgeSeal would grow its
        // semantics bounds past its ink, and the grid's bottom inset below is
        // measured against those bounds.
        Box(modifier = Modifier.align(Alignment.BottomEnd).padding(16.dp)) {
            ForgeSeal(canForge = canForge, onClick = controller::openForge)
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun DashboardDwarfCollection(
    state: SkidbladnirUiState.Dashboard,
    onVerify: () -> Unit,
    onOpen: (AgentTarget) -> Unit,
    onKill: (AgentTarget) -> Unit,
) {
    val machines = state.machines.filter {
        state.selectedMachine == null || it.machine.handle == state.selectedMachine
    }
    val agents = visibleAgents(state.machines, state.selectedMachine)
    val gridState = rememberLazyGridState()
    val topPadding = PullToRefreshDefaults.PositionalThreshold + 12.dp
    if (machines.any { it.access == MachineAccess.Ready }) {
        PullableDwarfCollection(state = state, onVerify = onVerify) {
            DashboardDwarfGrid(state, machines, agents, gridState, topPadding, onOpen, onKill)
        }
    } else {
        DashboardDwarfGrid(state, machines, agents, gridState, topPadding, onOpen, onKill)
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PullableDwarfCollection(
    state: SkidbladnirUiState.Dashboard,
    onVerify: () -> Unit,
    content: @Composable () -> Unit,
) {
    val pullState = rememberPullToRefreshState()
    PullToRefreshBox(
        isRefreshing = state.refreshing,
        onRefresh = {
            if (!state.refreshing) onVerify()
        },
        modifier = Modifier.fillMaxSize(),
        state = pullState,
        indicator = {
            DwarfCollectionPullIndicator(
                state = pullState,
                isRefreshing = state.refreshing,
                modifier = Modifier.align(Alignment.TopCenter),
            )
        },
    ) {
        content()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DwarfCollectionPullIndicator(
    state: PullToRefreshState,
    isRefreshing: Boolean,
    modifier: Modifier = Modifier,
) {
    PullToRefreshDefaults.IndicatorBox(
        state = state,
        isRefreshing = isRefreshing,
        modifier = modifier,
        containerColor = Color.Transparent,
        elevation = 0.dp,
    ) {
        when {
            isRefreshing -> CircularProgressIndicator(
                modifier = Modifier.size(24.dp).semantics {
                    contentDescription = "Checking tmux sessions"
                },
                color = Gold,
                strokeWidth = 2.dp,
            )
            state.distanceFraction > 0f -> CircularProgressIndicator(
                progress = { state.distanceFraction.coerceIn(0f, 1f) },
                modifier = Modifier.size(24.dp),
                color = Gold,
                strokeWidth = 2.dp,
                trackColor = Color.Transparent,
            )
        }
    }
}

@Composable
private fun DashboardDwarfGrid(
    state: SkidbladnirUiState.Dashboard,
    machines: List<MachineState>,
    agents: List<VisibleAgent>,
    gridState: LazyGridState,
    topPadding: Dp,
    onOpen: (AgentTarget) -> Unit,
    onKill: (AgentTarget) -> Unit,
) {
    val bottomPadding = 84.dp
    BoxWithConstraints(Modifier.fillMaxSize()) {
        val emptyItemHeight = (maxHeight - topPadding - bottomPadding).coerceAtLeast(0.dp)
        LazyVerticalGrid(
            columns = GridCells.Adaptive(170.dp),
            modifier = Modifier.fillMaxSize().testTag("agents-grid"),
            state = gridState,
            contentPadding = PaddingValues(
                start = 12.dp,
                top = topPadding,
                end = 12.dp,
                bottom = bottomPadding,
            ),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (agents.isEmpty()) {
                item(
                    key = "dashboard-empty-state",
                    span = { GridItemSpan(maxLineSpan) },
                ) {
                    Box(Modifier.fillMaxWidth().height(emptyItemHeight)) {
                        when {
                            state.machines.isEmpty() && state.unreadableMachines.isNotEmpty() -> EmptyState(
                                "Fleet reset required",
                                "Saved fleet credentials are unreadable. Reset the app data, then connect again.",
                                tone = NoticeTone.Failure,
                            )
                            else -> dashboardInventoryWaitCopy(machines)?.let {
                                EmptyState("Sessions not current", it.message, tone = it.tone)
                            } ?: EmptyState(
                                "No tmux sessions",
                                "Create a dwarf here, or launch tmux on the visible " +
                                    if (machines.size == 1) "machine." else "machines.",
                                ornament = true,
                            )
                        }
                    }
                }
            } else {
                items(
                    items = agents,
                    key = { "${it.target.machineHandle.encoded}:${it.target.session.id}:${it.target.session.identityToken}" },
                ) { agent ->
                    val machineState = state.machines.single { it.machine.handle == agent.target.machineHandle }
                    AgentCard(
                        agent,
                        machineState,
                        showMachineLabel = state.selectedMachine == null,
                        onOpen = { onOpen(agent.target) },
                        onKill = { onKill(agent.target) },
                    )
                }
            }
        }
    }
}

@Composable
internal fun DashboardTopBar(
    summary: String,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(64.dp)
            .padding(horizontal = 16.dp)
            .testTag("dashboard-top-bar"),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The Hlíðskjálf mark on the surface it names (design-language.md §8):
        // Gold, decorative, and silent — "Dwarves" beside it carries the label.
        HlidskjalfMark(color = Gold, markSize = 24.dp, tag = "dashboard-mark")
        Column(modifier = Modifier.weight(1f).testTag("dashboard-title")) {
            Text(
                "Dwarves",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.SemiBold,
                // The row is a fixed 64dp and now leads with the 24dp mark, so at a large
                // font scale an unbounded title would wrap and clip against it. The summary
                // line below has always bounded itself; this matches it.
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
    }
}

@Composable
internal fun UnreadableMachineStrip(
    machine: UnreadableStoredMachine,
) {
    NoticePanel(
        tone = NoticeTone.Failure,
        title = if (machine.collectionWide) "Unreadable pairing index" else "Unreadable pairing",
        body = if (machine.collectionWide) {
            "Saved machines cannot be identified safely. Reset the app data, then connect again."
        } else {
            "Its saved identity and destination are untrusted. Reset the app data, then connect again."
        },
    )
}

@Composable
private fun MachineFilters(
    machines: List<MachineState>,
    selected: MachineHandle?,
    onSelect: (MachineHandle?) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 16.dp)
            .testTag("machine-filters"),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        FilterChip(
            selected = selected == null,
            onClick = { onSelect(null) },
            label = { Text("All", fontFamily = NidavellirType.Data) },
            shape = NidavellirShapes.Chip,
            modifier = Modifier.testTag("machine-filter-all"),
        )
        machines.forEach { machine ->
            FilterChip(
                selected = selected == machine.machine.handle,
                onClick = { onSelect(machine.machine.handle) },
                label = { Text(machine.machine.label.text, fontFamily = NidavellirType.Data) },
                shape = NidavellirShapes.Chip,
                modifier = Modifier.testTag("machine-filter-${machine.machine.handle.encoded}"),
            )
        }
    }
}

@Composable
private fun MachineStrip(
    machine: MachineState,
    controller: SkidbladnirController,
    credentialWritesEnabled: Boolean,
) {
    val handle = machine.machine.handle.encoded
    val fresh = machine.inventory as? InventoryState.Fresh
    Column {
        MachinePressureStrip(
            machineLabel = machine.machine.label.text,
            state = machine.pressure,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 4.dp)
                .testTag("machine-strip-$handle"),
            headerModifier = Modifier
                .testTag("machine-state-${machineStateTag(machine)}-$handle")
                .semantics {
                    if (fresh != null) machineInventoryObservation = fresh.snapshot.receivedAtElapsedMillis
                },
            labelModifier = Modifier.testTag("machine-strip-label-$handle"),
            inventoryStale = machine.inventory is InventoryState.Stale,
            supporting = machineNotice(machine),
        )
        if (machineAvailability(machine) == MachineAvailability.AuthRequired) {
            TextButton(
                onClick = controller::requestFleetReconnect,
                enabled = credentialWritesEnabled,
                modifier = Modifier.padding(horizontal = 16.dp),
            ) { Text("Reconnect fleet") }
        }
    }
}

@Composable
internal fun MachinePressureStrip(
    machineLabel: String,
    state: PressureState,
    inventoryStale: Boolean,
    supporting: MachineNotice?,
    modifier: Modifier = Modifier,
    headerModifier: Modifier = Modifier,
    labelModifier: Modifier = Modifier,
) {
    val response = when (state) {
        is PressureState.Fresh -> state.response
        is PressureState.Stale -> state.response
        PressureState.Reading, is PressureState.Unavailable -> null
    }
    val stale = inventoryStale || state is PressureState.Stale
    Surface(color = RaisedSurface, modifier = modifier, shape = NidavellirShapes.Card) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 9.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically, modifier = headerModifier) {
                Text(
                    text = buildString {
                        append(machineLabel.uppercase(Locale.ROOT))
                        append(' ')
                        append(
                            when (state) {
                                PressureState.Reading -> "READING"
                                is PressureState.Fresh -> state.response.current.level.name.uppercase(Locale.ROOT)
                                is PressureState.Stale -> state.response.current.level.name.uppercase(Locale.ROOT)
                                is PressureState.Unavailable -> "UNAVAILABLE"
                            },
                        )
                    },
                    color = pressureStateColor(state),
                    style = MaterialTheme.typography.labelLarge,
                    fontFamily = NidavellirType.Data,
                    fontWeight = FontWeight.Bold,
                    modifier = labelModifier.weight(1f),
                )
                if (stale) {
                    Text(
                        "STALE",
                        color = noticeToneColor(NoticeTone.Degraded),
                        fontFamily = NidavellirType.Data,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
            if (response != null) {
                val current = response.current
                val known = pressureMetricValues(current.metrics)
                if (known.isNotEmpty()) {
                    Text(
                        text = known.joinToString(" · "),
                        color = Bone,
                        style = MaterialTheme.typography.labelMedium,
                        fontFamily = NidavellirType.Data,
                        modifier = Modifier
                            .fillMaxWidth()
                            .horizontalScroll(rememberScrollState())
                            .padding(top = 2.dp),
                        maxLines = 1,
                    )
                }
                if (response.history.isNotEmpty()) {
                    Canvas(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(16.dp)
                            .padding(top = 5.dp)
                            .semantics {
                                contentDescription = "$machineLabel pressure history: " +
                                    response.history.joinToString { it.level.name.lowercase(Locale.ROOT) }
                            },
                    ) {
                        val barWidth = size.width / response.history.size
                        response.history.forEachIndexed { index, sample ->
                            val proportion = when (sample.level) {
                                PressureLevel.Normal -> 0.25f
                                PressureLevel.Warm -> 0.58f
                                PressureLevel.Hot -> 1f
                                PressureLevel.Unknown -> 0.42f
                            }
                            drawRect(
                                color = pressureColor(sample.level),
                                topLeft = Offset(
                                    x = index * barWidth,
                                    y = size.height * (1f - proportion),
                                ),
                                size = Size(
                                    width = maxOf(1f, barWidth - 1f),
                                    height = size.height * proportion,
                                ),
                            )
                        }
                    }
                    Text(
                        "Recent pressure history · up to 15 min",
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
                    )
                }
                if (current.missing.isNotEmpty()) {
                    Text(
                        "Missing: ${current.missing.joinToString { pressureMetricLabel(it) }}",
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
                    )
                }
                if (response.unsupported.isNotEmpty()) {
                    Text(
                        "Unsupported: ${response.unsupported.joinToString { pressureMetricLabel(it) }}",
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
                    )
                }
                if (current.reasons.isNotEmpty()) {
                    Text(
                        "Pressure: ${current.reasons.joinToString { pressureReasonLabel(it) }}",
                        color = pressureColor(current.level),
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
                    )
                }
            }
            supporting?.let {
                Text(it.message, color = noticeToneColor(it.tone), style = MaterialTheme.typography.labelMedium)
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ForgeSheet(
    state: ForgeState,
    machines: List<MachineState>,
    onDismiss: () -> Unit,
    onDraftChange: ((ForgeForm) -> ForgeForm) -> Unit,
    onSubmit: () -> Unit,
) {
    val selected = state.form.machineHandle?.let { handle ->
        machines.singleOrNull { it.machine.handle == handle }
    }
    val inventory = selected?.inventory?.lastSnapshot()?.inventory
    val fieldsEnabled = !state.pending && selected?.canMutate == true
    // The one ambient animation in the app: the sheet warms from stone to
    // firelight once on open (design-language.md §12). A zero animator scale
    // collapses the tween, so the sheet simply opens lit.
    var lit by remember { mutableStateOf(false) }
    val containerColor by animateColorAsState(
        targetValue = if (lit) ForgeGlow else DeepSurface,
        animationSpec = NidavellirMotion.ForgeWarmIn,
        label = "forge warm-in",
    )
    LaunchedEffect(Unit) { lit = true }
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        shape = NidavellirShapes.Sheet,
        containerColor = containerColor,
    ) {
        Column(
            Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .imePadding()
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp)
                .testTag("forge-sheet"),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                "Create dwarf",
                style = MaterialTheme.typography.headlineSmall,
                fontFamily = NidavellirType.Display,
                fontWeight = FontWeight.SemiBold,
            )
            // The fret band (design-language.md §7): decorative only, drawn
            // under the title block, Gold at the family's 40% ceiling.
            Canvas(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(12.dp),
            ) {
                drawFretBand(Gold.copy(alpha = 0.40f))
            }
            Text("Machine", color = Muted, style = MaterialTheme.typography.labelLarge)
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                machines.forEach { machine ->
                    FilterChip(
                        selected = machine.machine.handle == state.form.machineHandle,
                        onClick = { onDraftChange { it.copy(machineHandle = machine.machine.handle) } },
                        enabled = !state.pending && machine.canMutate,
                        label = { Text(forgeMachineChoiceLabel(machine), fontFamily = NidavellirType.Data) },
                        shape = NidavellirShapes.Chip,
                        modifier = Modifier.testTag("forge-machine-${machine.machine.handle.encoded}"),
                    )
                }
            }
            if (selected == null) {
                Text("Choose a machine to load its profiles and paths.", color = Muted)
            } else {
                Text("Profiles on ${selected.machine.label.text}", color = Muted, style = MaterialTheme.typography.labelLarge)
                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    inventory?.profiles.orEmpty().forEach { profile ->
                        FilterChip(
                            selected = state.form.profile == profile.key,
                            onClick = { onDraftChange { it.copy(profile = profile.key) } },
                            enabled = fieldsEnabled,
                            label = { Text(profile.label, fontFamily = NidavellirType.Data) },
                            shape = NidavellirShapes.Chip,
                            modifier = Modifier.testTag("forge-profile-${selected.machine.handle.encoded}"),
                        )
                    }
                }
                forgeUnavailableCopy(selected)?.let {
                    Text(
                        it.message,
                        color = noticeToneColor(it.tone),
                        modifier = Modifier.testTag("forge-machine-unavailable"),
                    )
                }
            }
            OutlinedTextField(
                value = state.form.cwd,
                onValueChange = { value -> onDraftChange { it.copy(cwd = value) } },
                modifier = Modifier.fillMaxWidth().testTag("forge-cwd"), enabled = fieldsEnabled,
                label = { Text(selected?.let { "Working directory on ${it.machine.label.text}" } ?: "Working directory") },
                singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false),
            )
            OutlinedTextField(
                value = state.form.optionalTmuxName,
                onValueChange = { value -> onDraftChange { it.copy(optionalTmuxName = value) } },
                modifier = Modifier.fillMaxWidth().testTag("forge-name"), enabled = fieldsEnabled,
                label = { Text("tmux name (optional)") }, singleLine = true,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, autoCorrectEnabled = false),
            )
            OutlinedTextField(
                value = state.form.objective,
                onValueChange = { value -> onDraftChange { it.copy(objective = value) } },
                modifier = Modifier.fillMaxWidth().testTag("forge-objective"), enabled = fieldsEnabled,
                label = { Text("Objective (optional)") }, minLines = 2, maxLines = 4,
            )
            state.error?.let { Text(it, color = noticeToneColor(NoticeTone.Failure)) }
            Button(
                onClick = onSubmit,
                enabled = state.form.submission() != null && !state.pending && selected?.canMutate == true,
                modifier = Modifier.fillMaxWidth().testTag("forge-submit"),
            ) {
                if (state.pending) { CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp); Spacer(Modifier.width(8.dp)) }
                Text(selected?.let { forgeActionLabel(it.machine.label) } ?: "Choose a machine")
            }
        }
    }
}

@Composable
internal fun KillConfirmation(
    state: KillState,
    actionAdmissible: Boolean,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    // No ornament near destructive surfaces (design-language.md §7): the kill
    // dialog carries the cut-corner shape and nothing decorative.
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(killConfirmationTitle(state.machine.label, state.target)) },
        text = {
            Text(when {
                state.pending -> "The exact tmux lifetime on ${state.machine.label.text} is being killed."
                !actionAdmissible ->
                    "${state.machine.label.text} inventory is not fresh. Kill is disabled. " +
                        "Cancel, return to Dwarves, then pull down to check again."
                else -> "This kills only the confirmed tmux lifetime on ${state.machine.label.text}. It cannot be undone."
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
                modifier = Modifier.testTag("kill-confirm"),
            ) {
                Text(if (state.pending) "Killing on ${state.machine.label.text}…" else "Kill on ${state.machine.label.text}")
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
    body: String,
    tone: NoticeTone = NoticeTone.Degraded,
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
                    tag = "EmptyStateOrnament",
                    modifier = Modifier.padding(bottom = 12.dp),
                )
            }
            Text(title, style = MaterialTheme.typography.titleLarge)
            Text(body, color = noticeToneColor(tone), modifier = Modifier.padding(top = 8.dp))
        }
    }
}

private fun pressureStateColor(state: PressureState): Color = when (state) {
    PressureState.Reading -> Gold
    is PressureState.Fresh -> pressureColor(state.response.current.level)
    is PressureState.Stale, is PressureState.Unavailable -> Muted
}

private fun pressureColor(level: PressureLevel): Color = when (level) {
    PressureLevel.Normal -> Moss
    PressureLevel.Warm -> Gold
    PressureLevel.Hot -> Ember
    PressureLevel.Unknown -> Muted
}

private fun pressureMetricValues(metrics: PressureMetrics): List<String> = buildList {
    metrics.cpuPercent?.let { add("CPU ${it.toInt()}%") }
    metrics.memoryAvailablePercent?.let { add("RAM ${it.toInt()}% free") }
    metrics.swapUsedPercent?.let { add("swap ${it.toInt()}% used") }
    metrics.normalizedLoad?.let { add("load ${String.format(Locale.ROOT, "%.2f", it)}") }
    metrics.diskAvailablePercent?.let { add("disk ${it.toInt()}% free") }
    metrics.cpuPsiSomeAvg60Percent?.let { add("CPU PSI ${it.toInt()}%") }
    metrics.memoryPsiFullAvg60Percent?.let { add("memory PSI ${it.toInt()}%") }
    metrics.ioPsiFullAvg60Percent?.let { add("I/O PSI ${it.toInt()}%") }
    metrics.memoryPressure?.let { add("memory pressure ${it.name.uppercase(Locale.ROOT)}") }
}

private fun pressureMetricLabel(value: PressureMetric): String = when (value) {
    PressureMetric.CpuPercent -> "CPU"
    PressureMetric.NormalizedLoad -> "load"
    PressureMetric.MemoryAvailablePercent -> "memory available"
    PressureMetric.SwapUsedPercent -> "swap used"
    PressureMetric.DiskAvailablePercent -> "disk available"
    PressureMetric.CpuPsiSomeAvg60Percent -> "CPU PSI"
    PressureMetric.MemoryPsiFullAvg60Percent -> "memory PSI"
    PressureMetric.IoPsiFullAvg60Percent -> "I/O PSI"
    PressureMetric.MemoryPressure -> "system memory pressure"
}

private fun pressureReasonLabel(reason: PressureReason): String = when (reason) {
    PressureReason.Memory -> "memory"
    PressureReason.Disk -> "disk"
    PressureReason.Load -> "load"
    PressureReason.CpuPsi -> "CPU pressure"
    PressureReason.MemoryPsi -> "memory pressure"
    PressureReason.IoPsi -> "I/O pressure"
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
                MachineAccess.Ready -> if (
                    dashboard.selectedMachine == null || dashboard.selectedMachine == target.machine.handle
                ) {
                    "Pull down to check again before reviewing this draft."
                } else {
                    "Select $label, then pull down to check again before reviewing this draft."
                }
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
