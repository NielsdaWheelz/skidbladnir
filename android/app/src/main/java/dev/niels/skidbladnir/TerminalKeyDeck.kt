package dev.niels.skidbladnir

import android.view.HapticFeedbackConstants
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.semantics.traversalIndex
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

private data class TerminalKeyDeckItem(
    val accessory: TerminalAccessory,
    val label: String,
    val spokenName: String,
)

private val terminalKeyDeckRows = listOf(
    listOf(
        TerminalKeyDeckItem(TerminalAccessory.Escape, "Esc", "Escape"),
        TerminalKeyDeckItem(TerminalAccessory.Slash, "/", "Slash"),
        TerminalKeyDeckItem(TerminalAccessory.Hyphen, "-", "Hyphen"),
        TerminalKeyDeckItem(TerminalAccessory.Home, "Home", "Home"),
        TerminalKeyDeckItem(TerminalAccessory.Up, "↑", "Up arrow"),
        TerminalKeyDeckItem(TerminalAccessory.End, "End", "End"),
        TerminalKeyDeckItem(TerminalAccessory.PageUp, "PgUp", "Page up"),
    ),
    listOf(
        TerminalKeyDeckItem(TerminalAccessory.Tab, "Tab", "Tab"),
        TerminalKeyDeckItem(TerminalAccessory.Control, "Ctrl", "Control"),
        TerminalKeyDeckItem(TerminalAccessory.Alt, "Alt", "Alt"),
        TerminalKeyDeckItem(TerminalAccessory.Left, "←", "Left arrow"),
        TerminalKeyDeckItem(TerminalAccessory.Down, "↓", "Down arrow"),
        TerminalKeyDeckItem(TerminalAccessory.Right, "→", "Right arrow"),
        TerminalKeyDeckItem(TerminalAccessory.PageDown, "PgDn", "Page down"),
    ),
)

@Composable
internal fun TerminalKeyDeck(
    modifiers: TerminalModifiers,
    enabled: Boolean,
    onAccessory: (TerminalAccessory) -> Unit,
    modifier: Modifier = Modifier,
) {
    val view = LocalView.current
    Surface(
        modifier = modifier.fillMaxWidth(),
        color = RaisedSurface,
    ) {
        BoxWithConstraints(modifier = Modifier.fillMaxWidth()) {
            val keyTextStyle = MaterialTheme.typography.labelLarge.copy(fontFamily = NidavellirType.Data)
            val cellWidth = terminalKeyCellWidth(maxWidth, keyTextStyle)
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .horizontalScroll(rememberScrollState())
                    .semantics { isTraversalGroup = true }
                    .padding(DECK_OUTER_PADDING),
                verticalArrangement = Arrangement.spacedBy(DECK_GAP),
            ) {
                terminalKeyDeckRows.forEachIndexed { rowIndex, row ->
                    Row(horizontalArrangement = Arrangement.spacedBy(DECK_GAP)) {
                        row.forEachIndexed { columnIndex, item ->
                            val modifierPhase = item.modifierPhase(modifiers)
                            val armed = enabled && modifierPhase == TerminalModifierPhase.Armed
                            OutlinedButton(
                                onClick = {
                                    view.performHapticFeedback(HapticFeedbackConstants.KEYBOARD_TAP)
                                    onAccessory(item.accessory)
                                },
                                enabled = enabled,
                                modifier = Modifier
                                    .width(cellWidth)
                                    .heightIn(min = MINIMUM_KEY_SIZE)
                                    .semantics {
                                        contentDescription = item.spokenName
                                        traversalIndex = (rowIndex * COLUMN_COUNT + columnIndex).toFloat()
                                        if (modifierPhase != null) {
                                            toggleableState = if (armed) ToggleableState.On else ToggleableState.Off
                                            stateDescription = if (armed) "Armed" else "Off"
                                        }
                                    },
                                shape = NidavellirShapes.Key,
                                colors = ButtonDefaults.outlinedButtonColors(
                                    containerColor = if (armed) Gold.copy(alpha = 0.18f) else Color.Transparent,
                                    contentColor = if (armed) Gold else Bone,
                                    disabledContainerColor = Muted.copy(
                                        alpha = NidavellirMotion.DisabledAlpha.Container,
                                    ),
                                    disabledContentColor = Muted.copy(
                                        alpha = NidavellirMotion.DisabledAlpha.Content,
                                    ),
                                ),
                                border = BorderStroke(
                                    if (armed) 2.dp else 1.dp,
                                    when {
                                        !enabled -> Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Container)
                                        armed -> Gold
                                        else -> Muted.copy(alpha = 0.65f)
                                    },
                                ),
                                contentPadding = PaddingValues(horizontal = KEY_CONTENT_PADDING),
                            ) {
                                Text(
                                    text = item.label,
                                    style = keyTextStyle,
                                    maxLines = 1,
                                    softWrap = false,
                                    textAlign = TextAlign.Center,
                                    modifier = Modifier.fillMaxWidth(),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun terminalKeyCellWidth(
    viewportWidth: Dp,
    textStyle: TextStyle,
): Dp {
    val density = LocalDensity.current
    val textMeasurer = rememberTextMeasurer()
    val widestLabelPixels = terminalKeyDeckRows.flatten().maxOf { item ->
        textMeasurer.measure(
            text = item.label,
            style = textStyle,
            maxLines = 1,
            softWrap = false,
        ).size.width
    }
    val labelCellWidth = with(density) { (widestLabelPixels + 1).toDp() } + KEY_CONTENT_PADDING * 2
    val viewportCellWidth = (
        viewportWidth - DECK_OUTER_PADDING * 2 - DECK_GAP * (COLUMN_COUNT - 1)
    ) / COLUMN_COUNT.toFloat()
    return maxOf(MINIMUM_KEY_SIZE, labelCellWidth, viewportCellWidth)
}

private fun TerminalKeyDeckItem.modifierPhase(modifiers: TerminalModifiers): TerminalModifierPhase? =
    when (accessory) {
        TerminalAccessory.Control -> modifiers.control
        TerminalAccessory.Alt -> modifiers.alt
        else -> null
    }

private const val COLUMN_COUNT = 7
private val MINIMUM_KEY_SIZE = 48.dp
private val DECK_OUTER_PADDING = 4.dp
private val DECK_GAP = 2.dp
private val KEY_CONTENT_PADDING = 4.dp
