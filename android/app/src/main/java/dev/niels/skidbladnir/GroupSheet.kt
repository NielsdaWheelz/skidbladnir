package dev.niels.skidbladnir

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@Composable
internal fun GroupTextAction(
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

@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun GroupField(
    draft: GroupDraft,
    labels: List<GroupLabel>,
    enabled: Boolean,
    maxSuggestionsHeight: Dp,
    onChange: (String) -> Unit,
) {
    val text = when (draft) {
        GroupDraft.Unresolved -> ""
        is GroupDraft.Chosen -> draft.text
    }
    val canonical = GroupLabel.fromDraft(text)
    val invalid = text.isNotEmpty() && canonical == null
    val candidates = groupCandidates(text, labels)
    var expanded by remember { mutableStateOf(false) }
    val suggestions = remember { LazyListState() }
    val focus = LocalFocusManager.current
    val keyboard = LocalSoftwareKeyboardController.current
    val imeVisible = WindowInsets.isImeVisible
    LaunchedEffect(text, expanded) {
        if (expanded) suggestions.scrollToItem(0)
    }
    if (expanded && enabled) {
        ModalBackHandler {
            if (imeVisible) {
                focus.clearFocus()
                keyboard?.hide()
            } else {
                expanded = false
                focus.clearFocus()
            }
        }
    }
    val choose: (String) -> Unit = { chosen ->
        onChange(chosen)
        expanded = false
        focus.clearFocus()
        keyboard?.hide()
    }
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (draft == GroupDraft.Unresolved) Text("choose a group for this new session", color = Gold)
        OutlinedTextField(
            value = text,
            onValueChange = {
                onChange(it)
                expanded = true
            },
            enabled = enabled,
            singleLine = true,
            label = { Text("group") },
            isError = invalid,
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
                imeAction = ImeAction.Next,
            ),
            keyboardActions = KeyboardActions(onNext = {
                if (!invalid && draft is GroupDraft.Chosen) choose(canonical?.text.orEmpty())
            }),
            modifier = Modifier.fillMaxWidth().onFocusChanged { if (it.isFocused) expanded = true },
        )
        if (invalid) Text(GROUP_INVALID, color = Ember)
        GroupTextAction("observed groups", enabled, { expanded = true })
        if (expanded && enabled) {
            LazyColumn(
                modifier = Modifier.fillMaxWidth().weight(1f, fill = false)
                    .heightIn(max = maxSuggestionsHeight.coerceAtMost(240.dp)),
                state = suggestions,
            ) {
                if (text.isEmpty()) {
                    item { GroupTextAction("unassigned", enabled, { choose("") }, Modifier.fillMaxWidth()) }
                }
                if (candidates.observed.isEmpty()) {
                    item { Text("no observed matches", color = Muted, modifier = Modifier.padding(horizontal = 12.dp)) }
                }
                items(candidates.observed, key = { it.text }) { label ->
                    val description = if (label.text == "unassigned") "group: unassigned" else label.text
                    TextButton(
                        onClick = { choose(label.text) },
                        shape = NidavellirShapes.Chip,
                        modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                            .minimumInteractiveComponentSize().semantics { contentDescription = description },
                    ) {
                        Text(bidiIsolate(description), color = Bone, fontFamily = NidavellirType.Data)
                    }
                }
                candidates.literal?.let { label ->
                    item {
                        TextButton(
                            onClick = { choose(label.text) },
                            shape = NidavellirShapes.Chip,
                            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                                .minimumInteractiveComponentSize().semantics {
                                    contentDescription = "use label: ${label.text}"
                                },
                        ) {
                            Text("use label: ${bidiIsolate(label.text)}", color = Gold, fontFamily = NidavellirType.Data)
                        }
                    }
                }
                if (text.isNotEmpty()) {
                    item { GroupTextAction("unassigned", enabled, { choose("") }, Modifier.fillMaxWidth()) }
                }
            }
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
                    modifier = Modifier.fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    Column(
                        modifier = Modifier.fillMaxWidth().weight(1f, fill = false)
                            .verticalScroll(rememberScrollState()),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        Text("change group", style = MaterialTheme.typography.headlineSmall, fontFamily = NidavellirType.Display)
                        Text("${editor.target.session.tmuxName} on ${machine.machine.label.text}", fontFamily = NidavellirType.Data)
                        Text("current: ${editor.target.session.group?.text ?: "unassigned"}", color = Muted)
                    }
                    GroupField(
                        draft = GroupDraft.Chosen(editor.draft),
                        labels = labels,
                        enabled = editor.phase == GroupPhase.Editing,
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

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun GroupSelector(
    selected: DashboardGroupSelection,
    labels: List<GroupLabel>,
    onSelect: (DashboardGroupSelection) -> Unit,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    GroupTextAction(selected.displayLabel(), true, { expanded = true }, modifier = modifier)
    if (!expanded) return
    ModalBottomSheet(onDismissRequest = { expanded = false }, shape = NidavellirShapes.Sheet, containerColor = DeepSurface) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp).padding(bottom = 28.dp)) {
            Text("groups", modifier = Modifier.semantics { heading() }, style = MaterialTheme.typography.titleLarge)
            val choices = listOf(DashboardGroupSelection.All, DashboardGroupSelection.Unassigned) +
                labels.map { DashboardGroupSelection.Named(groupFingerprint(it), it) }
            choices.forEach { choice ->
                if (choice is DashboardGroupSelection.Named && choice == choices.getOrNull(2)) {
                    Text("observed groups", color = Muted, style = MaterialTheme.typography.labelSmall)
                }
                GroupTextAction(choice.displayLabel(), true, { onSelect(choice); expanded = false },
                    modifier = Modifier.fillMaxWidth().semantics { this.selected = choice.key == selected.key },
                    color = if (choice.key == selected.key) Gold else Bone,
                )
            }
        }
    }
}
