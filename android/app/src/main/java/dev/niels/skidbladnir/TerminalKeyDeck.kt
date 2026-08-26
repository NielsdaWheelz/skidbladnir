package dev.niels.skidbladnir

import android.view.HapticFeedbackConstants
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.semantics.traversalIndex
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.unit.dp

private data class TerminalKeyDeckItem(
    val accessory: TerminalAccessory,
    val label: String,
    val spokenName: String,
)

private val terminalKeyDeckItems = listOf(
    TerminalKeyDeckItem(TerminalAccessory.Escape, "Esc", "Escape"),
    TerminalKeyDeckItem(TerminalAccessory.Control, "Ctrl", "Control"),
    TerminalKeyDeckItem(TerminalAccessory.Tab, "Tab", "Tab"),
    TerminalKeyDeckItem(TerminalAccessory.LineFeed, "Line break", "Line break; sends line feed"),
    TerminalKeyDeckItem(TerminalAccessory.Left, "←", "Left arrow"),
    TerminalKeyDeckItem(TerminalAccessory.Up, "↑", "Up arrow"),
    TerminalKeyDeckItem(TerminalAccessory.Down, "↓", "Down arrow"),
    TerminalKeyDeckItem(TerminalAccessory.Right, "→", "Right arrow"),
    TerminalKeyDeckItem(TerminalAccessory.Home, "Home", "Home"),
    TerminalKeyDeckItem(TerminalAccessory.End, "End", "End"),
)

@Composable
internal fun TerminalKeyDeck(
    controlState: TerminalControlState,
    enabled: Boolean,
    onAccessory: (TerminalAccessory) -> Unit,
    modifier: Modifier = Modifier,
) {
    val view = LocalView.current
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = RaisedSurface,
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .semantics { isTraversalGroup = true }
                .padding(horizontal = 8.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            terminalKeyDeckItems.forEachIndexed { index, item ->
                val armed = enabled &&
                    item.accessory == TerminalAccessory.Control &&
                    controlState == TerminalControlState.Armed
                OutlinedButton(
                    onClick = {
                        view.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP)
                        onAccessory(item.accessory)
                    },
                    enabled = enabled,
                    modifier = Modifier
                        .sizeIn(minWidth = 48.dp, minHeight = 48.dp)
                        .semantics {
                            contentDescription = item.spokenName
                            traversalIndex = index.toFloat()
                            if (item.accessory == TerminalAccessory.Control) {
                                toggleableState = if (armed) ToggleableState.On else ToggleableState.Off
                                stateDescription = if (armed) "Armed" else "Off"
                            }
                        },
                    shape = NidavellirShapes.Key,
                    colors = ButtonDefaults.outlinedButtonColors(
                        containerColor = if (armed) Gold.copy(alpha = 0.18f) else Color.Transparent,
                        contentColor = if (armed) Gold else Bone,
                        disabledContainerColor = Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Container),
                        disabledContentColor = Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Content),
                    ),
                    border = BorderStroke(
                        if (armed) 2.dp else 1.dp,
                        when {
                            !enabled -> Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Container)
                            armed -> Gold
                            else -> Muted.copy(alpha = 0.65f)
                        },
                    ),
                    contentPadding = PaddingValues(horizontal = 12.dp),
                ) {
                    Text(item.label, fontFamily = NidavellirType.Data, maxLines = 1)
                }
            }
        }
    }
}
