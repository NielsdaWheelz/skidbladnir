package dev.niels.skidbladnir

import androidx.compose.foundation.background
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import java.time.Instant
import java.util.Locale

@Composable
internal fun DashboardScreen(state: SkidbladnirUiState.Dashboard, controller: SkidbladnirController) {
    val machines = state.machines.filter {
        state.selectedMachine == null || it.machine.handle == state.selectedMachine
    }
    val agents = visibleAgents(state.machines, state.selectedMachine)
    val canForge = state.machines.any { machine ->
        (state.selectedMachine == null || machine.machine.handle == state.selectedMachine) &&
            machine.canMutate &&
            (machine.inventory as? InventoryState.Fresh)
                ?.snapshot?.inventory?.profiles?.isNotEmpty() == true
    }
    Column(
        modifier = Modifier.fillMaxSize().background(Ink).systemBarsPadding(),
    ) {
        DashboardTopBar(
            summary = dashboardSummary(agents.size, machines.size),
            refreshing = state.refreshing,
            canForge = canForge,
            onRefresh = controller::refresh,
            onNewAgent = controller::openForge,
        )

        MachineFilters(state.machines, state.selectedMachine, controller::selectMachine)
        machines.forEach { machine ->
            key(machine.machine.handle) {
                MachineStrip(machine, controller, credentialWritesEnabled = state.unreadableMachines.isEmpty())
            }
        }
        state.unreadableMachines.forEach { UnreadableMachineStrip(it) }

        state.notice?.let { notice ->
            Surface(
                color = MaterialTheme.colorScheme.error.copy(alpha = 0.16f),
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
                shape = RoundedCornerShape(10.dp),
            ) {
                Text(notice, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(12.dp))
            }
        }

        state.forgeRecovery?.let { recovery ->
            Surface(
                color = Gold.copy(alpha = 0.16f),
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
                shape = RoundedCornerShape(10.dp),
            ) {
                Column(Modifier.padding(12.dp)) {
                    Text(
                        when (recovery) {
                            is ForgeRecovery.RefreshRequired ->
                                "${labelFor(state.machines, recovery.draft.machineHandle)}: create outcome unknown. Refresh before reviewing this draft."
                            is ForgeRecovery.ReviewReady ->
                                "${labelFor(state.machines, recovery.draft.machineHandle)} refreshed. Review its agents before resuming this draft."
                        },
                        color = Gold,
                    )
                    if (recovery is ForgeRecovery.ReviewReady) {
                        Row {
                            TextButton(onClick = controller::resumeForgeRecovery) { Text("Resume draft") }
                            TextButton(onClick = controller::discardForgeRecovery) { Text("Discard") }
                        }
                    }
                }
            }
        }

        when {
            state.machines.isEmpty() && state.unreadableMachines.isNotEmpty() -> EmptyState(
                "Provisioning repair required",
                "Saved machine credentials are unreadable. Machine administration is outside this app.",
            )
            state.machines.isEmpty() -> EmptyState(
                "No provisioned machines",
                "Install machine credentials outside the app to begin.",
            )
            agents.isEmpty() -> dashboardInventoryWaitCopy(machines)?.let {
                EmptyState("Sessions not current", it)
            } ?: EmptyState(
                "No tmux sessions",
                "Create an agent here, or launch tmux on the visible " +
                    if (machines.size == 1) "machine." else "machines.",
            )
            else -> LazyVerticalGrid(
                columns = GridCells.Adaptive(170.dp),
                modifier = Modifier.fillMaxSize().testTag("agents-grid"),
                contentPadding = PaddingValues(12.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                items(
                    items = agents,
                    key = { "${it.target.machineHandle.encoded}:${it.target.session.id}:${it.target.session.identityToken}" },
                ) { agent ->
                    val machineState = state.machines.single { it.machine.handle == agent.target.machineHandle }
                    AgentCard(
                        agent,
                        machineState,
                        onOpen = { controller.openTerminal(agent.target) },
                        onKill = { controller.requestKill(agent.target) },
                    )
                }
            }
        }
    }

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
internal fun DashboardTopBar(
    summary: String,
    refreshing: Boolean,
    canForge: Boolean,
    onRefresh: () -> Unit,
    onNewAgent: () -> Unit,
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
        Column(modifier = Modifier.weight(1f)) {
            Text("Agents", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
            Text(
                summary,
                color = Muted,
                style = MaterialTheme.typography.labelMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        TextButton(onClick = onRefresh, enabled = !refreshing) {
            Text(if (refreshing) "Reading…" else "Refresh")
        }
        Button(onClick = onNewAgent, enabled = canForge, modifier = Modifier.testTag("new-agent")) {
            Text("New agent")
        }
    }
}

@Composable
internal fun UnreadableMachineStrip(
    machine: UnreadableStoredMachine,
) {
    Surface(
        color = MaterialTheme.colorScheme.error.copy(alpha = 0.16f),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
        shape = RoundedCornerShape(10.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 9.dp)) {
            Text(
                if (machine.collectionWide) "Unreadable pairing index" else "Unreadable pairing",
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                if (machine.collectionWide) {
                    "Saved machines cannot be identified safely. Provisioning repair is required outside this app."
                } else {
                    "Its saved identity and destination are untrusted. Provisioning repair is required outside this app."
                },
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.labelMedium,
            )
        }
    }
}

@Composable
private fun MachineFilters(
    machines: List<MachineState>,
    selected: MachineHandle?,
    onSelect: (MachineHandle?) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(horizontal = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        FilterChip(
            selected = selected == null,
            onClick = { onSelect(null) },
            label = { Text("All") },
            modifier = Modifier.testTag("machine-filter-all"),
        )
        machines.forEach { machine ->
            FilterChip(
                selected = selected == machine.machine.handle,
                onClick = { onSelect(machine.machine.handle) },
                label = { Text(machine.machine.label.text) },
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
    val stale = machine.inventory is InventoryState.Stale
    Column {
        (machine.inventory as? InventoryState.Fresh)?.let { fresh ->
            Spacer(
                Modifier
                    .size(1.dp)
                    .testTag(
                        "machine-inventory-received-${machine.machine.handle.encoded}-${fresh.snapshot.receivedAtElapsedMillis}",
                    ),
            )
        }
        Spacer(
            Modifier.size(1.dp).testTag(
                "machine-${if (machine.canMutate) "actionable" else "nonmutating"}-${machine.machine.handle.encoded}",
            ),
        )
        MachinePressureStrip(
            machineLabel = machine.machine.label.text,
            state = machine.pressure,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 4.dp)
                .testTag("machine-strip-${machine.machine.handle.encoded}"),
            headerModifier = Modifier.testTag(
                "machine-state-${machineStateTag(machine)}-${machine.machine.handle.encoded}",
            ),
            labelModifier = Modifier.testTag("machine-strip-label-${machine.machine.handle.encoded}"),
            inventoryStale = stale,
            supportingMessage = machineStateMessage(machine),
            supportingMessageColor = if (machine.access == MachineAccess.Ready) Ember else Gold,
        )
        if (machine.access == MachineAccess.AuthRequired) {
            TextButton(
                onClick = { controller.repairMachine(machine.machine.handle) },
                enabled = credentialWritesEnabled,
                modifier = Modifier.padding(horizontal = 16.dp),
            ) { Text("Update bearer") }
        }
    }
}

@Composable
internal fun MachinePressureStrip(
    machineLabel: String,
    state: PressureState,
    modifier: Modifier = Modifier,
    headerModifier: Modifier = Modifier,
    labelModifier: Modifier = Modifier,
    inventoryStale: Boolean = false,
    supportingMessage: String? = null,
    supportingMessageColor: Color = Ember,
) {
    val response = when (state) {
        is PressureState.Fresh -> state.response
        is PressureState.Stale -> state.response
        PressureState.Reading, is PressureState.Unavailable -> null
    }
    val stale = inventoryStale || state is PressureState.Stale
    Surface(color = RaisedSurface, modifier = modifier, shape = RoundedCornerShape(10.dp)) {
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
                    fontWeight = FontWeight.Bold,
                    modifier = labelModifier.weight(1f),
                )
                if (stale) Text("STALE", color = Ember, fontWeight = FontWeight.Bold)
            }
            if (response != null) {
                val current = response.current
                val known = pressureMetricValues(current.metrics)
                if (known.isNotEmpty()) {
                    Text(
                        text = known.joinToString(" · "),
                        color = Bone,
                        style = MaterialTheme.typography.labelMedium,
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
                                topLeft = androidx.compose.ui.geometry.Offset(
                                    x = index * barWidth,
                                    y = size.height * (1f - proportion),
                                ),
                                size = androidx.compose.ui.geometry.Size(
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
                    )
                }
                if (current.missing.isNotEmpty()) {
                    Text(
                        "Missing: ${current.missing.joinToString { pressureMetricLabel(it) }}",
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                    )
                }
                if (response.unsupported.isNotEmpty()) {
                    Text(
                        "Unsupported: ${response.unsupported.joinToString { pressureMetricLabel(it) }}",
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                    )
                }
                if (current.reasons.isNotEmpty()) {
                    Text(
                        "Pressure: ${current.reasons.joinToString { pressureReasonLabel(it) }}",
                        color = pressureColor(current.level),
                        style = MaterialTheme.typography.labelSmall,
                    )
                }
            }
            supportingMessage?.let {
                Text(it, color = supportingMessageColor, style = MaterialTheme.typography.labelMedium)
            }
        }
    }
}

@Composable
private fun AgentCard(
    agent: VisibleAgent,
    machine: MachineState,
    onOpen: () -> Unit,
    onKill: () -> Unit,
) {
    val session = agent.target.session
    val inventory = when (val value = machine.inventory) {
        is InventoryState.Fresh -> value.snapshot
        is InventoryState.Stale -> value.snapshot
        InventoryState.Reading, is InventoryState.Unreachable -> return
    }
    val status = statusContent(
        session.status,
        Instant.parse(inventory.inventory.observedAt).plusMillis(
            (android.os.SystemClock.elapsedRealtime() - inventory.receivedAtElapsedMillis).coerceAtLeast(0),
        ),
    )
    Card(
        onClick = onOpen,
        enabled = machine.canMutate,
        modifier = Modifier.testTag(
            "agent-card-${agent.target.machineHandle.encoded}-${session.id}",
        ),
        colors = CardDefaults.cardColors(containerColor = DeepSurface),
        shape = RoundedCornerShape(14.dp),
    ) {
        Column(Modifier.padding(12.dp)) {
            Surface(color = Frost.copy(alpha = 0.18f), shape = RoundedCornerShape(7.dp)) {
                Text(
                    agent.machine.label.text,
                    color = Frost,
                    modifier = Modifier
                        .padding(horizontal = 8.dp, vertical = 4.dp)
                        .testTag(
                            "agent-machine-pill-${agent.target.machineHandle.encoded}-${session.id}",
                        ),
                    fontWeight = FontWeight.Bold,
                )
            }
            if (!machine.canMutate) {
                Text(
                    if (machine.inventoryRefreshRequired) "REFRESHING · actions disabled" else "STALE · actions disabled",
                    color = Ember,
                    style = MaterialTheme.typography.labelSmall,
                    modifier = Modifier.padding(top = 7.dp),
                )
            }
            Row(Modifier.padding(top = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(session.name, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(session.character?.displayName ?: "tmux session", color = Muted, style = MaterialTheme.typography.labelMedium)
                }
                if (session.attention) {
                    Text("!", color = Ember, fontWeight = FontWeight.Bold, modifier = Modifier.semantics { contentDescription = "Needs attention" })
                }
            }
            Surface(
                color = statusColor(session.status.kind).copy(alpha = 0.18f),
                shape = RoundedCornerShape(7.dp),
                modifier = Modifier.padding(top = 9.dp).semantics { contentDescription = status.accessibilityLabel },
            ) {
                Column(Modifier.padding(horizontal = 8.dp, vertical = 5.dp)) {
                    Text(status.kind, color = statusColor(session.status.kind), fontWeight = FontWeight.Bold)
                    Text(status.evidence, color = Muted, style = MaterialTheme.typography.labelSmall)
                }
            }
            session.objective?.let { Text(it, modifier = Modifier.padding(top = 9.dp), maxLines = 3, overflow = TextOverflow.Ellipsis) }
            session.cwd?.let { Text(it, color = Muted, style = MaterialTheme.typography.labelSmall, modifier = Modifier.padding(top = 8.dp), maxLines = 2) }
            Row(Modifier.fillMaxWidth().padding(top = 7.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(session.profile ?: "profile unknown", color = Muted, style = MaterialTheme.typography.labelSmall, modifier = Modifier.weight(1f))
                TextButton(
                    onClick = onKill,
                    enabled = machine.canMutate,
                    colors = ButtonDefaults.textButtonColors(contentColor = Ember),
                    modifier = Modifier.testTag(
                        "agent-kill-${agent.target.machineHandle.encoded}-${session.id}",
                    ),
                ) { Text("Kill") }
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
    val inventory = when (val state = selected?.inventory) {
        is InventoryState.Fresh -> state.snapshot.inventory
        is InventoryState.Stale -> state.snapshot.inventory
        InventoryState.Reading, is InventoryState.Unreachable, null -> null
    }
    val fieldsEnabled = !state.pending && selected?.canMutate == true
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .imePadding()
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp)
                .testTag("forge-sheet"),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text("Create agent", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
            Text("Machine", color = Muted, style = MaterialTheme.typography.labelLarge)
            Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                machines.forEach { machine ->
                    FilterChip(
                        selected = machine.machine.handle == state.form.machineHandle,
                        onClick = { onDraftChange { changeForgeMachine(it, machine.machine.handle) } },
                        enabled = !state.pending && machine.canMutate,
                        label = { Text(forgeMachineChoiceLabel(machine)) },
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
                            label = { Text(profile.label) },
                            modifier = Modifier.testTag("forge-profile-${selected.machine.handle.encoded}"),
                        )
                    }
                }
                forgeUnavailableCopy(selected)?.let {
                    Text(it, color = Ember, modifier = Modifier.testTag("forge-machine-unavailable"))
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
                value = state.form.optionalName,
                onValueChange = { value -> onDraftChange { it.copy(optionalName = value) } },
                modifier = Modifier.fillMaxWidth().testTag("forge-name"), enabled = fieldsEnabled,
                label = { Text("Name (optional)") }, singleLine = true,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, autoCorrectEnabled = false),
            )
            OutlinedTextField(
                value = state.form.objective,
                onValueChange = { value -> onDraftChange { it.copy(objective = value) } },
                modifier = Modifier.fillMaxWidth().testTag("forge-objective"), enabled = fieldsEnabled,
                label = { Text("Objective (optional)") }, minLines = 2, maxLines = 4,
            )
            state.error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            Button(
                onClick = onSubmit,
                enabled = state.form.cwd.isNotBlank() && state.form.profile.isNotBlank() &&
                    !state.pending && selected?.canMutate == true,
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
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(killConfirmationTitle(state.machine.label, state.target)) },
        text = {
            Text(when {
                state.pending -> "The exact tmux lifetime on ${state.machine.label.text} is being killed."
                !actionAdmissible ->
                    "${state.machine.label.text} inventory is not fresh. Kill is disabled; cancel and refresh."
                else -> "This kills only the confirmed tmux lifetime on ${state.machine.label.text}. It cannot be undone."
            })
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                enabled = actionAdmissible && !state.pending,
                colors = ButtonDefaults.buttonColors(containerColor = Ember),
                modifier = Modifier.testTag("kill-confirm"),
            ) {
                Text(if (state.pending) "Killing on ${state.machine.label.text}…" else "Kill on ${state.machine.label.text}")
            }
        },
        dismissButton = { OutlinedButton(onClick = onDismiss, enabled = !state.pending) { Text("Cancel") } },
    )
}

@Composable
private fun EmptyState(title: String, body: String) {
    Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(title, style = MaterialTheme.typography.titleLarge)
            Text(body, color = Muted, modifier = Modifier.padding(top = 8.dp))
        }
    }
}

private fun machineStateMessage(machine: MachineState): String? = when {
    machine.inventoryRefreshRequired ->
        "${machine.machine.label.text}: confirming the latest tmux inventory. Actions disabled."
    machine.access == MachineAccess.AuthRequired -> "${machine.machine.label.text}: authentication required. Actions disabled."
    machine.access == MachineAccess.IdentityChanged ->
        "${machine.machine.label.text}: identity changed. Provisioning repair is required."
    else -> when (val inventory = machine.inventory) {
        InventoryState.Reading -> "${machine.machine.label.text}: reading tmux sessions."
        is InventoryState.Stale -> "${machine.machine.label.text}: ${gatewayFailureMessage(inventory.cause)} Prior sessions are STALE; actions disabled."
        is InventoryState.Unreachable -> "${machine.machine.label.text}: ${gatewayFailureMessage(inventory.cause)}"
        is InventoryState.Fresh -> when (val pressure = machine.pressure) {
            is PressureState.Stale -> "${machine.machine.label.text}: pressure is STALE. Sessions remain current."
            is PressureState.Unavailable -> "${machine.machine.label.text}: pressure unavailable. Sessions remain current."
            PressureState.Reading, is PressureState.Fresh -> null
        }
    }
}

private fun machineStateTag(machine: MachineState): String = when (machineAvailability(machine)) {
    MachineAvailability.Ready -> "fresh"
    MachineAvailability.Refreshing -> "refreshing"
    MachineAvailability.AuthRequired -> "auth"
    MachineAvailability.IdentityChanged -> "identity"
    MachineAvailability.Reading -> "reading"
    MachineAvailability.Stale -> "stale"
    MachineAvailability.Unavailable -> "unreachable"
}

private fun pressureStateColor(state: PressureState): Color = when (state) {
    PressureState.Reading -> Gold
    is PressureState.Fresh -> pressureColor(state.response.current.level)
    is PressureState.Stale, is PressureState.Unavailable -> Muted
}

private fun statusColor(kind: SessionStatusKind): Color = when (kind) {
    SessionStatusKind.Working -> Moss
    SessionStatusKind.Running -> Frost
    SessionStatusKind.Idle -> Gold
    SessionStatusKind.Shell -> Frost
    SessionStatusKind.Unknown -> Muted
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

private fun labelFor(machines: List<MachineState>, handle: MachineHandle): String =
    machines.singleOrNull { it.machine.handle == handle }?.machine?.label?.text ?: "Machine"

internal fun dashboardSummary(sessionCount: Int, machineCount: Int): String =
    "$sessionCount tmux ${if (sessionCount == 1) "session" else "sessions"} across " +
        "$machineCount ${if (machineCount == 1) "machine" else "machines"}"

internal fun dashboardInventoryWaitCopy(machines: List<MachineState>): String? = machines.mapNotNull { machine ->
    when (machineAvailability(machine)) {
        MachineAvailability.Ready -> null
        MachineAvailability.Refreshing ->
            "${machine.machine.label.text}: confirming the latest tmux inventory."
        MachineAvailability.AuthRequired ->
            "${machine.machine.label.text}: authentication required; its sessions may be out of date."
        MachineAvailability.IdentityChanged ->
            "${machine.machine.label.text}: identity changed; provisioning repair is required."
        MachineAvailability.Reading -> "${machine.machine.label.text}: reading tmux sessions."
        MachineAvailability.Stale ->
            "${machine.machine.label.text}: showing its last inventory; it is STALE and actions are disabled."
        MachineAvailability.Unavailable ->
            "${machine.machine.label.text}: unavailable; its sessions cannot be read."
    }
}.takeIf(List<String>::isNotEmpty)?.joinToString(" ")

internal fun forgeMachineChoiceLabel(machine: MachineState): String = machine.machine.label.text + when (
    machineAvailability(machine)
) {
    MachineAvailability.Ready -> ""
    MachineAvailability.Refreshing -> " · REFRESHING"
    MachineAvailability.AuthRequired -> " · AUTH REQUIRED"
    MachineAvailability.IdentityChanged -> " · RE-PAIR"
    MachineAvailability.Reading -> " · READING"
    MachineAvailability.Stale -> " · STALE"
    MachineAvailability.Unavailable -> " · UNAVAILABLE"
}

internal fun forgeUnavailableCopy(machine: MachineState): String? = when (machineAvailability(machine)) {
    MachineAvailability.Ready -> null
    MachineAvailability.Refreshing ->
        "${machine.machine.label.text} is confirming its latest tmux inventory. " +
            "Draft fields and Create are disabled."
    MachineAvailability.AuthRequired ->
        "${machine.machine.label.text} needs an updated bearer. Draft fields and Create are disabled."
    MachineAvailability.IdentityChanged ->
        "${machine.machine.label.text} identity changed. Provisioning repair is required; " +
            "draft fields and Create are disabled."
    MachineAvailability.Reading ->
        "${machine.machine.label.text} is reading tmux sessions. " +
            "Draft fields and Create are disabled until the inventory is fresh."
    MachineAvailability.Stale ->
        "${machine.machine.label.text} inventory is STALE. " +
            "Draft fields and Create are disabled until a fresh read succeeds."
    MachineAvailability.Unavailable ->
        "${machine.machine.label.text} is unavailable. Draft fields and Create are disabled until it reconnects."
}

private enum class MachineAvailability {
    Ready,
    Refreshing,
    AuthRequired,
    IdentityChanged,
    Reading,
    Stale,
    Unavailable,
}

private fun machineAvailability(machine: MachineState): MachineAvailability = when {
    machine.access == MachineAccess.AuthRequired -> MachineAvailability.AuthRequired
    machine.access == MachineAccess.IdentityChanged -> MachineAvailability.IdentityChanged
    machine.inventoryRefreshRequired -> MachineAvailability.Refreshing
    else -> when (machine.inventory) {
        InventoryState.Reading -> MachineAvailability.Reading
        is InventoryState.Fresh -> MachineAvailability.Ready
        is InventoryState.Stale -> MachineAvailability.Stale
        is InventoryState.Unreachable -> MachineAvailability.Unavailable
    }
}
