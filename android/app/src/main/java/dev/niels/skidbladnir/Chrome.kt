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

@Composable
internal fun DetachButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        color = DeepSurface,
        border = BorderStroke(1.dp, Gold.copy(alpha = 0.40f)),
        shape = NidavellirShapes.Chip,
        modifier = modifier.clickable(
            interactionSource = remember { MutableInteractionSource() },
            indication = AngularIndication(NidavellirShapes.Chip),
            role = Role.Button,
            onClick = onClick,
        ),
    ) {
        Box(
            modifier = Modifier.minimumInteractiveComponentSize().padding(horizontal = 12.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = "Detach",
                color = Gold,
                style = MaterialTheme.typography.labelLarge,
            )
        }
    }
}

// The one destructive control (destructive-chrome.md). Its signal is geometry:
// Cleft is the only asymmetric shape in the product, so architecture.md's
// "detach and kill are visibly different actions" survives greyscale without an
// icon — §15 bans axe/hammer/helm clip-art and the app ships none. The word
// stays "Kill" because §4 keeps the dwarven voice in geometry, material and
// type, never in wording, and the label keeps the body face: it is a control,
// not a machine fact (§9).
@Composable
internal fun KillButton(
    machineLabel: MachineLabel,
    target: AgentTarget,
    enabled: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val failure = noticeToneColor(NoticeTone.Failure)
    val spoken = killActionLabel(machineLabel, target)
    Surface(
        // One ground for both states, at the 12% the delta specifies. Not
        // the chip register's 18%: that puts the label at 4.43:1 over a card,
        // under §14's 4.5 floor and below the 5.62:1 the plain TextButton had.
        // §12's disabled container is also 12%, so the two coincide and the
        // ground simply does not move — the state is carried by the hairline
        // and the label, both of which read with opacity ignored as §12 asks.
        // Written as a literal rather than DisabledAlpha.Container, whose name
        // would claim this fill means "disabled" on an enabled control.
        color = failure.copy(alpha = 0.12f),
        border = if (enabled) BorderStroke(1.dp, failure) else null,
        shape = NidavellirShapes.Cleft,
        modifier = modifier
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Cleft),
                enabled = enabled,
                // The TextButton this replaces carried Role.Button; a hand-built
                // Surface does not, and losing it makes the app's only
                // destructive control announce as a plain view to TalkBack.
                role = Role.Button,
                onClick = onClick,
            )
            // Every kill control on the grid speaks an identical bare "Kill"
            // today, so a screen-reader user cannot tell one from another.
            .semantics { contentDescription = spoken },
    ) {
        // The minimum target sits on this inner Box, not the outer chain:
        // Surface appends its background after the caller's modifiers, so
        // enforcing 48dp outside would leave the drawn chip smaller than its
        // own node and centred inside it, with the cut corners no longer on the
        // control's bounds.
        Box(
            modifier = Modifier.minimumInteractiveComponentSize().padding(horizontal = 12.dp),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = "Kill",
                // Disabled goes to Bone, not a dimmed Ember: Ember at 38% over
                // an Ember-tinted ground measures 1.82:1, where Bone holds
                // 3.21:1 — the legibility the TextButton had. The hue change
                // is itself a second non-opacity cue.
                color = if (enabled) failure else Bone.copy(alpha = NidavellirMotion.DisabledAlpha.Content),
                style = MaterialTheme.typography.labelLarge,
            )
        }
    }
}
