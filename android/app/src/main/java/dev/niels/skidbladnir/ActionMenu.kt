package dev.niels.skidbladnir

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
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
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// A surface lists its actions once; its menu or sheet renders that list through `ActionRows`.
internal class MenuAction(
    val label: String,
    val enabled: Boolean,
    val destructive: Boolean,
    val perform: () -> Unit,
)

// The terminal lifetime verbs in their one order: the interrupt, then the two confirmed closures, most complete last.
internal fun terminalLifetimeActions(
    enabled: Boolean,
    onStop: () -> Unit,
    onTerminalClose: () -> Unit,
    onClose: () -> Unit,
): List<MenuAction> = listOf(
    MenuAction(TERMINAL_STOP_ACTION, enabled, destructive = false, onStop),
    MenuAction(TERMINAL_ONLY_CLOSE_ACTION, enabled, destructive = true, onTerminalClose),
    MenuAction(TERMINAL_CLOSE_ACTION, enabled, destructive = true, onClose),
)

// A 48dp overflow trigger, live while any action is, and its menu. An open menu disables items in place as its
// list recomposes. `markCenterY` aligns the mark to the caller's content; the target stays 48dp from the anchor's top.
@Composable
internal fun ActionMenuButton(
    description: String,
    actions: List<MenuAction>,
    markCenterY: Dp,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    val enabled = actions.any(MenuAction::enabled)
    Box(modifier) {
        Box(
            modifier = Modifier
                .size(48.dp)
                .clickable(
                    interactionSource = null,
                    indication = AngularIndication(NidavellirShapes.Chip),
                    enabled = enabled,
                    onClick = { expanded = true },
                )
                .semantics { contentDescription = description },
        ) {
            val color = when {
                expanded -> Gold
                enabled -> Muted
                else -> Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Content)
            }
            // Square studs: the design language has no circles.
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
        // Carved, not printed: the theme's raised container and no shadow.
        DropdownMenu(
            expanded = expanded,
            onDismissRequest = { expanded = false },
            shape = NidavellirShapes.Card,
            tonalElevation = 0.dp,
            shadowElevation = 0.dp,
        ) {
            ActionRows(actions) { expanded = false }
        }
    }
}

// The rows of one action list for menus and the terminal's session sheet; each row dismisses its surface first.
@Composable
internal fun ActionRows(actions: List<MenuAction>, dismiss: () -> Unit) {
    actions.forEachIndexed { index, action ->
        // A rule opens the destructive run, so colour is not its only cue.
        if (action.destructive && actions.getOrNull(index - 1)?.destructive == false) {
            HorizontalDivider(Modifier.padding(vertical = 4.dp), color = Bone.copy(alpha = 0.12f))
        }
        // M3's DropdownMenuItem hardcodes a circular ripple, so the row is built here with the angular flash.
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 48.dp)
                .clickable(
                    interactionSource = null,
                    indication = AngularIndication(NidavellirShapes.Chip),
                    enabled = action.enabled,
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
}
