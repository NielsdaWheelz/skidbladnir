package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

private const val RENAME_OUTCOME_UNKNOWN = "name change outcome unknown. checking tmux."
internal const val RENAME_STALE_EDIT = "the session name changed. review and save again."
private const val GROUP_OUTCOME_UNKNOWN = "group outcome unknown. review the current membership before another attempt."
private val tmuxNamePattern = Regex("[A-Za-z0-9][A-Za-z0-9_-]{0,63}")
private fun isValidTmuxName(candidate: String): Boolean = tmuxNamePattern.matches(candidate)

/** What an edit changes: a rename carries its CAS naming; a group change carries nothing because the last write wins. */
internal sealed interface EditKind {
    data class Rename(val expected: SessionNaming, val submitted: SessionNaming? = null) : EditKind
    data object Group : EditKind
}

internal sealed interface EditPhase {
    data object Editing : EditPhase
    data object Sending : EditPhase
    /** Awaiting an ordered read. acknowledged: the host reported success. hidden: a rename sheet dismissed meanwhile. */
    data class Checking(val acknowledged: Boolean, val hidden: Boolean = false) : EditPhase
}

/** One edit of a session's name or group: Editing → Sending → Checking → closed, or back to Editing. */
internal data class SessionEdit(
    val kind: EditKind,
    val target: SessionTarget,
    val draft: String,
    val phase: EditPhase = EditPhase.Editing,
    val error: String? = null,
    /** The host's GroupInvalid message for the current group draft; shown in the field, never in [error]'s slot. */
    val labelRejection: String? = null,
)

internal fun beginRename(target: SessionTarget) =
    SessionEdit(EditKind.Rename(target.session.naming()), target, target.session.tmuxName)

internal fun beginGroupEdit(target: SessionTarget) =
    SessionEdit(EditKind.Group, target, target.session.group?.text.orEmpty())

/** The naming a rename was opened against: the CAS guard of its request. */
internal val SessionEdit.expectedNaming: SessionNaming get() = (kind as EditKind.Rename).expected

/** The rename in flight, submitting [naming]. */
internal fun SessionEdit.sendingRename(naming: SessionNaming): SessionEdit =
    copy(kind = EditKind.Rename(expectedNaming, naming), phase = EditPhase.Sending, error = null)

internal fun SessionEdit.withDraft(draft: String): SessionEdit =
    if (phase == EditPhase.Editing) copy(draft = draft, error = null, labelRejection = null) else this

/** Null closes. A sending edit refuses. A checking rename hides until its read lands. */
internal fun SessionEdit.dismissed(): SessionEdit? = when {
    phase == EditPhase.Sending -> this
    phase is EditPhase.Checking && kind is EditKind.Rename -> copy(phase = phase.copy(hidden = true))
    else -> null
}

/** needsRead: keep the mutation fence and await an ordered read; otherwise the host proved nothing was written. */
internal class EditReply(val edit: SessionEdit, val needsRead: Boolean)

/** Classifies a reply; the controller has already taken the access codes. A rename ignores dispatch; a group error is definite only when not sent. */
internal fun SessionEdit.afterReply(result: GatewayResult<Unit>): EditReply {
    val failure = when (result) {
        is GatewayResult.Success ->
            return EditReply(copy(phase = EditPhase.Checking(acknowledged = true), error = null), needsRead = true)
        is GatewayResult.Failure -> result.failure
    }
    val api = failure as? GatewayFailure.Api
    val message = gatewayFailureMessage(failure)
    return when (kind) {
        is EditKind.Rename -> when (api?.code) {
            ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge, ApiErrorCode.SessionNameInvalid ->
                EditReply(copy(phase = EditPhase.Editing, error = message), needsRead = false)
            ApiErrorCode.SessionNameConflict, ApiErrorCode.SessionNameChanged ->
                EditReply(copy(phase = EditPhase.Editing, error = message), needsRead = true)
            null, ApiErrorCode.InternalError ->
                EditReply(copy(phase = EditPhase.Checking(acknowledged = false), error = RENAME_OUTCOME_UNKNOWN), needsRead = true)
            else -> EditReply(copy(phase = EditPhase.Checking(acknowledged = false), error = message), needsRead = true)
        }
        EditKind.Group -> {
            val notSent = api?.dispatch == MutationDispatch.NotSent
            when {
                notSent && api.code == ApiErrorCode.GroupInvalid ->
                    EditReply(copy(phase = EditPhase.Editing, labelRejection = message), needsRead = false)
                notSent && api.code in setOf(ApiErrorCode.InvalidRequest, ApiErrorCode.RequestTooLarge) ->
                    EditReply(copy(phase = EditPhase.Editing, error = message), needsRead = false)
                else -> EditReply(copy(phase = EditPhase.Checking(acknowledged = false),
                    error = if (notSent) message else GROUP_OUTCOME_UNKNOWN), needsRead = true)
            }
        }
    }
}

/** Applies a fence-satisfying fresh read of the target; null closes. A sending edit waits for its reply. */
internal fun SessionEdit.afterRead(observed: TmuxSession?): SessionEdit? {
    if (phase == EditPhase.Sending) return this
    observed ?: return null
    val edit = copy(target = target.copy(session = observed))
    return when (kind) {
        EditKind.Group -> if (phase is EditPhase.Checking && phase.acknowledged) null else edit.copy(phase = EditPhase.Editing)
        is EditKind.Rename -> {
            val naming = observed.naming()
            when {
                phase is EditPhase.Checking && (naming == kind.submitted || phase.hidden) -> null
                phase is EditPhase.Checking || naming != kind.expected ->
                    edit.copy(kind = EditKind.Rename(naming), phase = EditPhase.Editing, error = RENAME_STALE_EDIT)
                else -> edit
            }
        }
    }
}

/** Whether the terminal's rename may submit naming now. */
internal fun SessionEdit.renameAdmits(naming: SessionNaming, terminalTarget: SessionTarget, actionsAdmissible: Boolean): Boolean =
    phase == EditPhase.Editing && actionsAdmissible && target.sameLifetime(terminalTarget) && when (naming) {
        SessionNaming.Automatic -> expectedNaming is SessionNaming.Manual
        is SessionNaming.Manual -> isValidTmuxName(naming.name) &&
            (expectedNaming == SessionNaming.Automatic || naming.name != target.session.tmuxName)
    }

/** Whether the group edit may submit its draft to [machine] now. */
internal fun SessionEdit.groupAdmits(machine: MachineState): Boolean {
    if (phase != EditPhase.Editing || !machine.canMutate || machine.machine.handle != target.machineHandle) return false
    val current = machine.inventory.observedSession(target.session) ?: return false
    return groupLabelOf(draft) != current.group
}

@Composable
internal fun GroupField(
    draft: GroupDraft,
    labels: List<GroupLabel>,
    rejection: String?,
    enabled: Boolean,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    maxSuggestionsHeight: Dp,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val text = when (draft) {
        GroupDraft.Unresolved -> ""
        is GroupDraft.Chosen -> draft.text
    }
    val observed = groupCandidates(text, labels)
    SuggestionField(
        text = text,
        label = "group",
        textStyle = MaterialTheme.typography.bodyLarge.copy(fontFamily = NidavellirType.Data,
            textDirection = TextDirection.Content),
        enabled = enabled,
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        onChange = onChange,
        literal = if (draft is GroupDraft.Chosen) text else null,
        // Accepting the unchanged draft writes nothing, so the host's rejection stays until a real edit.
        onAccept = { if (GroupDraft.Chosen(it) != draft) onChange(it); true },
        onNext = {},
        onFocus = {},
        maxSuggestionsHeight = maxSuggestionsHeight,
        message = rejection ?: if (draft == GroupDraft.Unresolved) "choose a group for this new session" else null,
        invalid = rejection != null,
        modifier = modifier,
    ) { choose ->
        if (text.isEmpty()) {
            item(key = "unassigned") { SuggestionTextChoice("unassigned", { choose("") }) }
        }
        item(key = "heading") { Text("observed groups", color = Muted, modifier = Modifier.padding(horizontal = 12.dp)) }
        if (observed.isEmpty()) {
            item(key = "empty") { Text("no observed matches", color = Muted, modifier = Modifier.padding(horizontal = 12.dp)) }
        }
        // Saveable viewport keys contain no labels; acceptance uses the exact domain value.
        itemsIndexed(observed, key = { ordinal, _ -> "observed:$ordinal" }) { _, label ->
            SuggestionTextChoice(if (label.text == "unassigned") "group: unassigned" else label.text, { choose(label.text) })
        }
        if (text.isNotEmpty() && GroupLabel(text) !in labels) {
            item(key = "literal") { SuggestionTextChoice("use label: $text", { choose(text) }) }
        }
        if (text.isNotEmpty()) {
            item(key = "unassigned") { SuggestionTextChoice("unassigned", { choose("") }) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun GroupSheet(edit: SessionEdit, machine: MachineState, labels: List<GroupLabel>, controller: SkidbladnirController) {
    val sending = edit.phase == EditPhase.Sending
    var expanded by remember { mutableStateOf(false) }
    ModalBottomSheet(
        onDismissRequest = controller::dismissGroupEditor,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        shape = NidavellirShapes.Sheet,
        sheetGesturesEnabled = !sending,
        properties = ModalBottomSheetProperties(shouldDismissOnBackPress = !sending, shouldDismissOnClickOutside = !sending),
    ) {
        SheetForm(
            actions = {
                OutlinedButton(onClick = controller::dismissGroupEditor, enabled = !sending, modifier = Modifier.padding(end = 8.dp)) {
                    Text("cancel")
                }
                Button(onClick = controller::submitGroup, enabled = edit.groupAdmits(machine)) { Text("save") }
            },
        ) { suggestionBudget ->
            Text("change group", style = MaterialTheme.typography.headlineSmall, fontFamily = NidavellirType.Display)
            Text("${edit.target.session.tmuxName} on ${machine.machine.label.text}", fontFamily = NidavellirType.Data)
            Text("current: ${edit.target.session.group?.text ?: "unassigned"}", color = Muted)
            GroupField(
                draft = GroupDraft.Chosen(edit.draft),
                labels = labels,
                rejection = edit.labelRejection,
                enabled = edit.phase == EditPhase.Editing,
                expanded = expanded,
                onExpandedChange = { expanded = it },
                maxSuggestionsHeight = suggestionBudget,
                onChange = controller::updateGroupDraft,
            )
            if (sending) Text("changing group", color = Gold)
            if (edit.phase is EditPhase.Checking) Text("checking current membership", color = Gold)
            edit.error?.let { Text(it, color = Ember) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun RenameSheet(edit: SessionEdit, terminal: SkidbladnirUiState.Terminal, controller: SkidbladnirController) {
    val phase = edit.phase
    if (phase is EditPhase.Checking && phase.hidden) return
    val fieldsEnabled = phase == EditPhase.Editing
    val actionsAdmissible = terminalActionAdmissible(terminal.machine.canMutate, terminal.connection)
    val canSubmit = edit.renameAdmits(SessionNaming.Manual(edit.draft), terminal.target, actionsAdmissible)
    val errorMessage = edit.error ?: if (!isValidTmuxName(edit.draft)) {
        apiErrorMessage(ApiErrorCode.SessionNameInvalid)
    } else null
    val focusRequester = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        if (fieldsEnabled) focusRequester.requestFocus()
    }
    ModalBottomSheet(
        onDismissRequest = controller::dismissRename,
        shape = NidavellirShapes.Sheet,
        sheetGesturesEnabled = phase != EditPhase.Sending,
    ) {
        SheetForm {
            SheetTitle("rename session")
            Text(
                text = "${edit.target.session.tmuxName} on ${terminal.machine.machine.label.text}",
                color = Muted,
                fontFamily = NidavellirType.Data,
            )
            OutlinedTextField(
                value = edit.draft,
                onValueChange = controller::updateRenameDraft,
                enabled = fieldsEnabled,
                singleLine = true,
                isError = errorMessage != null,
                label = { Text("session name") },
                keyboardOptions = KeyboardOptions(
                    capitalization = KeyboardCapitalization.None,
                    autoCorrectEnabled = false,
                    keyboardType = KeyboardType.Ascii,
                    imeAction = ImeAction.Done,
                ),
                keyboardActions = KeyboardActions(onDone = { if (canSubmit) controller.submitRename() }),
                modifier = Modifier
                    .fillMaxWidth()
                    .focusRequester(focusRequester),
            )
            Text(
                text = "saving a name stops automatic naming.",
                color = Muted,
                style = MaterialTheme.typography.labelSmall,
            )
            if (edit.expectedNaming == SessionNaming.Automatic) {
                Text("follows the active pane's terminal title.", color = Muted)
            } else {
                OutlinedButton(
                    onClick = controller::useAutomaticTitle,
                    enabled = edit.renameAdmits(SessionNaming.Automatic, terminal.target, actionsAdmissible),
                ) { Text("use automatic title") }
            }
            errorMessage?.let { error ->
                Text(
                    text = error,
                    color = noticeToneColor(NoticeTone.Failure),
                )
            }
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                OutlinedButton(
                    onClick = controller::dismissRename,
                    enabled = phase != EditPhase.Sending,
                ) {
                    Text("Cancel")
                }
                Spacer(Modifier.width(8.dp))
                Button(
                    onClick = controller::submitRename,
                    enabled = canSubmit,
                    shape = NidavellirShapes.Chip,
                ) {
                    if (phase == EditPhase.Sending) {
                        CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        Spacer(Modifier.width(8.dp))
                    }
                    Text("save name")
                }
            }
        }
    }
}
