package dev.niels.skidbladnir

import java.net.URI
import java.time.DateTimeException
import java.time.Duration
import java.time.Instant
import java.util.Locale
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

internal val productJson = Json { explicitNulls = false; ignoreUnknownKeys = false }

internal class ProtocolDecodeException(cause: Throwable) :
    RuntimeException("Protocol payload could not be decoded.", cause)

internal inline fun <Value> decodeProtocol(block: () -> Value): Value = try {
    block()
} catch (failure: ProtocolDecodeException) {
    throw failure
} catch (failure: SerializationException) {
    throw ProtocolDecodeException(failure)
} catch (failure: DateTimeException) {
    throw ProtocolDecodeException(failure)
} catch (failure: NoSuchElementException) {
    throw ProtocolDecodeException(failure)
} catch (failure: IllegalArgumentException) {
    throw ProtocolDecodeException(failure)
}

internal class MachineHandle private constructor(val encoded: String) {
    companion object {
        private val pattern = Regex("mh-[0-9a-f]{32}")
        fun parse(candidate: String): MachineHandle? = candidate.takeIf(pattern::matches)?.let(::MachineHandle)
    }
    override fun equals(other: Any?): Boolean = other is MachineHandle && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

internal class MachineLabel private constructor(val text: String) {
    companion object {
        fun parse(candidate: String): MachineLabel? {
            if (candidate.isEmpty() || candidate.length > 40 || candidate != candidate.trim()) return null
            if (candidate.any(Char::isISOControl)) return null
            return MachineLabel(candidate)
        }
    }
    override fun equals(other: Any?): Boolean = other is MachineLabel && text == other.text
    override fun hashCode(): Int = text.hashCode()
    override fun toString(): String = text
}

internal class MachineOrigin private constructor(val encoded: String) {
    companion object {
        fun parse(candidate: String): MachineOrigin? {
            val uri = try { URI(candidate) } catch (_: Exception) { return null }
            if (uri.scheme != "https" || uri.host.isNullOrEmpty() || uri.port != 8443) return null
            if (uri.rawUserInfo != null || uri.rawQuery != null || uri.rawFragment != null) return null
            if (uri.rawPath !in setOf("", "/") || candidate.any(Char::isWhitespace)) return null
            val canonicalHost = uri.host.lowercase(Locale.ROOT)
            val authority = if (canonicalHost.contains(':')) "[$canonicalHost]" else canonicalHost
            return MachineOrigin("https://$authority:8443/")
        }
    }
    override fun equals(other: Any?): Boolean = other is MachineOrigin && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

internal data class PairedMachine(val handle: MachineHandle, val label: MachineLabel, val origin: MachineOrigin)
internal data class AgentTarget(val machineHandle: MachineHandle, val session: AgentSession)
internal enum class MachinePlatform { Linux, Darwin }
internal data class MachineSummary(val handle: MachineHandle, val platform: MachinePlatform)

@Serializable private data class WireMachineSummary(val handle: String, val platform: WireMachinePlatform)
@Serializable private enum class WireMachinePlatform { Linux, Darwin }
@Serializable internal data class ProfileChoice(val key: String, val label: String)
@Serializable internal enum class SessionStatusKind { Working, Running, Idle, Shell, Unknown }
@Serializable internal enum class SessionStatusSignal { Lifecycle, Process, PollFailure }
@Serializable internal data class SessionStatus(val kind: SessionStatusKind, val signal: SessionStatusSignal, val signalAt: String)
@Serializable internal data class CharacterSummary(val key: String, val displayName: String)

@Serializable
internal data class AgentSession(
    val id: String,
    val name: String,
    val identityToken: String,
    val profile: String? = null,
    val objective: String? = null,
    val character: CharacterSummary? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val attention: Boolean,
    val status: SessionStatus,
)

internal data class SessionsResponse(
    val machine: MachineSummary,
    val observedAt: String,
    val profiles: List<ProfileChoice>,
    val sessions: List<AgentSession>,
)

@Serializable
private data class WireSessionsResponse(
    val machine: WireMachineSummary,
    val observedAt: String,
    val profiles: List<ProfileChoice>,
    val sessions: List<AgentSession>,
)

@Serializable internal enum class PressureLevel { Normal, Warm, Hot, Unknown }
@Serializable internal enum class PressureReason { Memory, Disk, Load, CpuPsi, MemoryPsi, IoPsi }
@Serializable internal enum class SystemMemoryPressure { Normal, Warning, Critical }

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

@Serializable
internal data class PressureMetrics(
    val cpuPercent: Double? = null,
    val normalizedLoad: Double? = null,
    val memoryAvailablePercent: Double? = null,
    val swapUsedPercent: Double? = null,
    val diskAvailablePercent: Double? = null,
    val cpuPsiSomeAvg60Percent: Double? = null,
    val memoryPsiFullAvg60Percent: Double? = null,
    val ioPsiFullAvg60Percent: Double? = null,
    val memoryPressure: SystemMemoryPressure? = null,
)

@Serializable
internal data class PressureSample(
    val sampledAt: String,
    val level: PressureLevel,
    val reasons: List<PressureReason>,
    val metrics: PressureMetrics,
    val missing: List<PressureMetric>,
)

@Serializable
internal data class PressureResponse(
    val unsupported: List<PressureMetric>,
    val current: PressureSample,
    val history: List<PressureSample>,
)

internal data class ForgeDraft(
    val machineHandle: MachineHandle,
    val cwd: String,
    val profile: String,
    val optionalName: String,
    val objective: String,
)

internal data class ForgeForm(
    val machineHandle: MachineHandle?,
    val cwd: String,
    val profile: String,
    val optionalName: String,
    val objective: String,
) {
    constructor(draft: ForgeDraft) : this(
        draft.machineHandle,
        draft.cwd,
        draft.profile,
        draft.optionalName,
        draft.objective,
    )

    fun submission(): ForgeDraft? = machineHandle?.let {
        ForgeDraft(it, cwd, profile, optionalName, objective)
    }
}

internal fun changeForgeMachine(form: ForgeForm, machineHandle: MachineHandle): ForgeForm =
    if (form.machineHandle == machineHandle) form else form.copy(machineHandle = machineHandle, cwd = "", profile = "")

internal fun forgeActionLabel(label: MachineLabel): String = "Create on ${label.text}"
internal fun killConfirmationTitle(label: MachineLabel, target: AgentTarget): String =
    "Kill ${target.session.name} on ${label.text}?"

@Serializable private data class CreateSessionRequest(
    val cwd: String,
    val profile: String,
    val optionalName: String? = null,
    val objective: String? = null,
)
@Serializable private data class KillSessionRequest(val name: String, val identityToken: String)

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    element.getValue("sessions").jsonArray.forEach { encodedSession ->
        encodedSession.jsonObject.requireAbsentOrNonNull(setOf("profile", "objective", "character", "cwd", "activeCommand"))
    }
    val wire = productJson.decodeFromJsonElement<WireSessionsResponse>(element)
    val observedAt = Instant.parse(wire.observedAt)
    val handle = requireNotNull(MachineHandle.parse(wire.machine.handle))
    require(wire.profiles.all { it.key.isNotEmpty() && it.label.isNotEmpty() })
    require(wire.profiles.map(ProfileChoice::key).allUnique())
    require(wire.profiles.map(ProfileChoice::label).allUnique())
    require(wire.sessions.map(AgentSession::id).allUnique())
    require(wire.sessions.map(AgentSession::identityToken).allUnique())
    wire.sessions.forEach { acceptSession(it, observedAt) }
    SessionsResponse(
        MachineSummary(handle, MachinePlatform.valueOf(wire.machine.platform.name)),
        wire.observedAt,
        wire.profiles,
        wire.sessions,
    )
}

internal fun decodePressureResponse(encoded: String): PressureResponse = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    val samples = listOf(element.getValue("current")) + element.getValue("history").jsonArray
    samples.forEach { sample ->
        sample.jsonObject.getValue("metrics").jsonObject.requireAbsentOrNonNull(
            setOf(
                "cpuPercent", "normalizedLoad", "memoryAvailablePercent", "swapUsedPercent",
                "diskAvailablePercent", "cpuPsiSomeAvg60Percent", "memoryPsiFullAvg60Percent",
                "ioPsiFullAvg60Percent", "memoryPressure",
            ),
        )
    }
    val response = productJson.decodeFromJsonElement<PressureResponse>(element)
    require(response.unsupported == response.unsupported.distinct().sortedBy(::pressureMetricWireName))
    val linux = listOf(PressureMetric.MemoryPressure)
    val darwin = listOf(
        PressureMetric.MemoryAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent,
    ).sortedBy(::pressureMetricWireName)
    require(response.unsupported == linux || response.unsupported == darwin)
    require(response.history.size in 1..180 && response.history.last() == response.current)
    val times = response.history.map { acceptPressureSample(it, response.unsupported.toSet()) }
    require(times.zipWithNext().all { (earlier, later) -> earlier.isBefore(later) })
    require(!times.first().isBefore(times.last().minus(Duration.ofMinutes(15))))
    response
}

internal fun decodeAgentSession(encoded: String): AgentSession = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    element.requireAbsentOrNonNull(setOf("profile", "objective", "character", "cwd", "activeCommand"))
    productJson.decodeFromJsonElement<AgentSession>(element).also { acceptSession(it, null) }
}

internal fun encodeCreateSessionRequest(draft: ForgeDraft): String = productJson.encodeToString(
    CreateSessionRequest(draft.cwd, draft.profile, draft.optionalName.ifEmpty { null }, draft.objective.ifEmpty { null }),
)
internal fun encodeKillSessionRequest(session: AgentSession): String =
    productJson.encodeToString(KillSessionRequest(session.name, session.identityToken))

internal data class InventorySnapshot(val inventory: SessionsResponse, val receivedAtElapsedMillis: Long)
internal sealed interface InventoryState {
    data object Reading : InventoryState
    data class Fresh(val snapshot: InventorySnapshot) : InventoryState
    data class Stale(val snapshot: InventorySnapshot, val cause: GatewayFailure) : InventoryState
    data class Unreachable(val cause: GatewayFailure) : InventoryState
}
internal sealed interface PressureState {
    data object Reading : PressureState
    data class Fresh(val response: PressureResponse) : PressureState
    data class Stale(val response: PressureResponse, val cause: GatewayFailure) : PressureState
    data class Unavailable(val cause: GatewayFailure) : PressureState
}
internal sealed interface MachineAccess {
    data object Ready : MachineAccess
    data object AuthRequired : MachineAccess
    data object IdentityChanged : MachineAccess
}
internal data class MachineState(
    val machine: PairedMachine,
    val access: MachineAccess,
    val inventory: InventoryState,
    val pressure: PressureState,
    val inventoryRefreshRequired: Boolean = false,
) {
    val canMutate: Boolean get() =
        access == MachineAccess.Ready && inventory is InventoryState.Fresh && !inventoryRefreshRequired
}
internal data class VisibleAgent(val machine: PairedMachine, val target: AgentTarget)

internal fun visibleAgents(machines: List<MachineState>, selectedMachine: MachineHandle?): List<VisibleAgent> = machines
    .asSequence()
    .filter { selectedMachine == null || it.machine.handle == selectedMachine }
    .flatMap { state ->
        val snapshot = when (val inventory = state.inventory) {
            is InventoryState.Fresh -> inventory.snapshot
            is InventoryState.Stale -> inventory.snapshot
            InventoryState.Reading, is InventoryState.Unreachable -> null
        }
        snapshot?.inventory?.sessions.orEmpty().asSequence().map { session ->
            VisibleAgent(state.machine, AgentTarget(state.machine.handle, session))
        }
    }
    .sortedWith(
        compareByDescending<VisibleAgent> { it.target.session.attention }
            .thenBy { statusRank(it.target.session.status.kind) }
            .thenBy { it.machine.label.text.lowercase(Locale.ROOT) }
            .thenBy { it.target.session.name.lowercase(Locale.ROOT) }
            .thenBy { it.target.session.id },
    )
    .toList()

internal fun markInventoryFailure(
    machines: List<MachineState>,
    handle: MachineHandle,
    failure: GatewayFailure,
): List<MachineState> = machines.map { state ->
    if (state.machine.handle != handle) return@map state
    state.copy(inventory = when (val inventory = state.inventory) {
        is InventoryState.Fresh -> InventoryState.Stale(inventory.snapshot, failure)
        is InventoryState.Stale -> InventoryState.Stale(inventory.snapshot, failure)
        InventoryState.Reading, is InventoryState.Unreachable -> InventoryState.Unreachable(failure)
    })
}

private fun statusRank(kind: SessionStatusKind): Int = when (kind) {
    SessionStatusKind.Working -> 0
    SessionStatusKind.Running -> 1
    SessionStatusKind.Idle -> 2
    SessionStatusKind.Shell -> 3
    SessionStatusKind.Unknown -> 4
}

internal enum class ApiErrorCode(val wireName: String) {
    Unauthenticated("Unauthenticated"), InvalidRequest("InvalidRequest"), RequestTooLarge("RequestTooLarge"),
    WorkingDirectoryInvalid("WorkingDirectoryInvalid"), WorkingDirectoryUnavailable("WorkingDirectoryUnavailable"),
    ProfileUnknown("ProfileUnknown"), SessionNameInvalid("SessionNameInvalid"), ObjectiveInvalid("ObjectiveInvalid"),
    SessionNameConflict("SessionNameConflict"), SessionNotFound("SessionNotFound"),
    SessionIdentityMismatch("SessionIdentityMismatch"), SessionGroupedConflict("SessionGroupedConflict"),
    MachineIdentityMismatch("MachineIdentityMismatch"), InternalError("InternalError"),
    ReconnectRequired("ReconnectRequired"),
}

internal fun apiErrorMessage(code: ApiErrorCode): String = when (code) {
    ApiErrorCode.Unauthenticated -> "Authentication required."
    ApiErrorCode.InvalidRequest -> "The request is not valid."
    ApiErrorCode.RequestTooLarge -> "The request is too large."
    ApiErrorCode.WorkingDirectoryInvalid -> "Choose a valid working directory."
    ApiErrorCode.WorkingDirectoryUnavailable -> "That directory does not exist or cannot be opened."
    ApiErrorCode.ProfileUnknown -> "Choose an available profile."
    ApiErrorCode.SessionNameInvalid -> "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number."
    ApiErrorCode.ObjectiveInvalid -> "Use 1–240 characters without terminal controls."
    ApiErrorCode.SessionNameConflict -> "A session with that name already exists."
    ApiErrorCode.SessionNotFound -> "That session no longer exists."
    ApiErrorCode.SessionIdentityMismatch -> "The session changed. Refresh before killing it."
    ApiErrorCode.SessionGroupedConflict -> "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it."
    ApiErrorCode.MachineIdentityMismatch -> "The machine identity changed. Provisioning repair is required."
    ApiErrorCode.InternalError -> "Skíðblaðnir could not complete the request."
    ApiErrorCode.ReconnectRequired -> "Reconnect required."
}

internal fun parseApiErrorCode(value: String): ApiErrorCode =
    ApiErrorCode.entries.singleOrNull { it.wireName == value } ?: throw SerializationException("unknown API error code")

internal data class StatusContent(val kind: String, val evidence: String, val accessibilityLabel: String)
internal fun statusContent(status: SessionStatus, now: Instant): StatusContent {
    val signal = when (status.signal) {
        SessionStatusSignal.Lifecycle -> "lifecycle"
        SessionStatusSignal.Process -> "process"
        SessionStatusSignal.PollFailure -> "poll failure"
    }
    val elapsed = Duration.between(Instant.parse(status.signalAt), now)
    if (elapsed.isNegative) throw SerializationException("status signal is after observation")
    val age = when {
        elapsed.seconds < 60 -> "${elapsed.seconds}s"
        elapsed.toMinutes() < 60 -> "${elapsed.toMinutes()}m"
        elapsed.toHours() < 24 -> "${elapsed.toHours()}h"
        else -> "${elapsed.toDays()}d"
    }
    val spokenAge = when {
        elapsed.seconds < 60 -> "${elapsed.seconds} seconds"
        elapsed.toMinutes() < 60 -> "${elapsed.toMinutes()} minutes"
        elapsed.toHours() < 24 -> "${elapsed.toHours()} hours"
        else -> "${elapsed.toDays()} days"
    }
    return StatusContent(status.kind.name.uppercase(), "$signal · $age", "Observed ${status.kind.name.lowercase()} from $signal $spokenAge ago")
}

internal enum class TerminalGeometry { Owner, Constrained }
internal sealed interface TerminalServerEvent {
    data class Hello(val attachedClients: Int, val geometry: TerminalGeometry) : TerminalServerEvent
    data class Presence(val attachedClients: Int, val geometry: TerminalGeometry) : TerminalServerEvent
    data class Error(val code: ApiErrorCode) : TerminalServerEvent
}
@Serializable private data class TerminalPresencePayload(val kind: String, val attachedClients: Int, val geometry: TerminalGeometry)
@Serializable private data class TerminalErrorPayload(val code: String, val message: String)
@Serializable private data class TerminalErrorEnvelope(val kind: String, val error: TerminalErrorPayload)
@Serializable private data class TerminalResize(val kind: String, val columns: Int, val rows: Int)
@Serializable private data class TerminalDetach(val kind: String)

internal fun decodeTerminalServerEvent(encoded: String): TerminalServerEvent = decodeProtocol {
    val objectValue = productJson.parseToJsonElement(encoded).jsonObject
    when (val kind = objectValue.requiredString("kind")) {
        "Hello", "Presence" -> {
            objectValue.requireExactKeys(setOf("kind", "attachedClients", "geometry"))
            val payload = productJson.decodeFromJsonElement<TerminalPresencePayload>(objectValue)
            if (payload.attachedClients < 1) throw SerializationException("terminal has no attached clients")
            if (kind == "Hello") TerminalServerEvent.Hello(payload.attachedClients, payload.geometry)
            else TerminalServerEvent.Presence(payload.attachedClients, payload.geometry)
        }
        "Error" -> {
            objectValue.requireExactKeys(setOf("kind", "error"))
            objectValue.getValue("error").jsonObject.requireExactKeys(setOf("code", "message"))
            val payload = productJson.decodeFromJsonElement<TerminalErrorEnvelope>(objectValue).error
            val code = parseApiErrorCode(payload.code)
            if (code !in setOf(ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge, ApiErrorCode.ReconnectRequired, ApiErrorCode.InternalError)) {
                throw SerializationException("error code is outside the terminal protocol")
            }
            if (payload.message != apiErrorMessage(code)) throw SerializationException("incorrect API error message")
            TerminalServerEvent.Error(code)
        }
        else -> throw SerializationException("unknown terminal event kind: $kind")
    }
}

internal fun encodeTerminalResize(columns: Int, rows: Int): String {
    if (columns !in 20..240 || rows !in 5..120) throw IllegalArgumentException("terminal geometry out of bounds")
    return productJson.encodeToString(TerminalResize("Resize", columns, rows))
}
internal fun encodeTerminalDetach(): String = productJson.encodeToString(TerminalDetach("Detach"))

private fun JsonObject.requiredString(key: String): String =
    this[key]?.jsonPrimitive?.content ?: throw SerializationException("missing $key")
private fun JsonObject.requireExactKeys(expected: Set<String>) {
    if (keys != expected) throw SerializationException("unexpected protocol fields")
}
private fun JsonObject.requireAbsentOrNonNull(optionalKeys: Set<String>) {
    if (optionalKeys.any { this[it] is JsonNull }) throw SerializationException("same-system optional field was null")
}
private fun <Value> List<Value>.allUnique(): Boolean = distinct().size == size

private fun acceptSession(session: AgentSession, observedAt: Instant?) {
    require(session.id.isNotEmpty() && session.name.isNotEmpty() && session.identityToken.isNotEmpty())
    require(session.attachedClients >= 0)
    require(session.profile?.isNotEmpty() != false && session.objective?.isNotEmpty() != false)
    require(session.cwd?.isNotEmpty() != false && session.activeCommand?.isNotEmpty() != false)
    require(session.character?.let { it.key.isNotEmpty() && it.displayName.isNotEmpty() } != false)
    val legal = when (session.status.kind) {
        SessionStatusKind.Working, SessionStatusKind.Idle -> session.status.signal == SessionStatusSignal.Lifecycle
        SessionStatusKind.Running, SessionStatusKind.Shell -> session.status.signal == SessionStatusSignal.Process
        SessionStatusKind.Unknown -> session.status.signal == SessionStatusSignal.PollFailure
    }
    require(legal)
    val signalAt = Instant.parse(session.status.signalAt)
    if (observedAt != null) require(!signalAt.isAfter(observedAt))
}

private fun acceptPressureSample(sample: PressureSample, unsupported: Set<PressureMetric>): Instant {
    val sampledAt = Instant.parse(sample.sampledAt)
    require(sample.missing.distinct().size == sample.missing.size)
    require(sample.reasons.distinct().size == sample.reasons.size && sample.missing.none(unsupported::contains))
    val values = mapOf<PressureMetric, Any?>(
        PressureMetric.CpuPercent to sample.metrics.cpuPercent,
        PressureMetric.NormalizedLoad to sample.metrics.normalizedLoad,
        PressureMetric.MemoryAvailablePercent to sample.metrics.memoryAvailablePercent,
        PressureMetric.SwapUsedPercent to sample.metrics.swapUsedPercent,
        PressureMetric.DiskAvailablePercent to sample.metrics.diskAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent to sample.metrics.cpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent to sample.metrics.memoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent to sample.metrics.ioPsiFullAvg60Percent,
        PressureMetric.MemoryPressure to sample.metrics.memoryPressure,
    )
    require(values.filterValues { it == null }.keys == sample.missing.toSet() + unsupported)
    val percentages = listOfNotNull(
        sample.metrics.cpuPercent, sample.metrics.memoryAvailablePercent, sample.metrics.swapUsedPercent,
        sample.metrics.diskAvailablePercent, sample.metrics.cpuPsiSomeAvg60Percent,
        sample.metrics.memoryPsiFullAvg60Percent, sample.metrics.ioPsiFullAvg60Percent,
    )
    require(percentages.all { it in 0.0..100.0 })
    require(sample.metrics.normalizedLoad?.let { it >= 0.0 && it.isFinite() } != false)
    val required = if (PressureMetric.MemoryPressure in unsupported) {
        setOf(
            PressureMetric.NormalizedLoad, PressureMetric.MemoryAvailablePercent, PressureMetric.DiskAvailablePercent,
            PressureMetric.CpuPsiSomeAvg60Percent, PressureMetric.MemoryPsiFullAvg60Percent, PressureMetric.IoPsiFullAvg60Percent,
        )
    } else {
        setOf(PressureMetric.NormalizedLoad, PressureMetric.DiskAvailablePercent, PressureMetric.MemoryPressure)
    }
    require((sample.level == PressureLevel.Unknown) == sample.missing.any(required::contains))
    require(sample.level != PressureLevel.Normal || sample.reasons.isEmpty())
    require(sample.level !in setOf(PressureLevel.Warm, PressureLevel.Hot) || sample.reasons.isNotEmpty())
    require(
        sample.metrics.memoryPressure != SystemMemoryPressure.Warning ||
            sample.level in setOf(PressureLevel.Warm, PressureLevel.Hot),
    )
    require(sample.metrics.memoryPressure != SystemMemoryPressure.Critical || sample.level == PressureLevel.Hot)
    return sampledAt
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
