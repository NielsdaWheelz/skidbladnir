package dev.niels.skidbladnir

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import java.time.Instant

@Composable
internal fun DashboardScreen(
    state: SkidbladnirUiState.Dashboard,
    controller: SkidbladnirController,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Ink)
            .systemBarsPadding(),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "Agents",
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    text = state.inventory?.let {
                        "${it.sessions.size} tmux sessions${if (state.inventoryStale) " · STALE" else ""}"
                    } ?: "Reading tmux sessions",
                    color = Muted,
                    style = MaterialTheme.typography.labelLarge,
                )
            }
            TextButton(
                onClick = controller::refresh,
                enabled = !state.refreshing,
            ) {
                Text(if (state.refreshing) "Reading…" else "Refresh")
            }
            Button(
                onClick = controller::openForge,
                enabled = state.inventory?.profiles?.isNotEmpty() == true &&
                    state.forgeRecovery !is ForgeRecovery.RefreshRequired,
            ) {
                Text(if (state.forgeRecovery is ForgeRecovery.ReviewReady) "Resume draft" else "New agent")
            }
        }

        state.pressure?.let { PressureStrip(it) }

        state.forgeRecovery?.let { recovery ->
            ForgeRecoveryBanner(
                recovery = recovery,
                onResume = controller::openForge,
                onDiscard = controller::discardForgeRecovery,
            )
        }

        if (state.notice != null) {
            Surface(
                color = MaterialTheme.colorScheme.error.copy(alpha = 0.16f),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 6.dp),
                shape = RoundedCornerShape(10.dp),
            ) {
                Row(
                    modifier = Modifier.padding(start = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = state.notice,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier
                            .weight(1f)
                            .padding(vertical = 12.dp),
                    )
                    TextButton(onClick = controller::dismissNotice) { Text("Dismiss") }
                }
            }
        }

        if (state.error != null) {
            Surface(
                color = MaterialTheme.colorScheme.error.copy(alpha = 0.16f),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 6.dp),
                shape = RoundedCornerShape(10.dp),
            ) {
                Text(
                    text = state.error,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(12.dp),
                )
            }
        }

        val inventory = state.inventory
        when {
            inventory == null -> Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator()
            }
            inventory.sessions.isEmpty() -> Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(32.dp),
                contentAlignment = Alignment.Center,
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("No tmux sessions", style = MaterialTheme.typography.titleLarge)
                    Text(
                        text = "Start an agent here, or launch a tmux session from your laptop.",
                        color = Muted,
                        modifier = Modifier.padding(top = 8.dp),
                    )
                }
            }
            else -> LazyVerticalGrid(
                columns = GridCells.Adaptive(170.dp),
                modifier = Modifier.fillMaxSize(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(12.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                items(
                    items = inventory.sessions,
                    key = { "${it.id}:${it.identityToken}" },
                ) { session ->
                    AgentCard(
                        session = session,
                        profiles = inventory.profiles,
                        observedAt = Instant.parse(inventory.observedAt)
                            .plusSeconds(state.inventoryAgeAdvanceSeconds),
                        onOpen = { controller.openTerminal(session) },
                        onKill = { controller.requestKill(session) },
                    )
                }
            }
        }
    }

    state.forge?.let { forge ->
        ForgeSheet(
            state = forge,
            profiles = state.inventory?.profiles.orEmpty(),
            onDismiss = controller::dismissForge,
            onDraftChange = controller::updateForgeDraft,
            onSubmit = controller::forge,
        )
    }
    state.kill?.let { kill -> KillConfirmation(kill, controller::dismissKill, controller::confirmKill) }
}

@Composable
private fun ForgeRecoveryBanner(
    recovery: ForgeRecovery,
    onResume: () -> Unit,
    onDiscard: () -> Unit,
) {
    Surface(
        color = Gold.copy(alpha = 0.16f),
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 6.dp),
        shape = RoundedCornerShape(10.dp),
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Text(
                text = when (recovery) {
                    is ForgeRecovery.RefreshRequired ->
                        "Start outcome unknown. Agents must refresh before this draft can be resumed."
                    is ForgeRecovery.ReviewReady ->
                        "Agents refreshed. Review the grid before resuming this saved draft."
                },
                color = Gold,
            )
            if (recovery is ForgeRecovery.ReviewReady) {
                Row(
                    modifier = Modifier.padding(top = 6.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    TextButton(onClick = onResume) { Text("Resume draft") }
                    TextButton(onClick = onDiscard) { Text("Discard draft") }
                }
            }
        }
    }
}

@Composable
private fun AgentCard(
    session: AgentSession,
    profiles: List<ProfileChoice>,
    observedAt: Instant,
    onOpen: () -> Unit,
    onKill: () -> Unit,
) {
    val status = statusContent(session.status, observedAt)
    Card(
        onClick = onOpen,
        colors = CardDefaults.cardColors(containerColor = DeepSurface),
        shape = RoundedCornerShape(14.dp),
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                DwarfPortrait(session.character)
                Spacer(Modifier.width(10.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = session.tmuxName,
                        fontWeight = FontWeight.SemiBold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Text(
                        text = session.character.displayName,
                        color = Muted,
                        style = MaterialTheme.typography.labelMedium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                if (session.attention) {
                    Text(
                        text = "!",
                        color = Ember,
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.semantics { contentDescription = "Needs attention" },
                    )
                }
            }
            Surface(
                color = statusColor(session.status.kind).copy(alpha = 0.18f),
                shape = RoundedCornerShape(7.dp),
                modifier = Modifier
                    .padding(top = 10.dp)
                    .semantics { contentDescription = status.accessibilityLabel },
            ) {
                Column(modifier = Modifier.padding(horizontal = 8.dp, vertical = 5.dp)) {
                    Text(
                        text = status.kind,
                        color = statusColor(session.status.kind),
                        style = MaterialTheme.typography.labelLarge,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        text = status.evidence,
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                    )
                }
            }
            session.objective?.let {
                Text(
                    text = it,
                    modifier = Modifier.padding(top = 10.dp),
                    maxLines = 3,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            session.cwd?.let {
                Text(
                    text = it,
                    modifier = Modifier.padding(top = 9.dp),
                    color = Muted,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.labelSmall,
                )
            }
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 9.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = agentCardRuntimeFacts(session, profiles).joinToString(" · "),
                    color = Muted,
                    style = MaterialTheme.typography.labelSmall,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    text = "${session.attachedClients} ${if (session.attachedClients == 1) "client" else "clients"}",
                    color = Muted,
                    style = MaterialTheme.typography.labelSmall,
                )
            }
            TextButton(
                onClick = onKill,
                colors = ButtonDefaults.textButtonColors(contentColor = Ember),
                modifier = Modifier.align(Alignment.End),
            ) {
                Text("Kill")
            }
        }
    }
}

internal fun agentCardRuntimeFacts(
    session: AgentSession,
    profiles: List<ProfileChoice>,
): List<String> = listOfNotNull(
    session.profile?.let { key -> profiles.firstOrNull { it.key == key }?.label }
        ?: "profile unknown",
    session.activeCommand,
)

@Composable
private fun DwarfPortrait(character: CharacterSummary) {
    val seed = character.key
    val hash = seed.fold(17) { value, symbol -> value * 31 + symbol.code }
    val palette = listOf(
        Color(0xFF8F503B),
        Color(0xFF4D6A63),
        Color(0xFF735E91),
        Color(0xFF6F7041),
        Color(0xFF865E35),
        Color(0xFF3F647D),
    )
    val field = palette[Math.floorMod(hash, palette.size)]
    val beard = palette[Math.floorMod(hash.rotateLeft(7), palette.size)]
    val label = character.displayName.take(1).uppercase()
    Box(
        modifier = Modifier
            .size(58.dp)
            .clip(RoundedCornerShape(12.dp))
            .semantics {
                contentDescription = "Portrait of ${character.displayName}"
            },
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.fillMaxSize()) {
            drawRect(field.copy(alpha = 0.38f))
            drawCircle(Color(0xFFD5AA7B), radius = size.minDimension * 0.25f, center = center.copy(y = size.height * 0.38f))
            val beardPath = Path().apply {
                moveTo(size.width * 0.25f, size.height * 0.44f)
                lineTo(size.width * 0.75f, size.height * 0.44f)
                lineTo(size.width * (0.58f + (hash and 3) * 0.025f), size.height * 0.9f)
                lineTo(size.width * (0.42f - (hash and 3) * 0.025f), size.height * 0.9f)
                close()
            }
            drawPath(beardPath, beard)
            drawArc(
                color = Color(0xFFB5A37E),
                startAngle = 180f,
                sweepAngle = 180f,
                useCenter = true,
                topLeft = center.copy(x = size.width * 0.2f, y = size.height * 0.13f),
                size = size.copy(width = size.width * 0.6f, height = size.height * 0.36f),
            )
        }
        Text(
            text = label,
            color = Bone,
            fontWeight = FontWeight.Black,
            modifier = Modifier.align(Alignment.BottomEnd).padding(4.dp),
            style = MaterialTheme.typography.labelMedium,
        )
    }
}

@Composable
private fun PressureStrip(response: PressureResponse) {
    val current = response.current
    val known = buildList {
        current.metrics.cpuPercent?.let { add("CPU ${it.toInt()}%") }
        current.metrics.memoryAvailablePercent?.let { add("RAM ${it.toInt()}% free") }
        current.metrics.swapUsedPercent?.let { add("swap ${it.toInt()}% used") }
        current.metrics.normalizedLoad?.let { add("load ${"%.2f".format(it)}") }
        current.metrics.diskAvailablePercent?.let { add("disk ${it.toInt()}% free") }
        current.metrics.cpuPsiSomeAvg60Percent?.let { add("CPU PSI ${it.toInt()}%") }
        current.metrics.memoryPsiFullAvg60Percent?.let { add("memory PSI ${it.toInt()}%") }
        current.metrics.ioPsiFullAvg60Percent?.let { add("I/O PSI ${it.toInt()}%") }
    }
    Surface(
        color = RaisedSurface,
        shape = RectangleShape,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(modifier = Modifier.padding(horizontal = 16.dp, vertical = 9.dp)) {
            Column {
                Text(
                    text = "DEVBOX ${current.level.name.uppercase()}",
                    color = pressureColor(current.level),
                    style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.Bold,
                )
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
                        .padding(top = 5.dp),
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
                Text("Recent pressure history · up to 15 min", color = Muted, style = MaterialTheme.typography.labelSmall)
            }
            if (current.missing.isNotEmpty()) {
                Text(
                    text = "Missing: ${current.missing.joinToString { pressureMetricLabel(it) }}",
                    color = Muted,
                    style = MaterialTheme.typography.labelSmall,
                )
            }
            if (current.reasons.isNotEmpty()) {
                Text(
                    text = "Pressure: ${current.reasons.joinToString { pressureReasonLabel(it) }}",
                    color = pressureColor(current.level),
                    style = MaterialTheme.typography.labelSmall,
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ForgeSheet(
    state: ForgeState,
    profiles: List<ProfileChoice>,
    onDismiss: () -> Unit,
    onDraftChange: ((ForgeDraft) -> ForgeDraft) -> Unit,
    onSubmit: () -> Unit,
) {
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        containerColor = DeepSurface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .imePadding()
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp),
        ) {
            Text("New agent", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.SemiBold)
            Text(
                "The Forge starts one reviewed launch profile in this directory.",
                color = Muted,
                modifier = Modifier.padding(top = 4.dp, bottom = 14.dp),
            )
            OutlinedTextField(
                value = state.draft.cwd,
                onValueChange = { value -> onDraftChange { it.copy(cwd = value) } },
                modifier = Modifier.fillMaxWidth(),
                enabled = !state.pending,
                label = { Text("Working directory") },
                placeholder = { Text("~/src/project") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(
                    capitalization = KeyboardCapitalization.None,
                    autoCorrectEnabled = false,
                    keyboardType = KeyboardType.Uri,
                ),
                supportingText = { Text("Existing directory on the devbox") },
            )
            Text("Profile", style = MaterialTheme.typography.labelLarge, modifier = Modifier.padding(top = 12.dp))
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                profiles.forEach { profile ->
                    FilterChip(
                        selected = state.draft.profile == profile.key,
                        onClick = { onDraftChange { it.copy(profile = profile.key) } },
                        enabled = !state.pending,
                        label = { Text(profile.label) },
                    )
                }
            }
            OutlinedTextField(
                value = state.draft.optionalTmuxName,
                onValueChange = { value -> onDraftChange { it.copy(optionalTmuxName = value) } },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
                enabled = !state.pending,
                label = { Text("tmux name (optional)") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(
                    capitalization = KeyboardCapitalization.None,
                    autoCorrectEnabled = false,
                    keyboardType = KeyboardType.Ascii,
                ),
            )
            OutlinedTextField(
                value = state.draft.objective,
                onValueChange = { value -> onDraftChange { it.copy(objective = value) } },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp),
                enabled = !state.pending,
                label = { Text("Objective (optional)") },
                minLines = 2,
                maxLines = 4,
                supportingText = { Text("${state.draft.objective.codePointCount()} / 240") },
            )
            if (state.error != null) {
                Text(
                    text = state.error,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(top = 10.dp),
                )
            }
            Button(
                onClick = onSubmit,
                enabled = !state.pending && state.draft.cwd.isNotEmpty(),
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 16.dp),
            ) {
                if (state.pending) {
                    CircularProgressIndicator(modifier = Modifier.size(20.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(8.dp))
                }
                Text("Start agent")
            }
        }
    }
}

@Composable
internal fun KillConfirmation(
    state: KillState,
    onDismiss: () -> Unit,
    onConfirm: () -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Kill session ${state.target.tmuxName}?") },
        text = {
            Column {
                Text("This tmux session and its processes end now.")
                Text(
                    "Detach leaves it running. Kill does not.",
                    color = Muted,
                    modifier = Modifier.padding(top = 8.dp),
                )
                if (state.error != null) {
                    Text(
                        state.error,
                        color = MaterialTheme.colorScheme.error,
                        modifier = Modifier.padding(top = 12.dp),
                    )
                }
            }
        },
        confirmButton = {
            Button(
                onClick = onConfirm,
                enabled = !state.pending,
                colors = ButtonDefaults.buttonColors(containerColor = Ember, contentColor = Ink),
            ) {
                if (state.pending) {
                    CircularProgressIndicator(modifier = Modifier.size(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(8.dp))
                }
                Text("Kill session")
            }
        },
        dismissButton = {
            OutlinedButton(onClick = onDismiss, enabled = !state.pending) { Text("Keep running") }
        },
        containerColor = DeepSurface,
    )
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

private fun pressureMetricLabel(value: PressureMetric): String = when (value) {
    PressureMetric.CpuPercent -> "CPU"
    PressureMetric.NormalizedLoad -> "load"
    PressureMetric.MemoryAvailablePercent -> "memory"
    PressureMetric.SwapUsedPercent -> "swap"
    PressureMetric.DiskAvailablePercent -> "disk"
    PressureMetric.CpuPsiSomeAvg60Percent -> "CPU pressure"
    PressureMetric.MemoryPsiFullAvg60Percent -> "memory pressure"
    PressureMetric.IoPsiFullAvg60Percent -> "I/O pressure"
}

private fun pressureReasonLabel(reason: PressureReason): String = when (reason) {
    PressureReason.Memory -> "memory"
    PressureReason.Disk -> "disk"
    PressureReason.Load -> "load"
    PressureReason.CpuPsi -> "CPU pressure"
    PressureReason.MemoryPsi -> "memory pressure"
    PressureReason.IoPsi -> "I/O pressure"
}

private fun String.codePointCount(): Int = codePointCount(0, length)
