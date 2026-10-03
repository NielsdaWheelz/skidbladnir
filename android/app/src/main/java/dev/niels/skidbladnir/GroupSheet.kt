package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@Composable
internal fun GroupField(
    draft: GroupDraft,
    labels: List<GroupLabel>,
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
    val canonical = GroupLabel.fromDraft(text)
    val invalid = text.isNotEmpty() && canonical == null
    val candidates = groupCandidates(text, labels)
    SuggestionField(
        text = text,
        label = "group",
        textStyle = MaterialTheme.typography.bodyLarge.copy(fontFamily = NidavellirType.Data,
            textDirection = TextDirection.Content),
        enabled = enabled,
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        onChange = onChange,
        literal = if (draft is GroupDraft.Chosen && !invalid) canonical?.text.orEmpty() else null,
        onAccept = { onChange(it); true },
        onNext = {},
        onFocus = {},
        maxSuggestionsHeight = maxSuggestionsHeight,
        message = when {
            invalid -> GROUP_INVALID
            draft == GroupDraft.Unresolved -> "choose a group for this new session"
            else -> null
        },
        invalid = invalid,
        modifier = modifier,
    ) { choose ->
        if (text.isEmpty()) {
            item(key = "unassigned") { SuggestionTextChoice("unassigned", "unassigned", { choose("") }) }
        }
        item(key = "heading") { Text("observed groups", color = Muted, modifier = Modifier.padding(horizontal = 12.dp)) }
        if (candidates.observed.isEmpty()) {
            item(key = "empty") { Text("no observed matches", color = Muted, modifier = Modifier.padding(horizontal = 12.dp)) }
        }
        // Saveable viewport keys contain no labels; acceptance uses the exact domain value.
        itemsIndexed(candidates.observed, key = { ordinal, _ -> "observed:$ordinal" }) { _, label ->
            val description = if (label.text == "unassigned") "group: unassigned" else label.text
            SuggestionTextChoice(bidiIsolate(description), description, { choose(label.text) })
        }
        candidates.literal?.let { label ->
            item(key = "literal") {
                SuggestionTextChoice("use label: ${bidiIsolate(label.text)}", "use label: ${label.text}", { choose(label.text) })
            }
        }
        if (text.isNotEmpty()) {
            item(key = "unassigned") { SuggestionTextChoice("unassigned", "unassigned", { choose("") }) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun GroupSheet(
    editor: GroupEditor,
    machine: MachineState,
    labels: List<GroupLabel>,
    onChange: (String) -> Unit,
    onDismiss: () -> Unit,
    onSubmit: () -> Unit,
) {
    val sending = editor.phase == GroupPhase.Sending
    var expanded by remember { mutableStateOf(false) }
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
        sheetGesturesEnabled = !sending,
        properties = ModalBottomSheetProperties(shouldDismissOnBackPress = !sending, shouldDismissOnClickOutside = !sending),
    ) {
        Column(
            modifier = Modifier.fillMaxWidth().imePadding()
                .padding(horizontal = 20.dp).padding(bottom = 28.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            BoxWithConstraints(Modifier.weight(1f, fill = false)) {
                val maxSuggestionsHeight = maxHeight / 3
                Column(
                    modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Text("change group", style = MaterialTheme.typography.headlineSmall, fontFamily = NidavellirType.Display)
                    Text("${editor.target.session.tmuxName} on ${machine.machine.label.text}", fontFamily = NidavellirType.Data)
                    Text("current: ${editor.target.session.group?.text ?: "unassigned"}", color = Muted)
                    GroupField(
                        draft = GroupDraft.Chosen(editor.draft),
                        labels = labels,
                        enabled = editor.phase == GroupPhase.Editing,
                        expanded = expanded,
                        onExpandedChange = { expanded = it },
                        maxSuggestionsHeight = maxSuggestionsHeight,
                        onChange = onChange,
                    )
                    if (sending) Text("changing group", color = Gold)
                    if (editor.phase is GroupPhase.Checking) Text("checking current membership", color = Gold)
                    editor.error?.let { Text(it, color = Ember) }
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                OutlinedButton(onClick = onDismiss, enabled = !sending, modifier = Modifier.padding(end = 8.dp)) {
                    Text("cancel")
                }
                Button(onClick = onSubmit, enabled = groupSubmissionAdmissible(editor, machine)) { Text("save") }
            }
        }
    }
}
