package dev.niels.skidbladnir

import android.animation.ValueAnimator
import android.window.OnBackInvokedCallback
import android.window.OnBackInvokedDispatcher
import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// Stone strata and accents: the app's only colour definitions.
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

// Corners are cut, never rounded.
internal object NidavellirShapes {
    val Card = CutCornerShape(10.dp)
    val Chip = CutCornerShape(4.dp)
    val Sheet = CutCornerShape(topStart = 12.dp, topEnd = 12.dp)

    // 29% cut, the sole owner of the octagon; octagonVertices expands the same number.
    const val OctagonCutPercent = 29
    val Octagon = CutCornerShape(OctagonCutPercent)

    // The commit control of a destructive confirmation, and nothing else.
    val Cleft = CutCornerShape(topStart = 14.dp, topEnd = 4.dp, bottomEnd = 4.dp, bottomStart = 4.dp)
}

// Display carries the wordmark, titles and dwarf names; Data carries machine facts; body text is the system face.
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

// Effects motion never bounces; the forge warm-in is the one ambient animation.
internal object NidavellirMotion {
    private val StandardEasing = CubicBezierEasing(0.2f, 0f, 0f, 1f)

    val EffectsTween: FiniteAnimationSpec<Float> = tween(durationMillis = 100, easing = StandardEasing)
    val ForgeWarmIn: FiniteAnimationSpec<Color> = tween(durationMillis = 400)

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

// FilterChip reads small. Sheets and dialogs pass their shapes: the drag handle reads extraLarge and must stay a pill.
private val NidavellirMaterialShapes = Shapes(small = NidavellirShapes.Chip)

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
            error = noticeToneColor(NoticeTone.Failure), // read only by M3 internals, such as text-field errors
            surfaceContainerLow = DeepSurface, // ModalBottomSheet
            surfaceContainerHigh = DeepSurface, // AlertDialog
            surfaceContainer = RaisedSurface, // DropdownMenu
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
        ValueAnimator.registerDurationScaleChangeListener(listener)
        durationScale = ValueAnimator.getDurationScale()
        onDispose { ValueAnimator.unregisterDurationScaleChangeListener(listener) }
    }
    return durationScale != 0f
}

// Severity, the one owner of notice colour.
internal enum class NoticeTone { Failure, Degraded, Armed }

internal fun noticeToneColor(tone: NoticeTone): Color = when (tone) {
    NoticeTone.Failure -> Ember // an attempt failed, trust broke, or you are ending something
    NoticeTone.Degraded -> Muted // knowledge is absent or old; nothing is broken
    NoticeTone.Armed -> Gold // a recovery is waiting on you
}

// The panel for every failure, degradation and armed recovery. Never ornamented.
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

/** A sheet's Display title. */
@Composable
internal fun SheetTitle(text: String, modifier: Modifier = Modifier) =
    Text(
        text,
        modifier,
        style = MaterialTheme.typography.headlineSmall,
        fontFamily = NidavellirType.Display,
        fontWeight = FontWeight.SemiBold,
    )

/** The body of a form sheet on the sheets' 20dp edge; everything scrolls. */
@Composable
internal fun SheetForm(content: @Composable ColumnScope.() -> Unit) = Column(
    Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp).padding(bottom = 28.dp),
    verticalArrangement = Arrangement.spacedBy(12.dp),
    content = content,
)

/** A form sheet's body with [actions] pinned below the scrolling content; suggestion lists get a third of its height. */
@Composable
internal fun SheetForm(
    actions: @Composable RowScope.() -> Unit,
    content: @Composable ColumnScope.(suggestionBudget: Dp) -> Unit,
) = Column(
    Modifier.fillMaxWidth().padding(horizontal = 20.dp).padding(bottom = 28.dp),
    verticalArrangement = Arrangement.spacedBy(12.dp),
) {
    BoxWithConstraints(Modifier.weight(1f, fill = false)) {
        val suggestionBudget = maxHeight / 3
        Column(
            Modifier.fillMaxWidth().verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) { content(suggestionBudget) }
    }
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End, content = actions)
}
