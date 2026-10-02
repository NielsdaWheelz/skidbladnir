package dev.niels.skidbladnir

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
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
