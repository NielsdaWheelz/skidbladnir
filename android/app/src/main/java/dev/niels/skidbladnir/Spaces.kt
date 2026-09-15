package dev.niels.skidbladnir

import java.security.MessageDigest
import android.icu.text.Normalizer2
import android.icu.text.UnicodeSet

internal const val SPACE_INVALID = "use 1–64 nfc characters; only interior ordinary spaces, without display controls."
internal const val SPACE_OUTCOME_UNKNOWN = "space outcome unknown. review the current membership before another attempt."

internal class SpaceLabel private constructor(val text: String) {
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

        fun parse(candidate: String): SpaceLabel? {
            if (candidate.isEmpty() || candidate.codePointCount(0, candidate.length) > 64 ||
                candidate.utf8ByteCountWithin(256) == null ||
                normalize(candidate) != candidate ||
                candidate.hasDisplayUnsafeCodePoint() || candidate.first() == ' ' || candidate.last() == ' ' ||
                candidate.codePoints().anyMatch {
                    it == 0x00a0 || it == 0x1680 || it in 0x2000..0x200a ||
                        it == 0x202f || it == 0x205f || it == 0x3000
                }
            ) return null
            return SpaceLabel(candidate)
        }

        fun fromDraft(candidate: String): SpaceLabel? = parse(normalize(candidate))
    }

    override fun equals(other: Any?): Boolean = other is SpaceLabel && text == other.text
    override fun hashCode(): Int = text.hashCode()
}

internal fun spaceFingerprint(label: SpaceLabel): String {
    val bytes = label.text.encodeToByteArray()
    val digest = MessageDigest.getInstance("SHA-256")
    digest.update("skidbladnir.space-label.v1".encodeToByteArray())
    digest.update(byteArrayOf(
        (bytes.size ushr 24).toByte(), (bytes.size ushr 16).toByte(),
        (bytes.size ushr 8).toByte(), bytes.size.toByte(),
    ))
    return digest.digest(bytes).joinToString("") { "%02x".format(it.toInt() and 0xff) }
}

internal sealed interface DashboardSpaceKey {
    data object All : DashboardSpaceKey
    data object Unassigned : DashboardSpaceKey
    data class Named(val fingerprint: String) : DashboardSpaceKey {
        init { require(DashboardCardKey.isFingerprint(fingerprint)) }
    }
}

internal sealed interface DashboardSpaceSelection {
    val key: DashboardSpaceKey
    data object All : DashboardSpaceSelection { override val key = DashboardSpaceKey.All }
    data object Unassigned : DashboardSpaceSelection { override val key = DashboardSpaceKey.Unassigned }
    data class Named(val fingerprint: String, val label: SpaceLabel? = null) : DashboardSpaceSelection {
        init {
            require(DashboardCardKey.isFingerprint(fingerprint))
            require(label == null || spaceFingerprint(label) == fingerprint)
        }
        override val key = DashboardSpaceKey.Named(fingerprint)
    }
}

internal fun DashboardSpaceKey.selection(): DashboardSpaceSelection = when (this) {
    DashboardSpaceKey.All -> DashboardSpaceSelection.All
    DashboardSpaceKey.Unassigned -> DashboardSpaceSelection.Unassigned
    is DashboardSpaceKey.Named -> DashboardSpaceSelection.Named(fingerprint)
}

internal fun DashboardSpaceSelection.matches(label: SpaceLabel?): Boolean = when (this) {
    DashboardSpaceSelection.All -> true
    DashboardSpaceSelection.Unassigned -> label == null
    is DashboardSpaceSelection.Named -> label != null && spaceFingerprint(label) == fingerprint
}

internal fun DashboardSpaceSelection.displayLabel(): String = when (this) {
    DashboardSpaceSelection.All -> "all spaces"
    DashboardSpaceSelection.Unassigned -> "unassigned"
    is DashboardSpaceSelection.Named -> label?.let { "space: ${it.text}" } ?: "previously selected space"
}

internal sealed interface SpaceDraft {
    data object Unresolved : SpaceDraft
    data class Chosen(val text: String) : SpaceDraft
}

internal fun DashboardSpaceSelection.creationDraft(): SpaceDraft = when (this) {
    DashboardSpaceSelection.All, DashboardSpaceSelection.Unassigned -> SpaceDraft.Chosen("")
    is DashboardSpaceSelection.Named -> label?.let { SpaceDraft.Chosen(it.text) } ?: SpaceDraft.Unresolved
}

internal fun observedSpaces(machines: List<MachineState>): List<SpaceLabel> = machines
    .flatMap { it.inventory.lastSnapshot()?.inventory?.sessions.orEmpty() }
    .mapNotNull(TmuxSession::space).distinct()
    .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }

internal sealed interface DashboardItem {
    val key: DashboardItemKey
    data class Heading(val label: SpaceLabel?) : DashboardItem {
        override val key = label?.let { DashboardItemKey.Space(spaceFingerprint(it)) }
            ?: DashboardItemKey.Unassigned
    }
    data class Session(val visible: VisibleSession) : DashboardItem { override val key = visible.cardKey }
}

internal fun dashboardItems(
    machines: List<MachineState>,
    scope: DashboardScope,
    space: DashboardSpaceSelection,
): List<DashboardItem> {
    val grouped = visibleSessions(machines, scope).filter { space.matches(it.target.session.space) }
        .groupBy { it.target.session.space }
    val labels = grouped.keys.filterNotNull()
        .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }
    return buildList {
        labels.forEach { label ->
            add(DashboardItem.Heading(label))
            grouped.getValue(label).forEach { add(DashboardItem.Session(it)) }
        }
        grouped[null]?.let { sessions ->
            add(DashboardItem.Heading(null))
            sessions.forEach { add(DashboardItem.Session(it)) }
        }
    }
}

internal sealed interface SpacePhase {
    data object Editing : SpacePhase
    data object Sending : SpacePhase
    data class Checking(val acknowledged: Boolean) : SpacePhase
}

internal data class SpaceEditor(
    val target: SessionTarget,
    val draft: String,
    val phase: SpacePhase = SpacePhase.Editing,
    val error: String? = null,
)

internal fun sameSessionLifetime(first: SessionTarget, second: SessionTarget): Boolean =
    first.machineHandle == second.machineHandle && first.session.tmuxId == second.session.tmuxId &&
        first.session.identityToken == second.session.identityToken

internal fun spaceSubmissionAdmissible(editor: SpaceEditor, machine: MachineState): Boolean {
    if (editor.phase != SpacePhase.Editing || !machine.canMutate ||
        machine.machine.handle != editor.target.machineHandle) return false
    val current = machine.inventory.lastSnapshot()?.inventory?.sessions?.singleOrNull {
        it.tmuxId == editor.target.session.tmuxId && it.identityToken == editor.target.session.identityToken
    } ?: return false
    val label = if (editor.draft.isEmpty()) null else SpaceLabel.fromDraft(editor.draft) ?: return false
    return label != current.space
}

internal fun completeSpaceHttp(editor: SpaceEditor, result: GatewayResult<Unit>): SpaceEditor = when (result) {
    is GatewayResult.Success -> editor.copy(phase = SpacePhase.Checking(acknowledged = true), error = null)
    is GatewayResult.Failure -> {
        val failure = result.failure
        val notSent = failure is GatewayFailure.Api && failure.dispatch == MutationDispatch.NotSent
        val validationRejected = notSent && failure.code in setOf(
            ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge, ApiErrorCode.SpaceInvalid,
        )
        editor.copy(
            phase = if (validationRejected) SpacePhase.Editing else SpacePhase.Checking(acknowledged = false),
            error = if (notSent) gatewayFailureMessage(failure) else SPACE_OUTCOME_UNKNOWN,
        )
    }
}

internal fun reconcileSpaceEditor(editor: SpaceEditor, machine: MachineState): SpaceEditor? {
    if (editor.phase == SpacePhase.Sending) return editor
    val current = machine.inventory.lastSnapshot()?.inventory?.sessions?.singleOrNull {
        it.tmuxId == editor.target.session.tmuxId && it.identityToken == editor.target.session.identityToken
    } ?: return null
    return when (val phase = editor.phase) {
        SpacePhase.Sending -> error("sending is handled before inventory reconciliation")
        SpacePhase.Editing -> editor.copy(target = editor.target.copy(session = current))
        is SpacePhase.Checking -> if (phase.acknowledged) null else editor.copy(
            target = editor.target.copy(session = current), phase = SpacePhase.Editing,
        )
    }
}
