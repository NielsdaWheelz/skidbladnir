package dev.niels.skidbladnir

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.DurationBasedAnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.testTag
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Stone strata + accents (design-language.md §5). Sole owner: MainActivity's
// color scheme and every screen reference these as same-package top-level
// symbols. No duplicate definitions may exist elsewhere (hard cut).
internal val Ink = Color(0xFF0C0D0F)
internal val DeepSurface = Color(0xFF15171A)
internal val RaisedSurface = Color(0xFF202329)
internal val Bone = Color(0xFFF3F0E8)
internal val Muted = Color(0xFFAAA69D)
internal val Gold = Color(0xFFD6A85F)
internal val Ember = Color(0xFFE46C55)
internal val Moss = Color(0xFF76B082)
internal val Frost = Color(0xFF78A9C6)
internal val Bronze = Color(0xFFCD7F32)
internal val Orpiment = Color(0xFFE8B923)
internal val ForgeGlow = Color(0xFF28231A)

// Corners are cut, never rounded (design-language.md §6). Facet unit: cards
// 10dp, chips/keys 4dp, sheets 12dp on the top corners only.
internal object NidavellirShapes {
    val Card = CutCornerShape(10.dp)
    val Chip = CutCornerShape(4.dp)
    val Key = CutCornerShape(4.dp)
    val Sheet = CutCornerShape(topStart = 12.dp, topEnd = 12.dp)

    // The dwarf seal frame (design-language.md §11, dwarf-seals.md): equal
    // 29% corner cuts on a square produce a regular octagon.
    val Octagon = CutCornerShape(29)
}

// Two roles only in this delta (design-language.md §9): Display carries the
// wordmark, screen titles, and dwarf names; Data carries every machine fact.
// Body text stays the system face untouched. FontVariation is the documented
// variable-font axis API and requires this opt-in.
@OptIn(ExperimentalTextApi::class)
internal object NidavellirType {
    val Display = FontFamily(
        Font(
            resId = R.font.big_shoulders,
            weight = FontWeight(650),
            variationSettings = FontVariation.Settings(FontWeight(650), FontStyle.Normal),
        ),
    )
    val Data = FontFamily(
        Font(
            resId = R.font.jetbrains_mono,
            weight = FontWeight(500),
            variationSettings = FontVariation.Settings(FontWeight(500), FontStyle.Normal),
        ),
    )
}

// Motion and interaction states (design-language.md §12). Effects motion
// never bounces; the Forge warm-in is the app's one ambient animation. The
// spatial spring bounds stay documented values with no consumer — no delta
// animates layout yet — so they are not shipped as tokens.
internal object NidavellirMotion {
    private val StandardEasing = CubicBezierEasing(0.2f, 0f, 0f, 1f)

    val EffectsTween: FiniteAnimationSpec<Float> = tween(durationMillis = 100, easing = StandardEasing)
    val ForgeWarmIn: FiniteAnimationSpec<Color> = tween(durationMillis = 400)

    // One half of the attention lozenge's opacity pulse; reversed to loop.
    val AttentionPulse: DurationBasedAnimationSpec<Float> =
        tween(durationMillis = 800, easing = StandardEasing)

    // Material state-layer constants (design-language.md §12). Only the
    // consumed value ships; hover/focus/dragged join with their first consumer.
    object StateLayer {
        const val Pressed = 0.10f
    }

    object DisabledAlpha {
        const val Content = 0.38f
        const val Container = 0.12f
    }
}

// Moved from DashboardScreen.kt (was `private`, defective: Shell and Running
// both returned Frost). Now internal so injectivity is a pure JVM proof
// (ThemeTest.kt). Fix: Shell -> Bronze (design-language.md §5).
internal fun statusColor(kind: SessionStatusKind): Color = when (kind) {
    SessionStatusKind.Working -> Moss
    SessionStatusKind.Running -> Frost
    SessionStatusKind.Idle -> Gold
    SessionStatusKind.Shell -> Bronze
    SessionStatusKind.Unknown -> Muted
}

// The attention pulse renders static when the system disables animator scale
// (design-language.md §12; the WCAG 2.2.2 stop-mechanism note lives with the
// screens delta that consumes this).
internal fun attentionPulseEnabled(animatorDurationScale: Float): Boolean = animatorDurationScale != 0f

// Ornament rendering (design-language.md §7; ornament-pipeline.md's refactor
// note). Fret and interlace are both repeating bands, so they share this one
// tile-drawing path: quantize the drawn width down to a whole unit count,
// center the result, then repeat every layer's frozen unit-box segments
// across it. Ornament.kt bakes all topology and over/under gaps at generation
// time — this is the whole runtime drawing step, no weave logic here.
internal fun DrawScope.drawOrnamentBand(unitAspect: Float, layers: List<Pair<List<OrnamentSegment>, Color>>) {
    val unitWidth = size.height * unitAspect
    val units = (size.width / unitWidth).toInt().coerceAtLeast(1)
    val startX = (size.width - units * unitWidth) / 2f
    val strokeWidth = size.height * 0.10f
    for (unit in 0 until units) {
        val offsetX = startX + unit * unitWidth
        layers.forEach { (segments, color) ->
            segments.forEach { segment ->
                drawLine(
                    color = color,
                    start = Offset(offsetX + segment.x1 * unitWidth, segment.y1 * size.height),
                    end = Offset(offsetX + segment.x2 * unitWidth, segment.y2 * size.height),
                    strokeWidth = strokeWidth,
                    cap = StrokeCap.Butt,
                )
            }
        }
    }
}

// The stroke that draws the Hlíðskjálf mark, as a fraction of the mark's own
// size. A fixed dp stroke is the defect it replaces: 2dp is 4% of a 48dp mark
// but 11% of an 18dp one, so as the mark shrinks the stroke swallows the
// crossing gaps `scripts/gen-ornament` baked in and the weave reads as a solid
// clot. Held against `_VALKNUT_GAP`, this keeps break and strand in the same
// proportion at every rendered size (design-language.md §8; OrnamentTest).
internal const val ValknutStrokeRatio = 0.055f

// The Hlíðskjálf mark (design-language.md §8): a single, non-repeating draw
// of the frozen `Valknut` segments. Not a band — it does not tile — so it
// does not share `drawOrnamentBand` (ornament-pipeline.md's refactor note
// scopes the shared helper to the two repeating bands only).
internal fun DrawScope.drawValknut(color: Color) {
    val stroke = size.minDimension * ValknutStrokeRatio
    Valknut.forEach { segment ->
        drawLine(
            color = color,
            start = Offset(segment.x1 * size.width, segment.y1 * size.height),
            end = Offset(segment.x2 * size.width, segment.y2 * size.height),
            strokeWidth = stroke,
            cap = StrokeCap.Butt,
        )
    }
}

// Every rendering of the mark in the app. It is decoration wherever it appears:
// it clears its own subtree semantics so the literal label beside it carries
// the whole meaning (ornament-pipeline.md "Ornament is silent and
// subordinate"), and `tag` exists only so tests can prove that silence.
@Composable
internal fun HlidskjalfMark(color: Color, markSize: Dp, tag: String, modifier: Modifier = Modifier) {
    Canvas(
        modifier = modifier
            .size(markSize)
            .clearAndSetSemantics { testTag = tag },
    ) {
        drawValknut(color)
    }
}

// The one composition of the affordance that returns to the Dwarves grid, so
// its label and its mark cannot drift apart between the two screens that offer
// it (TerminalScreen's reconnect panel and MainActivity's bearer repair). The
// mark takes `LocalContentColor` rather than a fixed accent so it dims with the
// button when the button is disabled — a drawn glyph gets no disabled state for
// free (design-language.md §12). `tag` differs per screen only so each site's
// silence is provable.
@Composable
internal fun BackToDwarvesContent(tag: String) {
    HlidskjalfMark(
        color = LocalContentColor.current,
        markSize = 18.dp,
        tag = tag,
        modifier = Modifier.padding(end = ButtonDefaults.IconSpacing),
    )
    Text("Back to Dwarves")
}
