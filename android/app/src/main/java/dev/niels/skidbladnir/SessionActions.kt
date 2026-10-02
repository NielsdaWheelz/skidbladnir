package dev.niels.skidbladnir

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// One verb on one session (docs/session-card.md "Actions"). A surface lists its
// verbs once and every presentation reads that list: the overflow menu below
// and the card's TalkBack actions, so order, label, enablement and tone cannot
// drift between what is seen and what is spoken.
internal class SessionAction(
    val label: String,
    val enabled: Boolean,
    val destructive: Boolean,
    val perform: () -> Unit,
)

// The terminal lifetime controls in their one order (agent-control.md): the
// unconfirmed interrupt, then the two confirmed closures, the most complete
// last and farthest from the anchor.
internal fun terminalLifetimeActions(
    enabled: Boolean,
    onStop: () -> Unit,
    onTerminalClose: () -> Unit,
    onClose: () -> Unit,
): List<SessionAction> = listOf(
    SessionAction(TERMINAL_STOP_ACTION, enabled, destructive = false, onStop),
    SessionAction(TERMINAL_ONLY_CLOSE_ACTION, enabled, destructive = true, onTerminalClose),
    SessionAction(TERMINAL_CLOSE_ACTION, enabled, destructive = true, onClose),
)

// The overflow: a 48dp target carrying a drawn mark, and the menu it anchors.
// Enablement is derived, never passed: the trigger is live while any verb is.
// An open menu follows its list as it recomposes, so a fence that lands while
// it is open disables items in place rather than closing or retargeting them.
// `markCenterY` sets the mark on the first line of the text it serves; the
// target stays a full 48dp square from the anchor's top.
@Composable
internal fun SessionActionsButton(
    spokenName: String,
    actions: List<SessionAction>,
    markCenterY: Dp,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    val enabled = actions.any(SessionAction::enabled)
    Box(modifier) {
        Box(
            modifier = Modifier
                .size(48.dp)
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = AngularIndication(NidavellirShapes.Chip),
                    enabled = enabled,
                    role = Role.Button,
                    onClick = { expanded = true },
                )
                .semantics { contentDescription = spokenName },
        ) {
            val color = when {
                expanded -> Gold
                enabled -> Muted
                else -> Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Content)
            }
            // Three struck studs, not round dots: §6 has no circles.
            Canvas(Modifier.size(48.dp)) {
                val stud = 3.dp.toPx()
                val pitch = 6.dp.toPx()
                val top = markCenterY.toPx() - stud / 2f
                val left = size.width / 2f - pitch - stud / 2f
                for (index in 0..2) {
                    drawRect(color, Offset(left + index * pitch, top), Size(stud, stud))
                }
            }
        }
        // Carved, not printed: the menu is one tonal step up with no shadow.
        DropdownMenu(
            expanded = expanded,
            onDismissRequest = { expanded = false },
            shape = NidavellirShapes.Card,
            containerColor = RaisedSurface,
            tonalElevation = 0.dp,
            shadowElevation = 0.dp,
        ) {
            SessionActionRows(actions) { expanded = false }
        }
    }
}

// The rows of one action list, for the card's menu and the terminal's session
// sheet alike. Each row dismisses its surface before acting.
@Composable
internal fun SessionActionRows(actions: List<SessionAction>, dismiss: () -> Unit) {
    actions.forEachIndexed { index, action ->
        // The destructive run opens with a rule: the closures are a
        // different kind of verb, and colour must not be their only cue.
        if (action.destructive && actions.getOrNull(index - 1)?.destructive == false) {
            HorizontalDivider(Modifier.padding(vertical = 4.dp), color = Bone.copy(alpha = 0.12f))
        }
        SessionActionItem(action, dismiss)
    }
}

// M3's DropdownMenuItem hardcodes a circular ripple (§12 forbids it), so the
// row is built here with the angular flash. Controls speak the body face (§9).
@Composable
private fun SessionActionItem(action: SessionAction, dismiss: () -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Chip),
                enabled = action.enabled,
                role = Role.Button,
                onClick = {
                    dismiss()
                    action.perform()
                },
            )
            .padding(horizontal = 16.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Text(
            text = action.label,
            color = when {
                !action.enabled -> Bone.copy(alpha = NidavellirMotion.DisabledAlpha.Content)
                action.destructive -> noticeToneColor(NoticeTone.Failure)
                else -> Bone
            },
            style = MaterialTheme.typography.labelLarge,
        )
    }
}
