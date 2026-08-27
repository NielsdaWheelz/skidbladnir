package dev.niels.skidbladnir

import androidx.compose.ui.graphics.Color
import java.nio.charset.StandardCharsets
import java.security.MessageDigest

// Deterministic seal traits (design-language.md §11; dwarf-seals.md owns the
// frozen formulas). Pure function of `character.key`: no caching, no
// randomness, no clock. A change to any formula below is a design event that
// re-rolls every user-visible identity — never a silent tweak.
internal enum class SealMetal { Gold, Bronze }

internal data class SealSpec(
    val mineral: Int,
    val runes: List<Int>,
    val beardTeeth: Int,
    val beardDepthStep: Int,
    val facetMask: Int,
    val metal: SealMetal,
)

// Fill-only mineral tones, frozen index order (dwarf-seals.md).
internal val SealMinerals: List<Color> = listOf(
    Color(0xFF830E0D), // 0 Garnet
    Color(0xFF6D2114), // 1 Hematite
    Color(0xFF26619C), // 2 BlueGlass
    Color(0xFF2A2D33), // 3 Basalt
    Color(0xFF333B34), // 4 MossStone
    Color(0xFF43392C), // 5 BronzeStone
    Color(0xFF3A2C33), // 6 Porphyry
    Color(0xFF2E3A3D), // 7 Slate
)

// One straight line segment of a Younger Futhark glyph, stave-relative: x in
// stave-widths from the shared vertical stave, y in 0..1 of stave height.
internal data class RuneSegment(val x0: Float, val y0: Float, val x1: Float, val y1: Float)

// The 16 Younger Futhark (long-branch) glyphs, frozen segment table
// (dwarf-seals.md); index order fé, úr, þurs, óss, reið, kaun, hagall, nauð,
// íss, ár, sól, týr, bjarkan, maðr, lǫgr, ýr. íss is the bare stave (no
// segments). Runes are ornament, never text (design-language.md §8).
internal val RuneSegments: List<List<RuneSegment>> = listOf(
    // fé
    listOf(RuneSegment(0f, .14f, .30f, .02f), RuneSegment(0f, .30f, .30f, .18f)),
    // úr
    listOf(RuneSegment(0f, .10f, .30f, .28f), RuneSegment(.30f, .28f, .30f, .92f)),
    // þurs
    listOf(RuneSegment(0f, .26f, .28f, .44f), RuneSegment(.28f, .44f, 0f, .62f)),
    // óss
    listOf(RuneSegment(0f, .14f, .30f, .30f), RuneSegment(0f, .30f, .30f, .46f)),
    // reið
    listOf(
        RuneSegment(0f, .14f, .26f, .28f),
        RuneSegment(.26f, .28f, 0f, .42f),
        RuneSegment(0f, .42f, .28f, .68f),
    ),
    // kaun
    listOf(RuneSegment(0f, .22f, .30f, .46f)),
    // hagall
    listOf(RuneSegment(-.26f, .32f, .26f, .54f), RuneSegment(-.26f, .54f, .26f, .32f)),
    // nauð
    listOf(RuneSegment(-.26f, .34f, .26f, .54f)),
    // íss (bare stave)
    emptyList(),
    // ár
    listOf(RuneSegment(-.26f, .54f, .26f, .34f)),
    // sól
    listOf(
        RuneSegment(0f, .28f, .24f, .40f),
        RuneSegment(.24f, .40f, .04f, .52f),
        RuneSegment(.04f, .52f, .28f, .64f),
    ),
    // týr
    listOf(RuneSegment(-.26f, .28f, 0f, .10f), RuneSegment(0f, .10f, .26f, .28f)),
    // bjarkan
    listOf(
        RuneSegment(0f, .14f, .28f, .28f),
        RuneSegment(.28f, .28f, 0f, .42f),
        RuneSegment(0f, .46f, .28f, .60f),
        RuneSegment(.28f, .60f, 0f, .74f),
    ),
    // maðr
    listOf(RuneSegment(0f, .18f, -.28f, .02f), RuneSegment(0f, .18f, .28f, .02f)),
    // lǫgr
    listOf(RuneSegment(0f, .10f, .30f, .26f)),
    // ýr
    listOf(RuneSegment(0f, .80f, -.28f, .96f), RuneSegment(0f, .80f, .28f, .96f)),
)

// Frozen byte-slice formulas over h = SHA-256(UTF-8(characterKey)); bytes
// read unsigned (dwarf-seals.md). A rarer shorter rune draw (fewer than
// runeCount distinct values in the eight draws) is defined, not an error.
internal fun sealSpec(characterKey: String): SealSpec {
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
        metal = if ((byteAt(14) and 1) == 1) SealMetal.Gold else SealMetal.Bronze,
    )
}
