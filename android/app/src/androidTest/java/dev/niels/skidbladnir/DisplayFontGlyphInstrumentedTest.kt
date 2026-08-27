package dev.niels.skidbladnir

import android.graphics.Paint
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class DisplayFontGlyphInstrumentedTest {
    @Test
    fun displayFaceResolvesWordmarkAndDvergatalGlyphs() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val paint = Paint().apply { typeface = context.resources.getFont(R.font.big_shoulders) }
        for (glyph in listOf("Ð", "ð", "Í", "í", "Þ", "þ", "Á", "á", "Ó", "ý")) {
            assertTrue(
                "display face big_shoulders lacks '$glyph'; the wordmark or a Dvergatal " +
                    "display name would render in a fallback face",
                paint.hasGlyph(glyph),
            )
        }
    }
}
