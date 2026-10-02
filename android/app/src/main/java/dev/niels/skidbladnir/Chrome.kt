package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp

// One panel for every failure, degradation, and armed recovery
// (destructive-chrome.md). Three hand-rolled banner Surfaces each picked their
// own colour, which is how Ember came to mean five unrelated things; here the
// tone is the only input and noticeToneColor is its sole owner.
// It takes no `modifier` on purpose: all three consumers applied the identical
// outer geometry, so that geometry is the duplication being removed, and a
// parameter no call site varies is what rules/simplicity.md forbids.
// No ornament, here or ever — §7 and §15 forbid it on error surfaces.
@Composable
internal fun NoticePanel(
    tone: NoticeTone,
    body: String,
    title: String? = null,
    actions: (@Composable RowScope.() -> Unit)? = null,
) {
    val toneColor = noticeToneColor(tone)
    Surface(
        color = toneColor.copy(alpha = 0.12f),
        border = BorderStroke(1.dp, toneColor),
        shape = NidavellirShapes.Card,
        modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 4.dp),
    ) {
        Column(Modifier.padding(12.dp)) {
            title?.let {
                Text(
                    text = it,
                    color = Bone,
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
            }
            Text(text = body, color = toneColor, style = MaterialTheme.typography.bodyMedium)
            actions?.let { Row(content = it) }
        }
    }
}

// The one header chip: DeepSurface ground, accent hairline, angular indication,
// and the 48dp floor on the inner Box for the reason CloseButton records below.
// Hand-rolling this per call site is how the header grew two owners for one
// treatment. `spokenName` is null wherever the visible label is already the
// control's name, and carries it where the label is a glyph.
@Composable
internal fun HeaderChip(
    label: String,
    spokenName: String?,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val accent = if (enabled) Gold else Muted
    Surface(
        color = DeepSurface,
        border = BorderStroke(1.dp, accent.copy(alpha = 0.40f)),
        shape = NidavellirShapes.Chip,
        modifier = modifier
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Chip),
                enabled = enabled,
                role = Role.Button,
                onClick = onClick,
            )
            .semantics(mergeDescendants = true) {
                spokenName?.let { contentDescription = it }
            },
    ) {
        Box(
            modifier = Modifier.minimumInteractiveComponentSize().padding(horizontal = 12.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = label,
                color = accent,
                style = MaterialTheme.typography.labelLarge,
            )
        }
    }
}
