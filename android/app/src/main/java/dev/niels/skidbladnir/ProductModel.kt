package dev.niels.skidbladnir

import java.nio.charset.StandardCharsets
import java.security.MessageDigest
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException

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
        if (machineHandle == null || launch == null) return null
        val directory = when (val input = classifyWorkingDirectory(cwd)) {
            WorkingDirectoryInput.Home -> "~"
            is WorkingDirectoryInput.Literal -> input.path.encoded
            is WorkingDirectoryInput.Query, WorkingDirectoryInput.Invalid -> return null
        }
        val chosen = group as? GroupDraft.Chosen ?: return null
        return ForgeDraft(machineHandle, directory, launch, optionalTmuxName, objective, groupLabelOf(chosen.text))
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

/**
 * Single owner of destructive copy: the close confirmation's title and body name the verb and its
 * exact target from this one string, so the dialog cannot name a different session from the one acted on.
 */
internal fun closeActionLabel(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    "${if (terminalOnly) TERMINAL_ONLY_CLOSE_ACTION else TERMINAL_CLOSE_ACTION}: ${target.session.tmuxName} on ${label.text}"
internal fun closeConfirmationTitle(label: MachineLabel, target: SessionTarget, terminalOnly: Boolean = false): String =
    closeActionLabel(label, target, terminalOnly) + "?"

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
 * Single owner of the availability tone. Trust events are its only failures: a broken bearer
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

/** The one word that names a machine's availability beside a card or a Forge machine choice. */
internal fun availabilityWord(availability: MachineAvailability): String? = when (availability) {
    MachineAvailability.Ready -> null
    MachineAvailability.Refreshing -> "REFRESHING"
    MachineAvailability.AuthRequired -> "AUTH REQUIRED"
    MachineAvailability.IdentityChanged -> "IDENTITY CHANGED"
    MachineAvailability.Reading -> "READING"
    is MachineAvailability.Stale -> "STALE"
    is MachineAvailability.Unavailable -> "UNAVAILABLE"
}

/**
 * Single owner of the retained-card availability marker. A card can remain visible while actions
 * are fenced, but the marker must name the actual reason instead of collapsing every fence into
 * staleness. Reading and unavailable inventories have no retained card to annotate.
 */
internal fun sessionAvailabilityContent(machine: MachineState): SessionAvailabilityContent? {
    val availability = machineAvailability(machine)
    if (availability == MachineAvailability.Reading || availability is MachineAvailability.Unavailable) return null
    return availabilityWord(availability)?.let { SessionAvailabilityContent("$it · actions disabled", availabilityTone(availability)) }
}

/**
 * Single owner of availability/pressure prose and severity: one `when` over [MachineAvailability] yields
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

internal fun workspaceRecoveryNotice(machine: MachineState): MachineNotice? {
    if (machine.access != MachineAccess.Ready) return null
    val recovery = (machine.inventory as? InventoryState.Fresh)?.snapshot?.inventory?.recovery ?: return null
    val notice = recovery.notice ?: return null
    val label = machine.machine.label.text
    val tone = if (recovery.state == WorkspaceRecoveryState.Broken) NoticeTone.Failure else NoticeTone.Degraded
    val action = notice.action ?: return MachineNotice("$label: ${notice.summary}", tone)
    // The save time reads in this device's zone, which the host cannot know.
    val saved = recovery.savedAt?.let {
        "last confirmed save: " + DateTimeFormatter.ofPattern("uuuu-MM-dd HH:mm:ss 'utc'xxx", Locale.ROOT)
            .withZone(ZoneId.systemDefault()).format(it)
    } ?: "no confirmed save"
    return MachineNotice("$label: ${notice.summary}. $saved. $action", tone)
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

internal enum class ApiErrorCode {
    Unauthenticated, InvalidRequest, RequestTooLarge,
    WorkingDirectoryInvalid, WorkingDirectoryUnavailable,
    DirectoryListingUnavailable, DirectoryListingTooLarge,
    DirectorySearchUnavailable, DirectorySearchTooLarge,
    TerminalContextUnavailable,
    TerminalTargetChanged, TerminalUnavailable, TerminalInputBlocked,
    ProfileUnknown, SessionNameInvalid, ObjectiveInvalid, GroupInvalid,
    SessionNameConflict, SessionNameChanged, SessionNotFound,
    SessionIdentityMismatch,
    MachineIdentityMismatch, InternalError,
    ReconnectRequired,
    TerminalConfigurationUnsupported,
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
    ApiErrorCode.GroupInvalid -> "use 1–64 nfc characters; only interior ordinary spaces, without display controls."
    ApiErrorCode.ObjectiveInvalid -> "Use 1–240 characters without terminal controls."
    ApiErrorCode.SessionNameConflict -> "another session on this machine uses that name."
    ApiErrorCode.SessionNameChanged -> RENAME_STALE_EDIT
    ApiErrorCode.SessionNotFound -> "That session no longer exists."
    ApiErrorCode.SessionIdentityMismatch -> "The session changed. Refresh and try again."
    ApiErrorCode.MachineIdentityMismatch -> "The machine identity changed. Fleet reset is required."
    ApiErrorCode.InternalError -> "Skíðblaðnir could not complete the request."
    ApiErrorCode.ReconnectRequired -> "Reconnect required."
    ApiErrorCode.TerminalConfigurationUnsupported ->
        "tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on."
}

internal fun parseApiErrorCode(value: String): ApiErrorCode =
    ApiErrorCode.entries.singleOrNull { it.name == value } ?: throw SerializationException("unknown API error code")

@Serializable internal enum class SessionStatusTone {
    @SerialName("working") Working,
    @SerialName("ready") Ready,
    @SerialName("attention") Attention,
    @SerialName("muted") Muted,
}

/**
 * Human-collection membership, in needs-input order. On the wire it is the host's verdict before
 * the phone's overlays, where Ready means only eligible; in [SessionStatusContent] it is final.
 */
@Serializable internal enum class SessionQueueCategory {
    @SerialName("ready") Ready,
    @SerialName("action") Action,
    @SerialName("none") Excluded,
}

// `detail` tells one session's state from another's; `evidence` names how status was
// known. The card shows only the first, since the qualifier is the same on every agent card.
internal data class SessionStatusContent(
    val label: String, val detail: String?, val evidence: String?,
    val tone: SessionStatusTone, val secondary: String?, val queueCategory: SessionQueueCategory,
    val detailExplainsQueue: Boolean,
)

/**
 * Status for the card and the terminal header: the host's [StatusPresentation] under the phone's
 * own knowledge. Staleness prefixes and mutes it, the hub's pending ready turns an eligible idle
 * row `ready`, and notification health adds a secondary line. A plain local terminal shows its
 * running command as detail.
 */
internal fun sessionStatusContent(
    session: TmuxSession, fresh: Boolean, notification: NotificationPresentation, showQueueReason: Boolean = false,
): SessionStatusContent {
    val status = session.status
    val ready = fresh && status.queue == SessionQueueCategory.Ready && notification.ready && !notification.unavailable
    val queueNotice = status.queueNotice?.takeIf { showQueueReason }
    return SessionStatusContent(
        label = if (!fresh) "last observed: ${status.label}" else if (ready) "ready" else status.label,
        detail = listOfNotNull(queueNotice, status.detail).joinToString(" · ").ifEmpty { null }
            ?: session.activeCommand.takeIf { status.evidence == null && session.connection == null },
        evidence = status.evidence,
        tone = if (!fresh) SessionStatusTone.Muted else if (ready) SessionStatusTone.Ready else status.tone,
        secondary = if (notification.unavailable) "notifications unavailable" else null,
        queueCategory = when {
            !fresh -> SessionQueueCategory.Excluded
            status.queue == SessionQueueCategory.Action -> SessionQueueCategory.Action
            ready -> SessionQueueCategory.Ready
            else -> SessionQueueCategory.Excluded
        },
        detailExplainsQueue = queueNotice != null,
    )
}

/** An agent's profile as the owner reads it: the host's configured [label], else its provider. */
internal fun agentLabel(provider: AgentProvider, label: String?): String = label ?: "${provider.name} · profile unknown"
