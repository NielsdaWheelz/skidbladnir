package dev.niels.skidbladnir

import android.animation.ValueAnimator
import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Stone strata + accents (design-language.md §5). Sole owner: NidavellirTheme's
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
    // and 29% is struck into every existing seal (design-language.md §6). The
    // percent is the sole owner of the cut: `octagonVertices` expands the same
    // number the clip reads, so a frame and its clip can never disagree.
    const val OctagonCutPercent = 29
    val Octagon = CutCornerShape(OctagonCutPercent)

    // The only asymmetric shape in the product: the chip facet is 4dp and the
    // cleft corner is 14dp, so the control reads as material cleaved off
    // rather than softened. The commit control of a destructive confirmation
    // only (docs/session-card.md). Named "Cleft" and not
    // "Struck" because the seal vocabulary reserves struck/unstruck for
    // minted-vs-blank, and one word cannot mean both.
    val Cleft = CutCornerShape(topStart = 14.dp, topEnd = 4.dp, bottomEnd = 4.dp, bottomStart = 4.dp)
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

private val NidavellirTypography = Typography().let { base ->
    base.copy(
        displayLarge = base.displayLarge.copy(fontFamily = NidavellirType.Display),
        headlineLarge = base.headlineLarge.copy(fontFamily = NidavellirType.Display),
        titleLarge = base.titleLarge.copy(fontFamily = NidavellirType.Display),
    )
}

private val NidavellirMaterialShapes = Shapes(
    small = NidavellirShapes.Chip,
    medium = NidavellirShapes.Card,
    large = NidavellirShapes.Sheet,
)

@Composable
internal fun NidavellirTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = Gold,
            onPrimary = Ink,
            secondary = Frost,
            background = Ink,
            onBackground = Bone,
            surface = DeepSurface,
            onSurface = Bone,
            surfaceVariant = RaisedSurface,
            onSurfaceVariant = Muted,
            // Unread by app code; the slot stays for M3-internal error
            // state such as text fields (destructive-chrome.md).
            error = noticeToneColor(NoticeTone.Failure),
        ),
        shapes = NidavellirMaterialShapes,
        typography = NidavellirTypography,
        content = content,
    )
}

internal fun sessionStatusColor(tone: SessionStatusTone): Color = when (tone) {
    SessionStatusTone.Working -> Frost
    SessionStatusTone.Ready -> Moss
    SessionStatusTone.Attention -> Ember
    SessionStatusTone.Muted -> Muted
}

/** One observed Android motion setting shared by every state-bound animation. */
@Composable
internal fun rememberMotionEnabled(): Boolean {
    var durationScale by remember { mutableFloatStateOf(ValueAnimator.getDurationScale()) }
    DisposableEffect(Unit) {
        val listener = ValueAnimator.DurationScaleChangeListener { durationScale = it }
        check(ValueAnimator.registerDurationScaleChangeListener(listener)) {
            "Animator duration-scale listener registration failed"
        }
        durationScale = ValueAnimator.getDurationScale()
        onDispose {
            check(ValueAnimator.unregisterDurationScaleChangeListener(listener)) {
                "Animator duration-scale listener unregistration failed"
            }
        }
    }
    return durationScale != 0f
}


// One owner for severity (destructive-chrome.md). Ember carried five
// unrelated jobs — destructive control, failed attempt, stale data, host load,
// corrupt record — and because one federated host is routinely stale in normal
// operation, Ember had become the dashboard's resting state and stopped
// reading as an alarm. Degraded is Muted because staleness is ABSENCE, not
// failure: it matches the muted epistemic-state treatment (design-language.md §5) and the honesty
// law (§1.4), absence is displayed rather than alarmed. No new hue is
// introduced, so §5's four-accent-family cap still holds.
internal enum class NoticeTone { Failure, Degraded, Armed }

internal fun noticeToneColor(tone: NoticeTone): Color = when (tone) {
    NoticeTone.Failure -> Ember // an attempt failed, trust broke, or you are ending something
    NoticeTone.Degraded -> Muted // knowledge is absent or old; nothing is broken
    NoticeTone.Armed -> Gold // a recovery is waiting on you
}

// One panel for every failure, degradation, and armed recovery
// (destructive-chrome.md). Three hand-rolled banner Surfaces each picked their
// own colour, which is how Ember came to mean five unrelated things; here the
// tone is the only input and noticeToneColor is its sole owner.
// It takes no `modifier` on purpose: all three consumers applied the identical
// outer geometry, so that geometry is the duplication being removed, and a
// parameter no call site varies is what docs/rules.md forbids.
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

/** Back for an in-window modal: an overlay-priority callback that outranks the activity's handlers. */
@Composable
internal fun ModalBackHandler(onBack: () -> Unit) {
    val currentOnBack by rememberUpdatedState(onBack)
    val view = LocalView.current
    DisposableEffect(view) {
        val dispatcher = checkNotNull(view.findOnBackInvokedDispatcher()) {
            "modal must be attached to a Back dispatcher"
        }
        val callback = OnBackInvokedCallback { currentOnBack() }
        dispatcher.registerOnBackInvokedCallback(
            OnBackInvokedDispatcher.PRIORITY_OVERLAY,
            callback,
        )
        onDispose { dispatcher.unregisterOnBackInvokedCallback(callback) }
    }
}

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
// so the fret is the app's only repeating band. Ornament.kt's frozen constants
// bake all topology and over/under gaps — this is the whole runtime drawing
// step, no weave logic here.
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

// The stroke that draws the Hlíðskjálf mark, as a fraction of the mark's own
// size. A fixed dp stroke is the defect it replaces: 2dp is 4% of a 48dp mark
// but 11% of an 18dp one, so as the mark shrinks the stroke swallows the
// crossing gaps baked into `Valknut` and the weave reads as a solid clot. Held
// against that gap (0.36 of each crossed edge, hlidskjalf-mark.md), this keeps
// break and strand in the same proportion at every rendered size
// (design-language.md §8).
internal const val ValknutStrokeRatio = 0.055f

// The Hlíðskjálf mark (design-language.md §8): a single, non-repeating draw
// of the frozen `Valknut` segments. Not a band — it does not tile — so it
// keeps its own path rather than joining `drawFretBand`, whose whole job is
// tiling a unit cell across a width.
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
// subordinate").
@Composable
internal fun HlidskjalfMark(color: Color, markSize: Dp, modifier: Modifier = Modifier) {
    Canvas(
        modifier = modifier
            .size(markSize)
            .clearAndSetSemantics {},
    ) {
        drawValknut(color)
    }
}
