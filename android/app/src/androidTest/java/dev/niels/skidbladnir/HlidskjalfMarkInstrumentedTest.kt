package dev.niels.skidbladnir

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.TextButton
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.assertHasNoClickAction
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlin.math.abs
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The Hlíðskjálf mark inside an interactive control (design-language.md §8).
 *
 * The dashboard's mark stands alone in a row, so a semantics leak there would
 * only add a stray node. These two sit *inside* buttons, where a merging parent
 * would fold any content description straight into what the button announces —
 * so "Back to Dwarves" would stop being what the operator hears. That is the
 * risk these tags exist to disprove, and it is why the mark takes
 * `LocalContentColor` rather than an accent: a drawn glyph inherits no disabled
 * state on its own.
 */
@RunWith(AndroidJUnit4::class)
class HlidskjalfMarkInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun theReconnectPanelsMarkLeadsTheLabelWithoutJoiningWhatTheButtonAnnounces() {
        compose.setContent {
            MaterialTheme {
                OutlinedButton(onClick = {}) { BackToDwarvesContent(tag = TERMINAL_TAG) }
            }
        }

        assertMarkIsSilentAndLeads(TERMINAL_TAG)
    }

    @Test
    fun theBearerRepairMarkLeadsTheLabelWithoutJoiningWhatTheButtonAnnounces() {
        compose.setContent {
            MaterialTheme {
                TextButton(onClick = {}) { BackToDwarvesContent(tag = BEARER_REPAIR_TAG) }
            }
        }

        assertMarkIsSilentAndLeads(BEARER_REPAIR_TAG)
    }

    @Test
    fun theMarkDimsWithTheButtonItLeadsInsteadOfStayingAtFullStrength() {
        var enabled by mutableStateOf(true)
        compose.setContent {
            MaterialTheme {
                TextButton(onClick = {}, enabled = enabled) {
                    BackToDwarvesContent(tag = BEARER_REPAIR_TAG)
                }
            }
        }

        val litContrast = compose.onNodeWithTag(BEARER_REPAIR_TAG, useUnmergedTree = true).markContrast()
        enabled = false
        compose.waitForIdle()
        val dimmedContrast = compose.onNodeWithTag(BEARER_REPAIR_TAG, useUnmergedTree = true).markContrast()

        assertTrue(
            "the mark must be drawn at all: its strands never departed from the ground behind " +
                "them (contrast $litContrast)",
            litContrast > 0f,
        )
        assertTrue(
            "the mark takes LocalContentColor so it dims with its button; a fixed accent would " +
                "leave a bright glyph on a disabled control. Lit contrast $litContrast, " +
                "disabled contrast $dimmedContrast",
            dimmedContrast < litContrast,
        )
    }

    private fun assertMarkIsSilentAndLeads(tag: String) {
        compose.onNodeWithText(LABEL).assertIsDisplayed()

        val mark = compose.onNodeWithTag(tag, useUnmergedTree = true)
        mark.assertHasNoClickAction()
        val config = mark.fetchSemanticsNode().config
        assertTrue(
            "the mark is decoration: it must add nothing spoken, but it offered " +
                "${config.getOrNull(SemanticsProperties.ContentDescription)} and " +
                "${config.getOrNull(SemanticsProperties.Text)}",
            config.getOrNull(SemanticsProperties.ContentDescription).isNullOrEmpty() &&
                config.getOrNull(SemanticsProperties.Text).isNullOrEmpty(),
        )

        val button = compose.onNodeWithText(LABEL).fetchSemanticsNode().config
        assertNull(
            "the button announces its literal label and nothing the mark contributed",
            button.getOrNull(SemanticsProperties.ContentDescription),
        )

        val markBounds = mark.getUnclippedBoundsInRoot()
        val labelBounds = compose.onNodeWithText(LABEL, useUnmergedTree = true).getUnclippedBoundsInRoot()
        assertTrue(
            "the mark must lead the label on one row, not stack above it: " +
                "mark=$markBounds label=$labelBounds",
            markBounds.right <= labelBounds.left &&
                markBounds.top < labelBounds.bottom &&
                labelBounds.top < markBounds.bottom,
        )
    }
}

/**
 * How far the mark's brightest strand departs from the ground it is drawn on,
 * summed across channels. Theme-agnostic on purpose: it asks only "how much
 * mark is there", so it reads the same whether content is light on dark or the
 * reverse. The mark's own top-left corner is empty in the frozen geometry
 * (`Valknut` spans x 0.087..0.963, y 0.044..0.846), so it samples the ground.
 */
private fun SemanticsNodeInteraction.markContrast(): Float {
    val pixels = captureToImage().toPixelMap()
    val ground = pixels[0, 0]
    var widest = 0f
    for (y in 0 until pixels.height) {
        for (x in 0 until pixels.width) {
            val pixel = pixels[x, y]
            val distance = abs(pixel.red - ground.red) +
                abs(pixel.green - ground.green) +
                abs(pixel.blue - ground.blue)
            if (distance > widest) widest = distance
        }
    }
    return widest
}

private const val LABEL = "Back to Dwarves"
private const val TERMINAL_TAG = "terminal-dwarves-mark"
private const val BEARER_REPAIR_TAG = "bearer-repair-dwarves-mark"
