package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.selection.SelectionContainer
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
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.SheetValue
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ConversationSheet(state: ConversationSheetState, controller: SkidbladnirController) {
    val pending = (state as? ConversationSheetState.Tracking)?.pending == true
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = controller::dismissConversationSheet,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet, containerColor = DeepSurface,
        sheetGesturesEnabled = !pending,
        properties = ModalBottomSheetProperties(shouldDismissOnBackPress = !pending, shouldDismissOnClickOutside = !pending),
    ) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding()
            .padding(horizontal = 20.dp).padding(bottom = 28.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            when (state) {
                is ConversationSheetState.Tracking -> {
                    Text("track conversation", style = MaterialTheme.typography.headlineSmall,
                        fontFamily = NidavellirType.Display, modifier = Modifier.semantics { heading() })
                    Text(state.target.session.tmuxName, fontFamily = NidavellirType.Data)
                    Text("status and unread will follow this id even when the terminal shows another conversation or a shell.", color = Muted)
                    var expanded by remember(state.target) { mutableStateOf(false) }
                    Box {
                        GroupTextAction(state.profiles.singleOrNull { it.key == state.profile }?.label ?: "profile unavailable",
                            !pending && state.profiles.isNotEmpty(), { expanded = true })
                        DropdownMenu(expanded = expanded && !pending, onDismissRequest = { expanded = false },
                            shape = NidavellirShapes.Card, containerColor = DeepSurface, tonalElevation = 0.dp, shadowElevation = 0.dp) {
                            state.profiles.forEach { profile ->
                                GroupTextAction(profile.label, true, {
                                    controller.editConversationTracking(profile = profile.key)
                                    expanded = false
                                }, Modifier.fillMaxWidth())
                            }
                        }
                    }
                    OutlinedTextField(value = state.id, onValueChange = { controller.editConversationTracking(id = it) },
                        enabled = !pending, singleLine = true, label = { Text("conversation id") },
                        keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.None, autoCorrectEnabled = false),
                        modifier = Modifier.fillMaxWidth())
                    if (state.profiles.isEmpty()) Text("tracking unavailable: no codex profile has a history scope.", color = Muted)
                    if (pending) Text("updating tracking", color = Gold)
                    state.error?.let { Text(it, color = Ember) }
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(onClick = controller::dismissConversationSheet, enabled = !pending) { Text("cancel") }
                        Button(onClick = { controller.saveConversationTracking() },
                            enabled = !pending && state.profile != null && validNativeId(state.id)) { Text("track") }
                    }
                    if (state.target.session.conversation?.binding?.conversation?.provider == AgentProvider.Codex) {
                        GroupTextAction("stop tracking", !pending, { controller.saveConversationTracking(clear = true) })
                    }
                }
                is ConversationSheetState.Replies -> {
                    Text("view replies", style = MaterialTheme.typography.headlineSmall,
                        fontFamily = NidavellirType.Display, modifier = Modifier.semantics { heading() })
                    Text("${state.machine.label.text} · ${state.conversation.profileKey}", fontFamily = NidavellirType.Data)
                    SelectionContainer { Text(state.conversation.conversationId, fontFamily = NidavellirType.Data) }
                    Text("native output for this conversation. opening clears only replies known at opening; later replies stay unread.", color = Muted)
                    if (state.loading) Text("reading replies", color = Gold)
                    state.output?.let { output ->
                        val content = sessionStatusContent(output.observation.status, fresh = true)
                        Text(content.label, color = Muted, fontFamily = NidavellirType.Data)
                        Text(when (output.outputState) {
                            "partial" -> "partial output"
                            "finalized" -> "finalized output"
                            "unknown" -> "output finality unavailable"
                            "none" -> "no assistant text"
                            else -> error("unknown output state") // justify-defect: the boundary accepts the closed state set.
                        }, color = Muted)
                        SelectionContainer { Text(output.text, color = Bone, style = MaterialTheme.typography.bodyMedium) }
                        if (output.truncated) Text("output truncated at 16 kib", color = Muted)
                        LaunchedEffect(state, sheetState.currentValue) {
                            if (!state.acknowledgementAttempted && !state.loading && state.error == null && sheetState.currentValue == SheetValue.Expanded) {
                                withFrameNanos { }
                                controller.repliesPresented(state)
                            }
                        }
                    }
                    state.error?.let { Text(it, color = Ember) }
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        OutlinedButton(onClick = controller::dismissConversationSheet) { Text("close") }
                        Button(onClick = controller::readReplies, enabled = !state.loading) { Text("refresh replies") }
                    }
                }
            }
        }
    }
}
