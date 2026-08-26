package dev.niels.skidbladnir

import androidx.compose.foundation.background
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

@Composable
internal fun DashboardScreen(state: SkidbladnirUiState.Dashboard, controller: SkidbladnirController) {
    val machines = state.machines.filter {
        state.selectedMachine == null || it.machine.handle == state.selectedMachine
    }
    val agents = visibleAgents(state.machines, state.selectedMachine)
    Column(
        modifier = Modifier.fillMaxSize().background(Ink).systemBarsPadding(),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text("Agents", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.SemiBold)
                Text(
                    dashboardSummary(agents.size, machines.size),
                    color = Muted,
                    style = MaterialTheme.typography.labelLarge,
                )
            }
            TextButton(onClick = controller::refresh, enabled = !state.refreshing) {
                Text(if (state.refreshing) "Reading…" else "Refresh")
            }
            TextButton(onClick = controller::addMachine) { Text("Add machine") }
        }

        MachineFilters(state.machines, state.selectedMachine, controller::selectMachine)
        machines.forEach { MachineStrip(it, controller) }

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

        Row(
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                if (state.selectedMachine == null) "Choose the target machine in Forge." else
                    "New agents use only this machine’s profiles and paths.",
                color = Muted,
                style = MaterialTheme.typography.labelMedium,
                modifier = Modifier.weight(1f),
            )
            Button(
                onClick = controller::openForge,
                enabled = state.machines.any { machine ->
                    (state.selectedMachine == null || machine.machine.handle == state.selectedMachine) &&
                        machine.canMutate &&
                        (machine.inventory as? InventoryState.Fresh)
                            ?.snapshot?.inventory?.profiles?.isNotEmpty() == true
                },
                modifier = Modifier.testTag("new-agent"),
            ) { Text("New agent") }
        }

        when {
            state.machines.isEmpty() -> EmptyState("No paired machines", "Pair a Tailscale-reachable machine to begin.")
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
    state.rename?.let { rename ->
        RenameMachineDialog(
            state = rename,
            onDraftChange = controller::updateRenameMachineDraft,
            onDismiss = controller::dismissRenameMachine,
            onConfirm = controller::confirmRenameMachine,
        )
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
private fun MachineStrip(machine: MachineState, controller: SkidbladnirController) {
    val stale = machine.inventory is InventoryState.Stale
    Surface(
        color = RaisedSurface,
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 4.dp)
            .testTag("machine-strip-${machine.machine.handle.encoded}"),
        shape = RoundedCornerShape(10.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 9.dp)) {
            (machine.inventory as? InventoryState.Fresh)?.let { fresh ->
                Spacer(
                    Modifier
                        .size(1.dp)
                        .testTag(
                            "machine-inventory-received-${machine.machine.handle.encoded}-${fresh.snapshot.receivedAtElapsedMillis}",
                        ),
                )
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.testTag(
                    "machine-state-${machineStateTag(machine)}-${machine.machine.handle.encoded}",
                ),
            ) {
                Text(
                    machine.machine.label.text,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier
                        .weight(1f)
                        .testTag("machine-strip-label-${machine.machine.handle.encoded}"),
                )
                if (stale) Text("STALE", color = Ember, fontWeight = FontWeight.Bold)
                Text(" · ${pressureSummary(machine.pressure)}", color = pressureStateColor(machine.pressure))
            }
            machineStateMessage(machine)?.let { message ->
                Text(message, color = if (machine.access == MachineAccess.Ready) Ember else Gold, style = MaterialTheme.typography.labelMedium)
            }
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                modifier = Modifier.testTag(
                    "machine-${if (machine.canMutate) "actionable" else "nonmutating"}-${machine.machine.handle.encoded}",
                ),
            ) {
                TextButton(onClick = { controller.requestRenameMachine(machine.machine.handle) }) {
                    Text("Rename")
                }
                if (machine.access == MachineAccess.AuthRequired) {
                    TextButton(onClick = { controller.repairMachine(machine.machine.handle) }) { Text("Update bearer") }
                }
                TextButton(onClick = { controller.removeMachine(machine.machine.handle) }) { Text("Remove machine") }
            }
        }
    }
}

@Composable
private fun RenameMachineDialog(
    state: RenameState,
    onDraftChange: (String) -> Unit,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Rename ${state.machine.label.text}") },
        text = {
            Column {
                Text(
                    "This changes only the label on this phone. The machine handle, origin, and bearer stay unchanged.",
                    color = Muted,
                )
                OutlinedTextField(
                    value = state.draft,
                    onValueChange = onDraftChange,
                    modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                    enabled = !state.pending,
                    label = { Text("Machine label") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(autoCorrectEnabled = false),
                )
                state.error?.let {
                    Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.padding(top = 8.dp))
                }
            }
        },
        confirmButton = {
            Button(onClick = onConfirm, enabled = state.draft.isNotEmpty() && !state.pending) {
                if (state.pending) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(8.dp))
                }
                Text(if (state.pending) "Saving…" else "Save label")
            }
        },
        dismissButton = { OutlinedButton(onClick = onDismiss, enabled = !state.pending) { Text("Cancel") } },
    )
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
    machine.access == MachineAccess.IdentityChanged -> "${machine.machine.label.text}: identity changed. Remove and pair again."
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

private fun pressureSummary(state: PressureState): String = when (state) {
    PressureState.Reading -> "pressure reading"
    is PressureState.Fresh -> "pressure ${state.response.current.level.name.uppercase()}"
    is PressureState.Stale -> "pressure STALE · ${state.response.current.level.name.uppercase()}"
    is PressureState.Unavailable -> "pressure unavailable"
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
            "${machine.machine.label.text}: identity changed; remove and pair it again."
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
        "${machine.machine.label.text} identity changed. Remove and pair it again; " +
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
