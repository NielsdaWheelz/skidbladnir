package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import java.util.Locale

// Presentation is total over every decoded snapshot: a host inconsistency renders
// literally (RECOVERING FROM NORMAL, bare UNKNOWN, NO DATA) instead of failing.

@Composable
internal fun MachinePressureRail(
    machine: PairedMachine,
    state: PressureState,
    onOpenDetails: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val (status, statusColor) = pressureHeader(state)
    val response = state.response()
    Surface(
        color = RaisedSurface,
        shape = NidavellirShapes.Card,
        modifier = modifier
            .minimumInteractiveComponentSize()
            .clickable(
                interactionSource = null,
                indication = AngularIndication(NidavellirShapes.Card),
                onClick = onOpenDetails,
            ),
    ) {
        Column(
            Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Text(
                text = buildAnnotatedString {
                    withStyle(SpanStyle(color = Bone, fontWeight = FontWeight.Bold)) {
                        append(machine.label.text)
                    }
                    append(' ')
                    withStyle(SpanStyle(color = statusColor, fontWeight = FontWeight.Medium)) {
                        append(status)
                    }
                },
                color = Bone,
                style = MaterialTheme.typography.labelLarge,
                fontFamily = NidavellirType.Data,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            if (response != null) {
                val signals = response.current.signals
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    listOf(
                        PressureMetric.CpuPercent to signals[PressureMetric.CpuPercent],
                        PressureMetric.MemoryAvailablePercent to
                            (signals[PressureMetric.MemoryAvailablePercent] ?: signals[PressureMetric.MemoryPressure]),
                        PressureMetric.SwapUsedPercent to signals[PressureMetric.SwapUsedPercent],
                        PressureMetric.NormalizedLoad to signals[PressureMetric.NormalizedLoad],
                        PressureMetric.DiskAvailablePercent to signals[PressureMetric.DiskAvailablePercent],
                    ).forEach { (metric, signal) ->
                        val (mark, valueColor, markColor) = when (signal?.state) {
                            null -> Triple("?", Muted, Muted)
                            PressureSignalState.Informational -> Triple("i", Bone, Muted)
                            PressureSignalState.Normal -> Triple("N", Bone, Muted)
                            PressureSignalState.Warm -> Triple("W", Gold, Gold)
                            PressureSignalState.Hot -> Triple("H", Ember, Ember)
                        }
                        Text(
                            text = buildAnnotatedString {
                                withStyle(SpanStyle(color = Muted)) {
                                    append(shortLabel(metric))
                                }
                                append(' ')
                                withStyle(SpanStyle(color = valueColor)) {
                                    append(signal?.let { railValue(metric, it.value) } ?: "NO DATA")
                                }
                                append(' ')
                                withStyle(SpanStyle(color = markColor)) {
                                    append(mark)
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
                PressureHistoryBand(response.history)
            }
        }
    }
}

// Frozen pixel contract: geometry and heights in design-language.md §13 (pressure rail), colours in §5.
@Composable
private fun PressureHistoryBand(history: List<PressureLevel>) {
    Canvas(
        modifier = Modifier
            .fillMaxWidth()
            .height(16.dp)
            .padding(top = 5.dp),
    ) {
        if (history.isEmpty()) return@Canvas
        val barWidth = size.width / history.size
        history.forEachIndexed { index, level ->
            val (proportion, color) = when (level) {
                PressureLevel.Normal -> 0.25f to Moss
                PressureLevel.Warm -> 0.58f to Gold
                PressureLevel.Hot -> 1f to Ember
                PressureLevel.Unknown -> 0.42f to Muted
            }
            drawRect(
                color = color,
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

@Composable
internal fun MachinePressureDetails(
    machine: PairedMachine,
    state: PressureState,
    onDismiss: () -> Unit,
) {
    val response = state.response()
    SheetForm {
        SheetTitle("${machine.label.text} pressure")
        Text(text = pressureSummary(state), color = Muted)
        if (response != null) {
            listOf(
                PressureMetric.CpuPercent,
                PressureMetric.MemoryAvailablePercent,
                PressureMetric.MemoryPressure,
                PressureMetric.SwapUsedPercent,
                PressureMetric.NormalizedLoad,
                PressureMetric.DiskAvailablePercent,
                PressureMetric.CpuPsiSomeAvg60Percent,
                PressureMetric.MemoryPsiFullAvg60Percent,
                PressureMetric.IoPsiFullAvg60Percent,
            ).filter { it !in response.unsupported }.forEach { metric ->
                val signal = response.current.signals[metric]
                val (word, color) = when (signal?.state) {
                    null -> "No data" to Muted
                    PressureSignalState.Informational -> "Informational" to Frost
                    PressureSignalState.Normal -> "Normal" to Moss
                    PressureSignalState.Warm -> "Warm" to Gold
                    PressureSignalState.Hot -> "Hot" to Ember
                }
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
                                fullLabel(metric),
                                color = Muted,
                                style = MaterialTheme.typography.labelMedium,
                            )
                            Text(
                                signal?.let { detailValue(metric, it.value) } ?: "NO DATA",
                                color = Bone,
                                fontFamily = NidavellirType.Data,
                            )
                        }
                        Text(
                            word,
                            color = color,
                            style = MaterialTheme.typography.labelMedium,
                            fontFamily = NidavellirType.Data,
                            fontWeight = FontWeight.Bold,
                        )
                    }
                }
            }
        }
        TextButton(
            onClick = onDismiss,
            modifier = Modifier.align(Alignment.End),
        ) {
            Text("Dismiss")
        }
    }
}

private fun pressureHeader(state: PressureState): Pair<String, Color> = when (state) {
    PressureState.Reading -> "READING" to Gold
    is PressureState.Unavailable -> "PRESSURE UNAVAILABLE" to Muted
    is PressureState.Stale ->
        "PRESSURE STALE · LAST ${state.response.current.level.name.uppercase(Locale.ROOT)}" to Muted
    is PressureState.Fresh -> {
        val current = state.response.current
        val level = current.level.name.uppercase(Locale.ROOT)
        val color = when (current.level) {
            PressureLevel.Normal, PressureLevel.Unknown -> Muted
            PressureLevel.Warm -> Gold
            PressureLevel.Hot -> Ember
        }
        when {
            current.level == PressureLevel.Unknown -> {
                // the first missing metric in wire order, not the host's bytewise-sorted list
                val missing = PressureMetric.entries.filter {
                    it in current.missing && it != PressureMetric.CpuPercent && it != PressureMetric.SwapUsedPercent
                }
                val first = missing.firstOrNull()
                if (first == null) "UNKNOWN" else "UNKNOWN · ${shortLabel(first)} NO DATA${countSuffix(missing.size)}"
            }
            current.phase == PressurePhase.Recovering -> "RECOVERING FROM $level${causeSuffix(current.reasons)}"
            else -> "$level${causeSuffix(current.reasons)}"
        } to color
    }
}

private fun causeSuffix(reasons: List<PressureReason>): String =
    reasons.firstOrNull()?.let { " · ${reasonLabel(it).uppercase(Locale.ROOT)}${countSuffix(reasons.size)}" }.orEmpty()

private fun countSuffix(count: Int): String = if (count > 1) " +${count - 1}" else ""

private fun pressureSummary(state: PressureState): String = when (state) {
    PressureState.Reading -> "Reading pressure."
    is PressureState.Unavailable -> "Pressure unavailable."
    is PressureState.Fresh -> with(state.response.current) {
        when (phase) {
            PressurePhase.Steady -> "Fresh pressure. Steady at ${level.name.lowercase(Locale.ROOT)}."
            PressurePhase.Recovering -> "Fresh pressure. Recovering from ${level.name.lowercase(Locale.ROOT)}."
        } + causesSentence(reasons)
    }
    is PressureState.Stale -> with(state.response.current) {
        when (phase) {
            PressurePhase.Steady -> "Stale pressure. Last steady at ${level.name.lowercase(Locale.ROOT)}."
            PressurePhase.Recovering -> "Stale pressure. Last recovering from ${level.name.lowercase(Locale.ROOT)}."
        } + causesSentence(reasons)
    }
}

private fun causesSentence(reasons: List<PressureReason>): String = when (reasons.size) {
    0 -> ""
    1 -> " Cause: ${reasonLabel(reasons[0])}."
    else -> " Causes: ${reasons.joinToString { reasonLabel(it) }}."
}

private fun railValue(metric: PressureMetric, value: PressureValue): String = when (value) {
    is PressureValue.Memory -> value.value.name.uppercase(Locale.ROOT)
    is PressureValue.Number -> if (metric == PressureMetric.NormalizedLoad) {
        String.format(Locale.ROOT, "%.1f", value.value).trimEnd('0').trimEnd('.')
    } else {
        "${String.format(Locale.ROOT, "%.0f", value.value)}%"
    }
}

private fun detailValue(metric: PressureMetric, value: PressureValue): String = when (value) {
    is PressureValue.Memory -> value.value.name.uppercase(Locale.ROOT)
    is PressureValue.Number -> {
        val number = if (value.value == 0.0) {
            "0"
        } else {
            String.format(Locale.ROOT, "%.2f", value.value).trimEnd('0').trimEnd('.')
        }
        if (metric == PressureMetric.NormalizedLoad) number else "$number%"
    }
}

private fun shortLabel(metric: PressureMetric): String = when (metric) {
    PressureMetric.CpuPercent -> "CPU"
    PressureMetric.MemoryAvailablePercent, PressureMetric.MemoryPressure -> "MEM"
    PressureMetric.SwapUsedPercent -> "SWAP"
    PressureMetric.NormalizedLoad -> "LOAD"
    PressureMetric.DiskAvailablePercent -> "DISK"
    PressureMetric.CpuPsiSomeAvg60Percent -> "CPU PSI"
    PressureMetric.MemoryPsiFullAvg60Percent -> "MEM PSI"
    PressureMetric.IoPsiFullAvg60Percent -> "I/O PSI"
}

private fun fullLabel(metric: PressureMetric): String = when (metric) {
    PressureMetric.CpuPercent -> "CPU used"
    PressureMetric.MemoryAvailablePercent -> "RAM available"
    PressureMetric.MemoryPressure -> "system memory pressure"
    PressureMetric.SwapUsedPercent -> "swap used"
    PressureMetric.NormalizedLoad -> "normalized load"
    PressureMetric.DiskAvailablePercent -> "disk available"
    PressureMetric.CpuPsiSomeAvg60Percent -> "CPU PSI some, 60-second average"
    PressureMetric.MemoryPsiFullAvg60Percent -> "memory PSI full, 60-second average"
    PressureMetric.IoPsiFullAvg60Percent -> "I/O PSI full, 60-second average"
}

private fun reasonLabel(reason: PressureReason): String = when (reason) {
    PressureReason.Memory -> "memory"
    PressureReason.Disk -> "disk"
    PressureReason.Load -> "load"
    PressureReason.CpuPsi -> "CPU pressure"
    PressureReason.MemoryPsi -> "memory pressure"
    PressureReason.IoPsi -> "I/O pressure"
}
