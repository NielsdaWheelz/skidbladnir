package dev.niels.skidbladnir

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.InlineTextContent
import androidx.compose.foundation.text.appendInlineContent
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
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
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.Placeholder
import androidx.compose.ui.text.PlaceholderVerticalAlign
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp

// The session card (docs/session-card.md): persona left, then four lines of
// work: what (tmux name), state (status), who (dwarf signature and runtime
// profile), where (host and directory). Verbs live behind the overflow. The
// clickable body owns the card's whole spoken account in one node; the
// overflow is its only sibling. M3's `Card(onClick)` hardcodes its ripple, so
// the body is a plain clickable with the angular press flash
// (docs/chrome-tokens.md "Interaction states").
@Composable
internal fun SessionCard(
    visibleSession: VisibleSession,
    machine: MachineState,
    machines: List<MachineState>,
    showMachineLabel: Boolean,
    motionEnabled: Boolean,
    terminalControlPending: Boolean,
    onOpen: () -> Unit,
    onGroup: () -> Unit,
    onStop: () -> Unit,
    onTerminalClose: () -> Unit,
    onClose: () -> Unit,
) {
    val session = visibleSession.target.session
    val snapshot = machine.inventory.lastSnapshot() ?: return
    val context = visibleSession.context
    val terminalMachine = visibleSession.machine.label.text
    val notification = machine.notifications[NotificationKey(visibleSession.target)] ?: NotificationPresentation()
    val status = sessionStatusContent(session, machine.canMutate, notification)
    val tone = sessionStatusColor(status.tone)
    val availability = sessionAvailabilityContent(machine)
    val actions = listOf(SessionAction("change group", machine.canMutate, destructive = false, onGroup)) +
        terminalLifetimeActions(machine.canMutate && !terminalControlPending, onStop, onTerminalClose, onClose)
    // A plain pane has no runtime to name: its status line already reads `terminal`.
    val profile = when (context) {
        is ExecutionContext.Local -> sessionProfileLabel(session, snapshot.inventory.profiles)
        is ExecutionContext.Remote -> context.agent?.let { remoteAgentLabel(context, machines) }
        ExecutionContext.RemoteUnknown -> null
    }
    val directory = when (context) {
        is ExecutionContext.Local -> context.cwd
        is ExecutionContext.Remote -> context.cwd
        ExecutionContext.RemoteUnknown -> null
    }
    // A machine filter already names the machine once, so a local card drops it
    // from sight; speech and every routed action keep it.
    val host = when (context) {
        is ExecutionContext.Local -> terminalMachine.takeIf { showMachineLabel }
        is ExecutionContext.Remote -> "running on ${context.machine.label.text} · terminal on $terminalMachine"
        ExecutionContext.RemoteUnknown -> "remote context unknown · terminal on $terminalMachine"
    }
    val shownDirectory = directory?.let(::abbreviatedDirectory)
        ?: "directory unavailable".takeIf { context is ExecutionContext.Remote }
    val density = LocalDensity.current
    val facetSize = with(density) { 12.dp.toSp() }
    val nameLineHeight = with(density) { MaterialTheme.typography.titleSmall.lineHeight.toDp() }
    val spoken = buildString {
        append("${session.tmuxName}. ${status.accessibilityLabel}. ${session.character.displayName}. ")
        append(
            when (context) {
                is ExecutionContext.Local -> "Machine $terminalMachine."
                is ExecutionContext.Remote -> "Running on ${context.machine.label.text}. Terminal on $terminalMachine."
                ExecutionContext.RemoteUnknown -> "Remote context unknown. Terminal on $terminalMachine."
            },
        )
        profile?.let { append(" Profile $it.") }
        directory?.let { append(" Directory $it.") }
        if (directory == null && context is ExecutionContext.Remote) append(" Directory unavailable.")
        availability?.let { append(" ${it.label}.") }
        session.objective?.let { append(" Objective: $it") }
    }
    Surface(color = DeepSurface, shape = NidavellirShapes.Card) {
        Box(Modifier.drawBehind { drawRect(Gold.copy(alpha = 0.25f), size = size.copy(height = 1.dp.toPx())) }) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = AngularIndication(NidavellirShapes.Card),
                        enabled = machine.canMutate,
                        onClickLabel = "open terminal",
                        onClick = onOpen,
                    )
                    .clearAndSetSemantics {
                        contentDescription = spoken
                        customActions = actions.filter(SessionAction::enabled).map { action ->
                            CustomAccessibilityAction(action.label) {
                                action.perform()
                                true
                            }
                        }
                    }
                    // The end inset is the overflow's 48dp column: text never runs under it.
                    .padding(start = CardPadding, top = CardPadding, end = 48.dp, bottom = CardPadding),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                DwarfPortrait(session.character)
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(
                        text = session.tmuxName,
                        color = Bone,
                        style = MaterialTheme.typography.titleSmall,
                        fontFamily = NidavellirType.Data,
                        fontWeight = FontWeight.Bold,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                    // The facet is the status line's first glyph, so it holds the
                    // first line at every font scale and wraps with the words. The
                    // label never yields; the detail takes what width remains, so a
                    // status change cannot reflow the list (session-card.md
                    // invariant 10).
                    Row {
                        Text(
                            text = buildAnnotatedString {
                                appendInlineContent(FACET)
                                append(" ")
                                withStyle(SpanStyle(color = tone, fontWeight = FontWeight.Bold)) { append(status.label) }
                            },
                        inlineContent = mapOf(
                            FACET to InlineTextContent(Placeholder(facetSize, facetSize, PlaceholderVerticalAlign.TextCenter)) {
                                ActivityFacet(
                                    working = session.agent != null &&
                                        session.terminalStatus.activity == TerminalActivity.Working,
                                    tone = tone,
                                    animate = machine.canMutate && motionEnabled,
                                )
                            },
                        ),
                            style = MaterialTheme.typography.labelMedium,
                            fontFamily = NidavellirType.Data,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier.alignByBaseline(),
                        )
                        status.detail?.let {
                            Text(
                                text = " · $it",
                                color = Muted,
                                style = MaterialTheme.typography.labelMedium,
                                fontFamily = NidavellirType.Data,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.weight(1f, fill = false).alignByBaseline(),
                            )
                        }
                    }
                    availability?.let { CardFact(it.label, noticeToneColor(it.tone)) }
                    status.secondary?.let { CardFact(it, Muted) }
                    session.objective?.let {
                        Text(
                            text = it,
                            style = MaterialTheme.typography.bodySmall,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    // Who: the dwarf signs in the display face; its runtime follows as a machine fact.
                    Text(
                        text = buildAnnotatedString {
                            withStyle(
                                SpanStyle(
                                    fontFamily = NidavellirType.Display,
                                    fontSize = MaterialTheme.typography.labelMedium.fontSize,
                                ),
                            ) { append(session.character.displayName) }
                            profile?.let { append(" · $it") }
                        },
                        color = Muted,
                        style = MaterialTheme.typography.labelSmall,
                        fontFamily = NidavellirType.Data,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    // Where: the host never yields to the path; the path yields from
                    // its head, so the segment that names the work stays legible.
                    if (host != null || shownDirectory != null) {
                        Row(Modifier.fillMaxWidth()) {
                            host?.let { CardFact(if (shownDirectory != null) "$it · " else it, Muted) }
                            shownDirectory?.let {
                                CardFact(it, Muted, Modifier.weight(1f, fill = false), TextOverflow.StartEllipsis)
                            }
                        }
                    }
                }
            }
            SessionActionsButton(
                spokenName = "session actions for ${session.tmuxName} on $terminalMachine",
                actions = actions,
                modifier = Modifier.align(Alignment.TopEnd),
                markCenterY = CardPadding + nameLineHeight / 2,
            )
        }
    }
}

private val CardPadding = 10.dp
private const val FACET = "facet"

@Composable
private fun CardFact(
    text: String,
    color: Color,
    modifier: Modifier = Modifier,
    overflow: TextOverflow = TextOverflow.Ellipsis,
) {
    Text(
        text = text,
        color = color,
        style = MaterialTheme.typography.labelSmall,
        fontFamily = NidavellirType.Data,
        maxLines = 1,
        overflow = overflow,
        modifier = modifier,
    )
}

internal fun sessionProfileLabel(session: TmuxSession, profiles: List<ProfileChoice>): String? {
    val agent = session.agent ?: return null
    return agent.profile?.let { runtimeProfile ->
        profiles.single {
            it.key == runtimeProfile && it.provider == agent.provider
        }.label
    } ?: when (agent.provider) {
        AgentProvider.Codex -> "Codex · profile unknown"
        AgentProvider.Claude -> "Claude · profile unknown"
    }
}

// The status line's leading facet: colour-only decoration beside the literal
// label, which owns meaning and speech. Fresh working status alone turns its
// notch (design-language.md §12).
@Composable
private fun ActivityFacet(
    working: Boolean,
    tone: Color,
    animate: Boolean,
) {
    val modifier = Modifier
        .fillMaxSize()
        .clip(NidavellirShapes.Chip)
    if (!working || !animate) {
        Box(modifier.background(tone))
        return
    }

    val transition = rememberInfiniteTransition(label = "active session activity")
    val rotation by transition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1_200, easing = LinearEasing),
        ),
        label = "active session facet rotation",
    )
    Canvas(modifier) {
        drawRect(tone)
        val inset = 3.dp.toPx()
        val angle = Path().apply {
            moveTo(inset, size.height - inset)
            lineTo(inset, inset)
            lineTo(size.width - inset, inset)
        }
        rotate(rotation) {
            drawPath(
                path = angle,
                color = DeepSurface,
                style = Stroke(width = 1.dp.toPx(), cap = StrokeCap.Butt, join = StrokeJoin.Miter),
            )
        }
    }
}

internal fun abbreviatedDirectory(directory: String): String {
    val segments = directory.split('/').filter(String::isNotEmpty)
    return if (segments.size <= 2) directory else "…/${segments.takeLast(2).joinToString("/")}"
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
