package dev.niels.skidbladnir

import java.time.Duration
import java.time.Instant
import java.util.Locale

internal data class PressureRailContent(
    val header: PressureRailHeaderContent,
    val metrics: List<PressureRailMetricContent>,
    val historySummary: String,
    val accessibilitySummary: String,
    val actionLabel: String,
)

internal data class PressureRailHeaderContent(
    val machineLabel: String,
    val statusText: String,
    val accent: PressureRailAccent,
)

internal data class PressureRailMetricContent(
    val metric: PressureMetric,
    val shortLabel: String,
    val value: String,
    val stateMark: String,
    val stateWord: String,
    val accent: PressureRailAccent,
)

internal data class PressureDetailsContent(
    val title: String,
    val summary: String,
    val rows: List<PressureDetailRow>,
    val dismissLabel: String,
)

internal data class PressureDetailRow(
    val metric: PressureMetric,
    val fullLabel: String,
    val value: String,
    val stateWord: String,
    val colorRole: PressureColorRole,
)

internal enum class PressureRailAccent { None, Gold, Ember, Muted }

internal enum class PressureColorRole { Frost, Moss, Gold, Ember, Muted }

internal fun pressureRailContent(machineLabel: String, state: PressureState): PressureRailContent {
    val response = state.response()
    val metrics = response?.current?.signals?.let(::railMetrics).orEmpty()
    val historySummary = response?.history?.let(::historySummary) ?: "No pressure samples yet."
    val header = pressureHeader(machineLabel, state)
    val metricSummary = metrics.joinToString("; ") {
        "${it.shortLabel} ${it.value} ${it.stateMark}, ${it.stateWord.lowercase(Locale.ROOT)}"
    }
    val visibleMetrics = metrics.map(PressureRailMetricContent::metric).toSet()
    val missingDetailSummary = response?.current?.signals
        ?.filterIsInstance<PressureSignal.Missing>()
        ?.filterNot { it.metric in visibleMetrics }
        ?.joinToString("; ") {
            "${metricCopy(it.metric).shortLabel} NO DATA ?, no data"
        }
        .orEmpty()
    return PressureRailContent(
        header = header,
        metrics = metrics,
        historySummary = historySummary,
        accessibilitySummary = listOf(
            machineLabel,
            pressureDetailsSummary(state).removeSuffix("."),
            metricSummary,
            missingDetailSummary,
            historySummary,
        )
            .filter(String::isNotEmpty)
            .joinToString(". "),
        actionLabel = "Show $machineLabel pressure details",
    )
}

internal fun pressureDetailsContent(machineLabel: String, state: PressureState): PressureDetailsContent {
    val response = state.response()
    return PressureDetailsContent(
        title = "$machineLabel pressure",
        summary = pressureDetailsSummary(state),
        rows = response?.current?.signals?.let(::detailRows).orEmpty(),
        dismissLabel = "Dismiss $machineLabel pressure details",
    )
}

internal fun PressureState.response(): PressureResponse? = when (this) {
    PressureState.Reading, is PressureState.Unavailable -> null
    is PressureState.Fresh -> response
    is PressureState.Stale -> response
}

private fun pressureHeader(machineLabel: String, state: PressureState): PressureRailHeaderContent = when (state) {
    PressureState.Reading -> PressureRailHeaderContent(machineLabel, "READING", PressureRailAccent.Gold)
    is PressureState.Unavailable ->
        PressureRailHeaderContent(machineLabel, "PRESSURE UNAVAILABLE", PressureRailAccent.Muted)
    is PressureState.Stale -> PressureRailHeaderContent(
        machineLabel,
        "PRESSURE STALE · LAST ${state.response.current.level.name.uppercase(Locale.ROOT)}",
        PressureRailAccent.Muted,
    )
    is PressureState.Fresh -> freshPressureHeader(machineLabel, state.response.current)
}

private fun freshPressureHeader(
    machineLabel: String,
    current: PressureSample,
): PressureRailHeaderContent = when {
    current.level == PressureLevel.Unknown -> {
        val missing = current.signals.filterIsInstance<PressureSignal.Missing>().filter {
            it.metric != PressureMetric.CpuPercent && it.metric != PressureMetric.SwapUsedPercent
        }
        PressureRailHeaderContent(
            machineLabel,
            "UNKNOWN · ${metricCopy(missing.first().metric).shortLabel} " +
                "NO DATA${countSuffix(missing.size)}",
            PressureRailAccent.Muted,
        )
    }
    current.phase == PressurePhase.Recovering -> when (current.level) {
        PressureLevel.Warm, PressureLevel.Hot -> PressureRailHeaderContent(
            machineLabel,
            "RECOVERING FROM ${current.level.name.uppercase(Locale.ROOT)}${causeSuffix(current.reasons)}",
            pressureRailAccent(current.level),
        )
        PressureLevel.Normal, PressureLevel.Unknown -> error("recovering pressure must retain Warm or Hot")
    }
    else -> PressureRailHeaderContent(
        machineLabel,
        "${current.level.name.uppercase(Locale.ROOT)}${causeSuffix(current.reasons)}",
        pressureRailAccent(current.level),
    )
}

private fun pressureRailAccent(level: PressureLevel): PressureRailAccent = when (level) {
    PressureLevel.Normal, PressureLevel.Unknown -> PressureRailAccent.Muted
    PressureLevel.Warm -> PressureRailAccent.Gold
    PressureLevel.Hot -> PressureRailAccent.Ember
}

private fun causeSuffix(reasons: List<PressureReason>): String = if (reasons.isEmpty()) {
    ""
} else {
    " · ${reasonLabel(reasons.first()).uppercase(Locale.ROOT)}${countSuffix(reasons.size)}"
}

private fun countSuffix(count: Int): String = if (count > 1) " +${count - 1}" else ""

private fun pressureDetailsSummary(state: PressureState): String = when (state) {
    PressureState.Reading -> "Reading pressure."
    is PressureState.Unavailable -> "Pressure unavailable."
    is PressureState.Stale -> when (state.response.current.phase) {
        PressurePhase.Steady ->
            "Stale pressure. Last steady at ${state.response.current.level.name.lowercase(Locale.ROOT)}." +
                reasonsSentence(state.response.current.reasons)
        PressurePhase.Recovering ->
            "Stale pressure. Last recovering from ${state.response.current.level.name.lowercase(Locale.ROOT)}." +
                reasonsSentence(state.response.current.reasons)
    }
    is PressureState.Fresh -> when (state.response.current.phase) {
        PressurePhase.Steady ->
            "Fresh pressure. Steady at ${state.response.current.level.name.lowercase(Locale.ROOT)}." +
                reasonsSentence(state.response.current.reasons)
        PressurePhase.Recovering ->
            "Fresh pressure. Recovering from ${state.response.current.level.name.lowercase(Locale.ROOT)}." +
                reasonsSentence(state.response.current.reasons)
    }
}

private fun reasonsSentence(reasons: List<PressureReason>): String = when (reasons.size) {
    0 -> ""
    1 -> " Cause: ${reasonLabel(reasons.single())}."
    else -> " Causes: ${reasons.joinToString { reasonLabel(it) }}."
}

private fun railMetrics(signals: List<PressureSignal>): List<PressureRailMetricContent> {
    val byMetric = signals.associateBy(PressureSignal::metric)
    return listOf(
        byMetric.getValue(PressureMetric.CpuPercent),
        byMetric[PressureMetric.MemoryAvailablePercent]
            ?: byMetric.getValue(PressureMetric.MemoryPressure),
        byMetric.getValue(PressureMetric.SwapUsedPercent),
        byMetric.getValue(PressureMetric.NormalizedLoad),
        byMetric.getValue(PressureMetric.DiskAvailablePercent),
    ).map(::railMetricContent)
}

private fun detailRows(signals: List<PressureSignal>): List<PressureDetailRow> {
    return signals.sortedBy { detailRank(it.metric) }.map(::detailRow)
}

private fun detailRank(metric: PressureMetric): Int = when (metric) {
    PressureMetric.CpuPercent -> 0
    PressureMetric.MemoryAvailablePercent -> 1
    PressureMetric.MemoryPressure -> 2
    PressureMetric.SwapUsedPercent -> 3
    PressureMetric.NormalizedLoad -> 4
    PressureMetric.DiskAvailablePercent -> 5
    PressureMetric.CpuPsiSomeAvg60Percent -> 6
    PressureMetric.MemoryPsiFullAvg60Percent -> 7
    PressureMetric.IoPsiFullAvg60Percent -> 8
}

private fun railMetricContent(signal: PressureSignal): PressureRailMetricContent {
    val metric = metricCopy(signal.metric)
    val presentation = signalPresentation(signal)
    return PressureRailMetricContent(
        metric = signal.metric,
        shortLabel = metric.shortLabel,
        value = when (signal) {
            is PressureSignal.Missing -> "NO DATA"
            is PressureSignal.Measured -> collapsedPressureValue(signal.value)
        },
        stateMark = presentation.stateMark,
        stateWord = presentation.stateWord,
        accent = presentation.railAccent,
    )
}

private fun detailRow(signal: PressureSignal): PressureDetailRow {
    val metric = metricCopy(signal.metric)
    val presentation = signalPresentation(signal)
    return PressureDetailRow(
        metric = signal.metric,
        fullLabel = metric.fullLabel,
        value = when (signal) {
            is PressureSignal.Missing -> "NO DATA"
            is PressureSignal.Measured -> detailPressureValue(signal.value)
        },
        stateWord = presentation.stateWord,
        colorRole = presentation.detailColorRole,
    )
}

private data class MetricCopy(val shortLabel: String, val fullLabel: String)

private fun metricCopy(metric: PressureMetric): MetricCopy = when (metric) {
    PressureMetric.CpuPercent -> MetricCopy("CPU", "CPU used")
    PressureMetric.MemoryAvailablePercent -> MetricCopy("MEM", "RAM available")
    PressureMetric.MemoryPressure -> MetricCopy("MEM", "system memory pressure")
    PressureMetric.SwapUsedPercent -> MetricCopy("SWAP", "swap used")
    PressureMetric.NormalizedLoad -> MetricCopy("LOAD", "normalized load")
    PressureMetric.DiskAvailablePercent -> MetricCopy("DISK", "disk available")
    PressureMetric.CpuPsiSomeAvg60Percent -> MetricCopy("CPU PSI", "CPU PSI some, 60-second average")
    PressureMetric.MemoryPsiFullAvg60Percent -> MetricCopy("MEM PSI", "memory PSI full, 60-second average")
    PressureMetric.IoPsiFullAvg60Percent -> MetricCopy("I/O PSI", "I/O PSI full, 60-second average")
}

private data class SignalPresentation(
    val stateMark: String,
    val stateWord: String,
    val railAccent: PressureRailAccent,
    val detailColorRole: PressureColorRole,
)

private fun signalPresentation(signal: PressureSignal): SignalPresentation = when (signal) {
    is PressureSignal.Missing -> SignalPresentation(
        "?",
        "No data",
        PressureRailAccent.Muted,
        PressureColorRole.Muted,
    )
    is PressureSignal.Measured -> when (signal.state) {
        PressureSignalState.Informational -> SignalPresentation(
            "i",
            "Informational",
            PressureRailAccent.None,
            PressureColorRole.Frost,
        )
        PressureSignalState.Normal -> SignalPresentation(
            "N",
            "Normal",
            PressureRailAccent.None,
            PressureColorRole.Moss,
        )
        PressureSignalState.Warm -> SignalPresentation(
            "W",
            "Warm",
            PressureRailAccent.Gold,
            PressureColorRole.Gold,
        )
        PressureSignalState.Hot -> SignalPresentation(
            "H",
            "Hot",
            PressureRailAccent.Ember,
            PressureColorRole.Ember,
        )
    }
}

private fun collapsedPressureValue(value: PressureValue): String = when (value) {
    is PressureValue.CpuPercent -> wholePercent(value.value)
    is PressureValue.NormalizedLoad ->
        String.format(Locale.ROOT, "%.1f", value.value).trimEnd('0').trimEnd('.')
    is PressureValue.MemoryAvailablePercent -> wholePercent(value.value)
    is PressureValue.SwapUsedPercent -> wholePercent(value.value)
    is PressureValue.DiskAvailablePercent -> wholePercent(value.value)
    is PressureValue.CpuPsiSomeAvg60Percent -> wholePercent(value.value)
    is PressureValue.MemoryPsiFullAvg60Percent -> wholePercent(value.value)
    is PressureValue.IoPsiFullAvg60Percent -> wholePercent(value.value)
    is PressureValue.MemoryPressure -> value.value.name.uppercase(Locale.ROOT)
}

private fun wholePercent(value: Double): String = "${String.format(Locale.ROOT, "%.0f", value)}%"

private fun detailPressureValue(value: PressureValue): String = when (value) {
    is PressureValue.CpuPercent -> "${formatNumber(value.value)}%"
    is PressureValue.NormalizedLoad -> formatNumber(value.value)
    is PressureValue.MemoryAvailablePercent -> "${formatNumber(value.value)}%"
    is PressureValue.SwapUsedPercent -> "${formatNumber(value.value)}%"
    is PressureValue.DiskAvailablePercent -> "${formatNumber(value.value)}%"
    is PressureValue.CpuPsiSomeAvg60Percent -> "${formatNumber(value.value)}%"
    is PressureValue.MemoryPsiFullAvg60Percent -> "${formatNumber(value.value)}%"
    is PressureValue.IoPsiFullAvg60Percent -> "${formatNumber(value.value)}%"
    is PressureValue.MemoryPressure -> value.value.name.uppercase(Locale.ROOT)
}

private fun formatNumber(value: Double): String = if (value == 0.0) {
    "0"
} else {
    String.format(Locale.ROOT, "%.2f", value).trimEnd('0').trimEnd('.')
}

private data class HistoryRun(val level: PressureLevel, val start: Instant, val end: Instant)

private fun historySummary(history: List<PressureHistorySample>): String {
    val runs = mutableListOf<HistoryRun>()
    history.forEachIndexed { index, sample ->
        val end = history.getOrNull(index + 1)?.sampledAt ?: sample.sampledAt.plusSeconds(5)
        if (runs.lastOrNull()?.level == sample.level) {
            runs[runs.lastIndex] = runs.last().copy(end = end)
        } else {
            runs += HistoryRun(sample.level, sample.sampledAt, end)
        }
    }
    val shown = runs.takeLast(3)
    val earlier = runs.size - shown.size
    val prefix = if (earlier == 0) {
        ""
    } else {
        "$earlier earlier ${if (earlier == 1) "run" else "runs"} over " +
            "${formatDuration(Duration.between(runs.first().start, shown.first().start))}; "
    }
    return "Trend: $prefix${shown.joinToString(", then ") { run ->
        "${run.level.name.lowercase(Locale.ROOT)} ${formatDuration(Duration.between(run.start, run.end))}"
    }}."
}

private fun formatDuration(duration: Duration): String {
    val minutes = duration.seconds / 60
    val seconds = duration.seconds % 60
    return listOfNotNull(
        minutes.takeIf { it > 0 }?.let { "$it ${if (it == 1L) "minute" else "minutes"}" },
        seconds.takeIf { it > 0 }?.let { "$it ${if (it == 1L) "second" else "seconds"}" },
    ).joinToString(" ")
}

private fun reasonLabel(reason: PressureReason): String = when (reason) {
    PressureReason.Memory -> "memory"
    PressureReason.Disk -> "disk"
    PressureReason.Load -> "load"
    PressureReason.CpuPsi -> "CPU pressure"
    PressureReason.MemoryPsi -> "memory pressure"
    PressureReason.IoPsi -> "I/O pressure"
}
