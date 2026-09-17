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
internal data class ProfileChoice(val key: ProfileKey, val label: String, val provider: AgentProvider)

@Serializable private data class WireMachineSummary(val handle: String, val platform: WireMachinePlatform)
@Serializable private enum class WireMachinePlatform { Linux, Darwin }
@Serializable private data class WireProfileChoice(
    val key: String,
    val label: String,
    val provider: AgentProvider,
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
    val status: AgentStatus,
    val methods: AgentMethods,
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

internal data class TmuxSession(
    val tmuxId: String,
    val tmuxName: String,
    val identityToken: String,
    val character: CharacterSummary,
    val launchProfile: ProfileKey? = null,
    val objective: String? = null,
    val space: SpaceLabel? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val agent: AgentRuntime? = null,
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
    val status: AgentStatus,
    val methods: AgentMethods,
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
    val objective: String? = null,
    val space: String? = null,
    val cwd: String? = null,
    val activeCommand: String? = null,
    val attachedClients: Int,
    val agent: WireAgentRuntime? = null,
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
    val space: SpaceLabel? = null,
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
    val space: SpaceDraft = SpaceDraft.Chosen(""),
) {
    constructor(draft: ForgeDraft) : this(
        draft.machineHandle,
        draft.cwd,
        draft.launch,
        draft.optionalTmuxName,
        draft.objective,
        SpaceDraft.Chosen(draft.space?.text.orEmpty()),
    )

    fun submission(): ForgeDraft? {
        if (machineHandle == null || launch == null || cwd.isBlank()) return null
        val chosen = space as? SpaceDraft.Chosen ?: return null
        val label = if (chosen.text.isEmpty()) null else SpaceLabel.fromDraft(chosen.text) ?: return null
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
 * Single owner of destructive copy: the action label is also the screen-reader description of every
 * kill control, so the spoken description and the dialog title cannot name different sessions.
 */
internal fun killActionLabel(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    "${if (target.session.agent == null || terminalOnly) "Kill" else "Stop"} ${target.session.tmuxName} on ${label.text}"
internal fun killConfirmationTitle(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    killActionLabel(label, target, terminalOnly) + "?"

@Serializable private data class CreateSessionRequest(
    val kind: String,
    val cwd: String,
    val profile: String? = null,
    val optionalTmuxName: String? = null,
    val objective: String? = null,
    val space: String? = null,
)
@Serializable private data class DirectoryListingRequest(val directory: String)
@Serializable private data class KillSessionRequest(val tmuxName: String, val identityToken: String)
@Serializable private data class RenameSessionRequest(
    val tmuxName: String,
    val newTmuxName: String,
    val identityToken: String,
)

internal fun decodeSessionsResponse(encoded: String): SessionsResponse = decodeProtocol {
    val element = strictJsonObject(encoded)
    element.getValue("sessions").jsonArray.forEach { encodedSession ->
        (encodedSession as? JsonObject ?: throw SerializationException("session is not an object"))
            .requireSessionOptionalFields()
    }
    val wire = productJson.decodeFromJsonElement<WireSessionsResponse>(element)
    val observedAt = acceptProjectionInstant(wire.observedAt)
    val handle = requireNotNull(MachineHandle.parse(wire.machine.handle))
    val profiles = wire.profiles.map { profile ->
        require(profile.label.isNotEmpty())
        ProfileChoice(requireNotNull(ProfileKey.parse(profile.key)), profile.label, profile.provider)
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
        draft.space?.text,
    ),
)
internal fun encodeDirectoryListingRequest(directory: HomeDirectory): String =
    productJson.encodeToString(DirectoryListingRequest(directory.encoded))
internal fun encodeKillSessionRequest(session: TmuxSession): String =
    productJson.encodeToString(KillSessionRequest(session.tmuxName, session.identityToken))
internal fun encodeRenameSessionRequest(target: SessionTarget, newTmuxName: String): String =
    productJson.encodeToString(
        RenameSessionRequest(
            tmuxName = target.session.tmuxName,
            newTmuxName = newTmuxName,
            identityToken = target.session.identityToken,
        ),
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
) {
    val canMutate: Boolean get() = when (access) {
        MachineAccess.Ready -> inventory is InventoryState.Fresh
        MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> false
    }

    val canForge: Boolean get() = canMutate

    fun inventoryFailed(cause: GatewayFailure): MachineState = copy(inventory = inventory.downgraded(cause))
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


internal fun visibleInventoryTargets(
    liveMachineHandles: Collection<MachineHandle>,
    scope: DashboardScope,
): Set<MachineHandle> = when (scope) {
    DashboardScope.All -> liveMachineHandles.toSet()
    is DashboardScope.Machine -> if (scope.handle in liveMachineHandles) setOf(scope.handle) else emptySet()
}

internal fun visibleSessions(machines: List<MachineState>, scope: DashboardScope): List<VisibleSession> = machines
    .filter { scope == DashboardScope.All || (scope as? DashboardScope.Machine)?.handle == it.machine.handle }
    .flatMap { state -> state.inventory.lastSnapshot()?.inventory?.sessions.orEmpty().map {
        VisibleSession(state.machine, SessionTarget(state.machine.handle, it))
    } }
    .sortedWith(compareBy<VisibleSession> { it.machine.label.text.lowercase(Locale.ROOT) }
        .thenBy { it.machine.label.text }
        .thenBy { it.machine.handle.encoded }
        .thenBy { it.target.session.tmuxName.lowercase(Locale.ROOT) }
        .thenBy { it.target.session.tmuxName }
        .thenBy { it.target.session.tmuxId })

internal enum class ApiErrorCode(val wireName: String) {
    Unauthenticated("Unauthenticated"), InvalidRequest("InvalidRequest"), RequestTooLarge("RequestTooLarge"),
    WorkingDirectoryInvalid("WorkingDirectoryInvalid"), WorkingDirectoryUnavailable("WorkingDirectoryUnavailable"),
    DirectoryListingUnavailable("DirectoryListingUnavailable"), DirectoryListingTooLarge("DirectoryListingTooLarge"),
    ProfileUnknown("ProfileUnknown"), SessionNameInvalid("SessionNameInvalid"), ObjectiveInvalid("ObjectiveInvalid"), SpaceInvalid("SpaceInvalid"),
    SessionNameConflict("SessionNameConflict"), SessionNotFound("SessionNotFound"),
    SessionIdentityMismatch("SessionIdentityMismatch"),
    PairingInviteRejected("PairingInviteRejected"),
    MachineIdentityMismatch("MachineIdentityMismatch"), InternalError("InternalError"),
    ReconnectRequired("ReconnectRequired"),
    TerminalConfigurationUnsupported("TerminalConfigurationUnsupported"),
    AgentTargetStale("AgentTargetStale"), AgentUnavailable("AgentUnavailable"),
    AgentBlocked("AgentBlocked"), AgentInputInvalid("AgentInputInvalid"),
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
    ApiErrorCode.ProfileUnknown -> "Choose an available profile."
    ApiErrorCode.SessionNameInvalid -> "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number."
    ApiErrorCode.SpaceInvalid -> SPACE_INVALID
    ApiErrorCode.ObjectiveInvalid -> "Use 1–240 characters without terminal controls."
    ApiErrorCode.SessionNameConflict -> "A session with that name already exists."
    ApiErrorCode.SessionNotFound -> "That session no longer exists."
    ApiErrorCode.SessionIdentityMismatch -> "The session changed. Refresh and try again."
    ApiErrorCode.PairingInviteRejected -> "This fleet invite is invalid, expired, or already used."
    ApiErrorCode.MachineIdentityMismatch -> "The machine identity changed. Fleet reset is required."
    ApiErrorCode.InternalError -> "Skíðblaðnir could not complete the request."
    ApiErrorCode.ReconnectRequired -> "Reconnect required."
    ApiErrorCode.TerminalConfigurationUnsupported ->
        "tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on."
    ApiErrorCode.AgentTargetStale -> "The agent changed. Refresh and try again."
    ApiErrorCode.AgentUnavailable -> "That agent method is unavailable."
    ApiErrorCode.AgentBlocked -> "Inspect the terminal and send a deliberate reply."
    ApiErrorCode.AgentInputInvalid -> "The agent input is not valid."
}

internal fun parseApiErrorCode(value: String): ApiErrorCode =
    ApiErrorCode.entries.singleOrNull { it.wireName == value } ?: throw SerializationException("unknown API error code")

internal data class SessionStatusContent(val label: String, val accessibilityLabel: String)

internal fun sessionStatusContent(status: AgentStatus?, fresh: Boolean): SessionStatusContent {
    val state = status?.state?.name?.uppercase() ?: "TERMINAL"
    val inferred = status?.source == AgentMethod.Terminal
    val label = state + if (inferred) " · inferred" else ""
    val spoken = (if (fresh) "" else "Last observed: ") + label.lowercase()
    return SessionStatusContent(label, spoken)
}

private fun JsonObject.requireSessionOptionalFields() {
    if ("space" in this) requiredString("space")
    requireAbsentOrNonNull(setOf("launchProfile", "objective", "space", "cwd", "activeCommand", "agent"))
    (this["agent"] as? JsonObject)?.let { agent ->
        agent.requireAbsentOrNonNull(setOf("profile", "providerSession"))
        (agent["status"] as? JsonObject)?.requireAbsentOrNonNull(setOf("reason"))
        (agent["providerSession"] as? JsonObject)?.requireAbsentOrNonNull(setOf("id", "name"))
    }
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
    tmuxName = session.tmuxName,
    identityToken = session.identityToken,
    character = session.character,
    launchProfile = session.launchProfile?.let { requireNotNull(ProfileKey.parse(it)) },
    objective = session.objective,
    space = session.space?.let { requireNotNull(SpaceLabel.parse(it)) },
    cwd = session.cwd,
    activeCommand = session.activeCommand,
    attachedClients = session.attachedClients,
    agent = session.agent?.let(::acceptAgentRuntime),
).also(::acceptSession)

private fun acceptAgentRuntime(runtime: WireAgentRuntime): AgentRuntime = AgentRuntime(
    provider = runtime.provider,
    pid = runtime.pid,
    paneId = runtime.paneId,
    startIdentity = runtime.startIdentity,
    status = runtime.status,
    methods = runtime.methods,
    profile = runtime.profile?.let { requireNotNull(ProfileKey.parse(it)) },
    providerSession = runtime.providerSession?.let(::acceptProviderSessionFacts),
)

private fun acceptProviderSessionFacts(facts: WireProviderSessionFacts): ProviderSessionFacts = when {
    facts.id != null -> ProviderSessionFacts.withId(facts.id, facts.name)
    facts.name != null -> ProviderSessionFacts.withName(facts.name)
    else -> throw IllegalArgumentException("provider session facts are empty")
}

private fun acceptSession(session: TmuxSession) {
    require(session.tmuxId.isNotEmpty() && session.tmuxName.isNotEmpty() && session.identityToken.isNotEmpty())
    require(session.attachedClients >= 0)
    require(session.cwd?.let(WorkingDirectoryPath::parse) != null || session.cwd == null)
    require(session.activeCommand?.isNotEmpty() != false)
    require(session.objective?.isNotEmpty() != false)
    require(session.character.key.isNotEmpty() && session.character.displayName.isNotEmpty())
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
