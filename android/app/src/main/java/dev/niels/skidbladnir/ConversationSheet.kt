package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.material3.SheetValue
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ConversationSheet(state: ConversationSheetState, controller: SkidbladnirController) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(
        onDismissRequest = controller::dismissConversationSheet,
        sheetState = sheetState,
        shape = NidavellirShapes.Sheet, containerColor = DeepSurface,
    ) {
        Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).imePadding()
            .padding(horizontal = 20.dp).padding(bottom = 28.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            Text("view replies", style = MaterialTheme.typography.headlineSmall,
                fontFamily = NidavellirType.Display, modifier = Modifier.semantics { heading() })
            Text("${state.machine.label.text} · ${state.conversation.profileKey}", fontFamily = NidavellirType.Data)
            SelectionContainer { Text(state.conversation.conversationId, fontFamily = NidavellirType.Data) }
            Text("native output for this conversation. opening clears only replies known at opening; later replies stay unread.", color = Muted)
            if (state.loading) Text("reading replies", color = Gold)
            state.output?.let { output ->
                val label = conversationStatusLabel(output.observation.status)
                Text(label, color = Muted, fontFamily = NidavellirType.Data)
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
