package dev.niels.skidbladnir

import java.net.URI
import java.net.URISyntaxException
import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.text.Normalizer
import java.time.DateTimeException
import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlinx.serialization.KSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.encodeToString
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonArray

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

// The gateway encodes every protocol instant with Go's RFC3339Nano against a
// UTC clock, so exactly one shape is legal. ISO_INSTANT would also accept an
// offset form and a lower-case designator, which would let two encodings of the
// same moment cross a boundary the hard cut declares closed.
private val wireInstantPattern = Regex(
    """[0-9]{4}-[0-9]{2}-[0-9]{2}T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]{0,8}[1-9])?Z""",
)
private val wireInstantBaseFormatter =
    DateTimeFormatter.ofPattern("uuuu-MM-dd'T'HH:mm:ss", Locale.ROOT).withZone(ZoneOffset.UTC)

internal fun acceptWireInstant(text: String): Instant {
    if (!wireInstantPattern.matches(text)) throw SerializationException("instant is not the canonical UTC wire form")
    return Instant.parse(text)
}

internal fun formatWireInstant(value: Instant): String {
    val base = try {
        wireInstantBaseFormatter.format(value)
    } catch (_: DateTimeException) {
        throw SerializationException("instant is outside the canonical UTC wire range")
    }
    val fraction = value.nano.takeIf { it != 0 }
        ?.toString()
        ?.padStart(9, '0')
        ?.trimEnd('0')
        ?.let { ".$it" }
        .orEmpty()
    val encoded = "$base${fraction}Z"
    if (!wireInstantPattern.matches(encoded)) {
        throw SerializationException("instant is outside the canonical UTC wire range")
    }
    return encoded
}

internal object IsoInstantSerializer : KSerializer<Instant> {
    override val descriptor =
        PrimitiveSerialDescriptor("dev.niels.skidbladnir.IsoInstant", PrimitiveKind.STRING)
    override fun deserialize(decoder: Decoder): Instant = acceptWireInstant(decoder.decodeString())
    override fun serialize(encoder: Encoder, value: Instant) = encoder.encodeString(formatWireInstant(value))
}

// Go's zero time is syntactically valid RFC 3339, but it cannot represent a captured projection.
private val unsetProjectionInstant = Instant.parse("0001-01-01T00:00:00Z")

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
            if (candidate.hasDisplayUnsafeCodePoint()) return null
            return MachineLabel(candidate)
        }
    }
    override fun equals(other: Any?): Boolean = other is MachineLabel && text == other.text
    override fun hashCode(): Int = text.hashCode()
    override fun toString(): String = text
}

internal fun String.hasDisplayUnsafeCodePoint(): Boolean = codePoints().anyMatch { codePoint ->
    Character.isISOControl(codePoint) ||
        codePoint in 0xd800..0xdfff ||
        codePoint == 0x061c ||
        codePoint in 0x200e..0x200f ||
        codePoint in 0x2028..0x202e ||
        codePoint in 0x2066..0x2069
}

internal fun String.utf8ByteCountWithin(maximum: Int): Int? {
    if (length > maximum) return null
    var byteCount = 0
    var index = 0
    while (index < length) {
        val character = this[index]
        val increment = when {
            character.code <= 0x7f -> 1
            character.code <= 0x7ff -> 2
            character.isHighSurrogate() &&
                index + 1 < length &&
                this[index + 1].isLowSurrogate() -> {
                index += 1
                4
            }
            character.isSurrogate() -> return null
            else -> 3
        }
        byteCount += increment
        if (byteCount > maximum) return null
        index += 1
    }
    return byteCount
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
internal data class ProfileChoice(val key: ProfileKey, val label: String, val provider: AgentProvider, val historyScope: String? = null)

@Serializable private data class WireMachineSummary(val handle: String, val platform: WireMachinePlatform)
@Serializable private enum class WireMachinePlatform { Linux, Darwin }
@Serializable private data class WireProfileChoice(
    val key: String,
    val label: String,
    val provider: AgentProvider,
    val historyScope: String? = null,
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
    val paneId: String,
    val startIdentity: String,
    val profile: ProfileKey? = null,
    val providerSession: ProviderSessionFacts? = null,
) {
    init {
        require(pid > 0)
        require(paneId.matches(Regex("%[0-9]+")))
        require(startIdentity.isNotEmpty())
        when (provider) {
            AgentProvider.Codex -> require(providerSession?.name == null)
            AgentProvider.Claude -> Unit
        }
    }
}

@Serializable internal enum class NameMode {
    @kotlinx.serialization.SerialName("automatic") Automatic,
    @kotlinx.serialization.SerialName("manual") Manual,
}

internal sealed interface SessionNaming {
    data object Automatic : SessionNaming
    data class Manual(val name: String) : SessionNaming
}

internal fun TmuxSession.naming(): SessionNaming = when (nameMode) {
    NameMode.Automatic -> SessionNaming.Automatic
    NameMode.Manual -> SessionNaming.Manual(tmuxName)
}

/** An [agent] is always the local foreground: ingress rejects one beside a [connection]. */
internal data class TmuxSession(
    val tmuxId: String,
    val activePaneId: String,
    val tmuxName: String,
    val nameMode: NameMode,
    val identityToken: String,
    val character: CharacterSummary,
    val launchProfile: ProfileKey? = null,
    val objective: String? = null,
    val group: GroupLabel? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val agent: AgentRuntime? = null,
    val conversation: Conversation? = null,
    val terminalStatus: TerminalStatus,
    val connection: RemoteConnection? = null,
)

@Serializable internal enum class RemoteTransport { @kotlinx.serialization.SerialName("ssh") Ssh, @kotlinx.serialization.SerialName("mosh") Mosh }
internal data class RemoteConnection(val transport: RemoteTransport, val id: String? = null) {
    init { require(id == null || id.matches(Regex("[0-9a-f]{32}"))) }
}
internal data class RemoteAgent(val provider: AgentProvider, val profile: ProfileKey?)
internal data class TerminalContext(
    val observedAt: Instant,
    val cwd: String?,
    val agent: RemoteAgent?,
    val connection: RemoteConnection?,
)

internal sealed interface ExecutionContext {
    data class Local(val cwd: String?, val agent: AgentRuntime?) : ExecutionContext
    data class Remote(val machine: PairedMachine, val cwd: String?, val agent: RemoteAgent?) : ExecutionContext
    data object RemoteUnknown : ExecutionContext
}

@Serializable private data class WireRemoteConnection(val transport: RemoteTransport, val id: String? = null)
@Serializable private data class WireRemoteAgent(val provider: AgentProvider, val profile: String? = null)
@Serializable private data class WireTerminalContext(
    @Serializable(with = IsoInstantSerializer::class) val observedAt: Instant,
    val cwd: String? = null,
    val agent: WireRemoteAgent? = null,
    val connection: WireRemoteConnection? = null,
)

internal fun decodeTerminalContext(encoded: String): TerminalContext = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.requireAbsentOrNonNull(setOf("cwd", "agent", "connection"))
    (element["agent"] as? JsonObject)?.requireAbsentOrNonNull(setOf("profile"))
    (element["connection"] as? JsonObject)?.requireAbsentOrNonNull(setOf("id"))
    val wire = productJson.decodeFromJsonElement<WireTerminalContext>(element)
    require(wire.agent == null || wire.connection == null)
    require(wire.cwd == null || WorkingDirectoryPath.parse(wire.cwd) != null)
    TerminalContext(
        acceptProjectionInstant(wire.observedAt),
        wire.cwd,
        wire.agent?.let { RemoteAgent(it.provider, it.profile?.let { key -> requireNotNull(ProfileKey.parse(key)) }) },
        wire.connection?.let { RemoteConnection(it.transport, it.id) },
    )
}

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
private data class WireCreatedSessionResponse(
    @Serializable(with = IsoInstantSerializer::class) val observedAt: Instant,
    val session: WireTmuxSession,
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
    val paneId: String,
    val startIdentity: String,
    val profile: String? = null,
    val providerSession: WireProviderSessionFacts? = null,
)

@Serializable
private data class WireTmuxSession(
    val tmuxId: String,
    val activePaneId: String,
    val tmuxName: String,
    val nameMode: NameMode,
    val identityToken: String,
    val character: CharacterSummary,
    val launchProfile: String? = null,
    val objective: String? = null,
    val group: String? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val agent: WireAgentRuntime? = null,
    val conversation: Conversation? = null,
    val terminalStatus: TerminalStatus,
    val connection: WireRemoteConnection? = null,
)

internal sealed interface LaunchChoice {
    data class Agent(val profile: ProfileKey) : LaunchChoice
    data object Terminal : LaunchChoice
}

internal data class ForgeDraft(
    val machineHandle: MachineHandle,
    val cwd: String,
    val launch: LaunchChoice,
    val optionalTmuxName: String,
    val objective: String,
    val group: GroupLabel? = null,
)

internal const val MAXIMUM_WORKING_DIRECTORY_BYTES = 4_096
internal const val MAXIMUM_DIRECTORY_LISTING_CHILDREN = 256
internal const val MAXIMUM_DIRECTORY_LISTING_PATH_TEXT_BYTES = 32 * 1_024

internal class HomeDirectory private constructor(val encoded: String) {
    companion object {
        val Home = HomeDirectory("~")

        fun parse(candidate: String): HomeDirectory? {
            if (candidate == "~") return Home
            if (!candidate.startsWith("~/") || candidate.endsWith('/')) return null
            if (candidate.utf8ByteCountWithin(MAXIMUM_WORKING_DIRECTORY_BYTES) == null) return null
            if (candidate.hasDisplayUnsafeCodePoint()) return null
            val components = candidate.substring(2).split('/')
            if (components.any { component -> component.isEmpty() || component == "." || component == ".." }) {
                return null
            }
            return HomeDirectory(candidate)
        }
    }

    val basename: String get() = if (this == Home) "~" else encoded.substringAfterLast('/')
    val hidden: Boolean get() = this != Home && basename.startsWith('.')

    internal fun parent(): ParentDirectory = if (this == Home) {
        ParentDirectory.Absent
    } else {
        ParentDirectory.Available(
            checkNotNull(parse(encoded.substringBeforeLast('/').ifEmpty { "~" })),
        )
    }

    internal fun isDirectChildOf(parent: HomeDirectory): Boolean =
        parent() == ParentDirectory.Available(parent)

    override fun equals(other: Any?): Boolean = other is HomeDirectory && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = encoded
}

internal enum class DirectoryEntryKind { Directory, SymbolicLink }
internal sealed interface ParentDirectory {
    data object Absent : ParentDirectory
    data class Available(val directory: HomeDirectory) : ParentDirectory
}
internal enum class DirectoryOmissions { None, Present }
internal data class DirectoryEntry(val directory: HomeDirectory, val kind: DirectoryEntryKind)
internal data class DirectoryListing(
    val machine: MachineSummary,
    val directory: HomeDirectory,
    val parent: ParentDirectory,
    val children: List<DirectoryEntry>,
    val omissions: DirectoryOmissions,
)

internal data class DirectorySearchResult(val directories: List<WorkingDirectoryPath>, val omitted: Boolean)

@Serializable private data class WireDirectorySearchResult(val directories: List<String>, val omitted: Boolean)

internal fun decodeDirectorySearchResult(encoded: String): DirectorySearchResult = decodeProtocol {
    val wire = productJson.decodeFromJsonElement<WireDirectorySearchResult>(strictJsonObject(encoded))
    require(wire.directories.size <= 64)
    val directories = wire.directories.map { path ->
        require(path.startsWith('/'))
        requireNotNull(WorkingDirectoryPath.parse(path))
    }
    require(directories.distinct().size == directories.size)
    require(directories.sumOf { it.encoded.toByteArray(StandardCharsets.UTF_8).size } <= 32 * 1024)
    DirectorySearchResult(directories, wire.omitted)
}

@Serializable
private data class WireDirectoryListingResponse(
    val machine: WireMachineSummary,
    val directory: String,
    val parentDirectory: String? = null,
    val children: List<WireDirectoryEntry>,
    val omitted: Boolean,
)

@Serializable
private data class WireDirectoryEntry(
    val directory: String,
    val kind: WireDirectoryEntryKind,
)

@Serializable
private enum class WireDirectoryEntryKind { Directory, SymbolicLink }

internal fun decodeDirectoryListingResponse(
    encoded: String,
    expectedMachine: MachineSummary,
): DirectoryListing = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.requireAbsentOrNonNull(setOf("parentDirectory"))
    val wire = productJson.decodeFromJsonElement<WireDirectoryListingResponse>(element)
    val machineHandle = requireNotNull(MachineHandle.parse(wire.machine.handle))
    val machine = MachineSummary(machineHandle, acceptMachinePlatform(wire.machine.platform))
    require(machine == expectedMachine)
    val directory = requireNotNull(HomeDirectory.parse(wire.directory))
    val parent = wire.parentDirectory?.let { encodedParent ->
        ParentDirectory.Available(requireNotNull(HomeDirectory.parse(encodedParent)))
    } ?: ParentDirectory.Absent
    require(parent == directory.parent())
    require(wire.children.size <= MAXIMUM_DIRECTORY_LISTING_CHILDREN)
    val children = wire.children.map { child ->
        val childDirectory = requireNotNull(HomeDirectory.parse(child.directory))
        require(childDirectory.isDirectChildOf(directory))
        DirectoryEntry(
            directory = childDirectory,
            kind = when (child.kind) {
                WireDirectoryEntryKind.Directory -> DirectoryEntryKind.Directory
                WireDirectoryEntryKind.SymbolicLink -> DirectoryEntryKind.SymbolicLink
            },
        )
    }
    require(children.map(DirectoryEntry::directory).allUnique())
    require(
        children == children.sortedWith { first, second ->
            compareCaseInsensitiveUtf8(first.directory.basename, second.directory.basename)
        },
    )
    val pathTextBytes = sequenceOf(directory) +
        when (parent) {
            ParentDirectory.Absent -> emptySequence()
            is ParentDirectory.Available -> sequenceOf(parent.directory)
        } + children.asSequence().map(DirectoryEntry::directory)
    require(
        pathTextBytes.sumOf { path -> path.encoded.toByteArray(StandardCharsets.UTF_8).size } <=
            MAXIMUM_DIRECTORY_LISTING_PATH_TEXT_BYTES,
    )
    DirectoryListing(
        machine = machine,
        directory = directory,
        parent = parent,
        children = children,
        omissions = if (wire.omitted) DirectoryOmissions.Present else DirectoryOmissions.None,
    )
}

internal fun compareCaseInsensitiveUtf8(first: String, second: String): Int {
    val folded = compareAsciiFoldedUtf8(first, second)
    return if (folded != 0) folded else compareUtf8(first, second)
}

private fun compareAsciiFoldedUtf8(first: String, second: String): Int {
    val firstBytes = first.toByteArray(StandardCharsets.UTF_8)
    val secondBytes = second.toByteArray(StandardCharsets.UTF_8)
    for (index in 0 until minOf(firstBytes.size, secondBytes.size)) {
        val firstByte = asciiLowercase(firstBytes[index].toInt() and 0xff)
        val secondByte = asciiLowercase(secondBytes[index].toInt() and 0xff)
        if (firstByte != secondByte) return firstByte - secondByte
    }
    return firstBytes.size - secondBytes.size
}

private fun asciiLowercase(byte: Int): Int = if (byte in 0x41..0x5a) byte + 0x20 else byte

private fun compareUtf8(first: String, second: String): Int {
    val firstBytes = first.toByteArray(StandardCharsets.UTF_8)
    val secondBytes = second.toByteArray(StandardCharsets.UTF_8)
    for (index in 0 until minOf(firstBytes.size, secondBytes.size)) {
        val difference = (firstBytes[index].toInt() and 0xff) - (secondBytes[index].toInt() and 0xff)
        if (difference != 0) return difference
    }
    return firstBytes.size - secondBytes.size
}

internal data class ForgeForm(
    val machineHandle: MachineHandle?,
    val cwd: String,
    val launch: LaunchChoice?,
    val optionalTmuxName: String,
    val objective: String,
    val group: GroupDraft = GroupDraft.Chosen(""),
) {
    constructor(draft: ForgeDraft) : this(
        draft.machineHandle,
        draft.cwd,
        draft.launch,
        draft.optionalTmuxName,
        draft.objective,
        GroupDraft.Chosen(draft.group?.text.orEmpty()),
    )

    fun submission(): ForgeDraft? {
        if (machineHandle == null || launch == null || cwd.isBlank()) return null
        val chosen = group as? GroupDraft.Chosen ?: return null
        val label = if (chosen.text.isEmpty()) null else GroupLabel.fromDraft(chosen.text) ?: return null
        return ForgeDraft(machineHandle, cwd, launch, optionalTmuxName, objective, label)
    }
}

/**
 * Single owner of the Forge draft transition: a machine change clears the machine-scoped working
 * directory and agent profile while preserving terminal choice and the independent metadata.
 */
internal fun changeForgeDraft(current: ForgeForm, proposed: ForgeForm): ForgeForm =
    if (proposed.machineHandle == current.machineHandle) proposed else proposed.copy(
        cwd = "",
        launch = proposed.launch.takeIf { it == LaunchChoice.Terminal },
    )

internal fun forgeActionLabel(label: MachineLabel): String = "Create on ${label.text}"

/**
 * Single owner of destructive copy: the close confirmation's title and body name the verb and its
 * exact target from this one string, so the dialog cannot name a different session from the one acted on.
 */
internal fun closeActionLabel(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    "${if (terminalOnly) TERMINAL_ONLY_CLOSE_ACTION else TERMINAL_CLOSE_ACTION}: ${target.session.tmuxName} on ${label.text}"
internal fun closeConfirmationTitle(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    closeActionLabel(label, target, terminalOnly) + "?"

@Serializable private data class CreateSessionRequest(
    val kind: String,
    val cwd: String,
    val profile: String? = null,
    val optionalTmuxName: String? = null,
    val objective: String? = null,
    val group: String? = null,
)
@Serializable private data class DirectoryListingRequest(val directory: String)
@Serializable private data class CloseTerminalRequest(val identityToken: String)
@Serializable private data class WireSessionNaming(val mode: NameMode, val name: String? = null)
@Serializable private data class RenameSessionRequest(
    val identityToken: String,
    val expectedNaming: WireSessionNaming,
    val naming: WireSessionNaming,
)

private fun SessionNaming.toWire(): WireSessionNaming = when (this) {
    SessionNaming.Automatic -> WireSessionNaming(NameMode.Automatic)
    is SessionNaming.Manual -> WireSessionNaming(NameMode.Manual, name)
}

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.getValue("sessions").jsonArray.forEach { encodedSession ->
        (encodedSession as? JsonObject ?: throw SerializationException("session is not an object"))
            .requireSessionOptionalFields()
    }
    element.getValue("profiles").jsonArray.forEach { profile ->
        (profile as? JsonObject ?: throw SerializationException("profile is not an object"))
            .requireAbsentOrNonNull(setOf("historyScope"))
    }
    val wire = productJson.decodeFromJsonElement<WireSessionsResponse>(element)
    val observedAt = acceptProjectionInstant(wire.observedAt)
    val handle = requireNotNull(MachineHandle.parse(wire.machine.handle))
    val profiles = wire.profiles.map { profile ->
        require(profile.label.isNotEmpty())
        ProfileChoice(requireNotNull(ProfileKey.parse(profile.key)), profile.label, profile.provider, profile.historyScope).also {
            require(it.historyScope == null || it.historyScope.matches(Regex("[0-9a-f]{64}")))
        }
    }
    require(profiles.map(ProfileChoice::key).allUnique())
    require(profiles.map(ProfileChoice::label).allUnique())
    val sessions = wire.sessions.map(::acceptSession)
    require(sessions.map(TmuxSession::tmuxId).allUnique())
    require(sessions.map(TmuxSession::identityToken).allUnique())
    sessions.forEach { session ->
        session.launchProfile?.let { launchProfile ->
            profiles.single { choice -> choice.key == launchProfile }
        }
        session.agent?.let { agent ->
            agent.profile?.let { runtimeProfile ->
                profiles.single { choice -> choice.key == runtimeProfile && choice.provider == agent.provider }
            }
        }
    }
    SessionsResponse(
        MachineSummary(handle, acceptMachinePlatform(wire.machine.platform)),
        observedAt,
        profiles,
        sessions,
    )
}

internal fun decodeCreatedSessionResponse(encoded: String): TmuxSession = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.requireExactKeys(setOf("observedAt", "session"))
    element.requiredObject("session").requireSessionOptionalFields()
    val wire = productJson.decodeFromJsonElement<WireCreatedSessionResponse>(element)
    acceptProjectionInstant(wire.observedAt)
    acceptSession(wire.session)
}

internal fun encodeCreateSessionRequest(draft: ForgeDraft): String = productJson.encodeToString(
    CreateSessionRequest(
        when (draft.launch) {
            is LaunchChoice.Agent -> "agent"
            LaunchChoice.Terminal -> "terminal"
        },
        draft.cwd,
        (draft.launch as? LaunchChoice.Agent)?.profile?.encoded,
        draft.optionalTmuxName.ifEmpty { null },
        draft.objective.ifEmpty { null },
        draft.group?.text,
    ),
)
internal fun encodeDirectoryListingRequest(directory: HomeDirectory): String =
    productJson.encodeToString(DirectoryListingRequest(directory.encoded))
internal fun encodeCloseTerminalRequest(session: TmuxSession): String =
    productJson.encodeToString(CloseTerminalRequest(session.identityToken))
internal fun encodeRenameSessionRequest(
    target: SessionTarget,
    expectedNaming: SessionNaming,
    naming: SessionNaming,
): String = productJson.encodeToString(
    RenameSessionRequest(target.session.identityToken, expectedNaming.toWire(), naming.toWire()),
)

internal data class InventorySnapshot(val inventory: SessionsResponse, val receivedAtElapsedMillis: Long)

internal sealed interface InventoryState {
    data object Reading : InventoryState
    data class Fresh(val snapshot: InventorySnapshot) : InventoryState
    /** A mutation landed on this machine and the snapshot has not caught up with it yet. */
    data class Superseded(val snapshot: InventorySnapshot, val requiredMutationFence: Long) : InventoryState
    data class Stale(val snapshot: InventorySnapshot, val cause: GatewayFailure) : InventoryState
    data class Unreachable(val cause: GatewayFailure) : InventoryState
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

internal data class MachineState(
    val machine: PairedMachine,
    val access: MachineAccess,
    val inventory: InventoryState,
    val pressure: PressureState,
    val remoteContexts: Map<String, ExecutionContext.Remote> = emptyMap(),
    val notifications: Map<NotificationKey, NotificationPresentation> = emptyMap(),
) {
    val canMutate: Boolean get() = when (access) {
        MachineAccess.Ready -> inventory is InventoryState.Fresh
        MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> false
    }

    val canForge: Boolean get() = canMutate

    fun inventoryFailed(cause: GatewayFailure): MachineState = copy(inventory = inventory.downgraded(cause))
}

internal fun MachineState.executionContext(session: TmuxSession): ExecutionContext {
    val current = (inventory as? InventoryState.Fresh)?.snapshot?.inventory?.sessions?.singleOrNull {
        it.tmuxId == session.tmuxId && it.identityToken == session.identityToken
    }
    val observed = current ?: session
    return if (observed.connection == null) ExecutionContext.Local(observed.cwd, observed.agent)
    else if (access == MachineAccess.Ready && current != null)
        remoteContexts[session.identityToken] ?: ExecutionContext.RemoteUnknown
    else ExecutionContext.RemoteUnknown
}

internal fun remoteAgentLabel(context: ExecutionContext.Remote, machines: List<MachineState>): String {
    val agent = context.agent ?: return "terminal"
    val profiles = machines.singleOrNull { it.machine.handle == context.machine.handle }
        ?.inventory?.lastSnapshot()?.inventory?.profiles.orEmpty()
    return agent.profile?.let { key -> profiles.singleOrNull { it.key == key && it.provider == agent.provider }?.label }
        ?: "${agent.provider.name} · profile unknown"
}

/**
 * Single classifier for what a machine can currently do. Every machine-state message, colour,
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

internal data class SessionAvailabilityContent(val label: String, val tone: NoticeTone)

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
 * Single owner of the retained-card availability marker. A card can remain visible while actions
 * are fenced, but the marker must name the actual reason instead of collapsing every fence into
 * staleness. Reading and unavailable inventories have no retained card to annotate.
 */
internal fun sessionAvailabilityContent(machine: MachineState): SessionAvailabilityContent? {
    val availability = machineAvailability(machine)
    val label = when (availability) {
        MachineAvailability.Ready -> return null
        MachineAvailability.Refreshing -> "REFRESHING · actions disabled"
        MachineAvailability.AuthRequired -> "AUTH REQUIRED · actions disabled"
        MachineAvailability.IdentityChanged -> "IDENTITY CHANGED · actions disabled"
        MachineAvailability.Reading, is MachineAvailability.Unavailable -> return null
        is MachineAvailability.Stale -> "STALE · actions disabled"
    }
    return SessionAvailabilityContent(label, availabilityTone(availability))
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

internal data class VisibleSession(
    val machine: PairedMachine,
    val target: SessionTarget,
    val context: ExecutionContext,
) {
    val cardKey: DashboardCardKey = dashboardCardKey(target)
}

internal fun dashboardCardKey(target: SessionTarget): DashboardCardKey {
    val digest = MessageDigest.getInstance("SHA-256")
    digest.update("skidbladnir.dashboard-card.v1".encodeToByteArray())
    listOf(
        target.machineHandle.encoded,
        target.session.tmuxId,
        target.session.identityToken,
    ).forEach { value ->
        val bytes = value.encodeToByteArray()
        digest.update(
            byteArrayOf(
                (bytes.size ushr 24).toByte(),
                (bytes.size ushr 16).toByte(),
                (bytes.size ushr 8).toByte(),
                bytes.size.toByte(),
            ),
        )
        digest.update(bytes)
    }
    val hexadecimal = "0123456789abcdef"
    return DashboardCardKey(buildString(64) {
        digest.digest().forEach { byte ->
            val value = byte.toInt() and 0xff
            append(hexadecimal[value ushr 4])
            append(hexadecimal[value and 0x0f])
        }
    })
}


internal enum class ApiErrorCode(val wireName: String) {
    Unauthenticated("Unauthenticated"), InvalidRequest("InvalidRequest"), RequestTooLarge("RequestTooLarge"),
    WorkingDirectoryInvalid("WorkingDirectoryInvalid"), WorkingDirectoryUnavailable("WorkingDirectoryUnavailable"),
    DirectoryListingUnavailable("DirectoryListingUnavailable"), DirectoryListingTooLarge("DirectoryListingTooLarge"),
    DirectorySearchUnavailable("DirectorySearchUnavailable"), DirectorySearchTooLarge("DirectorySearchTooLarge"),
    TerminalContextUnavailable("TerminalContextUnavailable"),
    TerminalTargetChanged("TerminalTargetChanged"), TerminalUnavailable("TerminalUnavailable"), TerminalInputBlocked("TerminalInputBlocked"),
    ProfileUnknown("ProfileUnknown"), SessionNameInvalid("SessionNameInvalid"), ObjectiveInvalid("ObjectiveInvalid"), GroupInvalid("GroupInvalid"),
    SessionNameConflict("SessionNameConflict"), SessionNameChanged("SessionNameChanged"), SessionNotFound("SessionNotFound"),
    SessionIdentityMismatch("SessionIdentityMismatch"),
    PairingInviteRejected("PairingInviteRejected"),
    MachineIdentityMismatch("MachineIdentityMismatch"), InternalError("InternalError"),
    ReconnectRequired("ReconnectRequired"),
    TerminalConfigurationUnsupported("TerminalConfigurationUnsupported"),
    AgentTargetStale("AgentTargetStale"),
    AgentUnavailable("AgentUnavailable"), AgentInputInvalid("AgentInputInvalid"), HistoryChanged("HistoryChanged"),
}

internal fun apiErrorMessage(code: ApiErrorCode): String = when (code) {
    ApiErrorCode.Unauthenticated -> "Authentication required."
    ApiErrorCode.InvalidRequest -> "The request is not valid."
    ApiErrorCode.RequestTooLarge -> "The request is too large."
    ApiErrorCode.WorkingDirectoryInvalid -> "Choose a valid working directory."
    ApiErrorCode.WorkingDirectoryUnavailable -> "That directory does not exist or cannot be opened."
    ApiErrorCode.DirectoryListingUnavailable ->
        "This directory cannot be browsed. Enter the path instead."
    ApiErrorCode.DirectoryListingTooLarge ->
        "This directory has too many folders to show. Enter the path instead."
    ApiErrorCode.DirectorySearchUnavailable -> "Directory search is unavailable on this machine."
    ApiErrorCode.DirectorySearchTooLarge -> "Too many directory search results. Narrow the search."
    ApiErrorCode.TerminalContextUnavailable -> "Remote context is unavailable."
    ApiErrorCode.TerminalTargetChanged -> "the terminal changed. refresh before trying again."
    ApiErrorCode.TerminalUnavailable -> "the terminal is unavailable. open it to inspect before trying again."
    ApiErrorCode.TerminalInputBlocked -> "send unavailable for this screen. open the terminal or use text/keys."
    ApiErrorCode.ProfileUnknown -> "Choose an available profile."
    ApiErrorCode.SessionNameInvalid -> "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number."
    ApiErrorCode.GroupInvalid -> GROUP_INVALID
    ApiErrorCode.ObjectiveInvalid -> "Use 1–240 characters without terminal controls."
    ApiErrorCode.SessionNameConflict -> "another session on this machine uses that name."
    ApiErrorCode.SessionNameChanged -> RENAME_STALE_EDIT
    ApiErrorCode.SessionNotFound -> "That session no longer exists."
    ApiErrorCode.SessionIdentityMismatch -> "The session changed. Refresh and try again."
    ApiErrorCode.PairingInviteRejected -> "This fleet invite is invalid, expired, or already used."
    ApiErrorCode.MachineIdentityMismatch -> "The machine identity changed. Fleet reset is required."
    ApiErrorCode.InternalError -> "Skíðblaðnir could not complete the request."
    ApiErrorCode.ReconnectRequired -> "Reconnect required."
    ApiErrorCode.TerminalConfigurationUnsupported ->
        "tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on."
    ApiErrorCode.AgentTargetStale -> "the session changed. refresh and try again."
    ApiErrorCode.AgentUnavailable -> "this action is unavailable for this session."
    ApiErrorCode.HistoryChanged -> "native history changed. restart the result scan."
    ApiErrorCode.AgentInputInvalid -> "the agent input is not valid."
}

internal fun parseApiErrorCode(value: String): ApiErrorCode =
    ApiErrorCode.entries.singleOrNull { it.wireName == value } ?: throw SerializationException("unknown API error code")

internal enum class SessionStatusTone { Working, Ready, Attention, Muted }
internal enum class SessionQueueCategory { Ready, Action, Excluded }
// `detail` tells one session's state from another's; `evidence` names how status was
// known. The card shows only the first, since the qualifier is the same on every agent card.
internal data class SessionStatusContent(
    val label: String, val accessibilityLabel: String, val detail: String?, val evidence: String?,
    val tone: SessionStatusTone, val secondary: String?, val queueCategory: SessionQueueCategory,
    val detailExplainsQueue: Boolean,
)

/**
 * Single status projection for the card and the terminal header (terminal-observation.md §6): the
 * first matching row wins. A response request, menu or current notice outranks visible work, which
 * then survives as `work continues`. Only a local agent's terminal sample makes an inference claim.
 */
internal fun sessionStatusContent(
    session: TmuxSession, fresh: Boolean, notification: NotificationPresentation, showQueueReason: Boolean = false,
): SessionStatusContent {
    val status = session.terminalStatus
    val inferred = status.source == TerminalStatusSource.Terminal && session.agent != null && session.connection == null
    val request = when (status.interaction) {
        TerminalInteraction.Permission, TerminalInteraction.Question, TerminalInteraction.Setup,
        TerminalInteraction.Confirmation, TerminalInteraction.Input -> true
        TerminalInteraction.None, TerminalInteraction.Menu, TerminalInteraction.Unknown -> false
    }
    val notice = when (status.notice) {
        TerminalNotice.Interrupted -> "interruption shown"
        TerminalNotice.Error -> "error shown"
        TerminalNotice.None -> null
    }
    val ready = inferred && fresh && notification.ready && !notification.unavailable &&
        status.activity == TerminalActivity.Idle && status.interaction == TerminalInteraction.None && status.notice == TerminalNotice.None
    val queueCategory = when {
        !inferred || !fresh -> SessionQueueCategory.Excluded
        request || status.notice != TerminalNotice.None -> SessionQueueCategory.Action
        ready -> SessionQueueCategory.Ready
        else -> SessionQueueCategory.Excluded
    }
    val requestMenuOrNotice = when (status.interaction) {
        TerminalInteraction.Permission -> "needs permission" to SessionStatusTone.Attention
        TerminalInteraction.Question -> "needs answer" to SessionStatusTone.Attention
        TerminalInteraction.Setup -> "needs setup" to SessionStatusTone.Attention
        TerminalInteraction.Confirmation -> "needs review" to SessionStatusTone.Attention
        TerminalInteraction.Input -> "needs input" to SessionStatusTone.Attention
        TerminalInteraction.Menu -> "menu open" to SessionStatusTone.Muted
        TerminalInteraction.None, TerminalInteraction.Unknown -> when (status.notice) {
            TerminalNotice.Interrupted -> "interruption shown" to SessionStatusTone.Muted
            TerminalNotice.Error -> "error shown" to SessionStatusTone.Attention
            TerminalNotice.None -> null
        }
    }
    val (state, tone) = when {
        status.source == TerminalStatusSource.Unavailable -> "status unavailable" to SessionStatusTone.Muted
        !inferred -> "terminal" to SessionStatusTone.Muted
        requestMenuOrNotice != null -> requestMenuOrNotice
        else -> when (status.activity) {
            TerminalActivity.Starting -> "starting" to SessionStatusTone.Working
            TerminalActivity.Working -> "working" to SessionStatusTone.Working
            // No request, menu or notice remains, so interaction is none or unknown here.
            TerminalActivity.Idle -> when {
                status.interaction == TerminalInteraction.Unknown -> "status unknown" to SessionStatusTone.Muted
                ready -> "ready" to SessionStatusTone.Ready
                else -> "idle" to SessionStatusTone.Muted
            }
            TerminalActivity.Unknown -> "status unknown" to SessionStatusTone.Muted
        }
    }
    val workContinues = inferred && requestMenuOrNotice != null && status.activity == TerminalActivity.Working
    val label = if (fresh) state else "last observed: $state"
    val secondary = if (notification.unavailable) "notifications unavailable" else null
    val queueNotice = notice.takeIf { showQueueReason && inferred && status.interaction == TerminalInteraction.Menu }
    val detail = if (queueNotice != null || workContinues) {
        listOfNotNull(queueNotice, "work continues".takeIf { workContinues }).joinToString(" · ")
    } else session.activeCommand.takeIf { !inferred && session.connection == null }
    return SessionStatusContent(
        label,
        label + (queueNotice?.let { "; $it" } ?: "") + (if (workContinues) "; work continues" else "") + (if (inferred) "; inferred from terminal" else "") +
            (secondary?.let { "; $it" } ?: ""),
        detail,
        if (inferred) "inferred from terminal" else null,
        if (fresh) tone else SessionStatusTone.Muted, secondary, queueCategory,
        detailExplainsQueue = queueNotice != null,
    )
}

private fun JsonObject.requireSessionOptionalFields() {
    if ("group" in this) requiredString("group")
    requireAbsentOrNonNull(setOf("launchProfile", "objective", "group", "cwd", "activeCommand", "agent", "conversation", "connection"))
    (this["agent"] as? JsonObject)?.let { agent ->
        agent.requireNativeValues()
        (agent["providerSession"] as? JsonObject)?.requireAbsentOrNonNull(setOf("id", "name"))
    }
    (this["conversation"] as? JsonObject)?.requireNativeValues()
    (this["connection"] as? JsonObject)?.requireAbsentOrNonNull(setOf("id"))
}
private fun <Value> List<Value>.allUnique(): Boolean = distinct().size == size

private fun acceptProjectionInstant(value: Instant): Instant {
    require(value != unsetProjectionInstant)
    return value
}

private fun acceptMachinePlatform(platform: WireMachinePlatform): MachinePlatform = when (platform) {
    WireMachinePlatform.Linux -> MachinePlatform.Linux
    WireMachinePlatform.Darwin -> MachinePlatform.Darwin
}

private fun acceptSession(session: WireTmuxSession): TmuxSession = TmuxSession(
    tmuxId = session.tmuxId,
    activePaneId = session.activePaneId,
    tmuxName = session.tmuxName,
    nameMode = session.nameMode,
    identityToken = session.identityToken,
    character = session.character,
    launchProfile = session.launchProfile?.let { requireNotNull(ProfileKey.parse(it)) },
    objective = session.objective,
    group = session.group?.let { requireNotNull(GroupLabel.parse(it)) },
    cwd = session.cwd,
    activeCommand = session.activeCommand,
    attachedClients = session.attachedClients,
    agent = session.agent?.let(::acceptAgentRuntime),
    conversation = session.conversation,
    terminalStatus = session.terminalStatus,
    connection = session.connection?.let { RemoteConnection(it.transport, it.id) },
).also(::acceptSession)

private fun acceptAgentRuntime(runtime: WireAgentRuntime): AgentRuntime = AgentRuntime(
    provider = runtime.provider,
    pid = runtime.pid,
    paneId = runtime.paneId,
    startIdentity = runtime.startIdentity,
    profile = runtime.profile?.let { requireNotNull(ProfileKey.parse(it)) },
    providerSession = runtime.providerSession?.let(::acceptProviderSessionFacts),
)

private fun acceptProviderSessionFacts(facts: WireProviderSessionFacts): ProviderSessionFacts = when {
    facts.id != null -> ProviderSessionFacts.withId(facts.id, facts.name)
    facts.name != null -> ProviderSessionFacts.withName(facts.name)
    else -> throw IllegalArgumentException("provider session facts are empty")
}

private fun acceptSession(session: TmuxSession) {
    require(session.tmuxId.matches(Regex("\\$[0-9]+")) && session.tmuxName.isNotEmpty() && session.identityToken.isNotEmpty())
    require(session.activePaneId.matches(Regex("%[0-9]+")))
    require(session.agent == null || session.agent.paneId == session.activePaneId)
    require(session.attachedClients >= 0)
    require(session.cwd?.let(WorkingDirectoryPath::parse) != null || session.cwd == null)
    require(session.activeCommand?.isNotEmpty() != false)
    require(session.objective?.isNotEmpty() != false)
    require(session.character.key.isNotEmpty() && session.character.displayName.isNotEmpty())
    require(session.agent == null || session.connection == null)
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
