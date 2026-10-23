package dev.niels.skidbladnir

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import java.nio.charset.StandardCharsets
import java.security.MessageDigest

/** One straight stroke in a unit box (rune segments: x in stave widths from the stave, y in 0..1 of its height). */
private data class Segment(val x0: Float, val y0: Float, val x1: Float, val y1: Float)

/** Butt-capped strokes of [segments] scaled into the full-height column at [left] that is [width] wide. */
private fun DrawScope.drawSegments(
    segments: List<Segment>,
    color: Color,
    strokeWidth: Float,
    left: Float = 0f,
    width: Float = size.width,
) = segments.forEach {
    drawLine(
        color,
        Offset(left + it.x0 * width, it.y0 * size.height),
        Offset(left + it.x1 * width, it.y1 * size.height),
        strokeWidth,
        StrokeCap.Butt,
    )
}

// Frozen: one 45° chevron unit that tiles without a seam.
private val FretCell: List<Segment> = listOf(
    Segment(0f, 1f, 0.5f, 0f),
    Segment(0.5f, 0f, 1f, 1f),
)

// Frozen: the tricursal valknut, its six over/under crossings baked as gaps.
private val Valknut: List<Segment> = listOf(
    Segment(0.364125f, 0.043864f, 0.086771f, 0.524255f),
    Segment(0.086771f, 0.524255f, 0.358141f, 0.524255f),
    Segment(0.641479f, 0.524255f, 0.364125f, 0.043864f),
    Segment(0.172912f, 0.845739f, 0.72762f, 0.845739f),
    Segment(0.72762f, 0.845739f, 0.549924f, 0.53796f),
    Segment(0.450266f, 0.365348f, 0.408445f, 0.437785f),
    Segment(0.308597f, 0.610726f, 0.172912f, 0.845739f),
    Segment(0.962963f, 0.610397f, 0.685609f, 0.130005f),
    Segment(0.685609f, 0.130005f, 0.599657f, 0.278878f),
    Segment(0.49981f, 0.451819f, 0.408255f, 0.610397f),
    Segment(0.408255f, 0.610397f, 0.491898f, 0.610397f),
    Segment(0.691593f, 0.610397f, 0.962963f, 0.610397f),
)

// Frozen: the 16 long-branch Younger Futhark glyphs in index order; íss is the bare stave.
private val RuneSegments: List<List<Segment>> = listOf(
    // fé
    listOf(Segment(0f, .14f, .30f, .02f), Segment(0f, .30f, .30f, .18f)),
    // úr
    listOf(Segment(0f, .10f, .30f, .28f), Segment(.30f, .28f, .30f, .92f)),
    // þurs
    listOf(Segment(0f, .26f, .28f, .44f), Segment(.28f, .44f, 0f, .62f)),
    // óss
    listOf(Segment(0f, .14f, .30f, .30f), Segment(0f, .30f, .30f, .46f)),
    // reið
    listOf(
        Segment(0f, .14f, .26f, .28f),
        Segment(.26f, .28f, 0f, .42f),
        Segment(0f, .42f, .28f, .68f),
    ),
    // kaun
    listOf(Segment(0f, .22f, .30f, .46f)),
    // hagall
    listOf(Segment(-.26f, .32f, .26f, .54f), Segment(-.26f, .54f, .26f, .32f)),
    // nauð
    listOf(Segment(-.26f, .34f, .26f, .54f)),
    // íss (bare stave)
    emptyList(),
    // ár
    listOf(Segment(-.26f, .54f, .26f, .34f)),
    // sól
    listOf(
        Segment(0f, .28f, .24f, .40f),
        Segment(.24f, .40f, .04f, .52f),
        Segment(.04f, .52f, .28f, .64f),
    ),
    // týr
    listOf(Segment(-.26f, .28f, 0f, .10f), Segment(0f, .10f, .26f, .28f)),
    // bjarkan
    listOf(
        Segment(0f, .14f, .28f, .28f),
        Segment(.28f, .28f, 0f, .42f),
        Segment(0f, .46f, .28f, .60f),
        Segment(.28f, .60f, 0f, .74f),
    ),
    // maðr
    listOf(Segment(0f, .18f, -.28f, .02f), Segment(0f, .18f, .28f, .02f)),
    // lǫgr
    listOf(Segment(0f, .10f, .30f, .26f)),
    // ýr
    listOf(Segment(0f, .80f, -.28f, .96f), Segment(0f, .80f, .28f, .96f)),
)

// Frozen: the unstruck seal, a bare stave crossed by a horizontal bar no rune branch can be.
private val UnstruckMark: List<Segment> = listOf(
    Segment(0.5f, 0.23f, 0.5f, 0.77f),
    Segment(0.5f - 0.19f, 0.5f, 0.5f + 0.19f, 0.5f),
)

// Frozen: fill-only mineral tones in index order.
private val SealMinerals: List<Color> = listOf(
    Color(0xFF830E0D), // 0 Garnet
    Color(0xFF6D2114), // 1 Hematite
    Color(0xFF26619C), // 2 BlueGlass
    Color(0xFF2A2D33), // 3 Basalt
    Color(0xFF333B34), // 4 MossStone
    Color(0xFF43392C), // 5 BronzeStone
    Color(0xFF3A2C33), // 6 Porphyry
    Color(0xFF2E3A3D), // 7 Slate
)

private data class SealSpec(
    val mineral: Int,
    val runes: List<Int>,
    val beardTeeth: Int,
    val beardDepthStep: Int,
    val facetMask: Int,
    val metal: Color, // Gold or Bronze
)

// Frozen formulas over SHA-256(UTF-8(key)), bytes unsigned (design-language.md §11): changing any re-rolls
// every identity. A draw with fewer distinct runes than runeCount is defined, not an error.
private fun sealSpec(characterKey: String): SealSpec {
    val digest = MessageDigest.getInstance("SHA-256").digest(characterKey.toByteArray(StandardCharsets.UTF_8))
    fun byteAt(index: Int): Int = digest[index].toInt() and 0xFF
    val runeCount = 2 + (byteAt(4) and 1)
    val runes = mutableListOf<Int>()
    for (i in 0..7) {
        val candidate = byteAt(5 + i) % 16
        if (candidate !in runes) runes += candidate
        if (runes.size == runeCount) break
    }
    return SealSpec(
        mineral = byteAt(0) % 8,
        runes = runes,
        beardTeeth = 3 + (byteAt(8) % 4),
        beardDepthStep = byteAt(9) % 4,
        facetMask = byteAt(12),
        metal = if ((byteAt(14) and 1) == 1) Gold else Bronze,
    )
}

/** The octagon's eight vertices in edge order from the top-left cut; facetMask indexes this order. */
private fun octagonVertices(size: Size): List<Offset> {
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

/** Whole fret units of the band's height, centred across its width. */
internal fun DrawScope.drawFretBand(color: Color) {
    val unitWidth = size.height
    val units = (size.width / unitWidth).toInt().coerceAtLeast(1)
    val startX = (size.width - units * unitWidth) / 2f
    val strokeWidth = size.height * 0.10f
    for (unit in 0 until units) {
        val offsetX = startX + unit * unitWidth
        drawSegments(FretCell, color, strokeWidth, left = offsetX, width = unitWidth)
    }
}

/** The Hlíðskjálf mark; decoration only, so the label beside it carries the meaning. */
@Composable
internal fun HlidskjalfMark(color: Color, markSize: Dp, modifier: Modifier = Modifier) {
    // The stroke scales with the mark so the baked crossing gaps survive at every size.
    Canvas(modifier.size(markSize)) { drawSegments(Valknut, color, size.minDimension * 0.055f) }
}

/** A dwarf's seal. Draw order is frozen: mineral, facets, beard, bind-rune, frame, initial. */
@Composable
internal fun DwarfPortrait(character: CharacterSummary) {
    val spec = sealSpec(character.key)
    val label = character.displayName.take(1).uppercase()
    val density = LocalDensity.current
    Box(
        modifier = Modifier
            .size(36.dp)
            .clip(NidavellirShapes.Octagon),
        contentAlignment = Alignment.Center,
    ) {
        Canvas(Modifier.fillMaxSize()) {
            val w = size.width
            val h = size.height
            val side = size.minDimension

            drawRect(SealMinerals[spec.mineral])

            drawPath(
                Path().apply {
                    moveTo(0f, 0f)
                    lineTo(w, 0f)
                    lineTo(0f, h)
                    close()
                },
                Color.White.copy(alpha = 0.045f),
            )
            drawPath(
                Path().apply {
                    moveTo(w, h * 0.55f)
                    lineTo(w, h)
                    lineTo(w * 0.35f, h)
                    close()
                },
                Color.Black.copy(alpha = 0.16f),
            )

            val beardTopY = h * 0.60f
            val beardLeftX = w * 0.24f
            val beardRightX = w * 0.76f
            val valleyY = h * 0.88f
            val tipY = valleyY - (0.10f + spec.beardDepthStep * 0.022f) * h
            val toothSpan = spec.beardTeeth - 1
            val toothWidth = (beardRightX - beardLeftX) / toothSpan
            val beardPath = Path().apply {
                moveTo(beardLeftX, beardTopY)
                lineTo(beardRightX, beardTopY)
                lineTo(beardRightX, tipY)
                for (tooth in 1..toothSpan) {
                    lineTo(beardRightX - (tooth - 0.5f) * toothWidth, valleyY)
                    lineTo(beardRightX - tooth * toothWidth, tipY)
                }
                close()
            }
            drawPath(beardPath, Color.Black.copy(alpha = 0.34f))
            drawPath(
                beardPath,
                Color.White.copy(alpha = 0.10f),
                style = Stroke(width = 1f, cap = StrokeCap.Butt, join = StrokeJoin.Miter),
            )

            val staveX = w * 0.5f
            val staveTop = h * 0.15f
            val staveBottom = h * 0.58f
            val runeWidth = w * 0.30f
            val bindRune = Path().apply {
                moveTo(staveX, staveTop)
                lineTo(staveX, staveBottom)
                spec.runes.forEach { rune ->
                    RuneSegments[rune].forEach { seg ->
                        moveTo(staveX + seg.x0 * runeWidth, staveTop + seg.y0 * (staveBottom - staveTop))
                        lineTo(staveX + seg.x1 * runeWidth, staveTop + seg.y1 * (staveBottom - staveTop))
                    }
                }
            }
            drawPath(
                bindRune,
                spec.metal,
                style = Stroke(width = side * 0.045f, cap = StrokeCap.Butt, join = StrokeJoin.Miter),
            )

            // Two passes: every grey edge, then Gold on facetMask's edges, so no grey end overdraws a Gold one.
            val vertices = octagonVertices(size)
            for (edge in vertices.indices) {
                drawLine(
                    color = Color(0xFF3A3E45),
                    start = vertices[edge],
                    end = vertices[(edge + 1) % vertices.size],
                    strokeWidth = side * 0.012f,
                    cap = StrokeCap.Butt,
                )
            }
            for (edge in vertices.indices) {
                if ((spec.facetMask shr edge) and 1 == 1) {
                    drawLine(
                        color = Gold,
                        start = vertices[edge],
                        end = vertices[(edge + 1) % vertices.size],
                        strokeWidth = side * 0.025f,
                        cap = StrokeCap.Butt,
                    )
                }
            }
        }
        Text(
            text = label,
            color = Bone,
            fontFamily = NidavellirType.Display,
            fontWeight = FontWeight.Black,
            modifier = Modifier.align(Alignment.BottomEnd).padding(6.dp),
            // The initial is part of the seal; the neighboring full name scales with text settings.
            fontSize = with(density) { 12.dp.toSp() },
            lineHeight = with(density) { 16.dp.toSp() },
            style = MaterialTheme.typography.labelMedium,
        )
    }
}

/**
 * The dashboard's create control: a 56dp octagon with the unstruck mark, lit when some machine can forge.
 * No modifier: the 56dp square is its touch target and semantics bounds.
 */
@Composable
internal fun ForgeSeal(canForge: Boolean, onClick: () -> Unit) {
    val field = if (canForge) ForgeGlow else DeepSurface
    val metal = if (canForge) Gold else Muted.copy(alpha = NidavellirMotion.DisabledAlpha.Content)
    Canvas(
        modifier = Modifier
            .size(56.dp)
            .clickable(
                interactionSource = null,
                indication = AngularIndication(NidavellirShapes.Octagon),
                enabled = canForge,
                onClick = onClick,
            )
            .semantics {
                contentDescription = "New dwarf"
            },
    ) {
        // Unclipped, unlike DwarfPortrait: with no mineral ground, a clip would halve the hairline.
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
        drawSegments(UnstruckMark, metal, 3.dp.toPx())
    }
}
