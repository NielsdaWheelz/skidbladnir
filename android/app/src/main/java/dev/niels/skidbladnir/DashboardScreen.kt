package dev.niels.skidbladnir

import android.provider.Settings
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
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
import androidx.compose.foundation.text.KeyboardOptions
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
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTag
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
                shape = NidavellirShapes.Card,
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
                shape = NidavellirShapes.Card,
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
            inventory.sessions.isEmpty() -> EmptyGridState()
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
internal fun EmptyGridState() {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .padding(32.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            // The Hlíðskjálf mark (design-language.md §8): decorative only,
            // so it clears semantics explicitly rather than relying on
            // Canvas having none by default — the literal text below it
            // carries the meaning (ornament-pipeline.md "Ornament is silent
            // and subordinate").
            Canvas(
                modifier = Modifier
                    .padding(bottom = 12.dp)
                    .size(48.dp)
                    .clearAndSetSemantics { testTag = "EmptyStateOrnament" },
            ) {
                drawValknut(Muted.copy(alpha = 0.40f))
            }
            Text("No tmux sessions", style = MaterialTheme.typography.titleLarge)
            Text(
                text = "Start an agent here, or launch a tmux session from your laptop.",
                color = Muted,
                modifier = Modifier.padding(top = 8.dp),
            )
        }
    }
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
        shape = NidavellirShapes.Card,
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

// M3's `Card(onClick)` hardcodes its internal ripple and never reads
// LocalIndication, so the card is a plain Surface carrying the same
// `clickable` the Card built for it — same click action, same merged
// descendant semantics, same roleless node, same minimum interactive size —
// with the angular press flash (docs/chrome-tokens.md "Interaction states").
@Composable
internal fun AgentCard(
    session: AgentSession,
    profiles: List<ProfileChoice>,
    observedAt: Instant,
    onOpen: () -> Unit,
    onKill: () -> Unit,
) {
    val status = statusContent(session.status, observedAt)
    val tone = statusColor(session.status.kind)
    Surface(
        color = DeepSurface,
        shape = NidavellirShapes.Card,
        modifier = Modifier
            .minimumInteractiveComponentSize()
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication,
                onClick = onOpen,
            ),
    ) {
        Column(
            modifier = Modifier
                .drawBehind {
                    drawRect(color = Gold.copy(alpha = 0.25f), size = size.copy(height = 1.dp.toPx()))
                }
                .padding(12.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                DwarfPortrait(session.character)
                Spacer(Modifier.width(10.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = session.character.displayName,
                        style = MaterialTheme.typography.titleLarge,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    Text(
                        text = session.tmuxName,
                        color = Muted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                if (session.attention) AttentionLozenge()
            }
            Surface(
                color = tone.copy(alpha = 0.18f),
                shape = NidavellirShapes.Chip,
                border = BorderStroke(1.dp, tone),
                modifier = Modifier
                    .padding(top = 10.dp)
                    .semantics { contentDescription = status.accessibilityLabel },
            ) {
                Column(modifier = Modifier.padding(horizontal = 8.dp, vertical = 5.dp)) {
                    Text(
                        text = status.kind,
                        color = tone,
                        style = MaterialTheme.typography.labelLarge,
                        fontFamily = NidavellirType.Data,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(
                        text = status.evidence,
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
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
                    fontFamily = NidavellirType.Data,
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
                    fontFamily = NidavellirType.Data,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    text = "${session.attachedClients} ${if (session.attachedClients == 1) "client" else "clients"}",
                    color = Muted,
                    style = MaterialTheme.typography.labelSmall,
                    fontFamily = NidavellirType.Data,
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

// The attention mark is an Orpiment lozenge — a rotated square, the fret
// family's atom (design-language.md §6). It pulses 1.0 -> 0.55 on a ~1.6s
// no-bounce loop, and renders static at full opacity when the system disables
// animations (§12); opening the card clears attention, which is the WCAG
// 2.2.2 stop mechanism.
@Composable
private fun AttentionLozenge() {
    val resolver = LocalContext.current.contentResolver
    val pulsing = remember(resolver) {
        attentionPulseEnabled(
            Settings.Global.getFloat(resolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f),
        )
    }
    val alpha = if (pulsing) {
        rememberInfiniteTransition(label = "attention").animateFloat(
            initialValue = 1f,
            targetValue = 0.55f,
            animationSpec = infiniteRepeatable(
                animation = NidavellirMotion.AttentionPulse,
                repeatMode = RepeatMode.Reverse,
            ),
            label = "attention alpha",
        ).value
    } else {
        1f
    }
    Box(
        modifier = Modifier
            // The rotated square paints ~1.7dp past its layout box; keep the
            // left tip clear of a full-width name line.
            .padding(start = 2.dp)
            .size(8.dp)
            .graphicsLayer(rotationZ = 45f, alpha = alpha)
            .background(Orpiment)
            .semantics { contentDescription = "Needs attention" },
    )
}

internal fun agentCardRuntimeFacts(
    session: AgentSession,
    profiles: List<ProfileChoice>,
): List<String> = listOfNotNull(
    session.profile?.let { key -> profiles.firstOrNull { it.key == key }?.label }
        ?: "profile unknown",
    session.activeCommand,
)

// The Niðavellir seal (design-language.md §11, dwarf-seals.md): a
// deterministic, pure function of `character.key` via `sealSpec`. Draw order
// is frozen in dwarf-seals.md: mineral fill, facet planes, beard silhouette,
// bind-rune, octagon frame, Bone initial.
@Composable
private fun DwarfPortrait(character: CharacterSummary) {
    val spec = sealSpec(character.key)
    val metal = if (spec.metal == SealMetal.Gold) Gold else Bronze
    val label = character.displayName.take(1).uppercase()
    Box(
        modifier = Modifier
            .size(58.dp)
            .clip(NidavellirShapes.Octagon)
            .semantics {
                contentDescription = "Portrait of ${character.displayName}"
            },
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.fillMaxSize()) {
            val w = size.width
            val h = size.height
            val side = size.minDimension

            drawRect(SealMinerals[spec.mineral])

            // Facet planes: two flat 45° highlight/shadow triangles.
            drawPath(
                Path().apply {
                    moveTo(0f, 0f)
                    lineTo(w, 0f)
                    lineTo(0f, h)
                    close()
                },
                Color.White.copy(alpha = 0.045f),
            )
            drawPath(
                Path().apply {
                    moveTo(w, h * 0.55f)
                    lineTo(w, h)
                    lineTo(w * 0.35f, h)
                    close()
                },
                Color.Black.copy(alpha = 0.16f),
            )

            // Beard silhouette: a trapezoid whose bottom edge is cut with
            // beardTeeth angular notches, tips shorter than valleys by
            // beardDepthStep. No curve anywhere (design-language.md §11).
            val beardTopY = h * 0.60f
            val beardLeftX = w * 0.24f
            val beardRightX = w * 0.76f
            val valleyY = h * 0.88f
            val tipY = valleyY - (0.10f + spec.beardDepthStep * 0.022f) * h
            val toothSpan = spec.beardTeeth - 1
            val toothWidth = (beardRightX - beardLeftX) / toothSpan
            val beardPath = Path().apply {
                moveTo(beardLeftX, beardTopY)
                lineTo(beardRightX, beardTopY)
                lineTo(beardRightX, tipY)
                for (tooth in 1..toothSpan) {
                    lineTo(beardRightX - (tooth - 0.5f) * toothWidth, valleyY)
                    lineTo(beardRightX - tooth * toothWidth, tipY)
                }
                close()
            }
            drawPath(beardPath, Color.Black.copy(alpha = 0.34f))
            drawPath(
                beardPath,
                Color.White.copy(alpha = 0.10f),
                style = Stroke(width = 1f, cap = StrokeCap.Butt, join = StrokeJoin.Miter),
            )

            // Bind-rune: a shared vertical stave plus every drawn rune's
            // segments, monoline in the seal's metal (design-language.md
            // §8 — ornament, never text; carries no contentDescription).
            val staveX = w * 0.5f
            val staveTop = h * 0.15f
            val staveBottom = h * 0.58f
            val runeWidth = w * 0.30f
            val bindRune = Path().apply {
                moveTo(staveX, staveTop)
                lineTo(staveX, staveBottom)
                spec.runes.forEach { rune ->
                    RuneSegments[rune].forEach { seg ->
                        moveTo(staveX + seg.x0 * runeWidth, staveTop + seg.y0 * (staveBottom - staveTop))
                        lineTo(staveX + seg.x1 * runeWidth, staveTop + seg.y1 * (staveBottom - staveTop))
                    }
                }
            }
            drawPath(
                bindRune,
                metal,
                style = Stroke(width = side * 0.045f, cap = StrokeCap.Butt, join = StrokeJoin.Miter),
            )

            // Octagon frame: neutral base hairline on all 8 edges, Gold
            // overlaid thicker on the edges set in facetMask. Vertices come
            // from the same 29% cut geometry as the clip shape.
            val cut = side * 0.29f
            val vertices = listOf(
                Offset(cut, 0f),
                Offset(w - cut, 0f),
                Offset(w, cut),
                Offset(w, h - cut),
                Offset(w - cut, h),
                Offset(cut, h),
                Offset(0f, h - cut),
                Offset(0f, cut),
            )
            for (edge in vertices.indices) {
                drawLine(
                    color = Color(0xFF3A3E45),
                    start = vertices[edge],
                    end = vertices[(edge + 1) % vertices.size],
                    strokeWidth = side * 0.012f,
                    cap = StrokeCap.Butt,
                )
            }
            for (edge in vertices.indices) {
                if ((spec.facetMask shr edge) and 1 == 1) {
                    drawLine(
                        color = Gold,
                        start = vertices[edge],
                        end = vertices[(edge + 1) % vertices.size],
                        strokeWidth = side * 0.025f,
                        cap = StrokeCap.Butt,
                    )
                }
            }
        }
        Text(
            text = label,
            color = Bone,
            fontFamily = NidavellirType.Display,
            fontWeight = FontWeight.Black,
            modifier = Modifier.align(Alignment.BottomEnd).padding(6.dp),
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
internal fun ForgeSheet(
    state: ForgeState,
    profiles: List<ProfileChoice>,
    onDismiss: () -> Unit,
    onDraftChange: ((ForgeDraft) -> ForgeDraft) -> Unit,
    onSubmit: () -> Unit,
) {
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
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .imePadding()
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp),
        ) {
            Text(
                text = "New agent",
                style = MaterialTheme.typography.headlineSmall,
                fontFamily = NidavellirType.Display,
                fontWeight = FontWeight.SemiBold,
            )
            Text(
                "The Forge starts one reviewed launch profile in this directory.",
                color = Muted,
                modifier = Modifier.padding(top = 4.dp, bottom = 14.dp),
            )
            // The fret band (design-language.md §7): decorative only, drawn
            // under the title block, Gold at the family's 40% ceiling.
            Canvas(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(12.dp)
                    .padding(bottom = 14.dp),
            ) {
                drawOrnamentBand(unitAspect = 1f, layers = listOf(FretCell to Gold.copy(alpha = 0.40f)))
            }
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
        shape = NidavellirShapes.Card,
        containerColor = DeepSurface,
    )
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
