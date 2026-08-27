package dev.niels.skidbladnir

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.DurationBasedAnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.tween
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
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
