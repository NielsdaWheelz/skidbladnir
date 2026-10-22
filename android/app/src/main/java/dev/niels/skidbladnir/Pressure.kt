package dev.niels.skidbladnir

import java.time.Instant
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.double

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
    data class Number(val value: Double) : PressureValue
    data class Memory(val value: SystemMemoryPressure) : PressureValue
}

internal data class PressureSignal(val value: PressureValue, val state: PressureSignalState)

internal data class PressureSample(
    val level: PressureLevel,
    val phase: PressurePhase,
    val reasons: List<PressureReason>,
    val signals: Map<PressureMetric, PressureSignal>,
    val missing: List<PressureMetric>,
)

internal data class PressureResponse(
    val unsupported: List<PressureMetric>,
    val current: PressureSample,
    val history: List<PressureLevel>,
)

@Serializable
private data class WirePressureSignal(val value: JsonPrimitive, val state: PressureSignalState)

@Serializable
private data class WirePressureSample(
    @Serializable(with = IsoInstantSerializer::class) val sampledAt: Instant,
    val level: PressureLevel,
    val phase: PressurePhase,
    val reasons: List<PressureReason>,
    val signals: Map<PressureMetric, WirePressureSignal>,
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

// The host owns pressure policy; the phone decodes types and enums only.
internal fun decodePressureResponse(encoded: String): PressureResponse = decodeProtocol {
    val wire = productJson.decodeFromJsonElement<WirePressureResponse>(strictJsonObject(encoded))
    val current = wire.current
    PressureResponse(
        unsupported = wire.unsupported,
        current = PressureSample(
            level = current.level,
            phase = current.phase,
            reasons = current.reasons,
            signals = current.signals.mapValues { (metric, signal) ->
                val value = when (metric) {
                    PressureMetric.MemoryPressure ->
                        PressureValue.Memory(SystemMemoryPressure.valueOf(signal.value.content))
                    else -> PressureValue.Number(signal.value.double)
                }
                PressureSignal(value, signal.state)
            },
            missing = current.missing,
        ),
        history = wire.history.map { it.level },
    )
}
