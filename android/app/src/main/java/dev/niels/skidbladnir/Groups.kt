package dev.niels.skidbladnir

import java.security.MessageDigest
import java.util.Locale
import kotlinx.serialization.Serializable

/** A group label as the host sent it or the owner typed it; the host's group.Parse is the only check. */
@JvmInline @Serializable internal value class GroupLabel(val text: String)

/** Empty is unassigned; any other draft goes to the host as typed. */
internal fun groupLabelOf(draft: String): GroupLabel? = if (draft.isEmpty()) null else GroupLabel(draft)

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

/** The view strip's choices in order: needs input, all, observed groups (and a selected label not observed), an unresolved selected group, then unassigned when selected or present. */
internal fun dashboardViews(machines: List<MachineState>, selected: DashboardViewSelection): List<DashboardViewSelection> {
    val labels = observedGroups(machines).toMutableList()
    val selectedGroup = selected as? DashboardViewSelection.Named
    selectedGroup?.label?.let { if (it !in labels) labels.add(it) }
    return buildList<DashboardViewSelection> {
        add(DashboardViewSelection.NeedsInput)
        add(DashboardViewSelection.All)
        labels.sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }
            .forEach { add(DashboardViewSelection.Named(groupFingerprint(it), it)) }
        if (selectedGroup != null && selectedGroup.label == null && none { it.key == selectedGroup.key }) {
            add(selectedGroup)
        }
        if (selected == DashboardViewSelection.Unassigned || machines.any { machine ->
                machine.inventory.lastSnapshot()?.inventory?.sessions.orEmpty().any { it.group == null }
            }) add(DashboardViewSelection.Unassigned)
    }
}

/** Observed labels ranked against the typed text (exact, ascii-folded equal, prefix, substring, subsequence). */
internal fun groupCandidates(text: String, labels: List<GroupLabel>): List<GroupLabel> {
    if (text.isEmpty()) return labels
    val foldedQuery = foldGroupAscii(text)
    // Stable sorting retains the observed label order for equal ranks.
    return labels.mapNotNull { label ->
        val foldedLabel = foldGroupAscii(label.text)
        val rank = when {
            label.text == text -> 0
            foldedLabel == foldedQuery -> 1
            foldedLabel.startsWith(foldedQuery) -> 2
            foldedLabel.contains(foldedQuery) -> 3
            else -> {
                var queryIndex = 0
                var labelIndex = 0
                while (queryIndex < foldedQuery.length && labelIndex < foldedLabel.length) {
                    val labelScalar = foldedLabel.codePointAt(labelIndex)
                    val queryScalar = foldedQuery.codePointAt(queryIndex)
                    if (labelScalar == queryScalar) queryIndex += Character.charCount(queryScalar)
                    labelIndex += Character.charCount(labelScalar)
                }
                if (queryIndex == foldedQuery.length) 4 else return@mapNotNull null
            }
        }
        rank to label
    }.sortedBy { it.first }.map { it.second }
}

private fun foldGroupAscii(text: String): String = buildString(text.length) {
    text.forEach { append(if (it in 'A'..'Z') it + ('a' - 'A') else it) }
}

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
    return when (view) {
        DashboardViewSelection.NeedsInput -> sessions.filter { it.second != SessionQueueCategory.Excluded }
            .sortedBy { it.second }.map { DashboardItem.Session(it.first) }
        is DashboardViewSelection.Named, DashboardViewSelection.Unassigned ->
            sessions.map { DashboardItem.Session(it.first) }
        DashboardViewSelection.All -> {
            val grouped = sessions.map { it.first }.groupBy { it.target.session.group }
            val labels = grouped.keys.filterNotNull()
                .sortedWith { first, second -> compareCaseInsensitiveUtf8(first.text, second.text) }
            buildList {
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
    }
}
