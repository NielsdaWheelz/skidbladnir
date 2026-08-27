package dev.niels.skidbladnir

import kotlin.math.abs
import kotlin.math.hypot
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

// The Hlíðskjálf mark's legibility is a property of its geometry measured
// against its own stroke, never of the size it happens to be drawn at:
// `drawValknut` scales the stroke with the mark (`ValknutStrokeRatio`), so both
// facts below either hold at every rendered size or at none of them.
//
// The pre-0.36 generator gap failed both. Its narrowest break was 0.0555 of the
// unit box and its shortest strand 0.0286 — against a 2dp stroke on the 48dp
// empty state that left 0.34dp of clearance per crossing, about one physical
// pixel on the S22+, and a strand wider than it was long. The weave was
// invisible on the panel it shipped to. These are the proofs that caught it.
class OrnamentTest {
    @Test
    fun `every valknut strand is at least as long as its own stroke is wide`() {
        val shortest = Valknut.minOf { hypot(it.x2 - it.x1, it.y2 - it.y1) }

        assertTrue(
            "a strand shorter than the stroke is wide renders as a speck rather than a line; " +
                "shortest strand is $shortest of the unit box against a stroke of $ValknutStrokeRatio",
            shortest >= ValknutStrokeRatio,
        )
    }

    @Test
    fun `every baked crossing break is wider than the strand that crosses it`() {
        val breaks = bakedBreaks()

        assertEquals(
            "the valknut is a weave, and how much of it still weaves is the point: of its six " +
                "crossings, three bake an interior break and three cut out to a vertex instead. " +
                "A wider gap consumes interior breaks — the count is not monotonic in the gap — " +
                "so a future `_VALKNUT_GAP` could satisfy the width bound below while leaving " +
                "almost nothing woven. Pinned: changing this is a design event that reopens " +
                "docs/hlidskjalf-mark.md, never a silent tweak.",
            ExpectedBakedBreaks,
            breaks.size,
        )
        val narrowest = breaks.min()
        assertTrue(
            "a break narrower than twice the stroke leaves no clear ground beside the strand " +
                "crossing it, so the crossing reads as a solid join instead of a weave; " +
                "narrowest break is $narrowest of the unit box against a stroke of $ValknutStrokeRatio",
            narrowest >= 2 * ValknutStrokeRatio,
        )
    }
}

// A break is the gap between the two surviving pieces of one edge that
// `scripts/gen-ornament` cut at a crossing. Both pieces lie on that edge's own
// line, so same-line pairs are exactly the broken edges, and the gap is the
// distance between their facing endpoints.
private fun bakedBreaks(): List<Float> = Valknut.flatMapIndexed { index, segment ->
    Valknut.drop(index + 1).filter { segment.sharesLineWith(it) }.map { other ->
        segment.endpoints().nearestDistanceTo(other.endpoints())
    }
}

// Interior breaks in the checked-in geometry at `_VALKNUT_GAP` = 0.36.
private const val ExpectedBakedBreaks = 3

private const val CollinearTolerance = 1e-4f

private fun OrnamentSegment.endpoints(): List<Pair<Float, Float>> = listOf(x1 to y1, x2 to y2)

private fun List<Pair<Float, Float>>.nearestDistanceTo(others: List<Pair<Float, Float>>): Float =
    minOf { (ax, ay) -> others.minOf { (bx, by) -> hypot(ax - bx, ay - by) } }

// True when `other` lies on this segment's own infinite line: both of its
// endpoints sit within tolerance of that line.
private fun OrnamentSegment.sharesLineWith(other: OrnamentSegment): Boolean {
    val dx = x2 - x1
    val dy = y2 - y1
    val length = hypot(dx, dy)
    return other.endpoints().all { (px, py) ->
        abs((px - x1) * dy - (py - y1) * dx) / length <= CollinearTolerance
    }
}
