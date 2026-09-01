package dev.niels.skidbladnir

import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
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
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp

internal class ForgeSheetActions(
    val dismiss: () -> Unit,
    val updateDraft: ((ForgeForm) -> ForgeForm) -> Unit,
    val submit: () -> Unit,
    val openWorkingDirectoryPicker: () -> Unit,
    val openExactWorkingDirectoryPicker: () -> Unit,
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
            ForgeSurface.Form -> ForgeFormContent(state, machines, actions)
            is ForgeSurface.DirectoryPicker -> {
                ModalBackHandler(onBack = actions.workingDirectory.back)
                WorkingDirectoryPickerScreen(
                    picker = surface.picker,
                    actions = actions.workingDirectory,
                    modifier = Modifier.fillMaxWidth().fillMaxHeight(),
                )
            }
        }
    }
}

@Composable
private fun ModalBackHandler(onBack: () -> Unit) {
    val currentOnBack by rememberUpdatedState(onBack)
    val view = LocalView.current
    DisposableEffect(view) {
        val dispatcher = checkNotNull(view.findOnBackInvokedDispatcher()) {
            "Forge modal must be attached to a Back dispatcher"
        }
        val callback = OnBackInvokedCallback { currentOnBack() }
        dispatcher.registerOnBackInvokedCallback(
            OnBackInvokedDispatcher.PRIORITY_OVERLAY,
            callback,
        )
        onDispose { dispatcher.unregisterOnBackInvokedCallback(callback) }
    }
}

@Composable
private fun ForgeFormContent(
    state: ForgeState,
    machines: List<MachineState>,
    actions: ForgeSheetActions,
) {
    val selected = state.form.machineHandle?.let { handle ->
        machines.singleOrNull { it.machine.handle == handle }
    }
    val inventory = selected?.inventory?.lastSnapshot()?.inventory
    val fieldsEnabled = !state.pending && selected?.canMutate == true

    Column(
        Modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .imePadding()
            .padding(horizontal = 20.dp)
            .padding(bottom = 28.dp)
            .testTag("forge-sheet"),
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
                    modifier = Modifier.testTag(
                        "forge-machine-${machine.machine.handle.encoded}",
                    ).semantics {
                        contentDescription = forgeMachineChoiceLabel(machine)
                    },
                )
            }
        }
        if (selected == null) {
            Text(
                "Choose a machine to choose a working directory and profile.",
                color = Muted,
            )
        } else {
            Text(
                "Profiles on ${bidiIsolate(selected.machine.label.text)}",
                color = Muted,
                style = MaterialTheme.typography.labelLarge,
            )
            Row(
                Modifier.horizontalScroll(rememberScrollState()),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                inventory?.profiles.orEmpty().forEach { profile ->
                    FilterChip(
                        selected = state.form.profile == profile.key,
                        onClick = { actions.updateDraft { it.copy(profile = profile.key) } },
                        enabled = fieldsEnabled,
                        label = { Text(profile.label, fontFamily = NidavellirType.Data) },
                        shape = NidavellirShapes.Chip,
                        modifier = Modifier.testTag(
                            "forge-profile-${selected.machine.handle.encoded}",
                        ),
                    )
                }
            }
            ForgeWorkingDirectorySelection(
                state = state,
                machine = selected.machine,
                enabled = fieldsEnabled,
                onChoose = actions.openWorkingDirectoryPicker,
                onRepair = actions.openExactWorkingDirectoryPicker,
            )
            forgeUnavailableCopy(selected)?.let { notice ->
                Text(
                    bidiIsolate(notice.message),
                    color = noticeToneColor(notice.tone),
                    modifier = Modifier.testTag("forge-machine-unavailable").semantics {
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
            modifier = Modifier.fillMaxWidth().testTag("forge-name"),
            enabled = fieldsEnabled,
            label = { Text("tmux name (optional)") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
            ),
        )
        OutlinedTextField(
            value = state.form.objective,
            onValueChange = { value -> actions.updateDraft { it.copy(objective = value) } },
            modifier = Modifier.fillMaxWidth().testTag("forge-objective"),
            enabled = fieldsEnabled,
            label = { Text("Objective (optional)") },
            minLines = 2,
            maxLines = 4,
        )
        when (val failure = state.failure) {
            ForgeFailure.None -> Unit
            is ForgeFailure.Definite -> Text(
                gatewayFailureMessage(failure.rejection),
                color = noticeToneColor(NoticeTone.Failure),
                modifier = Modifier.testTag("forge-failure"),
            )
        }
        Button(
            onClick = actions.submit,
            enabled = state.admissibleSubmission() != null && selected?.canMutate == true,
            modifier = Modifier.fillMaxWidth().testTag("forge-submit").semantics {
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
private fun ForgeWorkingDirectorySelection(
    state: ForgeState,
    machine: PairedMachine,
    enabled: Boolean,
    onChoose: () -> Unit,
    onRepair: () -> Unit,
) {
    if (state.form.cwd.isEmpty()) {
        OutlinedButton(
            onClick = onChoose,
            enabled = enabled,
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                .minimumInteractiveComponentSize().testTag("forge-working-directory"),
            shape = NidavellirShapes.Chip,
        ) {
            Text("Choose a working directory")
        }
        return
    }

    Surface(
        color = RaisedSurface,
        border = BorderStroke(1.dp, Gold.copy(alpha = 0.40f)),
        shape = NidavellirShapes.Card,
        modifier = Modifier.fillMaxWidth().testTag("forge-working-directory"),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
            Text(
                "Working directory on ${bidiIsolate(machine.label.text)}",
                color = Muted,
                style = MaterialTheme.typography.labelLarge,
                modifier = Modifier.semantics {
                    contentDescription = "Working directory on ${machine.label.text}"
                },
            )
            WorkingDirectoryPathLine(
                path = state.form.cwd,
                modifier = Modifier.fillMaxWidth().testTag("working-directory-path-scroll"),
            )
            TextButton(
                onClick = if (state.failure.isWorkingDirectoryRejection()) onRepair else onChoose,
                enabled = enabled,
                modifier = Modifier.heightIn(min = 48.dp).minimumInteractiveComponentSize()
                    .testTag("forge-working-directory-change"),
            ) {
                Text("Change")
            }
        }
    }
}
