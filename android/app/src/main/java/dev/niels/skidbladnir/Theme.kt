package dev.niels.skidbladnir

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.DurationBasedAnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.tween
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
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
    // corner cuts on a square produce an octagon — not an exactly regular
    // one. The regular cut is (2-sqrt2)/2 ~ 29.29%; at the shipped 29% each
    // vertex sits 0.16dp off its regular position on a 56dp frame and the
    // axis edges run 0.4200 of the side against the diagonals' 0.4101 (23.52dp
    // against 22.97dp). Both are under the perceptual floor for a hairline,
    // and 29% is struck into every existing seal (design-language.md §6). The percent is the sole owner of the cut:
    // `octagonVertices` expands the same number the clip reads, so a frame
    // and its clip can never disagree.
    const val OctagonCutPercent = 29
    val Octagon = CutCornerShape(OctagonCutPercent)
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

// The octagon's eight vertices in edge order, starting at the top-left cut
// (design-language.md §6). Sole owner of the cut's expansion into pixels:
// `DwarfPortrait` strokes them inside its octagon clip and `ForgeSeal` fills
// and strokes them unclipped, so the frame a user sees and the shape that
// cuts it are one geometry (forge-seal.md, "Reuse and consolidation").
internal fun octagonVertices(size: Size): List<Offset> {
    val cut = size.minDimension * (NidavellirShapes.OctagonCutPercent / 100f)
    val w = size.width
    val h = size.height
    return listOf(
        Offset(cut, 0f),
        Offset(w - cut, 0f),
        Offset(w, cut),
        Offset(w, h - cut),
        Offset(w - cut, h),
        Offset(cut, h),
        Offset(0f, h - cut),
        Offset(0f, cut),
    )
}

// The fret band (design-language.md §7; ornament-pipeline.md's refactor
// note): quantize the drawn width down to a whole unit count, center the
// result, then repeat `FretCell`'s frozen unit-box segments across it. The
// interlace band this once shared a path with died with the pairing screen,
// so the fret is the app's only repeating band. Ornament.kt bakes all
// topology and over/under gaps at generation time — this is the whole
// runtime drawing step, no weave logic here.
internal fun DrawScope.drawFretBand(color: Color) {
    val unitWidth = size.height
    val units = (size.width / unitWidth).toInt().coerceAtLeast(1)
    val startX = (size.width - units * unitWidth) / 2f
    val strokeWidth = size.height * 0.10f
    for (unit in 0 until units) {
        val offsetX = startX + unit * unitWidth
        FretCell.forEach { segment ->
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

// The Hlíðskjálf mark (design-language.md §8): a single, non-repeating draw
// of the frozen `Valknut` segments. Not a band — it does not tile — so it
// keeps its own path rather than joining `drawFretBand`, whose whole job is
// tiling a unit cell across a width.
internal fun DrawScope.drawValknut(color: Color, strokeWidth: Dp = 2.dp) {
    val stroke = strokeWidth.toPx()
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
