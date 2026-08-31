package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.onClick
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp

@Composable
internal fun MachinePressureRail(
    machine: PairedMachine,
    state: PressureState,
    onOpenDetails: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val content = pressureRailContent(machine.label.text, state)
    val response = state.response()
    val handle = machine.handle.encoded
    Surface(
        color = RaisedSurface,
        shape = NidavellirShapes.Card,
        modifier = modifier
            .minimumInteractiveComponentSize()
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Card),
                role = Role.Button,
                onClickLabel = content.actionLabel,
                onClick = onOpenDetails,
            )
            .clearAndSetSemantics {
                this[SemanticsProperties.TestTag] = "machine-strip-$handle"
                this[SemanticsProperties.Role] = Role.Button
                contentDescription = content.accessibilitySummary
                onClick(label = content.actionLabel) {
                    onOpenDetails()
                    true
                }
            },
    ) {
        Column(
            Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            val headerAccent = pressureRailAccentColor(content.header.accent)
            Text(
                text = buildAnnotatedString {
                    withStyle(SpanStyle(color = Bone, fontWeight = FontWeight.Bold)) {
                        append(content.header.machineLabel)
                    }
                    append(' ')
                    withStyle(SpanStyle(color = headerAccent, fontWeight = FontWeight.Medium)) {
                        append(content.header.statusText)
                    }
                },
                color = Bone,
                style = MaterialTheme.typography.labelLarge,
                fontFamily = NidavellirType.Data,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.testTag("machine-strip-label-$handle"),
            )
            if (response != null) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .horizontalScroll(rememberScrollState())
                        .testTag("pressure-metrics-$handle"),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    content.metrics.forEach { metric ->
                        val accent = pressureRailAccentColor(metric.accent)
                        Text(
                            text = buildAnnotatedString {
                                withStyle(SpanStyle(color = Muted)) {
                                    append(metric.shortLabel)
                                }
                                append(' ')
                                withStyle(
                                    SpanStyle(
                                        color = if (metric.accent == PressureRailAccent.None) Bone else accent,
                                    ),
                                ) {
                                    append(metric.value)
                                }
                                append(' ')
                                withStyle(
                                    SpanStyle(
                                        color = if (metric.accent == PressureRailAccent.None) Muted else accent,
                                    ),
                                ) {
                                    append(metric.stateMark)
                                }
                            },
                            color = Bone,
                            style = MaterialTheme.typography.labelSmall,
                            fontFamily = NidavellirType.Data,
                            fontWeight = FontWeight.Medium,
                            maxLines = 1,
                        )
                    }
                }
                Box(Modifier.testTag("pressure-history-band-$handle")) {
                    PressureHistoryBand(response.history)
                }
            }
        }
    }
}

@Composable
internal fun PressureHistoryBand(history: List<PressureHistorySample>) {
    Canvas(
        modifier = Modifier
            .fillMaxWidth()
            .height(16.dp)
            .padding(top = 5.dp)
            .clearAndSetSemantics {},
    ) {
        if (history.isEmpty()) return@Canvas
        val barWidth = size.width / history.size
        history.forEachIndexed { index, sample ->
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
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun MachinePressureDetailsSheet(
    machine: PairedMachine,
    state: PressureState,
    onDismiss: () -> Unit,
) {
    val content = pressureDetailsContent(machine.label.text, state)
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp)
                .padding(bottom = 28.dp)
                .testTag("pressure-details-sheet-${machine.handle.encoded}"),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                text = content.title,
                style = MaterialTheme.typography.headlineSmall,
                fontFamily = NidavellirType.Display,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.semantics { heading() },
            )
            Text(text = content.summary, color = Muted)
            content.rows.forEach { row ->
                val color = pressureColor(row.colorRole)
                Surface(
                    color = RaisedSurface,
                    shape = NidavellirShapes.Chip,
                    border = BorderStroke(1.dp, color),
                ) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            Text(
                                row.fullLabel,
                                color = Muted,
                                style = MaterialTheme.typography.labelMedium,
                            )
                            Text(row.value, color = Bone, fontFamily = NidavellirType.Data)
                        }
                        Text(
                            row.stateWord,
                            color = color,
                            style = MaterialTheme.typography.labelMedium,
                            fontFamily = NidavellirType.Data,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }
            TextButton(
                onClick = onDismiss,
                modifier = Modifier.align(Alignment.End).semantics {
                    contentDescription = content.dismissLabel
                },
            ) {
                Text("Dismiss")
            }
        }
    }
}

private fun pressureRailAccentColor(accent: PressureRailAccent): Color = when (accent) {
    PressureRailAccent.None -> Bone
    PressureRailAccent.Gold -> Gold
    PressureRailAccent.Ember -> Ember
    PressureRailAccent.Muted -> Muted
}

private fun pressureColor(role: PressureColorRole): Color = when (role) {
    PressureColorRole.Frost -> Frost
    PressureColorRole.Moss -> Moss
    PressureColorRole.Gold -> Gold
    PressureColorRole.Ember -> Ember
    PressureColorRole.Muted -> Muted
}

private fun pressureColor(level: PressureLevel): Color = when (level) {
    PressureLevel.Normal -> Moss
    PressureLevel.Warm -> Gold
    PressureLevel.Hot -> Ember
    PressureLevel.Unknown -> Muted
}
