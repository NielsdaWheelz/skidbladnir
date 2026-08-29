package dev.niels.skidbladnir

import java.net.URI
import java.net.URISyntaxException
import java.text.Normalizer
import java.time.DateTimeException
import java.time.Duration
import java.time.Instant
import java.util.Locale
import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.encodeToString
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

internal val productJson = Json { explicitNulls = false; ignoreUnknownKeys = false }

// justify-defect: the app and the gateway own one closed wire schema, so an undecodable protocol
// payload is a same-system contract violation. The reason is content-free by construction — a fixed
// literal or a failure class name, never the offending payload or a cause that embeds it — so the
// architecture §7 credential-free/content-free log guarantee holds even when the platform prints
// this fatal defect.
internal class ProtocolDecodeException(reason: String) :
    RuntimeException("Protocol payload could not be decoded: $reason.")

internal inline fun <Value> decodeProtocol(block: () -> Value): Value = try {
    block()
} catch (failure: ProtocolDecodeException) {
    throw failure
} catch (failure: SerializationException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
} catch (failure: DateTimeException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
} catch (failure: NoSuchElementException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
} catch (failure: IllegalArgumentException) {
    throw ProtocolDecodeException(failure.javaClass.simpleName)
}

internal object IsoInstantSerializer : KSerializer<Instant> {
    override val descriptor =
        PrimitiveSerialDescriptor("dev.niels.skidbladnir.IsoInstant", PrimitiveKind.STRING)
    override fun deserialize(decoder: Decoder): Instant = Instant.parse(decoder.decodeString())
    override fun serialize(encoder: Encoder, value: Instant) = encoder.encodeString(value.toString())
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
            val uri = try {
                URI(candidate)
            } catch (_: URISyntaxException) {
                // justify-ignore-error: an origin that is not even a URI is simply not an origin; the
                // only classification this parser owns is accepted or rejected.
                return null
            }
            if (uri.scheme != "https" || uri.host.isNullOrEmpty() || uri.port != 8443) return null
            if (uri.rawUserInfo != null || uri.rawQuery != null || uri.rawFragment != null) return null
            if (uri.rawPath !in setOf("", "/") || candidate.any(Char::isWhitespace)) return null
            // URI.getHost() already returns the RFC 2732 bracketed form for an IPv6 literal, so the
            // canonical authority is the lowercased host verbatim. Re-bracketing it would produce a
            // value that neither this parser nor OkHttp can read back.
            return MachineOrigin("https://${uri.host.lowercase(Locale.ROOT)}:8443/")
        }
    }
    override fun equals(other: Any?): Boolean = other is MachineOrigin && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

internal class ProfileKey private constructor(val encoded: String) {
    companion object {
        private val pattern = Regex("[a-z][a-z0-9_-]{0,31}")
        fun parse(candidate: String): ProfileKey? =
            candidate.takeIf(pattern::matches)?.let(::ProfileKey)
    }
    override fun equals(other: Any?): Boolean = other is ProfileKey && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

internal data class PairedMachine(val handle: MachineHandle, val label: MachineLabel, val origin: MachineOrigin)
internal data class SessionTarget(val machineHandle: MachineHandle, val session: TmuxSession)
internal enum class MachinePlatform { Linux, Darwin }
internal data class MachineSummary(val handle: MachineHandle, val platform: MachinePlatform)
@Serializable internal enum class AgentProvider { Codex, Claude }
internal data class ProfileChoice(val key: ProfileKey, val label: String, val provider: AgentProvider)

@Serializable private data class WireMachineSummary(val handle: String, val platform: WireMachinePlatform)
@Serializable private enum class WireMachinePlatform { Linux, Darwin }
@Serializable private data class WireProfileChoice(
    val key: String,
    val label: String,
    val provider: AgentProvider,
)
@Serializable internal enum class SessionStatusKind { Working, Running, Idle, Shell, Unknown }
@Serializable internal enum class SessionStatusSignal { Lifecycle, Process, PollFailure }
@Serializable internal data class SessionStatus(
    val kind: SessionStatusKind,
    val signal: SessionStatusSignal,
    @Serializable(with = IsoInstantSerializer::class) val signalAt: Instant,
)
@Serializable internal data class CharacterSummary(val key: String, val displayName: String)

@ConsistentCopyVisibility
internal data class ProviderSessionFacts private constructor(
    val id: String? = null,
    val name: String? = null,
) {
    init {
        require(id != null || name != null)
        require(id?.let(::isProviderSessionId) != false)
        require(name?.let(::isProviderSessionName) != false)
    }

    companion object {
        fun withId(id: String, name: String? = null): ProviderSessionFacts =
            ProviderSessionFacts(id = id, name = name)

        fun withName(name: String): ProviderSessionFacts = ProviderSessionFacts(name = name)
    }
}

internal data class AgentRuntime(
    val provider: AgentProvider,
    val pid: Long,
    val profile: ProfileKey? = null,
    val providerSession: ProviderSessionFacts? = null,
) {
    init {
        require(pid > 0)
        when (provider) {
            AgentProvider.Codex -> require(providerSession?.name == null)
            AgentProvider.Claude -> Unit
        }
    }
}

internal data class TmuxSession(
    val tmuxId: String,
    val tmuxName: String,
    val identityToken: String,
    val character: CharacterSummary,
    val launchProfile: ProfileKey? = null,
    val agent: AgentRuntime? = null,
    val objective: String? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val attention: Boolean,
    val status: SessionStatus,
)

internal data class SessionsResponse(
    val machine: MachineSummary,
    val observedAt: Instant,
    val profiles: List<ProfileChoice>,
    val sessions: List<TmuxSession>,
)

@Serializable
private data class WireSessionsResponse(
    val machine: WireMachineSummary,
    @Serializable(with = IsoInstantSerializer::class) val observedAt: Instant,
    val profiles: List<WireProfileChoice>,
    val sessions: List<WireTmuxSession>,
)

@Serializable
private data class WireProviderSessionFacts(
    val id: String? = null,
    val name: String? = null,
)

@Serializable
private data class WireAgentRuntime(
    val provider: AgentProvider,
    val pid: Long,
    val profile: String? = null,
    val providerSession: WireProviderSessionFacts? = null,
)

@Serializable
private data class WireTmuxSession(
    val tmuxId: String,
    val tmuxName: String,
    val identityToken: String,
    val character: CharacterSummary,
    val launchProfile: String? = null,
    val agent: WireAgentRuntime? = null,
    val objective: String? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val attention: Boolean,
    val status: SessionStatus,
)

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

internal data class ForgeDraft(
    val machineHandle: MachineHandle,
    val cwd: String,
    val profile: ProfileKey,
    val optionalTmuxName: String,
    val objective: String,
)

internal data class ForgeForm(
    val machineHandle: MachineHandle?,
    val cwd: String,
    val profile: ProfileKey?,
    val optionalTmuxName: String,
    val objective: String,
) {
    constructor(draft: ForgeDraft) : this(
        draft.machineHandle,
        draft.cwd,
        draft.profile,
        draft.optionalTmuxName,
        draft.objective,
    )

    fun submission(): ForgeDraft? {
        if (machineHandle == null || profile == null || cwd.isBlank()) return null
        return ForgeDraft(machineHandle, cwd, profile, optionalTmuxName, objective)
    }
}

/**
 * Single owner of the Forge draft transition: a machine change clears the machine-scoped working
 * directory and profile while preserving the machine-independent name and objective.
 */
internal fun changeForgeDraft(current: ForgeForm, proposed: ForgeForm): ForgeForm =
    if (proposed.machineHandle == current.machineHandle) proposed else proposed.copy(cwd = "", profile = null)

internal fun forgeActionLabel(label: MachineLabel): String = "Create on ${label.text}"

/**
 * Single owner of destructive copy: the action label is also the screen-reader description of every
 * kill control, so the spoken description and the dialog title cannot name different sessions.
 */
internal fun killActionLabel(label: MachineLabel, target: SessionTarget): String =
    "Kill ${target.session.tmuxName} on ${label.text}"
internal fun killConfirmationTitle(label: MachineLabel, target: SessionTarget): String =
    killActionLabel(label, target) + "?"

@Serializable private data class CreateSessionRequest(
    val cwd: String,
    val profile: String,
    val optionalTmuxName: String? = null,
    val objective: String? = null,
)
@Serializable private data class KillSessionRequest(val tmuxName: String, val identityToken: String)

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.getValue("sessions").jsonArray.forEach { encodedSession ->
        encodedSession.jsonObject.requireSessionOptionalFields()
    }
    val wire = productJson.decodeFromJsonElement<WireSessionsResponse>(element)
    val handle = requireNotNull(MachineHandle.parse(wire.machine.handle))
    val profiles = wire.profiles.map { profile ->
        require(profile.label.isNotEmpty())
        ProfileChoice(requireNotNull(ProfileKey.parse(profile.key)), profile.label, profile.provider)
    }
    require(profiles.map(ProfileChoice::key).allUnique())
    require(profiles.map(ProfileChoice::label).allUnique())
    val sessions = wire.sessions.map { session -> acceptSession(session, wire.observedAt) }
    require(sessions.map(TmuxSession::tmuxId).allUnique())
    require(sessions.map(TmuxSession::identityToken).allUnique())
    sessions.forEach { session ->
        session.launchProfile?.let { launchProfile ->
            profiles.single { choice -> choice.key == launchProfile }
        }
        session.agent?.profile?.let { runtimeProfile ->
            profiles.single { choice ->
                choice.key == runtimeProfile && choice.provider == session.agent.provider
            }
        }
    }
    SessionsResponse(
        MachineSummary(handle, acceptMachinePlatform(wire.machine.platform)),
        wire.observedAt,
        profiles,
        sessions,
    )
}

internal fun decodePressureResponse(encoded: String): PressureResponse = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.getValue("current").jsonObject.getValue("signals").jsonObject.requireAbsentOrNonNull(
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

internal fun decodeTmuxSession(encoded: String): TmuxSession = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.requireSessionOptionalFields()
    acceptSession(productJson.decodeFromJsonElement<WireTmuxSession>(element), null)
}

internal fun encodeCreateSessionRequest(draft: ForgeDraft): String = productJson.encodeToString(
    CreateSessionRequest(
        draft.cwd,
        draft.profile.encoded,
        draft.optionalTmuxName.ifEmpty { null },
        draft.objective.ifEmpty { null },
    ),
)
internal fun encodeKillSessionRequest(session: TmuxSession): String =
    productJson.encodeToString(KillSessionRequest(session.tmuxName, session.identityToken))

internal data class InventorySnapshot(val inventory: SessionsResponse, val receivedAtElapsedMillis: Long)

internal sealed interface InventoryState {
    data object Reading : InventoryState
    data class Fresh(val snapshot: InventorySnapshot) : InventoryState
    /** A mutation landed on this machine and the snapshot has not caught up with it yet. */
    data class Superseded(val snapshot: InventorySnapshot, val requiredMutationFence: Long) : InventoryState
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

internal fun InventoryState.lastSnapshot(): InventorySnapshot? = when (this) {
    is InventoryState.Fresh -> snapshot
    is InventoryState.Superseded -> snapshot
    is InventoryState.Stale -> snapshot
    InventoryState.Reading, is InventoryState.Unreachable -> null
}

/** Single owner of the read-failure downgrade: a failed read never discards another machine's facts. */
internal fun InventoryState.downgraded(cause: GatewayFailure): InventoryState = when (this) {
    is InventoryState.Fresh -> InventoryState.Stale(snapshot, cause)
    is InventoryState.Superseded -> InventoryState.Stale(snapshot, cause)
    is InventoryState.Stale -> InventoryState.Stale(snapshot, cause)
    InventoryState.Reading, is InventoryState.Unreachable -> InventoryState.Unreachable(cause)
}

internal fun PressureState.downgraded(cause: GatewayFailure): PressureState = when (this) {
    is PressureState.Fresh -> PressureState.Stale(response, cause)
    is PressureState.Stale -> PressureState.Stale(response, cause)
    PressureState.Reading, is PressureState.Unavailable -> PressureState.Unavailable(cause)
}

internal data class MachineState(
    val machine: PairedMachine,
    val access: MachineAccess,
    val inventory: InventoryState,
    val pressure: PressureState,
) {
    val canMutate: Boolean get() = when (access) {
        MachineAccess.Ready -> inventory is InventoryState.Fresh
        MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> false
    }

    val canForge: Boolean get() = when (access) {
        MachineAccess.Ready ->
            inventory is InventoryState.Fresh && inventory.snapshot.inventory.profiles.isNotEmpty()
        MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> false
    }

    fun inventoryFailed(cause: GatewayFailure): MachineState = copy(inventory = inventory.downgraded(cause))
}

/**
 * Single classifier for what a machine can currently do. Every machine-state message, tag, colour,
 * and Forge affordance reads this one derivation, so a new access or inventory variant breaks the
 * build in exactly one place.
 */
internal sealed interface MachineAvailability {
    data object Ready : MachineAvailability
    data object Refreshing : MachineAvailability
    data object AuthRequired : MachineAvailability
    data object IdentityChanged : MachineAvailability
    data object Reading : MachineAvailability
    data class Stale(val cause: GatewayFailure) : MachineAvailability
    data class Unavailable(val cause: GatewayFailure) : MachineAvailability
}

internal fun machineAvailability(machine: MachineState): MachineAvailability = when (machine.access) {
    MachineAccess.AuthRequired -> MachineAvailability.AuthRequired
    MachineAccess.IdentityChanged -> MachineAvailability.IdentityChanged
    MachineAccess.Ready -> when (val inventory = machine.inventory) {
        InventoryState.Reading -> MachineAvailability.Reading
        is InventoryState.Fresh -> MachineAvailability.Ready
        is InventoryState.Superseded -> MachineAvailability.Refreshing
        is InventoryState.Stale -> MachineAvailability.Stale(inventory.cause)
        is InventoryState.Unreachable -> MachineAvailability.Unavailable(inventory.cause)
    }
}

internal data class MachineNotice(val message: String, val tone: NoticeTone)

/**
 * Single owner of how loud a machine state is. Trust events are the only failures: a broken bearer
 * or a changed identity means we no longer know who we are talking to. Everything else is absent or
 * ageing knowledge, and architecture.md treats one host being out as normal federated operation, so
 * an outage withdraws the alarm colour while its message still names it literally.
 */
internal fun availabilityTone(availability: MachineAvailability): NoticeTone = when (availability) {
    MachineAvailability.AuthRequired, MachineAvailability.IdentityChanged -> NoticeTone.Failure
    MachineAvailability.Ready,
    MachineAvailability.Refreshing,
    MachineAvailability.Reading,
    is MachineAvailability.Stale,
    is MachineAvailability.Unavailable,
    -> NoticeTone.Degraded
}

/**
 * Single owner of machine-state prose and its severity: one `when` over [MachineAvailability] yields
 * both, so a message and a tone read at different granularities stop being representable.
 */
internal fun machineNotice(machine: MachineState): MachineNotice? {
    val label = machine.machine.label.text
    val availability = machineAvailability(machine)
    val tone = availabilityTone(availability)
    return when (availability) {
        MachineAvailability.AuthRequired ->
            MachineNotice("$label: authentication required. Actions disabled.", tone)
        MachineAvailability.IdentityChanged ->
            MachineNotice("$label: identity changed. Fleet reset is required.", tone)
        MachineAvailability.Refreshing ->
            MachineNotice("$label: confirming the latest tmux inventory. Actions disabled.", tone)
        MachineAvailability.Reading -> MachineNotice("$label: reading tmux sessions.", tone)
        is MachineAvailability.Stale -> MachineNotice(
            "$label: ${gatewayFailureMessage(availability.cause)} Prior sessions are STALE; actions disabled. " +
                "Pull down to check again.",
            tone,
        )
        is MachineAvailability.Unavailable ->
            MachineNotice("$label: ${gatewayFailureMessage(availability.cause)} Pull down to check again.", tone)
        MachineAvailability.Ready -> when (machine.pressure) {
            is PressureState.Stale ->
                MachineNotice("$label: pressure is STALE. Sessions remain current.", tone)
            is PressureState.Unavailable ->
                MachineNotice("$label: pressure unavailable. Sessions remain current.", tone)
            PressureState.Reading, is PressureState.Fresh -> null
        }
    }
}

internal data class VisibleSession(val machine: PairedMachine, val target: SessionTarget)

internal fun visibleInventoryTargets(
    liveMachineHandles: Collection<MachineHandle>,
    selectedMachine: MachineHandle?,
): Set<MachineHandle> = when {
    selectedMachine == null -> liveMachineHandles.toSet()
    selectedMachine in liveMachineHandles -> setOf(selectedMachine)
    else -> emptySet()
}

internal fun pressureRailsVisible(selectedMachine: MachineHandle?): Boolean = selectedMachine != null

internal fun visibleSessions(machines: List<MachineState>, selectedMachine: MachineHandle?): List<VisibleSession> = machines
    .asSequence()
    .filter { selectedMachine == null || it.machine.handle == selectedMachine }
    .flatMap { state ->
        state.inventory.lastSnapshot()?.inventory?.sessions.orEmpty().asSequence().map { session ->
            VisibleSession(state.machine, SessionTarget(state.machine.handle, session))
        }
    }
    .sortedWith(
        compareByDescending<VisibleSession> { it.target.session.attention }
            .thenBy { statusRank(it.target.session.status.kind) }
            .thenBy { it.machine.label.text.lowercase(Locale.ROOT) }
            .thenBy { it.target.session.tmuxName.lowercase(Locale.ROOT) }
            .thenBy { it.target.session.tmuxId },
    )
    .toList()

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
    PairingInviteRejected("PairingInviteRejected"),
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
    ApiErrorCode.PairingInviteRejected -> "This fleet invite is invalid, expired, or already used."
    ApiErrorCode.MachineIdentityMismatch -> "The machine identity changed. Fleet reset is required."
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
    val elapsed = Duration.between(status.signalAt, now)
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
    val objectValue = strictJsonObject(encoded)
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
        else -> throw SerializationException("unknown terminal event kind")
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
private fun JsonObject.requireSessionOptionalFields() {
    requireAbsentOrNonNull(setOf("launchProfile", "agent", "objective", "cwd", "activeCommand"))
    this["agent"]?.jsonObject?.let { agent ->
        agent.requireAbsentOrNonNull(setOf("profile", "providerSession"))
        agent["providerSession"]?.jsonObject?.requireAbsentOrNonNull(setOf("id", "name"))
    }
}
private fun <Value> List<Value>.allUnique(): Boolean = distinct().size == size

private fun acceptMachinePlatform(platform: WireMachinePlatform): MachinePlatform = when (platform) {
    WireMachinePlatform.Linux -> MachinePlatform.Linux
    WireMachinePlatform.Darwin -> MachinePlatform.Darwin
}

private fun acceptSession(session: WireTmuxSession, observedAt: Instant?): TmuxSession = TmuxSession(
    tmuxId = session.tmuxId,
    tmuxName = session.tmuxName,
    identityToken = session.identityToken,
    character = session.character,
    launchProfile = session.launchProfile?.let { requireNotNull(ProfileKey.parse(it)) },
    agent = session.agent?.let(::acceptAgentRuntime),
    objective = session.objective,
    cwd = session.cwd,
    activeCommand = session.activeCommand,
    attachedClients = session.attachedClients,
    attention = session.attention,
    status = session.status,
).also { acceptSession(it, observedAt) }

private fun acceptAgentRuntime(runtime: WireAgentRuntime): AgentRuntime = AgentRuntime(
    provider = runtime.provider,
    pid = runtime.pid,
    profile = runtime.profile?.let { requireNotNull(ProfileKey.parse(it)) },
    providerSession = runtime.providerSession?.let(::acceptProviderSessionFacts),
)

private fun acceptProviderSessionFacts(facts: WireProviderSessionFacts): ProviderSessionFacts = when {
    facts.id != null -> ProviderSessionFacts.withId(facts.id, facts.name)
    facts.name != null -> ProviderSessionFacts.withName(facts.name)
    else -> throw IllegalArgumentException("provider session facts are empty")
}

private fun acceptSession(session: TmuxSession, observedAt: Instant?) {
    require(session.tmuxId.isNotEmpty() && session.tmuxName.isNotEmpty() && session.identityToken.isNotEmpty())
    require(session.attachedClients >= 0)
    require(session.cwd?.isNotEmpty() != false && session.activeCommand?.isNotEmpty() != false)
    require(session.objective?.isNotEmpty() != false)
    require(session.character.key.isNotEmpty() && session.character.displayName.isNotEmpty())
    val legal = when (session.status.kind) {
        SessionStatusKind.Working, SessionStatusKind.Idle -> session.status.signal == SessionStatusSignal.Lifecycle
        SessionStatusKind.Running, SessionStatusKind.Shell -> session.status.signal == SessionStatusSignal.Process
        SessionStatusKind.Unknown -> session.status.signal == SessionStatusSignal.PollFailure
    }
    require(legal)
    val agentStatusLegal = when (session.agent?.provider) {
        AgentProvider.Codex -> when (session.status.kind) {
            SessionStatusKind.Working, SessionStatusKind.Running, SessionStatusKind.Idle -> true
            SessionStatusKind.Shell, SessionStatusKind.Unknown -> false
        }
        AgentProvider.Claude -> session.status.kind == SessionStatusKind.Running
        null -> when (session.status.kind) {
            SessionStatusKind.Shell, SessionStatusKind.Unknown -> true
            SessionStatusKind.Working, SessionStatusKind.Running, SessionStatusKind.Idle -> false
        }
    }
    require(agentStatusLegal)
    if (observedAt != null) require(!session.status.signalAt.isAfter(observedAt))
}

private fun isProviderSessionId(value: String): Boolean =
    value.length in 1..128 && value.all { it.code in 0x21..0x7e }

private fun isProviderSessionName(value: String): Boolean {
    if (!Normalizer.isNormalized(value, Normalizer.Form.NFC)) return false
    val codePoints = value.codePoints().toArray()
    return codePoints.size in 1..128 && codePoints.none { codePoint ->
        Character.isISOControl(codePoint) ||
            codePoint in 0xd800..0xdfff ||
            codePoint == 0x061c ||
            codePoint in 0x200e..0x200f ||
            codePoint in 0x2028..0x202e ||
            codePoint in 0x2066..0x2069
    }
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
