package dev.niels.skidbladnir

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.drawable.AdaptiveIconDrawable
import android.graphics.drawable.Drawable
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

// Keeps defect 5 of docs/launcher-mark.md dead. The adaptive-icon descriptor
// used to point `monochrome` at the foreground drawable, and the foreground
// separates the sail's gores and the shield row by fill colour alone. A
// launcher that paints the themed layer in one flat tint erased every one of
// those separations, so the mark reached themed home screens as a blob with
// slivers.
//
// The layers are compared by coverage, not by pixel: they differ in colour
// whether or not they are aliased, so only their silhouettes distinguish a mark
// that degrades under one tint from one that collapses. The drift gate
// byte-compares generated files and never loads the installed icon, which is
// why this assertion has to run on the device.
@RunWith(AndroidJUnit4::class)
class LauncherIconInstrumentedTest {
    @Test
    fun installedIconThemesToItsOwnMonochromeGeometry() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val icon = context.packageManager.getApplicationIcon(context.packageName)
        assertTrue(
            "the installed application icon must be adaptive before a launcher can theme it " +
                "at all; it loaded as ${icon.javaClass.name}",
            icon is AdaptiveIconDrawable,
        )
        val adaptive = icon as AdaptiveIconDrawable
        val monochrome = adaptive.monochrome
        assertNotNull(
            "the adaptive icon must declare a monochrome layer; without one a themed launcher " +
                "tints the foreground instead, which is the collapse this proof exists to catch",
            monochrome,
        )

        val foregroundCover = coveredPixels(adaptive.foreground)
        val monochromeCover = coveredPixels(monochrome!!)
        val kept = foregroundCover.indices.count { foregroundCover[it] && monochromeCover[it] }
        val cut = foregroundCover.indices.count { foregroundCover[it] && !monochromeCover[it] }

        assertTrue(
            "the monochrome layer must cut daylight wherever the foreground carries structure " +
                "in colour; at ${CANVAS_PX}px it covers all $kept of the foreground's covered " +
                "pixels and cuts none away, so the two layers are one shape and a flat tint " +
                "fills the gores and the shield row solid",
            cut > 0,
        )
        assertTrue(
            "the monochrome layer must still be the whole mark after those cuts; it keeps only " +
                "$kept of the foreground's ${kept + cut} covered pixels and drops $cut, more " +
                "than the quarter that separating the gores and notching the sheer can account " +
                "for, so this is a fragment of the mark rather than the mark degraded",
            cut * 4 < kept,
        )
    }

    // One pixel per unit of the icons' 108-unit viewport, so a cut at the
    // legibility floor is several pixels wide. Half alpha and up counts as
    // covered, so an anti-aliased edge does not read as daylight.
    private fun coveredPixels(layer: Drawable): BooleanArray {
        val bitmap = Bitmap.createBitmap(CANVAS_PX, CANVAS_PX, Bitmap.Config.ARGB_8888)
        layer.setBounds(0, 0, CANVAS_PX, CANVAS_PX)
        layer.draw(Canvas(bitmap))
        val pixels = IntArray(CANVAS_PX * CANVAS_PX)
        bitmap.getPixels(pixels, 0, CANVAS_PX, 0, 0, CANVAS_PX, CANVAS_PX)
        return BooleanArray(pixels.size) { Color.alpha(pixels[it]) >= 128 }
    }

    private companion object {
        const val CANVAS_PX = 108
    }
}
