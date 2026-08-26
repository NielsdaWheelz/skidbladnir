package dev.niels.skidbladnir

import java.time.Duration
import java.time.DateTimeException
import java.time.Instant
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

internal val productJson = Json {
    explicitNulls = false
    ignoreUnknownKeys = false
}

internal class ProtocolDecodeException(cause: Throwable) :
    RuntimeException("Protocol payload could not be decoded.", cause) // justify-defect: the app and gateway own one closed wire schema.

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

@Serializable
internal data class ProfileChoice(
    val key: String,
    val label: String,
)

@Serializable
internal enum class SessionStatusKind {
    Working,
    Running,
    Idle,
    Shell,
    Unknown,
}

@Serializable
internal enum class SessionStatusSignal {
    Lifecycle,
    Process,
    PollFailure,
}

@Serializable
internal data class SessionStatus(
    val kind: SessionStatusKind,
    val signal: SessionStatusSignal,
    val signalAt: String,
)

@Serializable
internal data class CharacterSummary(
    val key: String,
    val displayName: String,
)

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

@Serializable
internal data class SessionsResponse(
    val observedAt: String,
    val profiles: List<ProfileChoice>,
    val sessions: List<AgentSession>,
)

@Serializable
internal enum class PressureLevel {
    Normal,
    Warm,
    Hot,
    Unknown,
}

@Serializable
internal enum class PressureReason {
    Memory,
    Disk,
    Load,
    CpuPsi,
    MemoryPsi,
    IoPsi,
}

@Serializable
internal enum class PressureMetric {
    @SerialName("cpuPercent")
    CpuPercent,

    @SerialName("normalizedLoad")
    NormalizedLoad,

    @SerialName("memoryAvailablePercent")
    MemoryAvailablePercent,

    @SerialName("swapUsedPercent")
    SwapUsedPercent,

    @SerialName("diskAvailablePercent")
    DiskAvailablePercent,

    @SerialName("cpuPsiSomeAvg60Percent")
    CpuPsiSomeAvg60Percent,

    @SerialName("memoryPsiFullAvg60Percent")
    MemoryPsiFullAvg60Percent,

    @SerialName("ioPsiFullAvg60Percent")
    IoPsiFullAvg60Percent,
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
    val current: PressureSample,
    val history: List<PressureSample>,
)

internal data class ForgeDraft(
    val cwd: String,
    val profile: String,
    val optionalName: String,
    val objective: String,
)

@Serializable
private data class CreateSessionRequest(
    val cwd: String,
    val profile: String,
    val optionalName: String? = null,
    val objective: String? = null,
)

@Serializable
private data class KillSessionRequest(
    val name: String,
    val identityToken: String,
)

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    element.getValue("sessions").jsonArray.forEach { encodedSession ->
        encodedSession.jsonObject.requireAbsentOrNonNull(
            setOf("profile", "objective", "character", "cwd", "activeCommand"),
        )
    }
    val response = productJson.decodeFromJsonElement<SessionsResponse>(element)
    val observedAt = Instant.parse(response.observedAt)
    require(response.profiles.all { it.key.isNotEmpty() && it.label.isNotEmpty() })
    require(response.profiles.map(ProfileChoice::key).allUnique())
    require(response.profiles.map(ProfileChoice::label).allUnique())
    require(response.sessions.map(AgentSession::id).allUnique())
    require(response.sessions.map(AgentSession::identityToken).allUnique())
    response.sessions.forEach { acceptSession(it, observedAt) }
    response
}

internal fun decodePressureResponse(encoded: String): PressureResponse = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    val samples = listOf(element.getValue("current")) + element.getValue("history").jsonArray
    samples.forEach { sample ->
        sample.jsonObject.getValue("metrics").jsonObject.requireAbsentOrNonNull(
            setOf(
                "cpuPercent",
                "normalizedLoad",
                "memoryAvailablePercent",
                "swapUsedPercent",
                "diskAvailablePercent",
                "cpuPsiSomeAvg60Percent",
                "memoryPsiFullAvg60Percent",
                "ioPsiFullAvg60Percent",
            ),
        )
    }
    val response = productJson.decodeFromJsonElement<PressureResponse>(element)
    require(response.history.size in 1..180 && response.history.last() == response.current)
    val sampleTimes = response.history.map(::acceptPressureSample)
    require(sampleTimes.zipWithNext().all { (earlier, later) -> earlier.isBefore(later) })
    val currentTime = sampleTimes.last()
    require(!sampleTimes.first().isBefore(currentTime.minus(Duration.ofMinutes(15))))
    response
}

internal fun decodeAgentSession(encoded: String): AgentSession = decodeProtocol {
    val element = productJson.parseToJsonElement(encoded).jsonObject
    element.requireAbsentOrNonNull(setOf("profile", "objective", "character", "cwd", "activeCommand"))
    productJson.decodeFromJsonElement<AgentSession>(element).also { acceptSession(it, null) }
}

internal fun encodeCreateSessionRequest(draft: ForgeDraft): String =
    productJson.encodeToString(
        CreateSessionRequest(
            cwd = draft.cwd,
            profile = draft.profile,
            optionalName = draft.optionalName.ifEmpty { null },
            objective = draft.objective.ifEmpty { null },
        ),
    )

internal fun encodeKillSessionRequest(session: AgentSession): String =
    productJson.encodeToString(KillSessionRequest(session.name, session.identityToken))

internal enum class ApiErrorCode(val wireName: String) {
    Unauthenticated("Unauthenticated"),
    InvalidRequest("InvalidRequest"),
    RequestTooLarge("RequestTooLarge"),
    WorkingDirectoryInvalid("WorkingDirectoryInvalid"),
    WorkingDirectoryUnavailable("WorkingDirectoryUnavailable"),
    ProfileUnknown("ProfileUnknown"),
    SessionNameInvalid("SessionNameInvalid"),
    ObjectiveInvalid("ObjectiveInvalid"),
    SessionNameConflict("SessionNameConflict"),
    SessionNotFound("SessionNotFound"),
    SessionIdentityMismatch("SessionIdentityMismatch"),
    SessionGroupedConflict("SessionGroupedConflict"),
    InternalError("InternalError"),
    ReconnectRequired("ReconnectRequired"),
}

internal fun apiErrorMessage(code: ApiErrorCode): String = when (code) {
    ApiErrorCode.Unauthenticated -> "Authentication required."
    ApiErrorCode.InvalidRequest -> "The request is not valid."
    ApiErrorCode.RequestTooLarge -> "The request is too large."
    ApiErrorCode.WorkingDirectoryInvalid -> "Choose a valid working directory."
    ApiErrorCode.WorkingDirectoryUnavailable -> "That directory does not exist or cannot be opened."
    ApiErrorCode.ProfileUnknown -> "Choose an available profile."
    ApiErrorCode.SessionNameInvalid ->
        "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number."
    ApiErrorCode.ObjectiveInvalid -> "Use 1–240 characters without terminal controls."
    ApiErrorCode.SessionNameConflict -> "A session with that name already exists."
    ApiErrorCode.SessionNotFound -> "That session no longer exists."
    ApiErrorCode.SessionIdentityMismatch -> "The session changed. Refresh before killing it."
    ApiErrorCode.SessionGroupedConflict ->
        "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it."
    ApiErrorCode.InternalError -> "Skíðblaðnir could not complete the request."
    ApiErrorCode.ReconnectRequired -> "Reconnect required."
}

internal fun parseApiErrorCode(value: String): ApiErrorCode =
    ApiErrorCode.entries.singleOrNull { it.wireName == value }
        ?: throw SerializationException("unknown API error code")

internal data class StatusContent(
    val kind: String,
    val evidence: String,
    val accessibilityLabel: String,
)

internal fun statusContent(status: SessionStatus, now: Instant): StatusContent {
    val kind = status.kind.name.uppercase()
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
    val spokenKind = status.kind.name.lowercase()
    val spokenAge = when {
        elapsed.seconds < 60 -> "${elapsed.seconds} seconds"
        elapsed.toMinutes() < 60 -> "${elapsed.toMinutes()} minutes"
        elapsed.toHours() < 24 -> "${elapsed.toHours()} hours"
        else -> "${elapsed.toDays()} days"
    }
    return StatusContent(
        kind = kind,
        evidence = "$signal · $age",
        accessibilityLabel = "Observed $spokenKind from $signal $spokenAge ago",
    )
}

internal enum class TerminalGeometry {
    Owner,
    Constrained,
}

internal sealed interface TerminalServerEvent {
    data class Hello(val attachedClients: Int, val geometry: TerminalGeometry) : TerminalServerEvent
    data class Presence(val attachedClients: Int, val geometry: TerminalGeometry) : TerminalServerEvent
    data class Error(val code: ApiErrorCode) : TerminalServerEvent
}

@Serializable
private data class TerminalPresencePayload(
    val kind: String,
    val attachedClients: Int,
    val geometry: TerminalGeometry,
)

@Serializable
private data class TerminalErrorPayload(
    val code: String,
    val message: String,
)

@Serializable
private data class TerminalErrorEnvelope(
    val kind: String,
    val error: TerminalErrorPayload,
)

@Serializable
private data class TerminalResize(
    val kind: String,
    val columns: Int,
    val rows: Int,
)

@Serializable
private data class TerminalDetach(val kind: String)

internal fun decodeTerminalServerEvent(encoded: String): TerminalServerEvent = decodeProtocol {
    val objectValue = productJson.parseToJsonElement(encoded).jsonObject
    when (val kind = objectValue.requiredString("kind")) {
        "Hello", "Presence" -> {
            objectValue.requireExactKeys(setOf("kind", "attachedClients", "geometry"))
            val payload = productJson.decodeFromJsonElement<TerminalPresencePayload>(objectValue)
            if (payload.attachedClients < 1) throw SerializationException("terminal has no attached clients")
            if (kind == "Hello") {
                TerminalServerEvent.Hello(payload.attachedClients, payload.geometry)
            } else {
                TerminalServerEvent.Presence(payload.attachedClients, payload.geometry)
            }
        }
        "Error" -> {
            objectValue.requireExactKeys(setOf("kind", "error"))
            objectValue.getValue("error").jsonObject.requireExactKeys(setOf("code", "message"))
            val payload = productJson.decodeFromJsonElement<TerminalErrorEnvelope>(objectValue).error
            val code = parseApiErrorCode(payload.code)
            if (code !in setOf(
                    ApiErrorCode.InvalidRequest,
                    ApiErrorCode.RequestTooLarge,
                    ApiErrorCode.ReconnectRequired,
                    ApiErrorCode.InternalError,
                )
            ) {
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
    return productJson.encodeToString(TerminalResize(kind = "Resize", columns = columns, rows = rows))
}

internal fun encodeTerminalDetach(): String = productJson.encodeToString(TerminalDetach("Detach"))

private fun JsonObject.requiredString(key: String): String =
    this[key]?.jsonPrimitive?.content ?: throw SerializationException("missing $key")

private fun JsonObject.requireExactKeys(expected: Set<String>) {
    if (keys != expected) throw SerializationException("unexpected terminal event fields")
}

private fun JsonObject.requireAbsentOrNonNull(optionalKeys: Set<String>) {
    if (optionalKeys.any { key -> this[key] is JsonNull }) {
        throw SerializationException("same-system optional field was null")
    }
}

private fun <Value> List<Value>.allUnique(): Boolean = distinct().size == size

private fun acceptSession(session: AgentSession, observedAt: Instant?) {
    require(session.id.isNotEmpty() && session.name.isNotEmpty() && session.identityToken.isNotEmpty())
    require(session.attachedClients >= 0)
    require(session.profile?.isNotEmpty() != false)
    require(session.objective?.isNotEmpty() != false)
    require(session.cwd?.isNotEmpty() != false)
    require(session.activeCommand?.isNotEmpty() != false)
    require(session.character?.let { it.key.isNotEmpty() && it.displayName.isNotEmpty() } != false)
    val legalSignal = when (session.status.kind) {
        SessionStatusKind.Working, SessionStatusKind.Idle ->
            session.status.signal == SessionStatusSignal.Lifecycle
        SessionStatusKind.Running, SessionStatusKind.Shell ->
            session.status.signal == SessionStatusSignal.Process
        SessionStatusKind.Unknown -> session.status.signal == SessionStatusSignal.PollFailure
    }
    require(legalSignal)
    val signalAt = Instant.parse(session.status.signalAt)
    if (observedAt != null) require(!signalAt.isAfter(observedAt))
}

private fun acceptPressureSample(sample: PressureSample): Instant {
    val sampledAt = Instant.parse(sample.sampledAt)
    require(sample.missing.distinct().size == sample.missing.size)
    require(sample.reasons.distinct().size == sample.reasons.size)
    val missing = sample.missing.toSet()
    val metrics = mapOf(
        PressureMetric.CpuPercent to sample.metrics.cpuPercent,
        PressureMetric.NormalizedLoad to sample.metrics.normalizedLoad,
        PressureMetric.MemoryAvailablePercent to sample.metrics.memoryAvailablePercent,
        PressureMetric.SwapUsedPercent to sample.metrics.swapUsedPercent,
        PressureMetric.DiskAvailablePercent to sample.metrics.diskAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent to sample.metrics.cpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent to sample.metrics.memoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent to sample.metrics.ioPsiFullAvg60Percent,
    )
    require(metrics.all { (metric, value) -> (value == null) == (metric in missing) })
    val percentages = listOfNotNull(
        sample.metrics.cpuPercent,
        sample.metrics.memoryAvailablePercent,
        sample.metrics.swapUsedPercent,
        sample.metrics.diskAvailablePercent,
        sample.metrics.cpuPsiSomeAvg60Percent,
        sample.metrics.memoryPsiFullAvg60Percent,
        sample.metrics.ioPsiFullAvg60Percent,
    )
    require(percentages.all { it in 0.0..100.0 })
    require(sample.metrics.normalizedLoad?.let { it >= 0.0 && it.isFinite() } != false)
    val required = setOf(
        PressureMetric.NormalizedLoad,
        PressureMetric.MemoryAvailablePercent,
        PressureMetric.DiskAvailablePercent,
        PressureMetric.CpuPsiSomeAvg60Percent,
        PressureMetric.MemoryPsiFullAvg60Percent,
        PressureMetric.IoPsiFullAvg60Percent,
    )
    require((sample.level == PressureLevel.Unknown) == missing.any(required::contains))
    require(sample.level != PressureLevel.Normal || sample.reasons.isEmpty())
    require(sample.level !in setOf(PressureLevel.Warm, PressureLevel.Hot) || sample.reasons.isNotEmpty())
    val reasonMetrics = mapOf(
        PressureReason.Memory to PressureMetric.MemoryAvailablePercent,
        PressureReason.Disk to PressureMetric.DiskAvailablePercent,
        PressureReason.Load to PressureMetric.NormalizedLoad,
        PressureReason.CpuPsi to PressureMetric.CpuPsiSomeAvg60Percent,
        PressureReason.MemoryPsi to PressureMetric.MemoryPsiFullAvg60Percent,
        PressureReason.IoPsi to PressureMetric.IoPsiFullAvg60Percent,
    )
    require(sample.reasons.none { reason -> reasonMetrics.getValue(reason) in missing })
    return sampledAt
}
