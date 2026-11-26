package dev.niels.skidbladnir

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
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
    childCount: Int,
    parentUnavailable: Boolean,
    contextOnly: Boolean,
    onChildren: () -> Unit,
    actions: List<MenuAction>,
    onOpen: () -> Unit,
) {
    val session = visibleSession.target.session
    val context = visibleSession.context
    val terminalMachine = visibleSession.machine.label.text
    val status = visibleSession.status
    val availability = sessionAvailabilityContent(machine)
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
    val nameLineHeight = with(LocalDensity.current) { MaterialTheme.typography.titleSmall.lineHeight.toDp() }
    Surface(color = DeepSurface, shape = NidavellirShapes.Card) {
        Box(Modifier.drawBehind { drawRect(Gold.copy(alpha = 0.25f), size = size.copy(height = 1.dp.toPx())) }) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = if (childCount > 0) 48.dp else 0.dp)
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
                        FlowRow(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            verticalArrangement = Arrangement.spacedBy(2.dp),
                        ) {
                            Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                                StatusSign(status.sign, status.workContinues, Modifier.alignByBaseline())
                                when {
                                    needsInputView || session.parent != null ->
                                        Text(
                                            text = buildAnnotatedString {
                                                withStyle(SpanStyle(color = signFace(status.sign).ink, fontWeight = FontWeight.Bold)) {
                                                    append(status.label)
                                                }
                                                status.detail?.let {
                                                    withStyle(SpanStyle(color = Muted)) { append(" · $it") }
                                                }
                                            },
                                            style = MaterialTheme.typography.labelMedium,
                                            fontFamily = NidavellirType.Data,
                                            modifier = Modifier.alignByBaseline(),
                                        )
                                    status.sign == SessionSign.Terminal -> status.detail?.let {
                                        Text(
                                            it,
                                            color = Muted,
                                            style = MaterialTheme.typography.labelMedium,
                                            fontFamily = NidavellirType.Data,
                                            maxLines = 1,
                                            overflow = TextOverflow.Ellipsis,
                                            modifier = Modifier.alignByBaseline(),
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
                if (parentUnavailable) CardFact("parent unavailable", Muted)
                if (contextOnly) CardFact("parent of a session in this view", Muted)
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
            if (childCount > 0) {
                TextButton(onClick = onChildren, modifier = Modifier.align(Alignment.BottomStart)) {
                    Text(if (childCount == 1) "1 child" else "$childCount children")
                }
            }
        }
    }
}

private val CardVerticalPadding = 6.dp

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


private fun abbreviatedDirectory(directory: String): String {
    val segments = directory.split('/').filter(String::isNotEmpty)
    return if (segments.size <= 2) directory else "…/${segments.takeLast(2).joinToString("/")}"
}
