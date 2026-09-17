package dev.niels.skidbladnir

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.Surface
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.Alignment
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

@Composable
internal fun SpaceTextAction(
    label: String,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    description: String = label,
    color: Color = Gold,
) {
    Surface(color = Color.Transparent, shape = NidavellirShapes.Chip,
        modifier = modifier.clickable(
            interactionSource = remember { MutableInteractionSource() },
            indication = AngularIndication(NidavellirShapes.Chip),
            enabled = enabled, role = Role.Button, onClick = onClick,
        ).semantics { contentDescription = description }) {
        Box(Modifier.minimumInteractiveComponentSize().padding(horizontal = 12.dp), contentAlignment = Alignment.CenterStart) {
            Text(label, color = if (enabled) color else Muted, fontFamily = NidavellirType.Data,
                maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
internal fun SpaceField(
    draft: SpaceDraft,
    labels: List<SpaceLabel>,
    enabled: Boolean,
    onChange: (String) -> Unit,
) {
    val text = when (draft) {
        SpaceDraft.Unresolved -> ""
        is SpaceDraft.Chosen -> draft.text
    }
    val invalid = text.isNotEmpty() && SpaceLabel.fromDraft(text) == null
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (draft == SpaceDraft.Unresolved) Text("choose a space for this new session", color = Gold)
        OutlinedTextField(
            value = text,
            onValueChange = onChange,
            enabled = enabled,
            singleLine = true,
            label = { Text("space") },
            isError = invalid,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, autoCorrectEnabled = false),
            modifier = Modifier.fillMaxWidth(),
        )
        if (invalid) Text(SPACE_INVALID, color = Ember)
        SpaceTextAction("unassigned", enabled, { onChange("") })
        if (labels.isNotEmpty()) {
            var expanded by remember { mutableStateOf(false) }
            Box {
                SpaceTextAction("observed spaces", enabled, { expanded = true })
                DropdownMenu(expanded = expanded && enabled, onDismissRequest = { expanded = false },
                    shape = NidavellirShapes.Card, containerColor = DeepSurface,
                    tonalElevation = 0.dp, shadowElevation = 0.dp) {
                    labels.forEach { label ->
                        SpaceTextAction("space: ${label.text}", enabled,
                            { onChange(label.text); expanded = false }, Modifier.fillMaxWidth())
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SpaceSheet(
    editor: SpaceEditor,
    machine: MachineState,
    labels: List<SpaceLabel>,
    onChange: (String) -> Unit,
    onDismiss: () -> Unit,
    onSubmit: () -> Unit,
) {
    val sending = editor.phase == SpacePhase.Sending
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet,
        containerColor = DeepSurface,
        sheetGesturesEnabled = !sending,
        properties = ModalBottomSheetProperties(shouldDismissOnBackPress = !sending, shouldDismissOnClickOutside = !sending),
    ) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding()
            .padding(horizontal = 20.dp).padding(bottom = 28.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("space", style = MaterialTheme.typography.headlineSmall, fontFamily = NidavellirType.Display)
            Text("${editor.target.session.tmuxName} on ${machine.machine.label.text}", fontFamily = NidavellirType.Data)
            Text("current: ${editor.target.session.space?.let { "space: ${it.text}" } ?: "unassigned"}", color = Muted)
            SpaceField(SpaceDraft.Chosen(editor.draft), labels, editor.phase == SpacePhase.Editing, onChange)
            if (sending) Text("assigning space", color = Gold)
            if (editor.phase is SpacePhase.Checking) Text("checking current membership", color = Gold)
            editor.error?.let { Text(it, color = Ember) }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                OutlinedButton(onClick = onDismiss, enabled = !sending, modifier = Modifier.padding(end = 8.dp)) {
                    Text("cancel")
                }
                Button(onClick = onSubmit, enabled = spaceSubmissionAdmissible(editor, machine)) { Text("save") }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun SpaceSelector(
    selected: DashboardSpaceSelection,
    labels: List<SpaceLabel>,
    onSelect: (DashboardSpaceSelection) -> Unit,
) {
    var expanded by remember { mutableStateOf(false) }
    SpaceTextAction(selected.displayLabel(), true, { expanded = true },
        modifier = Modifier.fillMaxWidth())
    if (!expanded) return
    ModalBottomSheet(onDismissRequest = { expanded = false }, shape = NidavellirShapes.Sheet, containerColor = DeepSurface) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp).padding(bottom = 28.dp)) {
            Text("spaces", modifier = Modifier.semantics { heading() }, style = MaterialTheme.typography.titleLarge)
            val choices = listOf(DashboardSpaceSelection.All, DashboardSpaceSelection.Unassigned) +
                labels.map { DashboardSpaceSelection.Named(spaceFingerprint(it), it) }
            choices.forEach { choice ->
                if (choice is DashboardSpaceSelection.Named && choice == choices.getOrNull(2)) {
                    Text("observed spaces", color = Muted, style = MaterialTheme.typography.labelSmall)
                }
                SpaceTextAction(choice.displayLabel(), true, { onSelect(choice); expanded = false },
                    modifier = Modifier.fillMaxWidth().semantics { this.selected = choice.key == selected.key },
                    color = if (choice.key == selected.key) Gold else Bone,
                )
            }
        }
    }
}
