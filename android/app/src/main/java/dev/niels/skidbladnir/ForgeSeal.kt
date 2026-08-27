package dev.niels.skidbladnir

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTag
import androidx.compose.ui.unit.dp

// Every top-level declaration in this file must stay JVM-safe: reading
// `UnstruckMark` from the pure-JVM `ForgeSealTest` runs this file's class
// initialiser, and a top-level Path, Paint, or Stroke is android.graphics-backed
// and would fail that proof for a reason unrelated to the mark. Such things live
// inside the composable's DrawScope.

// The unstruck seal's mark (design-language.md §8, forge-seal.md "Geometry"):
// the §11 seal with every trait at zero — a bare stave crossed by one
// horizontal bar and nothing else — in unit fractions of the control's square.
// The bar is perpendicular to the stave, which no Younger Futhark branch ever
// is, so the mark cannot be read as a rune. Sole owner of the frozen fractions.
internal val UnstruckMark: List<Pair<Offset, Offset>> = listOf(
    Offset(0.5f, 0.23f) to Offset(0.5f, 0.77f),
    Offset(0.5f - 0.19f, 0.5f) to Offset(0.5f + 0.19f, 0.5f),
)

// The Forge seal (design-language.md §13, forge-seal.md): the dashboard's
// create control, a 56dp octagon carrying the unstruck mark. Lit in ForgeGlow
// and Gold when some machine can forge, cold stone when none can — field and
// hue both move, never opacity alone (§12). The flip is instant: §12 budgets
// the app one ambient animation and the Forge sheet spends it. 56dp clears the
// §14 target floor on its own, so no `minimumInteractiveComponentSize()`.
//
// It takes no `modifier`: the control is exactly its own 56dp square, so its
// semantics bounds and its touch target are one rectangle. (The frame is
// stroked on that rectangle's edges, so its outer half — 0.75dp — overhangs
// them.) Placement is the caller's, in a padded wrapper; padding threaded
// through here is a layout modifier on this same node, and would report
// semantics bounds 32dp larger than the seal a user can see.
@Composable
internal fun ForgeSeal(canForge: Boolean, onClick: () -> Unit) {
    val field = if (canForge) ForgeGlow else DeepSurface
    val metal = if (canForge) Gold else Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Content)
    Canvas(
        modifier = Modifier
            .size(56.dp)
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = AngularIndication(NidavellirShapes.Octagon),
                enabled = canForge,
                role = Role.Button,
                onClick = onClick,
            )
            .semantics {
                testTag = "new-agent"
                contentDescription = "New dwarf"
            },
    ) {
        // Filled and then stroked unclipped, unlike DwarfPortrait: the seal has
        // no mineral ground behind its frame, so clipping to the octagon would
        // leave half of the 1.5dp hairline and go weak against Ink
        // (forge-seal.md, "Geometry"). Same vertices, so the frame and the clip
        // are still one geometry.
        val vertices = octagonVertices(size)
        val octagon = Path().apply {
            moveTo(vertices.first().x, vertices.first().y)
            vertices.drop(1).forEach { lineTo(it.x, it.y) }
            close()
        }
        drawPath(octagon, field)
        drawPath(
            octagon,
            metal,
            style = Stroke(width = 1.5.dp.toPx(), cap = StrokeCap.Butt, join = StrokeJoin.Miter),
        )
        UnstruckMark.forEach { (start, end) ->
            drawLine(
                color = metal,
                start = Offset(start.x * size.width, start.y * size.height),
                end = Offset(end.x * size.width, end.y * size.height),
                strokeWidth = 3.dp.toPx(),
                cap = StrokeCap.Butt,
            )
        }
    }
}
