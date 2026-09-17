package dev.niels.skidbladnir

import java.time.Duration
import java.time.Instant
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.decodeFromJsonElement

@Serializable internal enum class PressureLevel { Normal, Warm, Hot, Unknown }
@Serializable internal enum class PressurePhase { Steady, Recovering }
@Serializable internal enum class PressureReason { Memory, Disk, Load, CpuPsi, MemoryPsi, IoPsi }
@Serializable internal enum class SystemMemoryPressure { Normal, Warning, Critical }
@Serializable internal enum class PressureSignalState { Informational, Normal, Warm, Hot }

@Serializable
internal enum class PressureMetric {
    @SerialName("cpuPercent") CpuPercent,
    @SerialName("normalizedLoad") NormalizedLoad,
    @SerialName("memoryAvailablePercent") MemoryAvailablePercent,
    @SerialName("swapUsedPercent") SwapUsedPercent,
    @SerialName("diskAvailablePercent") DiskAvailablePercent,
    @SerialName("cpuPsiSomeAvg60Percent") CpuPsiSomeAvg60Percent,
    @SerialName("memoryPsiFullAvg60Percent") MemoryPsiFullAvg60Percent,
    @SerialName("ioPsiFullAvg60Percent") IoPsiFullAvg60Percent,
    @SerialName("memoryPressure") MemoryPressure,
}

internal sealed interface PressureValue {
    val metric: PressureMetric

    data class CpuPercent(val value: Double) : PressureValue {
        override val metric = PressureMetric.CpuPercent
    }
    data class NormalizedLoad(val value: Double) : PressureValue {
        override val metric = PressureMetric.NormalizedLoad
    }
    data class MemoryAvailablePercent(val value: Double) : PressureValue {
        override val metric = PressureMetric.MemoryAvailablePercent
    }
    data class SwapUsedPercent(val value: Double) : PressureValue {
        override val metric = PressureMetric.SwapUsedPercent
    }
    data class DiskAvailablePercent(val value: Double) : PressureValue {
        override val metric = PressureMetric.DiskAvailablePercent
    }
    data class CpuPsiSomeAvg60Percent(val value: Double) : PressureValue {
        override val metric = PressureMetric.CpuPsiSomeAvg60Percent
    }
    data class MemoryPsiFullAvg60Percent(val value: Double) : PressureValue {
        override val metric = PressureMetric.MemoryPsiFullAvg60Percent
    }
    data class IoPsiFullAvg60Percent(val value: Double) : PressureValue {
        override val metric = PressureMetric.IoPsiFullAvg60Percent
    }
    data class MemoryPressure(val value: SystemMemoryPressure) : PressureValue {
        override val metric = PressureMetric.MemoryPressure
    }
}

internal sealed interface PressureSignal {
    val metric: PressureMetric

    data class Measured(val value: PressureValue, val state: PressureSignalState) : PressureSignal {
        override val metric get() = value.metric
    }
    data class Missing(override val metric: PressureMetric) : PressureSignal
}

internal data class PressureSample(
    val sampledAt: Instant,
    val level: PressureLevel,
    val phase: PressurePhase,
    val reasons: List<PressureReason>,
    val signals: List<PressureSignal>,
)

internal data class PressureHistorySample(
    val sampledAt: Instant,
    val level: PressureLevel,
)

internal data class PressureResponse(
    val unsupported: List<PressureMetric>,
    val current: PressureSample,
    val history: List<PressureHistorySample>,
)

@Serializable
private data class WirePressureSignal<Value>(val value: Value, val state: PressureSignalState)

@Serializable
private data class WirePressureSignals(
    val cpuPercent: WirePressureSignal<Double>? = null,
    val normalizedLoad: WirePressureSignal<Double>? = null,
    val memoryAvailablePercent: WirePressureSignal<Double>? = null,
    val swapUsedPercent: WirePressureSignal<Double>? = null,
    val diskAvailablePercent: WirePressureSignal<Double>? = null,
    val cpuPsiSomeAvg60Percent: WirePressureSignal<Double>? = null,
    val memoryPsiFullAvg60Percent: WirePressureSignal<Double>? = null,
    val ioPsiFullAvg60Percent: WirePressureSignal<Double>? = null,
    val memoryPressure: WirePressureSignal<SystemMemoryPressure>? = null,
)

@Serializable
private data class WirePressureSample(
    @Serializable(with = IsoInstantSerializer::class) val sampledAt: Instant,
    val level: PressureLevel,
    val phase: PressurePhase,
    val reasons: List<PressureReason>,
    val signals: WirePressureSignals,
    val missing: List<PressureMetric>,
)

@Serializable
private data class WirePressureHistorySample(
    @Serializable(with = IsoInstantSerializer::class) val sampledAt: Instant,
    val level: PressureLevel,
)

@Serializable
private data class WirePressureResponse(
    val unsupported: List<PressureMetric>,
    val current: WirePressureSample,
    val history: List<WirePressureHistorySample>,
)

internal sealed interface PressureState {
    data object Reading : PressureState
    data class Fresh(val response: PressureResponse) : PressureState
    data class Stale(val response: PressureResponse, val cause: GatewayFailure) : PressureState
    data class Unavailable(val cause: GatewayFailure) : PressureState
}

internal fun PressureState.response(): PressureResponse? = when (this) {
    PressureState.Reading, is PressureState.Unavailable -> null
    is PressureState.Fresh -> response
    is PressureState.Stale -> response
}

internal fun PressureState.downgraded(cause: GatewayFailure): PressureState = when (this) {
    is PressureState.Fresh -> PressureState.Stale(response, cause)
    is PressureState.Stale -> PressureState.Stale(response, cause)
    PressureState.Reading, is PressureState.Unavailable -> PressureState.Unavailable(cause)
}

internal fun decodePressureResponse(encoded: String): PressureResponse = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.requiredObject("current").requiredObject("signals").requireAbsentOrNonNull(
        setOf(
            "cpuPercent", "normalizedLoad", "memoryAvailablePercent", "swapUsedPercent",
            "diskAvailablePercent", "cpuPsiSomeAvg60Percent", "memoryPsiFullAvg60Percent",
            "ioPsiFullAvg60Percent", "memoryPressure",
        ),
    )
    val wire = productJson.decodeFromJsonElement<WirePressureResponse>(element)
    require(wire.unsupported == wire.unsupported.distinct().sortedBy(::pressureMetricWireName))
    val linux = listOf(PressureMetric.MemoryPressure)
    val darwin = listOf(
        PressureMetric.MemoryAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent,
    ).sortedBy(::pressureMetricWireName)
    require(wire.unsupported == linux || wire.unsupported == darwin)
    val current = acceptPressureSample(wire.current, wire.unsupported.toSet())
    require(wire.history.size in 1..180)
    require(
        wire.history.last() == WirePressureHistorySample(current.sampledAt, current.level),
    )
    val times = wire.history.map(WirePressureHistorySample::sampledAt)
    require(times.zipWithNext().all { (earlier, later) -> earlier.isBefore(later) })
    require(!times.first().isBefore(times.last().minus(Duration.ofMinutes(15))))
    PressureResponse(
        unsupported = wire.unsupported,
        current = current,
        history = wire.history.map { PressureHistorySample(it.sampledAt, it.level) },
    )
}

private fun acceptPressureSample(
    sample: WirePressureSample,
    unsupported: Set<PressureMetric>,
): PressureSample {
    require(sample.missing == sample.missing.distinct().sortedBy(::pressureMetricWireName))
    require(sample.missing.none(unsupported::contains))
    require(sample.reasons.distinct().size == sample.reasons.size)

    val missing = sample.missing.toSet()
    val present = sample.signals.presentMetrics()
    require(present == PressureMetric.entries.toSet() - unsupported - missing)
    val signals = PressureMetric.entries.filterNot(unsupported::contains).map { metric ->
        acceptPressureSignal(metric, sample.signals, missing)
    }
    val policy = if (PressureMetric.MemoryPressure in unsupported) {
        listOf(
            PressureMetric.MemoryAvailablePercent to PressureReason.Memory,
            PressureMetric.DiskAvailablePercent to PressureReason.Disk,
            PressureMetric.NormalizedLoad to PressureReason.Load,
            PressureMetric.CpuPsiSomeAvg60Percent to PressureReason.CpuPsi,
            PressureMetric.MemoryPsiFullAvg60Percent to PressureReason.MemoryPsi,
            PressureMetric.IoPsiFullAvg60Percent to PressureReason.IoPsi,
        )
    } else {
        listOf(
            PressureMetric.DiskAvailablePercent to PressureReason.Disk,
            PressureMetric.NormalizedLoad to PressureReason.Load,
            PressureMetric.MemoryPressure to PressureReason.Memory,
        )
    }
    val required = policy.mapTo(mutableSetOf()) { it.first }
    val requiredSignals = signals.filter { it.metric in required }
    val requiredMissing = requiredSignals.any { signal ->
        when (signal) {
            is PressureSignal.Measured -> false
            is PressureSignal.Missing -> true
        }
    }
    require((sample.level == PressureLevel.Unknown) == requiredMissing)
    require(sample.level != PressureLevel.Normal || sample.reasons.isEmpty())
    require(sample.level !in setOf(PressureLevel.Warm, PressureLevel.Hot) || sample.reasons.isNotEmpty())

    val instantaneous = if (requiredMissing) {
        PressureLevel.Unknown
    } else {
        requiredSignals.asSequence()
            .map { signal ->
                when (signal) {
                    is PressureSignal.Measured -> signal.state.aggregateLevel()
                    is PressureSignal.Missing -> error("required pressure signal cannot be missing here")
                }
            }
            .maxBy(::pressureSeverity)
    }
    val recovering = if (instantaneous == PressureLevel.Unknown) {
        false
    } else {
        require(pressureSeverity(sample.level) >= pressureSeverity(instantaneous))
        sample.level in setOf(PressureLevel.Warm, PressureLevel.Hot) &&
            pressureSeverity(sample.level) > pressureSeverity(instantaneous)
    }
    when (sample.phase) {
        PressurePhase.Steady -> {
            require(!recovering)
            val expectedReasons = policy.mapNotNull { (metric, reason) ->
                when (val signal = signals.single { it.metric == metric }) {
                    is PressureSignal.Missing -> null
                    is PressureSignal.Measured -> when (signal.state) {
                        PressureSignalState.Informational ->
                            error("required pressure signal cannot be informational")
                        PressureSignalState.Normal -> null
                        PressureSignalState.Warm, PressureSignalState.Hot -> reason
                    }
                }
            }
            require(sample.reasons == expectedReasons)
        }
        PressurePhase.Recovering -> {
            require(recovering && sample.reasons.isNotEmpty())
            require(
                sample.reasons == policy.map { it.second }
                    .filter(sample.reasons.toSet()::contains),
            )
        }
    }
    return PressureSample(sample.sampledAt, sample.level, sample.phase, sample.reasons, signals)
}

private fun WirePressureSignals.presentMetrics(): Set<PressureMetric> = buildSet {
    if (cpuPercent != null) add(PressureMetric.CpuPercent)
    if (normalizedLoad != null) add(PressureMetric.NormalizedLoad)
    if (memoryAvailablePercent != null) add(PressureMetric.MemoryAvailablePercent)
    if (swapUsedPercent != null) add(PressureMetric.SwapUsedPercent)
    if (diskAvailablePercent != null) add(PressureMetric.DiskAvailablePercent)
    if (cpuPsiSomeAvg60Percent != null) add(PressureMetric.CpuPsiSomeAvg60Percent)
    if (memoryPsiFullAvg60Percent != null) add(PressureMetric.MemoryPsiFullAvg60Percent)
    if (ioPsiFullAvg60Percent != null) add(PressureMetric.IoPsiFullAvg60Percent)
    if (memoryPressure != null) add(PressureMetric.MemoryPressure)
}

private fun acceptPressureSignal(
    metric: PressureMetric,
    signals: WirePressureSignals,
    missing: Set<PressureMetric>,
): PressureSignal = when (metric) {
    PressureMetric.CpuPercent ->
        acceptNumericPressureSignal(metric, signals.cpuPercent, missing, PressureValue::CpuPercent)
    PressureMetric.NormalizedLoad ->
        acceptNumericPressureSignal(metric, signals.normalizedLoad, missing, PressureValue::NormalizedLoad)
    PressureMetric.MemoryAvailablePercent ->
        acceptNumericPressureSignal(
            metric,
            signals.memoryAvailablePercent,
            missing,
            PressureValue::MemoryAvailablePercent,
        )
    PressureMetric.SwapUsedPercent ->
        acceptNumericPressureSignal(metric, signals.swapUsedPercent, missing, PressureValue::SwapUsedPercent)
    PressureMetric.DiskAvailablePercent ->
        acceptNumericPressureSignal(
            metric,
            signals.diskAvailablePercent,
            missing,
            PressureValue::DiskAvailablePercent,
        )
    PressureMetric.CpuPsiSomeAvg60Percent ->
        acceptNumericPressureSignal(
            metric,
            signals.cpuPsiSomeAvg60Percent,
            missing,
            PressureValue::CpuPsiSomeAvg60Percent,
        )
    PressureMetric.MemoryPsiFullAvg60Percent ->
        acceptNumericPressureSignal(
            metric,
            signals.memoryPsiFullAvg60Percent,
            missing,
            PressureValue::MemoryPsiFullAvg60Percent,
        )
    PressureMetric.IoPsiFullAvg60Percent ->
        acceptNumericPressureSignal(
            metric,
            signals.ioPsiFullAvg60Percent,
            missing,
            PressureValue::IoPsiFullAvg60Percent,
        )
    PressureMetric.MemoryPressure -> acceptMemoryPressureSignal(signals.memoryPressure, missing)
}

private fun acceptNumericPressureSignal(
    metric: PressureMetric,
    signal: WirePressureSignal<Double>?,
    missing: Set<PressureMetric>,
    value: (Double) -> PressureValue,
): PressureSignal {
    if (metric in missing) return PressureSignal.Missing(metric)
    val measured = requireNotNull(signal)
    require(measured.value.isFinite() && measured.value >= 0.0)
    require(metric == PressureMetric.NormalizedLoad || measured.value <= 100.0)
    val informational = when (metric) {
        PressureMetric.CpuPercent, PressureMetric.SwapUsedPercent -> true
        PressureMetric.NormalizedLoad,
        PressureMetric.MemoryAvailablePercent,
        PressureMetric.DiskAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent,
        -> false
        PressureMetric.MemoryPressure -> error("memory pressure is not numeric")
    }
    if (informational) {
        require(measured.state == PressureSignalState.Informational)
    } else {
        require(measured.state != PressureSignalState.Informational)
    }
    return PressureSignal.Measured(value(measured.value), measured.state)
}

private fun acceptMemoryPressureSignal(
    signal: WirePressureSignal<SystemMemoryPressure>?,
    missing: Set<PressureMetric>,
): PressureSignal {
    if (PressureMetric.MemoryPressure in missing) {
        return PressureSignal.Missing(PressureMetric.MemoryPressure)
    }
    val measured = requireNotNull(signal)
    val expectedState = when (measured.value) {
        SystemMemoryPressure.Normal -> PressureSignalState.Normal
        SystemMemoryPressure.Warning -> PressureSignalState.Warm
        SystemMemoryPressure.Critical -> PressureSignalState.Hot
    }
    require(measured.state == expectedState)
    return PressureSignal.Measured(PressureValue.MemoryPressure(measured.value), measured.state)
}

private fun PressureSignalState.aggregateLevel(): PressureLevel = when (this) {
    PressureSignalState.Informational -> error("informational pressure signal cannot classify aggregate pressure")
    PressureSignalState.Normal -> PressureLevel.Normal
    PressureSignalState.Warm -> PressureLevel.Warm
    PressureSignalState.Hot -> PressureLevel.Hot
}

private fun pressureSeverity(level: PressureLevel): Int = when (level) {
    PressureLevel.Normal -> 0
    PressureLevel.Warm -> 1
    PressureLevel.Hot -> 2
    PressureLevel.Unknown -> error("unknown pressure has no severity")
}

private fun pressureMetricWireName(metric: PressureMetric): String = when (metric) {
    PressureMetric.CpuPercent -> "cpuPercent"
    PressureMetric.NormalizedLoad -> "normalizedLoad"
    PressureMetric.MemoryAvailablePercent -> "memoryAvailablePercent"
    PressureMetric.SwapUsedPercent -> "swapUsedPercent"
    PressureMetric.DiskAvailablePercent -> "diskAvailablePercent"
    PressureMetric.CpuPsiSomeAvg60Percent -> "cpuPsiSomeAvg60Percent"
    PressureMetric.MemoryPsiFullAvg60Percent -> "memoryPsiFullAvg60Percent"
    PressureMetric.IoPsiFullAvg60Percent -> "ioPsiFullAvg60Percent"
    PressureMetric.MemoryPressure -> "memoryPressure"
}
