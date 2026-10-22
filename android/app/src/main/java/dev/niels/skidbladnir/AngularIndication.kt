package dev.niels.skidbladnir

import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.drawOutline
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch

// The press flash: an inset Bone outline of the caller's own shape, never a ripple.
// A data class so inline construction keeps `clickable` stable.
internal data class AngularIndication(val shape: Shape) : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode =
        AngularIndicationNode(interactionSource, shape)
}

private val PressInset = 2.dp
private const val PressedAlpha = 0.10f

private class AngularIndicationNode(
    private val interactionSource: InteractionSource,
    private val shape: Shape,
) : Modifier.Node(), DrawModifierNode {
    private val alpha = Animatable(0f)

    override fun onAttach() {
        coroutineScope.launch {
            // Keyed by press, so a synthetic press/release pair cannot end a held physical press.
            val presses = mutableSetOf<PressInteraction.Press>()
            interactionSource.interactions.collect { interaction ->
                val wasPressed = presses.isNotEmpty()
                when (interaction) {
                    is PressInteraction.Press -> presses.add(interaction)
                    is PressInteraction.Release -> presses.remove(interaction.press)
                    is PressInteraction.Cancel -> presses.remove(interaction.press)
                }
                val pressed = presses.isNotEmpty()
                if (pressed != wasPressed) {
                    launch {
                        alpha.animateTo(
                            if (pressed) PressedAlpha else 0f,
                            NidavellirMotion.EffectsTween,
                        )
                    }
                }
            }
        }
    }

    override fun ContentDrawScope.draw() {
        drawContent()
        val current = alpha.value
        if (current > 0f) {
            val insetPx = PressInset.toPx()
            val insetSize = Size(size.width - insetPx * 2f, size.height - insetPx * 2f)
            val outline = shape.createOutline(insetSize, layoutDirection, this)
            translate(left = insetPx, top = insetPx) {
                drawOutline(outline, color = Bone, alpha = current)
            }
        }
    }
}
