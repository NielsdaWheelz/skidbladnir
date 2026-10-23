package dev.niels.skidbladnir

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ForgeSheet(
    forge: ForgeState,
    machines: List<MachineState>,
    controller: SkidbladnirController,
) {
    val pickerVisible = forge.surface is ForgeSurface.DirectoryPicker
    var lit by remember { mutableStateOf(false) }
    var focusDirectoryOnReturn by remember { mutableStateOf(false) }
    val containerColor by animateColorAsState(
        targetValue = if (lit) ForgeGlow else DeepSurface,
        animationSpec = NidavellirMotion.ForgeWarmIn,
        label = "forge warm-in",
    )
    LaunchedEffect(Unit) { lit = true }

    ModalBottomSheet(
        onDismissRequest = controller::dismissForge,
        modifier = if (pickerVisible) Modifier.fillMaxHeight() else Modifier,
        sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
        shape = NidavellirShapes.Sheet,
        containerColor = containerColor,
        properties = ModalBottomSheetProperties(shouldDismissOnBackPress = !pickerVisible),
    ) {
        when (val surface = forge.surface) {
            ForgeSurface.Form -> ForgeFormContent(forge, machines, controller, focusDirectoryOnReturn) {
                focusDirectoryOnReturn = false
            }
            is ForgeSurface.DirectoryPicker -> WorkingDirectoryPickerScreen(
                picker = surface.picker,
                controller = controller,
                enabled = machines.singleOrNull { it.machine.handle == surface.picker.machine.handle }?.canMutate == true,
                onReturnToField = { focusDirectoryOnReturn = true },
            )
        }
    }
}

private enum class ForgeEntryField { Directory, Group }

@Composable
private fun ForgeFormContent(
    forge: ForgeState,
    machines: List<MachineState>,
    controller: SkidbladnirController,
    requestDirectoryFocus: Boolean,
    onDirectoryFocused: () -> Unit,
) {
    var expandedField by remember { mutableStateOf<ForgeEntryField?>(null) }
    val directoryFocus = remember { FocusRequester() }
    val groupFocus = remember { FocusRequester() }
    LaunchedEffect(requestDirectoryFocus) {
        if (requestDirectoryFocus) {
            directoryFocus.requestFocus()
            onDirectoryFocused()
        }
    }
    val selected = forge.form.machineHandle?.let { handle ->
        machines.singleOrNull { it.machine.handle == handle }
    }
    val inventory = selected?.inventory?.lastSnapshot()?.inventory
    val fieldsEnabled = !forge.pending && selected?.canMutate == true

    SheetForm(actions = {
        Button(
            onClick = controller::forge,
            enabled = forge.admissibleSubmission() != null && selected?.canMutate == true,
            modifier = Modifier.fillMaxWidth(),
        ) {
            if (forge.pending) {
                CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(8.dp))
            }
            Text(selected?.let { "Create on ${it.machine.label.text}" } ?: "Choose a machine")
        }
    }) { maxSuggestionsHeight ->
        SheetTitle("Create dwarf")
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
                    selected = machine.machine.handle == forge.form.machineHandle,
                    onClick = {
                        controller.updateForgeDraft { it.copy(machineHandle = machine.machine.handle) }
                    },
                    enabled = !forge.pending && machine.canMutate,
                    label = { Text(forgeMachineChoiceLabel(machine), fontFamily = NidavellirType.Data) },
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
                "Launch on ${selected.machine.label.text}",
                color = Muted,
                style = MaterialTheme.typography.labelLarge,
            )
            Row(
                Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                inventory?.profiles.orEmpty().forEach { profile ->
                    FilterChip(
                        selected = forge.form.launch == LaunchChoice.Agent(profile.key),
                        onClick = { controller.updateForgeDraft { it.copy(launch = LaunchChoice.Agent(profile.key)) } },
                        enabled = fieldsEnabled,
                        label = { Text(profile.label, fontFamily = NidavellirType.Data) },
                    )
                }
                FilterChip(
                    selected = forge.form.launch == LaunchChoice.Terminal,
                    onClick = { controller.updateForgeDraft { it.copy(launch = LaunchChoice.Terminal) } },
                    enabled = fieldsEnabled,
                    label = { Text("Terminal", fontFamily = NidavellirType.Data) },
                )
            }
            forgeUnavailableCopy(selected)?.let { notice ->
                Text(notice.message, color = noticeToneColor(notice.tone))
            }
        }
        OutlinedTextField(
            value = forge.form.optionalTmuxName,
            onValueChange = { value ->
                controller.updateForgeDraft { it.copy(optionalTmuxName = value) }
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
            value = forge.form.objective,
            onValueChange = { value -> controller.updateForgeDraft { it.copy(objective = value) } },
            modifier = Modifier.fillMaxWidth(),
            enabled = fieldsEnabled,
            label = { Text("Objective (optional)") },
            minLines = 2,
            maxLines = 4,
        )
        DirectoryField(
            forge = forge,
            machine = selected,
            enabled = fieldsEnabled,
            expanded = expandedField == ForgeEntryField.Directory,
            onExpandedChange = { expanded ->
                if (expanded) expandedField = ForgeEntryField.Directory
                else if (expandedField == ForgeEntryField.Directory) expandedField = null
            },
            maxSuggestionsHeight = maxSuggestionsHeight,
            controller = controller,
            modifier = Modifier.focusRequester(directoryFocus),
            onNext = { groupFocus.requestFocus() },
        )
        GroupField(
            draft = forge.form.group,
            labels = observedGroups(machines),
            enabled = !forge.pending,
            expanded = expandedField == ForgeEntryField.Group,
            onExpandedChange = { expanded ->
                if (expanded) expandedField = ForgeEntryField.Group
                else if (expandedField == ForgeEntryField.Group) expandedField = null
            },
            maxSuggestionsHeight = maxSuggestionsHeight,
            onChange = { text -> controller.updateForgeDraft { it.copy(group = GroupDraft.Chosen(text)) } },
            modifier = Modifier.focusRequester(groupFocus),
        )
        when (val failure = forge.failure) {
            ForgeFailure.None -> Unit
            is ForgeFailure.Definite -> Text(
                gatewayFailureMessage(failure.rejection),
                color = noticeToneColor(NoticeTone.Failure),
            )
        }
    }
}

@Composable
private fun DirectoryField(
    forge: ForgeState,
    machine: MachineState?,
    enabled: Boolean,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    maxSuggestionsHeight: Dp,
    controller: SkidbladnirController,
    modifier: Modifier,
    onNext: () -> Unit,
) {
    val focus = LocalFocusManager.current
    val keyboard = LocalSoftwareKeyboardController.current
    val input = classifyWorkingDirectory(forge.form.cwd)
    val paths = workingDirectoryChoices(forge, machine)
    SuggestionField(
        text = forge.form.cwd,
        label = machine?.let { "directory on ${it.machine.label.text}" } ?: "directory",
        textStyle = MaterialTheme.typography.bodyLarge.copy(fontFamily = NidavellirType.Data,
            textDirection = TextDirection.Ltr),
        enabled = enabled,
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        onChange = { text -> controller.updateForgeDraft { it.copy(cwd = text) } },
        literal = when (input) {
            WorkingDirectoryInput.Home -> "~"
            is WorkingDirectoryInput.Literal -> input.path.encoded
            WorkingDirectoryInput.Invalid, is WorkingDirectoryInput.Query -> null
        },
        onAccept = { controller.chooseWorkingDirectory(requireNotNull(WorkingDirectoryPath.parse(it))) },
        onNext = onNext,
        onFocus = controller::focusWorkingDirectory,
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
            val status = when (val search = forge.directorySearch) {
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
        // Keys carry no paths; acceptance uses the exact domain value.
        itemsIndexed(paths, key = { ordinal, _ -> "path:$ordinal" }) { _, path ->
            if (path.encoded == "~") {
                SuggestionTextChoice("home (~)", { choose(path.encoded) })
            } else {
                SuggestionChoice({ choose(path.encoded) }) {
                    WorkingDirectoryPathLine(path.encoded, Modifier.fillMaxWidth())
                }
            }
        }
        item(key = "browse") {
            SuggestionTextChoice("browse home") {
                focus.clearFocus()
                keyboard?.hide()
                controller.browseWorkingDirectoryHome()
            }
        }
    }
}

private fun forgeMachineChoiceLabel(machine: MachineState): String =
    machine.machine.label.text + availabilityWord(machineAvailability(machine))?.let { " · $it" }.orEmpty()

/** The forge names the disabled draft fields in its own prose; the tone is availabilityTone's, so the strip and the forge agree on loudness. */
private fun forgeUnavailableCopy(machine: MachineState): MachineNotice? {
    val label = machine.machine.label.text
    val availability = machineAvailability(machine)
    val tone = availabilityTone(availability)
    return when (availability) {
        MachineAvailability.Ready -> null
        MachineAvailability.Refreshing -> MachineNotice(
            "$label is confirming its latest tmux inventory. Draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.AuthRequired -> MachineNotice(
            "$label needs the fleet reconnected. Draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.IdentityChanged -> MachineNotice(
            "$label identity changed. Fleet reset is required; draft fields and Create are disabled.",
            tone,
        )
        MachineAvailability.Reading -> MachineNotice(
            "$label is reading tmux sessions. Draft fields and Create are disabled until the inventory is fresh.",
            tone,
        )
        is MachineAvailability.Stale -> MachineNotice(
            "$label inventory is STALE. Draft fields and Create are disabled until a fresh read succeeds.",
            tone,
        )
        is MachineAvailability.Unavailable -> MachineNotice(
            "$label is unavailable. Draft fields and Create are disabled until it reconnects.",
            tone,
        )
    }
}
