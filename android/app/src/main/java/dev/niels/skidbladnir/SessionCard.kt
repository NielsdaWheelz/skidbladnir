package dev.niels.skidbladnir

import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.text.InlineTextContent
import androidx.compose.foundation.text.appendInlineContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
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
import androidx.compose.ui.text.Placeholder
import androidx.compose.ui.text.PlaceholderVerticalAlign
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp

/** The compact roster card: a plain clickable body (M3 Card(onClick) hardcodes its ripple) whose only sibling is the [actions] overflow. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun SessionCard(
    visibleSession: VisibleSession,
    machine: MachineState,
    needsInputView: Boolean,
    motionEnabled: Boolean,
    actions: List<MenuAction>,
    onOpen: () -> Unit,
) {
    val session = visibleSession.target.session
    val context = visibleSession.context
    val terminalMachine = visibleSession.machine.label.text
    val notification = machine.notifications[NotificationKey(visibleSession.target)] ?: NotificationPresentation()
    val status = sessionStatusContent(session, machine.canMutate, notification, showQueueReason = needsInputView)
    val tone = sessionStatusColor(status.tone)
    val availability = sessionAvailabilityContent(machine)
    // A plain pane has no runtime to name: its status line already reads `terminal`.
    val profile = when (context) {
        is ExecutionContext.Local -> session.agent?.let { agentLabel(it.provider, it.label) }
        is ExecutionContext.Remote -> context.agent?.let { agentLabel(it.provider, it.label) }
        ExecutionContext.RemoteUnknown -> null
    }
    val directory = when (context) {
        is ExecutionContext.Local -> context.cwd
        is ExecutionContext.Remote -> context.cwd
        ExecutionContext.RemoteUnknown -> null
    }
    val host = when (context) {
        is ExecutionContext.Local -> terminalMachine
        is ExecutionContext.Remote -> "running on ${context.machine.label.text} · terminal on $terminalMachine"
        ExecutionContext.RemoteUnknown -> "remote context unknown · terminal on $terminalMachine"
    }
    val density = LocalDensity.current
    val facetSize = with(density) { 12.dp.toSp() }
    val nameLineHeight = with(density) { MaterialTheme.typography.titleSmall.lineHeight.toDp() }
    Surface(color = DeepSurface, shape = NidavellirShapes.Card) {
        Box(Modifier.drawBehind { drawRect(Gold.copy(alpha = 0.25f), size = size.copy(height = 1.dp.toPx())) }) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(
                        interactionSource = null,
                        indication = AngularIndication(NidavellirShapes.Card),
                        enabled = machine.canMutate,
                        onClick = onOpen,
                    )
                    .padding(horizontal = 8.dp, vertical = CardVerticalPadding),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(end = 48.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
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
                        val primaryStatus = buildAnnotatedString {
                            appendInlineContent(FACET)
                            append(" ")
                            withStyle(SpanStyle(color = tone, fontWeight = FontWeight.Bold)) { append(status.label) }
                        }
                        val statusInlineContent = mapOf(
                            FACET to InlineTextContent(Placeholder(facetSize, facetSize, PlaceholderVerticalAlign.TextCenter)) {
                                ActivityFacet(
                                    working = session.agent != null && session.terminalStatus.activity == TerminalActivity.Working,
                                    tone = tone,
                                    animate = machine.canMutate && motionEnabled,
                                )
                            },
                        )
                        FlowRow(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            verticalArrangement = Arrangement.spacedBy(2.dp),
                        ) {
                            if (needsInputView && status.detailExplainsQueue) {
                                // Queue reasons wrap completely; ordinary detail remains a single line.
                                Text(
                                    text = buildAnnotatedString {
                                        append(primaryStatus)
                                        status.detail?.let { withStyle(SpanStyle(color = Muted)) { append("\n$it") } }
                                    },
                                    inlineContent = statusInlineContent,
                                    style = MaterialTheme.typography.labelMedium,
                                    fontFamily = NidavellirType.Data,
                                )
                            } else {
                                Row {
                                    Text(
                                        text = primaryStatus,
                                        inlineContent = statusInlineContent,
                                        style = MaterialTheme.typography.labelMedium,
                                        fontFamily = NidavellirType.Data,
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
                            }
                            Text(
                                text = session.character.displayName,
                                color = Muted,
                                style = MaterialTheme.typography.labelMedium,
                                fontFamily = NidavellirType.Display,
                            )
                        }
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
                FlowRow(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalArrangement = Arrangement.spacedBy(2.dp),
                ) {
                    CardFact(profile?.let { "$it · $host" } ?: host, Muted)
                    if (directory != null) {
                        CardFact(
                            text = abbreviatedDirectory(directory),
                            color = Muted,
                            maxLines = 1,
                            overflow = TextOverflow.StartEllipsis,
                        )
                    } else if (context is ExecutionContext.Remote) {
                        CardFact("directory unavailable", Muted)
                    }
                }
                if (needsInputView) {
                    CardFact(
                        text = session.group?.let { "group: ${it.text}" } ?: "unassigned",
                        color = Muted,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            ActionMenuButton(
                description = "session actions for ${session.tmuxName} on $terminalMachine",
                actions = actions,
                modifier = Modifier.align(Alignment.TopEnd),
                markCenterY = CardVerticalPadding + nameLineHeight / 2,
            )
        }
    }
}

private val CardVerticalPadding = 6.dp
private const val FACET = "facet"

@Composable
private fun CardFact(
    text: String,
    color: Color,
    maxLines: Int = Int.MAX_VALUE,
    overflow: TextOverflow = TextOverflow.Clip,
) {
    Text(
        text = text,
        color = color,
        style = MaterialTheme.typography.labelSmall,
        fontFamily = NidavellirType.Data,
        maxLines = maxLines,
        overflow = overflow,
    )
}

/** The status line's colour-only leading facet; only fresh working status turns its notch. */
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

private fun abbreviatedDirectory(directory: String): String {
    val segments = directory.split('/').filter(String::isNotEmpty)
    return if (segments.size <= 2) directory else "…/${segments.takeLast(2).joinToString("/")}"
}
