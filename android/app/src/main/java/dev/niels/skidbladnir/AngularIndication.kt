package dev.niels.skidbladnir

import androidx.compose.animation.core.Animatable
import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.drawOutline
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch

// The dwarven press flash (design-language.md §12): a circular ripple is
// off-grammar, so this draws an inset copy of the component's own cut-corner
// outline at Bone, fading in and out at the pressed state-layer alpha. One
// shared implementation for the whole app; no configuration surface — see
// docs/chrome-tokens.md "Interaction states".
internal object AngularIndication : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode =
        AngularIndicationNode(interactionSource)

    override fun equals(other: Any?): Boolean = other === this

    override fun hashCode(): Int = System.identityHashCode(this)
}

private val PressInset = 2.dp

private class AngularIndicationNode(
    private val interactionSource: InteractionSource,
) : Modifier.Node(), DrawModifierNode {
    private val alpha = Animatable(0f)

    override fun onAttach() {
        coroutineScope.launch {
            interactionSource.interactions.collect { interaction ->
                when (interaction) {
                    is PressInteraction.Press ->
                        alpha.animateTo(NidavellirMotion.StateLayer.Pressed, NidavellirMotion.EffectsTween)
                    is PressInteraction.Release, is PressInteraction.Cancel ->
                        alpha.animateTo(0f, NidavellirMotion.EffectsTween)
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
            val outline = NidavellirShapes.Card.createOutline(insetSize, layoutDirection, this)
            translate(left = insetPx, top = insetPx) {
                drawOutline(outline, color = Bone, alpha = current)
            }
        }
    }
}
