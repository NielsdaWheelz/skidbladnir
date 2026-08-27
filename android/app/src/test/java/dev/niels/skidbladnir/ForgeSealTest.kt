package dev.niels.skidbladnir

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.unit.Density
import kotlin.math.abs
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ForgeSealTest {
    // octagonVertices maps a unit-box cut onto real pixels, so it is proved on
    // the one side the seal ships at (forge-seal.md "Placement and semantics")
    // rather than on a unit box: a fraction-where-pixels-were-meant bug in it
    // cannot hide behind a side of 1.
    private val side = 56f
    private val tolerance = 1e-4f

    @Test
    fun `the unstruck mark is a stave crossed perpendicular, which no rune branch ever is`() {
        // The whole table at once, in unit fractions: a swept-and-rejected variant
        // (BarHalfWidth 0.27, or BarY moved off centre) is a different mark, and
        // clearance minima alone cannot tell it from the shipped one.
        assertEquals(
            "the frozen geometry table (forge-seal.md \"Geometry\"): stave 0.23..0.77 at x = 0.5, " +
                "crossbar at y = 0.50 spanning +/-0.19. Actual was $UnstruckMark",
            listOf(
                Offset(0.5f, 0.23f) to Offset(0.5f, 0.77f),
                Offset(0.5f - 0.19f, 0.5f) to Offset(0.5f + 0.19f, 0.5f),
            ),
            UnstruckMark,
        )
        // The counter-table the §8 argument rests on. Index 8 (íss) is the bare
        // stave and legitimately carries no segments; the other fifteen carry
        // thirty between them. Both counts are asserted so a later rune edit
        // cannot quietly narrow the domain this proof ranges over.
        // The domain guard: 16 is a real contract (Younger Futhark is a closed
        // alphabet) and non-emptiness stops an emptied table proving nothing.
        // The segment total is deliberately NOT pinned — adding a branch to a
        // rune is a dwarf-seals.md ornament edit with no bearing on the crossbar,
        // and it must not fail a proof about the Forge seal.
        assertEquals(
            "RuneSegments must still be the whole Younger Futhark table (dwarf-seals.md); " +
                "[glyphs, any segments] disagreed — per-glyph sizes were ${RuneSegments.map { it.size }}",
            listOf(16, true),
            listOf(RuneSegments.size, RuneSegments.any { it.isNotEmpty() }),
        )
        val horizontalBranches = RuneSegments.flatMapIndexed { rune, segments ->
            segments.filter { it.y0 == it.y1 }.map { "rune $rune: $it" }
        }
        assertEquals(
            "Runic branches are cut across the wood grain, never along it, so no RuneSegments " +
                "entry may be horizontal — that is exactly what makes the unstruck mark's crossbar " +
                "unreadable as a rune (design-language.md §8). Horizontal branches found: " +
                horizontalBranches,
            emptyList<String>(),
            horizontalBranches,
        )
    }

    @Test
    fun `the frame is the shipped octagon and no mark endpoint reaches an edge`() {
        // NidavellirShapes.Octagon's Outline is an android.graphics-backed Path
        // and does not resolve off-device, so the cut is read off the shape's
        // own corner size — still the shipped value, never a restated one
        // (design-language.md §6).
        val octagon = NidavellirShapes.Octagon
        val cuts = listOf(octagon.topStart, octagon.topEnd, octagon.bottomEnd, octagon.bottomStart)
            .map { corner -> corner.toPx(Size(side, side), Density(1f)) }
        assertTrue(
            "Whatever draws the octagon reads the same cut the clip does, on all four corners " +
                "(design-language.md §6) — a shape cut unevenly leaves the frame and the clip " +
                "disagreeing on the corners nobody read. NidavellirShapes.Octagon reported " +
                "[topStart, topEnd, bottomEnd, bottomStart] = ${cuts.map { it / side }} of the side",
            cuts.all { abs(it - 0.29f * side) <= tolerance },
        )
        val cut = cuts.first()

        val vertices = octagonVertices(Size(side, side))
        assertEquals(
            "octagonVertices is the single owner of the 29% expansion — the frame a user sees and " +
                "the clip that cuts it can no longer disagree (forge-seal.md \"Reuse and " +
                "consolidation\"). Actual was $vertices",
            listOf(
                Offset(cut, 0f),
                Offset(side - cut, 0f),
                Offset(side, cut),
                Offset(side, side - cut),
                Offset(side - cut, side),
                Offset(cut, side),
                Offset(0f, side - cut),
                Offset(0f, cut),
            ),
            vertices,
        )

        val clearances = UnstruckMark
            .flatMap { (start, end) -> listOf(start * side, end * side) }
            .map { endpoint ->
                endpoint to vertices.indices.minOf { edge ->
                    distanceToEdge(endpoint, vertices[edge], vertices[(edge + 1) % vertices.size])
                } / side
            }
        val report = clearances.joinToString { (endpoint, gap) -> "%s -> %.6f".format(endpoint, gap) }
        // Two things ride on this one number. It is the floor forge-seal.md
        // "Geometry" sets at 0.10 of the side — pinning the minimum subsumes
        // asserting the floor. And it is the fixed point that proves
        // distanceToEdge measures to segments: a point-to-vertex bug reads
        // 0.311 at the stave tips and would still clear 0.10 unnoticed. The
        // geometry itself is already pinned by the golden table above.
        assertEquals(
            "The frozen minimum endpoint-to-edge clearance is 0.230000, at both stave tips " +
                "(forge-seal.md \"Geometry\"). Clearances were $report",
            0.23f,
            clearances.minOf { (_, gap) -> gap },
            tolerance,
        )
    }

    // Point-to-segment, not point-to-vertex: a vertex-only check would pass a
    // mark that pokes through the middle of an edge.
    private fun distanceToEdge(point: Offset, start: Offset, end: Offset): Float {
        val edge = end - start
        val toPoint = point - start
        val along = ((toPoint.x * edge.x + toPoint.y * edge.y) / edge.getDistanceSquared()).coerceIn(0f, 1f)
        return (toPoint - edge * along).getDistance()
    }
}
