package dev.niels.skidbladnir

import android.os.SystemClock
import android.provider.Settings
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// M3's `Card(onClick)` hardcodes its internal ripple and never reads
// LocalIndication, so the card is a plain Surface carrying the same
// `clickable` the Card built for it — same click action, same merged
// descendant semantics, same roleless node, same minimum interactive size —
// with the angular press flash (docs/chrome-tokens.md "Interaction states").
@Composable
internal fun AgentCard(
    agent: VisibleAgent,
    machine: MachineState,
    showMachineLabel: Boolean,
    onOpen: () -> Unit,
    onKill: () -> Unit,
) {
    val session = agent.target.session
    val snapshot = machine.inventory.lastSnapshot() ?: return
    val status = statusContent(
        session.status,
        snapshot.inventory.observedAt.plusMillis(
            (SystemClock.elapsedRealtime() - snapshot.receivedAtElapsedMillis).coerceAtLeast(0),
        ),
    )
    val tone = statusColor(session.status.kind)
    val profile = snapshot.inventory.profiles
        .firstOrNull { it.key.encoded == session.profile }?.label
        ?: session.profile
        ?: "profile unknown"
    val visibleContext = if (showMachineLabel) "${agent.machine.label.text} · $profile" else profile
    Surface(
        color = DeepSurface,
        shape = NidavellirShapes.Card,
        modifier = Modifier
            .testTag("agent-card-${agent.target.machineHandle.encoded}-${session.id}")
            .minimumInteractiveComponentSize()
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Card),
                enabled = machine.canMutate,
                onClick = onOpen,
            ),
    ) {
        Column(
            modifier = Modifier
                .drawBehind {
                    drawRect(color = Gold.copy(alpha = 0.25f), size = size.copy(height = 1.dp.toPx()))
                }
                .padding(10.dp),
        ) {
            SessionIdentityHeader(
                tmuxName = session.tmuxName,
                dwarfName = session.character.displayName,
                attention = session.attention,
                statusTone = tone,
                statusFacetTag = "agent-status-facet-${agent.target.machineHandle.encoded}-${session.id}",
            )
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                DwarfPortrait(session.character)
                StatusBay(status = status, tone = tone, modifier = Modifier.weight(1f))
            }
            if (!machine.canMutate) {
                // The tone is the machine's own, never a fixed Degraded: a machine whose bearer
                // broke or whose identity changed still has a Fresh inventory, so it reaches this
                // marker as a trust failure, and painting that calm is the inversion this delta
                // exists to abolish. The marker's wording remains architecture-owned.
                Text(
                    if (machine.inventory is InventoryState.Superseded) {
                        "REFRESHING · actions disabled"
                    } else {
                        "STALE · actions disabled"
                    },
                    color = noticeToneColor(availabilityTone(machineAvailability(machine))),
                    style = MaterialTheme.typography.labelSmall,
                    fontFamily = NidavellirType.Data,
                    modifier = Modifier.padding(top = 8.dp),
                )
            }
            session.objective?.let {
                Text(
                    text = it,
                    modifier = Modifier
                        .padding(top = 8.dp)
                        .testTag("agent-objective-${agent.target.machineHandle.encoded}-${session.id}"),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            session.cwd?.let { directory ->
                Text(
                    text = abbreviatedDirectory(directory),
                    modifier = Modifier
                        .padding(top = 8.dp)
                        .testTag("agent-directory-${agent.target.machineHandle.encoded}-${session.id}")
                        .semantics { contentDescription = "Directory $directory" },
                    color = Muted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.labelSmall,
                    fontFamily = NidavellirType.Data,
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = visibleContext,
                    color = Muted,
                    style = MaterialTheme.typography.labelSmall,
                    fontFamily = NidavellirType.Data,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .testTag("agent-context-${agent.target.machineHandle.encoded}-${session.id}")
                        .semantics {
                            contentDescription = "Machine ${agent.machine.label.text}. Profile $profile."
                        },
                )
                KillButton(
                    machineLabel = agent.machine.label,
                    target = agent.target,
                    enabled = machine.canMutate,
                    onClick = onKill,
                    modifier = Modifier.testTag(
                        "agent-kill-${agent.target.machineHandle.encoded}-${session.id}",
                    ),
                )
            }
        }
    }
}

@Composable
private fun SessionIdentityHeader(
    tmuxName: String,
    dwarfName: String,
    attention: Boolean,
    statusTone: Color,
    statusFacetTag: String,
) {
    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.Top) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = tmuxName,
                color = Bone,
                style = MaterialTheme.typography.titleMedium,
                fontFamily = NidavellirType.Data,
                fontWeight = FontWeight.Bold,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = dwarfName,
                color = Muted,
                style = MaterialTheme.typography.labelMedium,
                fontFamily = NidavellirType.Display,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(8.dp))
        Row(
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (attention) AttentionLozenge()
            StatusFacet(statusTone, statusFacetTag)
        }
    }
}

@Composable
private fun StatusFacet(tone: Color, tag: String) {
    Box(
        modifier = Modifier
            .size(12.dp)
            .clip(NidavellirShapes.Chip)
            .background(tone)
            .testTag(tag),
    )
}

@Composable
private fun StatusBay(status: StatusContent, tone: Color, modifier: Modifier = Modifier) {
    Surface(
        color = tone.copy(alpha = 0.18f),
        shape = NidavellirShapes.Chip,
        border = BorderStroke(1.dp, tone),
        modifier = modifier
            .heightIn(min = 48.dp)
            .semantics { contentDescription = status.accessibilityLabel },
    ) {
        Column(modifier = Modifier.padding(horizontal = 3.dp, vertical = 4.dp)) {
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
                style = MaterialTheme.typography.labelSmall.copy(letterSpacing = (-0.6).sp),
                fontFamily = NidavellirType.Data,
            )
        }
    }
}

internal fun abbreviatedDirectory(directory: String): String {
    val segments = directory.split('/').filter(String::isNotEmpty)
    return if (segments.size <= 2) directory else "…/${segments.takeLast(2).joinToString("/")}"
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
    Box(Modifier.size(width = 14.dp, height = 8.dp)) {
        Box(
            modifier = Modifier
                .size(8.dp)
                .semantics { contentDescription = "Needs attention" }
                .graphicsLayer(rotationZ = 45f, alpha = alpha)
                .background(Orpiment),
        )
    }
}

// The Niðavellir seal (design-language.md §11, dwarf-seals.md): a
// deterministic, pure function of `character.key` via `sealSpec`. Draw order
// is frozen in dwarf-seals.md: mineral fill, facet planes, beard silhouette,
// bind-rune, octagon frame, Bone initial.
@Composable
internal fun DwarfPortrait(character: CharacterSummary) {
    val spec = sealSpec(character.key)
    val metal = if (spec.metal == SealMetal.Gold) Gold else Bronze
    val label = character.displayName.take(1).uppercase()
    Box(
        modifier = Modifier
            .size(48.dp)
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
            // overlaid thicker on the edges set in facetMask. The vertices are
            // the clip shape's own cut, expanded once in Theme.kt, so the two
            // cannot drift; their order is what facetMask indexes.
            val vertices = octagonVertices(size)
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
