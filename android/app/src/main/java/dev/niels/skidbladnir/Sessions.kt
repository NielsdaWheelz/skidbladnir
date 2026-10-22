package dev.niels.skidbladnir

import java.time.DateTimeException
import java.time.Instant
import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.decodeFromString
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder

@JvmInline @Serializable internal value class ProfileKey(val encoded: String)
@Serializable internal enum class AgentProvider { Codex, Claude }
@Serializable internal data class ProfileChoice(val key: ProfileKey, val label: String, val provider: AgentProvider? = null)
@Serializable internal data class CharacterSummary(val key: String, val displayName: String)
@Serializable internal data class MachineSummary(val handle: MachineHandle)

internal data class ForegroundObservation(
    val provider: AgentProvider,
    val pid: Long,
    val startIdentity: String,
)

@Serializable internal data class AgentRuntime(
    val provider: AgentProvider,
    val pid: Long,
    val startIdentity: String,
    val profile: ProfileKey? = null,
    /** The host's configured label for [profile]; absent when unset or unconfigured. */
    val label: String? = null,
)

@Serializable internal enum class NameMode {
    @SerialName("automatic") Automatic,
    @SerialName("manual") Manual,
}

internal sealed interface SessionNaming {
    data object Automatic : SessionNaming
    data class Manual(val name: String) : SessionNaming
}

internal fun TmuxSession.naming(): SessionNaming = when (nameMode) {
    NameMode.Automatic -> SessionNaming.Automatic
    NameMode.Manual -> SessionNaming.Manual(tmuxName)
}

@Serializable internal enum class RemoteTransport { @SerialName("ssh") Ssh, @SerialName("mosh") Mosh }
@Serializable internal data class RemoteConnection(val transport: RemoteTransport, val id: String? = null)
@Serializable internal data class RemoteAgent(val provider: AgentProvider, val profile: ProfileKey? = null, val label: String? = null)
@Serializable internal data class TerminalContext(
    val cwd: String? = null,
    val agent: RemoteAgent? = null,
    val connection: RemoteConnection? = null,
)

/** An [agent] is always the local foreground: the host never reports one beside a [connection]. */
@Serializable internal data class TmuxSession(
    val tmuxId: String,
    val activePaneId: String,
    val tmuxName: String,
    val nameMode: NameMode,
    val identityToken: String,
    val character: CharacterSummary,
    val objective: String? = null,
    @Serializable(with = GroupLabelSerializer::class) val group: GroupLabel? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val agent: AgentRuntime? = null,
    val terminalStatus: TerminalStatus,
    val status: StatusPresentation,
    val connection: RemoteConnection? = null,
) {
    /** tmux reuses session ids, so one lifetime is the id plus the gateway's identity token. */
    fun sameLifetime(other: TmuxSession): Boolean =
        tmuxId == other.tmuxId && identityToken == other.identityToken
}

/** The host's status copy for one observation, rendered as if fresh and not ready (terminal-observation.md §6). */
@Serializable internal data class StatusPresentation(
    val label: String,
    val tone: SessionStatusTone,
    val queue: SessionQueueCategory,
    val detail: String? = null,
    val evidence: String? = null,
    val queueNotice: String? = null,
)

internal data class SessionTarget(val machineHandle: MachineHandle, val session: TmuxSession) {
    fun sameLifetime(other: SessionTarget): Boolean =
        machineHandle == other.machineHandle && session.sameLifetime(other.session)
}

@Serializable internal data class SessionsResponse(
    val machine: MachineSummary,
    val recovery: WorkspaceRecovery,
    val profiles: List<ProfileChoice>,
    val sessions: List<TmuxSession>,
)

@Serializable internal enum class WorkspaceRecoveryState {
    @SerialName("tracking") Tracking,
    @SerialName("recovered") Recovered,
    @SerialName("delayed") Delayed,
    @SerialName("broken") Broken,
}

/** The host's recovery copy without the machine label and the save time, which the phone renders in its own zone. */
@Serializable internal data class RecoveryNotice(val summary: String, val action: String? = null)

/** [notice] is absent exactly while the host is tracking. */
@Serializable internal data class WorkspaceRecovery(
    val state: WorkspaceRecoveryState,
    @Serializable(with = InstantSerializer::class) val savedAt: Instant? = null,
    val notice: RecoveryNotice? = null,
)

@Serializable private data class WireCreatedSession(val session: TmuxSession)

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = productJson.decodeFromString<SessionsResponse>(encoded)

internal fun decodeCreatedSession(encoded: String): TmuxSession =
    productJson.decodeFromString<WireCreatedSession>(encoded).session

private object InstantSerializer : KSerializer<Instant> {
    override val descriptor = PrimitiveSerialDescriptor("dev.niels.skidbladnir.Instant", PrimitiveKind.STRING)
    override fun deserialize(decoder: Decoder): Instant = try {
        Instant.parse(decoder.decodeString())
    } catch (_: DateTimeException) {
        throw SerializationException("instant is not ISO-8601")
    }
    override fun serialize(encoder: Encoder, value: Instant) = encoder.encodeString(value.toString())
}

// GroupLabel has no trusted constructor, so a label the phone cannot parse loses the response it came in.
private object GroupLabelSerializer : KSerializer<GroupLabel> {
    override val descriptor = PrimitiveSerialDescriptor("dev.niels.skidbladnir.GroupLabel", PrimitiveKind.STRING)
    override fun deserialize(decoder: Decoder): GroupLabel =
        GroupLabel.parse(decoder.decodeString()) ?: throw SerializationException("group label is not canonical")
    override fun serialize(encoder: Encoder, value: GroupLabel) = encoder.encodeString(value.text)
}
