package dev.niels.skidbladnir

import java.security.MessageDigest
import java.util.Locale
import android.icu.text.Normalizer2
import android.icu.text.UnicodeSet

internal const val GROUP_INVALID = "use 1–64 nfc characters; only interior ordinary spaces, without display controls."
internal const val GROUP_OUTCOME_UNKNOWN = "group outcome unknown. review the current membership before another attempt."

internal class GroupLabel private constructor(val text: String) {
    companion object {
        private val normalizer = Normalizer2.getNFCInstance()
        private val repertoire = UnicodeSet("[:age=15.0:]").freeze()

        // Later-assigned scalars remain inert boundaries, matching the host's Unicode 15 tables.
        private fun normalize(candidate: String): String = buildString {
            var start = 0
            while (start < candidate.length) {
                val knownEnd = repertoire.span(candidate, start, UnicodeSet.SpanCondition.CONTAINED)
                append(normalizer.normalize(candidate.subSequence(start, knownEnd)))
                val end = repertoire.span(candidate, knownEnd, UnicodeSet.SpanCondition.NOT_CONTAINED)
                append(candidate, knownEnd, end)
                start = end
            }
        }

        fun parse(candidate: String): GroupLabel? {
            if (candidate.isEmpty() || candidate.codePointCount(0, candidate.length) > 64 ||
                candidate.utf8ByteCountWithin(256) == null ||
                normalize(candidate) != candidate ||
                candidate.hasDisplayUnsafeCodePoint() || candidate.first() == ' ' || candidate.last() == ' ' ||
                candidate.codePoints().anyMatch {
                    it == 0x00a0 || it == 0x1680 || it in 0x2000..0x200a ||
                        it == 0x202f || it == 0x205f || it == 0x3000
                }
            ) return null
            return GroupLabel(candidate)
        }

        fun fromDraft(candidate: String): GroupLabel? = parse(normalize(candidate))
    }

    override fun equals(other: Any?): Boolean = other is GroupLabel && text == other.text
    override fun hashCode(): Int = text.hashCode()
}

internal fun groupFingerprint(label: GroupLabel): String {
    val bytes = label.text.encodeToByteArray()
    val digest = MessageDigest.getInstance("SHA-256")
    // The selection fingerprint keeps its original domain across the product rename.
    digest.update("skidbladnir.space-label.v1".encodeToByteArray())
    digest.update(byteArrayOf(
        (bytes.size ushr 24).toByte(), (bytes.size ushr 16).toByte(),
        (bytes.size ushr 8).toByte(), bytes.size.toByte(),
    ))
    return digest.digest(bytes).joinToString("") { "%02x".format(it.toInt() and 0xff) }
}

internal sealed interface DashboardViewKey {
    data object NeedsInput : DashboardViewKey
    data object All : DashboardViewKey
    data object Unassigned : DashboardViewKey
    data class Named(val fingerprint: String) : DashboardViewKey {
        init { require(DashboardCardKey.isFingerprint(fingerprint)) }
    }
}

internal sealed interface DashboardViewSelection {
    val key: DashboardViewKey
    data object NeedsInput : DashboardViewSelection { override val key = DashboardViewKey.NeedsInput }
    data object All : DashboardViewSelection { override val key = DashboardViewKey.All }
    data object Unassigned : DashboardViewSelection { override val key = DashboardViewKey.Unassigned }
    data class Named(val fingerprint: String, val label: GroupLabel? = null) : DashboardViewSelection {
        init {
            require(DashboardCardKey.isFingerprint(fingerprint))
            require(label == null || groupFingerprint(label) == fingerprint)
        }
        override val key = DashboardViewKey.Named(fingerprint)
    }
}

internal fun DashboardViewKey.selection(): DashboardViewSelection = when (this) {
    DashboardViewKey.NeedsInput -> DashboardViewSelection.NeedsInput
    DashboardViewKey.All -> DashboardViewSelection.All
    DashboardViewKey.Unassigned -> DashboardViewSelection.Unassigned
    is DashboardViewKey.Named -> DashboardViewSelection.Named(fingerprint)
}

internal fun DashboardViewSelection.matchesGroup(label: GroupLabel?): Boolean = when (this) {
    DashboardViewSelection.NeedsInput, DashboardViewSelection.All -> true
    DashboardViewSelection.Unassigned -> label == null
    is DashboardViewSelection.Named -> label != null && groupFingerprint(label) == fingerprint
}

internal fun DashboardViewSelection.displayLabel(): String = when (this) {
    DashboardViewSelection.NeedsInput -> "needs input"
    DashboardViewSelection.All -> "all"
    DashboardViewSelection.Unassigned -> "unassigned"
    is DashboardViewSelection.Named -> label?.text ?: "previously selected group"
}

internal sealed interface GroupDraft {
    data object Unresolved : GroupDraft
    data class Chosen(val text: String) : GroupDraft
}

internal fun DashboardViewSelection.creationDraft(): GroupDraft = when (this) {
    DashboardViewSelection.NeedsInput, DashboardViewSelection.All, DashboardViewSelection.Unassigned -> GroupDraft.Chosen("")
    is DashboardViewSelection.Named -> label?.let { GroupDraft.Chosen(it.text) } ?: GroupDraft.Unresolved
}

internal fun observedGroups(machines: List<MachineState>): List<GroupLabel> = machines
    .flatMap { it.inventory.lastSnapshot()?.inventory?.sessions.orEmpty() }
    .mapNotNull(TmuxSession::group).distinct()
    .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }

internal sealed interface DashboardItem {
    val key: DashboardItemKey
    data class Heading(val label: GroupLabel?) : DashboardItem {
        override val key = label?.let { DashboardItemKey.Group(groupFingerprint(it)) }
            ?: DashboardItemKey.Unassigned
    }
    data class Session(val visible: VisibleSession) : DashboardItem { override val key = visible.cardKey }
}

/** One projection owns queue priority, group order, rendering keys and restored anchors. */
internal fun dashboardItems(
    machines: List<MachineState>,
    view: DashboardViewSelection,
): List<DashboardItem> {
    val sessions = machines.flatMap { state ->
        state.inventory.lastSnapshot()?.inventory?.sessions.orEmpty()
            .filter { view.matchesGroup(it.group) }
            .map { session ->
                val target = SessionTarget(state.machine.handle, session)
                val notification = state.notifications[NotificationKey(target)] ?: NotificationPresentation()
                VisibleSession(state.machine, target, state.executionContext(session)) to
                    sessionStatusContent(session, state.canMutate, notification).queueCategory
            }
    }.sortedWith(compareBy<Pair<VisibleSession, SessionQueueCategory>> { it.first.machine.label.text.lowercase(Locale.ROOT) }
        .thenBy { it.first.machine.label.text }
        .thenBy { it.first.machine.handle.encoded }
        .thenBy { it.first.target.session.tmuxId.substring(1).toBigInteger() })
    if (view == DashboardViewSelection.NeedsInput) {
        return sessions.filter { it.second != SessionQueueCategory.Excluded }
            .sortedBy { it.second }.map { DashboardItem.Session(it.first) }
    }
    val grouped = sessions.map { it.first }.groupBy { it.target.session.group }
    val labels = grouped.keys.filterNotNull()
        .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }
    return buildList {
        labels.forEach { label ->
            add(DashboardItem.Heading(label))
            grouped.getValue(label).forEach { add(DashboardItem.Session(it)) }
        }
        grouped[null]?.let { unassigned ->
            add(DashboardItem.Heading(null))
            unassigned.forEach { add(DashboardItem.Session(it)) }
        }
    }
}

internal sealed interface GroupPhase {
    data object Editing : GroupPhase
    data object Sending : GroupPhase
    data class Checking(val acknowledged: Boolean) : GroupPhase
}

internal data class GroupEditor(
    val target: SessionTarget,
    val draft: String,
    val phase: GroupPhase = GroupPhase.Editing,
    val error: String? = null,
)

internal fun sameSessionLifetime(first: SessionTarget, second: SessionTarget): Boolean =
    first.machineHandle == second.machineHandle && first.session.tmuxId == second.session.tmuxId &&
        first.session.identityToken == second.session.identityToken

internal fun groupSubmissionAdmissible(editor: GroupEditor, machine: MachineState): Boolean {
    if (editor.phase != GroupPhase.Editing || !machine.canMutate ||
        machine.machine.handle != editor.target.machineHandle) return false
    val current = machine.inventory.lastSnapshot()?.inventory?.sessions?.singleOrNull {
        it.tmuxId == editor.target.session.tmuxId && it.identityToken == editor.target.session.identityToken
    } ?: return false
    val label = if (editor.draft.isEmpty()) null else GroupLabel.fromDraft(editor.draft) ?: return false
    return label != current.group
}

internal fun completeGroupHttp(editor: GroupEditor, result: GatewayResult<Unit>): GroupEditor = when (result) {
    is GatewayResult.Success -> editor.copy(phase = GroupPhase.Checking(acknowledged = true), error = null)
    is GatewayResult.Failure -> {
        val failure = result.failure
        val notSent = failure is GatewayFailure.Api && failure.dispatch == MutationDispatch.NotSent
        val validationRejected = notSent && failure.code in setOf(
            ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge, ApiErrorCode.GroupInvalid,
        )
        editor.copy(
            phase = if (validationRejected) GroupPhase.Editing else GroupPhase.Checking(acknowledged = false),
            error = if (notSent) gatewayFailureMessage(failure) else GROUP_OUTCOME_UNKNOWN,
        )
    }
}

internal fun reconcileGroupEditor(editor: GroupEditor, machine: MachineState): GroupEditor? {
    if (editor.phase == GroupPhase.Sending) return editor
    val current = machine.inventory.lastSnapshot()?.inventory?.sessions?.singleOrNull {
        it.tmuxId == editor.target.session.tmuxId && it.identityToken == editor.target.session.identityToken
    } ?: return null
    return when (val phase = editor.phase) {
        GroupPhase.Sending -> error("sending is handled before inventory reconciliation")
        GroupPhase.Editing -> editor.copy(target = editor.target.copy(session = current))
        is GroupPhase.Checking -> if (phase.acknowledged) null else editor.copy(
            target = editor.target.copy(session = current), phase = GroupPhase.Editing,
        )
    }
}
