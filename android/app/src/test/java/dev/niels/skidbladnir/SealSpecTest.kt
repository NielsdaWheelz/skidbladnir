package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Test

class SealSpecTest {
    // Golden vectors computed 2026-08-26 (dwarf-seals.md); the frozen byte-slice
    // formulas over SHA-256(UTF-8(characterKey)) must decode exactly.
    @Test
    fun `sealSpec decodes the norse modsognir golden vector exactly`() {
        val spec = sealSpec("norse.modsognir")
        assertEquals(
            "norse.modsognir must decode to the frozen golden vector; actual was $spec",
            SealSpec(
                mineral = 6,
                runes = listOf(12, 9),
                beardTeeth = 6,
                beardDepthStep = 1,
                facetMask = 229,
                metal = SealMetal.Bronze,
            ),
            spec,
        )
    }

    @Test
    fun `sealSpec decodes the norse durinn golden vector exactly`() {
        val spec = sealSpec("norse.durinn")
        assertEquals(
            "norse.durinn must decode to the frozen golden vector; actual was $spec",
            SealSpec(
                mineral = 4,
                runes = listOf(9, 10),
                beardTeeth = 3,
                beardDepthStep = 0,
                facetMask = 24,
                metal = SealMetal.Bronze,
            ),
            spec,
        )
    }

    @Test
    fun `sealSpec decodes the tolkien gimli golden vector exactly`() {
        val spec = sealSpec("tolkien.gimli")
        assertEquals(
            "tolkien.gimli must decode to the frozen golden vector; actual was $spec",
            SealSpec(
                mineral = 4,
                runes = listOf(5, 10),
                beardTeeth = 3,
                beardDepthStep = 0,
                facetMask = 239,
                metal = SealMetal.Gold,
            ),
            spec,
        )
    }

    @Test
    fun `sealSpec is stable across repeated calls for the same key`() {
        val key = "norse.modsognir"
        val first = sealSpec(key)
        val second = sealSpec(key)
        assertEquals(
            "sealSpec must be a pure function of the key; repeated calls diverged: $first vs $second",
            first,
            second,
        )
    }

    @Test
    fun `sealSpec is stable across a freshly re-decoded, non-interned copy of the key string`() {
        val original = "tolkien.gimli"
        // Force a distinct String instance built from raw UTF-8 bytes so this
        // proof cannot pass merely because the JVM reused one interned
        // String; sealSpec must hash the UTF-8 bytes, not rely on identity or
        // the platform default charset.
        val recoded = String(original.toByteArray(Charsets.UTF_8), Charsets.UTF_8)
        assertEquals(
            "sealSpec must depend only on UTF-8 byte content, not String identity",
            sealSpec(original),
            sealSpec(recoded),
        )
    }
}
