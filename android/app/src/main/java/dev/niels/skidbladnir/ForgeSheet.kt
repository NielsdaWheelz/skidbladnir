package dev.niels.skidbladnir

import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.ModalBottomSheetProperties
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

internal class ForgeSheetActions(
    val dismiss: () -> Unit,
    val updateDraft: ((ForgeForm) -> ForgeForm) -> Unit,
    val submit: () -> Unit,
    val focusWorkingDirectory: () -> Unit,
    val chooseWorkingDirectory: (WorkingDirectoryPath) -> Boolean,
    val browseWorkingDirectoryHome: () -> Unit,
    val workingDirectory: WorkingDirectoryPickerActions,
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ForgeSheet(
    state: ForgeState,
    machines: List<MachineState>,
    actions: ForgeSheetActions,
) {
    val pickerVisible = state.surface is ForgeSurface.DirectoryPicker
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    var lit by remember { mutableStateOf(false) }
    var focusDirectoryOnReturn by remember { mutableStateOf(false) }
    val containerColor by animateColorAsState(
        targetValue = if (lit) ForgeGlow else DeepSurface,
        animationSpec = NidavellirMotion.ForgeWarmIn,
        label = "forge warm-in",
    )
    LaunchedEffect(Unit) { lit = true }

    ModalBottomSheet(
        onDismissRequest = actions.dismiss,
        modifier = if (pickerVisible) Modifier.fillMaxHeight() else Modifier,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet,
        containerColor = containerColor,
        properties = ModalBottomSheetProperties(
            shouldDismissOnBackPress = !pickerVisible,
            shouldDismissOnClickOutside = true,
        ),
    ) {
        when (val surface = state.surface) {
            ForgeSurface.Form -> ForgeFormContent(state, machines, actions, focusDirectoryOnReturn) {
                focusDirectoryOnReturn = false
            }
            is ForgeSurface.DirectoryPicker -> {
                WorkingDirectoryPickerScreen(
                    picker = surface.picker,
                    actions = actions.workingDirectory,
                    enabled = machines.singleOrNull {
                        it.machine.handle == surface.picker.machine.handle
                    }?.canForge == true,
                    onReturnToField = { focusDirectoryOnReturn = true },
                    modifier = Modifier.fillMaxWidth().fillMaxHeight(),
                )
            }
        }
    }
}

@Composable
internal fun ModalBackHandler(onBack: () -> Unit) {
    val currentOnBack by rememberUpdatedState(onBack)
    val view = LocalView.current
    DisposableEffect(view) {
        val dispatcher = checkNotNull(view.findOnBackInvokedDispatcher()) {
            "modal must be attached to a Back dispatcher"
        }
        val callback = OnBackInvokedCallback { currentOnBack() }
        dispatcher.registerOnBackInvokedCallback(
            OnBackInvokedDispatcher.PRIORITY_OVERLAY,
            callback,
        )
        onDispose { dispatcher.unregisterOnBackInvokedCallback(callback) }
    }
}

private enum class ForgeEntryField { Directory, Group }

@Composable
private fun ForgeFormContent(
    state: ForgeState,
    machines: List<MachineState>,
    actions: ForgeSheetActions,
    requestDirectoryFocus: Boolean,
    onDirectoryFocused: () -> Unit,
) {
    var expandedField by remember { mutableStateOf<ForgeEntryField?>(null) }
    val directoryFocus = remember { FocusRequester() }
    val groupFocus = remember { FocusRequester() }
    val focus = LocalFocusManager.current
    val keyboard = LocalSoftwareKeyboardController.current
    LaunchedEffect(requestDirectoryFocus) {
        if (requestDirectoryFocus) {
            directoryFocus.requestFocus()
            onDirectoryFocused()
        }
    }
    val selected = state.form.machineHandle?.let { handle ->
        machines.singleOrNull { it.machine.handle == handle }
    }
    val inventory = selected?.inventory?.lastSnapshot()?.inventory
    val fieldsEnabled = !state.pending && selected?.canMutate == true

    Column(
        Modifier
            .fillMaxWidth()
            .imePadding()
            .padding(horizontal = 20.dp)
            .padding(bottom = 28.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        BoxWithConstraints(Modifier.weight(1f, fill = false)) {
            val maxSuggestionsHeight = maxHeight / 3
            Column(
                modifier = Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                Text(
                    "Create dwarf",
                    style = MaterialTheme.typography.headlineSmall,
                    fontFamily = NidavellirType.Display,
                    fontWeight = FontWeight.SemiBold,
                )
                Canvas(Modifier.fillMaxWidth().height(12.dp)) {
                    drawFretBand(Gold.copy(alpha = 0.40f))
                }
                Text("Machine", color = Muted, style = MaterialTheme.typography.labelLarge)
                Row(
                    Modifier.horizontalScroll(rememberScrollState()),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    machines.forEach { machine ->
                        FilterChip(
                            selected = machine.machine.handle == state.form.machineHandle,
                            onClick = {
                                actions.updateDraft { it.copy(machineHandle = machine.machine.handle) }
                            },
                            enabled = !state.pending && machine.canMutate,
                            label = {
                                Text(
                                    bidiIsolate(forgeMachineChoiceLabel(machine)),
                                    fontFamily = NidavellirType.Data,
                                )
                            },
                            shape = NidavellirShapes.Chip,
                            modifier = Modifier.semantics {
                                contentDescription = forgeMachineChoiceLabel(machine)
                            },
                        )
                    }
                }
                if (selected == null) {
                    Text(
                        "Choose a machine to choose a working directory and launch.",
                        color = Muted,
                    )
                } else {
                    Text(
                        "Launch on ${bidiIsolate(selected.machine.label.text)}",
                        color = Muted,
                        style = MaterialTheme.typography.labelLarge,
                    )
                    Row(
                        Modifier.horizontalScroll(rememberScrollState()),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        inventory?.profiles.orEmpty().forEach { profile ->
                            FilterChip(
                                selected = state.form.launch == LaunchChoice.Agent(profile.key),
                                onClick = { actions.updateDraft { it.copy(launch = LaunchChoice.Agent(profile.key)) } },
                                enabled = fieldsEnabled,
                                label = { Text(profile.label, fontFamily = NidavellirType.Data) },
                                shape = NidavellirShapes.Chip,
                            )
                        }
                        FilterChip(
                            selected = state.form.launch == LaunchChoice.Terminal,
                            onClick = { actions.updateDraft { it.copy(launch = LaunchChoice.Terminal) } },
                            enabled = fieldsEnabled,
                            label = { Text("Terminal", fontFamily = NidavellirType.Data) },
                            shape = NidavellirShapes.Chip,
                        )
                    }
                    forgeUnavailableCopy(selected)?.let { notice ->
                        Text(
                            bidiIsolate(notice.message),
                            color = noticeToneColor(notice.tone),
                            modifier = Modifier.semantics {
                                contentDescription = notice.message
                            },
                        )
                    }
                }
                OutlinedTextField(
                    value = state.form.optionalTmuxName,
                    onValueChange = { value ->
                        actions.updateDraft { it.copy(optionalTmuxName = value) }
                    },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEnabled,
                    label = { Text("session name (optional)") },
                    supportingText = { Text("leave blank to follow the terminal title.") },
                    singleLine = true,
                    keyboardOptions = KeyboardOptions(
                        capitalization = KeyboardCapitalization.None,
                        autoCorrectEnabled = false,
                    ),
                )
                OutlinedTextField(
                    value = state.form.objective,
                    onValueChange = { value -> actions.updateDraft { it.copy(objective = value) } },
                    modifier = Modifier.fillMaxWidth(),
                    enabled = fieldsEnabled,
                    label = { Text("Objective (optional)") },
                    minLines = 2,
                    maxLines = 4,
                )
                DirectoryField(
                    state = state,
                    machine = selected,
                    enabled = fieldsEnabled,
                    expanded = expandedField == ForgeEntryField.Directory,
                    onExpandedChange = { expanded ->
                        if (expanded) expandedField = ForgeEntryField.Directory
                        else if (expandedField == ForgeEntryField.Directory) expandedField = null
                    },
                    maxSuggestionsHeight = maxSuggestionsHeight,
                    actions = actions,
                    modifier = Modifier.focusRequester(directoryFocus),
                    onNext = { groupFocus.requestFocus() },
                    onBrowse = {
                        focus.clearFocus()
                        keyboard?.hide()
                        actions.browseWorkingDirectoryHome()
                    },
                )
                GroupField(
                    draft = state.form.group,
                    labels = observedGroups(machines),
                    enabled = !state.pending,
                    expanded = expandedField == ForgeEntryField.Group,
                    onExpandedChange = { expanded ->
                        if (expanded) expandedField = ForgeEntryField.Group
                        else if (expandedField == ForgeEntryField.Group) expandedField = null
                    },
                    maxSuggestionsHeight = maxSuggestionsHeight,
                    onChange = { text -> actions.updateDraft { it.copy(group = GroupDraft.Chosen(text)) } },
                    modifier = Modifier.focusRequester(groupFocus),
                )
                when (val failure = state.failure) {
                    ForgeFailure.None -> Unit
                    is ForgeFailure.Definite -> Text(
                        gatewayFailureMessage(failure.rejection),
                        color = noticeToneColor(NoticeTone.Failure),
                    )
                }
            }
        }
        Button(
            onClick = actions.submit,
            enabled = state.admissibleSubmission() != null && selected?.canMutate == true,
            modifier = Modifier.fillMaxWidth().semantics {
                contentDescription = selected?.let { forgeActionLabel(it.machine.label) }
                    ?: "Choose a machine"
            },
        ) {
            if (state.pending) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(8.dp))
            }
            Text(
                selected?.let { "Create on ${bidiIsolate(it.machine.label.text)}" }
                    ?: "Choose a machine",
            )
        }
    }
}

@Composable
private fun DirectoryField(
    state: ForgeState,
    machine: MachineState?,
    enabled: Boolean,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    maxSuggestionsHeight: Dp,
    actions: ForgeSheetActions,
    modifier: Modifier,
    onNext: () -> Unit,
    onBrowse: () -> Unit,
) {
    val input = classifyWorkingDirectory(state.form.cwd)
    val paths = workingDirectoryChoices(state, machine)
    SuggestionField(
        text = state.form.cwd,
        label = machine?.let { "directory on ${bidiIsolate(it.machine.label.text)}" }
            ?: "directory",
        textStyle = MaterialTheme.typography.bodyLarge.copy(fontFamily = NidavellirType.Data,
            textDirection = TextDirection.Ltr),
        enabled = enabled,
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        onChange = { text -> actions.updateDraft { it.copy(cwd = text) } },
        literal = when (input) {
            WorkingDirectoryInput.Home -> "~"
            is WorkingDirectoryInput.Literal -> input.path.encoded
            WorkingDirectoryInput.Invalid, is WorkingDirectoryInput.Query -> null
        },
        onAccept = { actions.chooseWorkingDirectory(requireNotNull(WorkingDirectoryPath.parse(it))) },
        onNext = onNext,
        onFocus = actions.focusWorkingDirectory,
        maxSuggestionsHeight = maxSuggestionsHeight,
        message = when (input) {
            WorkingDirectoryInput.Home -> "home (~)"
            is WorkingDirectoryInput.Literal -> null
            is WorkingDirectoryInput.Query -> "choose a matching directory"
            WorkingDirectoryInput.Invalid -> "use ~, ~/… or an absolute path, or 1–8 search words."
        },
        invalid = input == WorkingDirectoryInput.Invalid,
        modifier = modifier,
    ) { choose ->
        if (input is WorkingDirectoryInput.Query) {
            val status = when (val search = state.directorySearch) {
                DirectorySearchState.Idle -> "type to search visited directories"
                is DirectorySearchState.Loading -> "searching…"
                is DirectorySearchState.Failed -> gatewayFailureMessage(search.failure)
                is DirectorySearchState.Ready -> when {
                    search.result.directories.isEmpty() -> "no matching directories"
                    search.result.omitted -> "some directories are not shown"
                    else -> null
                }
            }
            status?.let { item(key = "status") { Text(it, color = Muted) } }
        }
        // Saveable viewport keys contain no paths; acceptance uses the exact domain value.
        itemsIndexed(paths, key = { ordinal, _ -> "path:$ordinal" }) { _, path ->
            val description = if (path.encoded == "~") "home (~)" else path.encoded
            val spoken = "$description on ${machine?.machine?.label?.text}. select working directory."
            if (path.encoded == "~") {
                SuggestionTextChoice(description, spoken, { choose(path.encoded) })
            } else {
                SuggestionChoice(spoken, { choose(path.encoded) }) {
                    WorkingDirectoryPathLine(path.encoded, Modifier.fillMaxWidth(), contentDescription = null)
                }
            }
        }
        item(key = "browse") {
            SuggestionTextChoice("browse home", "browse home on ${machine?.machine?.label?.text}", onBrowse)
        }
    }
}
