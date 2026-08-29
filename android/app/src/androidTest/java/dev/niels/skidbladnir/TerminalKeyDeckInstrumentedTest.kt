package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.click
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.DpRect
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlin.math.abs
import kotlin.math.ceil
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TerminalKeyDeckInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun showsAndDispatchesTheExactTwoBySevenDeckInRowMajorOrder() {
        val sent = mutableListOf<TerminalAccessory>()
        setDeckContent(onAccessory = sent::add)

        val controls = compose.onAllNodes(hasClickAction()).fetchSemanticsNodes()
        assertEquals(
            "the deck must expose only the reviewed 2 x 7 visible key matrix",
            VISIBLE_KEYS,
            controls.map(::visibleText),
        )
        assertEquals(
            "the spoken deck must match the visible row-major matrix",
            SPOKEN_KEYS,
            controls.map(::contentDescription),
        )
        assertEquals(
            "TalkBack traversal must remain deterministic and row-major",
            VISIBLE_KEYS.indices.map(Int::toFloat),
            controls.map { it.config.getOrNull(SemanticsProperties.TraversalIndex) },
        )
        compose.onNodeWithText("Detach").assertDoesNotExist()

        SPOKEN_KEYS.forEach { spokenName ->
            compose.onNodeWithContentDescription(spokenName).performScrollTo().performClick()
        }
        compose.runOnIdle {
            assertEquals(
                "each key must cross the typed TerminalAccessory boundary exactly once",
                REVIEWED_ACCESSORIES,
                sent,
            )
        }
    }

    @Test
    fun fitsOneAlignedFullSizedGridAtThe356DpContractWidth() {
        setDeckContent()

        val host = compose.onNodeWithTag(DECK_HOST_TAG).getUnclippedBoundsInRoot()
        assertDpClose("the normal-font deck height must be the reviewed 106dp", DECK_HEIGHT, host.measuredHeight)
        assertDpClose("the test viewport must exercise the exact 356dp contract", DECK_WIDTH, host.measuredWidth)
        assertSingleHorizontalScrollOwner(expectOverflow = false)

        SPOKEN_KEYS.forEach { spokenName ->
            compose.onNodeWithContentDescription(spokenName).assertIsDisplayed()
        }
        assertAlignedGrid(host)
        assertTrue(
            "the last column lost the reviewed outer padding: host=$host pageUp=${keyBounds("Page up")}",
            host.right - keyBounds("Page up").right >= OUTER_PADDING - GEOMETRY_TOLERANCE,
        )
    }

    @Test
    fun narrowAndLargeTextDecksKeepOneSharedOverflowWithoutClipping() {
        var layout by mutableStateOf(DeckLayout(NARROW_WIDTH, 1f))
        setDeckContent(width = { layout.width }, fontScale = { layout.fontScale })

        assertSingleHorizontalScrollOwner(expectOverflow = true)
        val pageUpBefore = keyBounds("Page up")
        val pageDownBefore = keyBounds("Page down")
        compose.onNodeWithContentDescription("Page down").performScrollTo().assertIsDisplayed()
        compose.onNodeWithContentDescription("Page up").assertIsDisplayed()
        val pageUpAfter = keyBounds("Page up")
        val pageDownAfter = keyBounds("Page down")
        val topShift = pageUpAfter.left - pageUpBefore.left
        val bottomShift = pageDownAfter.left - pageDownBefore.left
        assertTrue(
            "bringing Page down onscreen must move the shared two-row grid: before=$pageDownBefore after=$pageDownAfter",
            topShift < -GEOMETRY_TOLERANCE,
        )
        assertDpClose(
            "both rows must move by the same shared scroll offset",
            topShift,
            bottomShift,
        )
        assertColumnAligned("Page up", "Page down")

        compose.runOnIdle { layout = DeckLayout(DECK_WIDTH, LARGE_FONT_SCALE) }
        assertSingleHorizontalScrollOwner(expectOverflow = true)
        assertAlignedGrid(compose.onNodeWithTag(DECK_HOST_TAG).getUnclippedBoundsInRoot())
        compose.onNodeWithContentDescription("Page down").performScrollTo().assertIsDisplayed()
        compose.onNodeWithContentDescription("Page up").assertIsDisplayed()
        VISIBLE_KEYS.forEach(::assertTextDoesNotOverflow)
    }

    @Test
    fun modifiersRenderBoundaryOwnedStateAndDisabledDeckDispatchesNothing() {
        var modifiers by mutableStateOf(OFF_OFF)
        var enabled by mutableStateOf(true)
        val sent = mutableListOf<TerminalAccessory>()
        setDeckContent(
            modifiers = { modifiers },
            enabled = { enabled },
            onAccessory = sent::add,
        )

        val control = compose.onNodeWithContentDescription("Control")
        val alt = compose.onNodeWithContentDescription("Alt")
        val restingBounds = keyBounds()
        control.assertIsOff().assert(stateDescription("Off"))
        alt.assertIsOff().assert(stateDescription("Off"))

        control.performClick()
        control.assertIsOff().assert(stateDescription("Off"))
        compose.runOnIdle {
            assertEquals(
                "the deck must dispatch rather than reduce a modifier tap",
                listOf(TerminalAccessory.Control),
                sent,
            )
            modifiers = CONTROL_ARMED
        }
        control.assertIsOn().assert(stateDescription("Armed"))
        alt.assertIsOff().assert(stateDescription("Off"))
        assertEquals("arming Control must not move the stable deck", restingBounds, keyBounds())

        compose.runOnIdle { modifiers = ALT_ARMED }
        control.assertIsOff().assert(stateDescription("Off"))
        alt.assertIsOn().assert(stateDescription("Armed"))
        assertEquals("arming Alt must not move the stable deck", restingBounds, keyBounds())

        compose.runOnIdle { modifiers = BOTH_ARMED }
        control.assertIsOn().assert(stateDescription("Armed"))
        alt.assertIsOn().assert(stateDescription("Armed"))
        assertEquals("arming Ctrl+Alt must not move the stable deck", restingBounds, keyBounds())

        alt.performClick()
        control.assertIsOn().assert(stateDescription("Armed"))
        alt.assertIsOn().assert(stateDescription("Armed"))
        compose.runOnIdle {
            assertEquals(
                "Alt must cross the same typed dispatch boundary without local reduction",
                listOf(TerminalAccessory.Control, TerminalAccessory.Alt),
                sent,
            )
            sent.clear()
            enabled = false
        }

        SPOKEN_KEYS.forEach { spokenName ->
            compose.onNodeWithContentDescription(spokenName)
                .assertIsNotEnabled()
                .performTouchInput { click() }
        }
        control.assertIsOff().assert(stateDescription("Off"))
        alt.assertIsOff().assert(stateDescription("Off"))
        compose.runOnIdle {
            assertTrue("a disabled deck accepted terminal input: $sent", sent.isEmpty())
        }
    }

    private fun setDeckContent(
        width: () -> Dp = { DECK_WIDTH },
        fontScale: () -> Float = { 1f },
        modifiers: () -> TerminalModifiers = { OFF_OFF },
        enabled: () -> Boolean = { true },
        onAccessory: (TerminalAccessory) -> Unit = {},
    ) {
        compose.setContent {
            MaterialTheme {
                val density = LocalDensity.current
                val currentWidth = width()
                val currentFontScale = fontScale()
                CompositionLocalProvider(
                    LocalDensity provides Density(density.density, currentFontScale),
                ) {
                    key(currentWidth, currentFontScale) {
                        Box(
                            Modifier
                                .width(currentWidth)
                                .testTag(DECK_HOST_TAG),
                        ) {
                            TerminalKeyDeck(
                                modifiers = modifiers(),
                                enabled = enabled(),
                                onAccessory = onAccessory,
                            )
                        }
                    }
                }
            }
        }
    }

    private fun assertAlignedGrid(host: DpRect) {
        val rows = SPOKEN_KEYS.chunked(COLUMN_COUNT)
        val rowBounds = rows.map { row -> row.map(::keyBounds) }
        val allBounds = rowBounds.flatten()

        allBounds.forEachIndexed { index, bounds ->
            val spokenName = SPOKEN_KEYS[index]
            assertTrue(
                "$spokenName target is narrower than $MINIMUM_TARGET: bounds=$bounds",
                bounds.measuredWidth >= MINIMUM_TARGET,
            )
            assertTrue(
                "$spokenName target is shorter than $MINIMUM_TARGET: bounds=$bounds",
                bounds.measuredHeight >= MINIMUM_TARGET,
            )
        }
        allBounds.drop(1).forEach { bounds ->
            assertDpClose("all deck cells must have equal width", allBounds.first().measuredWidth, bounds.measuredWidth)
        }

        rowBounds.forEachIndexed { rowIndex, row ->
            row.drop(1).forEach { bounds ->
                assertDpClose("row ${rowIndex + 1} keys must share a top edge", row.first().top, bounds.top)
                assertDpClose("row ${rowIndex + 1} keys must share a bottom edge", row.first().bottom, bounds.bottom)
            }
            row.zipWithNext().forEachIndexed { column, (left, right) ->
                val gap = right.left - left.right
                assertTrue(
                    "row ${rowIndex + 1} columns ${column + 1}/${column + 2} have less than a $MINIMUM_GAP gap: " +
                        "left=$left right=$right gap=$gap",
                    gap >= MINIMUM_GAP - GEOMETRY_TOLERANCE,
                )
            }
        }
        rows.first().zip(rows.last()).forEach { (top, bottom) -> assertColumnAligned(top, bottom) }

        val verticalGap = rowBounds.last().first().top - rowBounds.first().first().bottom
        assertTrue(
            "the two rows have less than a $MINIMUM_GAP gap: rows=$rowBounds gap=$verticalGap",
            verticalGap >= MINIMUM_GAP - GEOMETRY_TOLERANCE,
        )
        assertTrue(
            "the first column lost the reviewed outer padding: host=$host grid=$rowBounds",
            rowBounds.first().first().left - host.left >= OUTER_PADDING - GEOMETRY_TOLERANCE,
        )
        assertTrue(
            "the first row lost the reviewed outer padding: host=$host grid=$rowBounds",
            rowBounds.first().first().top - host.top >= OUTER_PADDING - GEOMETRY_TOLERANCE,
        )
        assertTrue(
            "the second row lost the reviewed outer padding: host=$host grid=$rowBounds",
            host.bottom - rowBounds.last().first().bottom >= OUTER_PADDING - GEOMETRY_TOLERANCE,
        )
    }

    private fun assertColumnAligned(topName: String, bottomName: String) {
        val top = keyBounds(topName)
        val bottom = keyBounds(bottomName)
        assertDpClose("$topName and $bottomName must share a left edge", top.left, bottom.left)
        assertDpClose("$topName and $bottomName must share a right edge", top.right, bottom.right)
    }

    private fun assertSingleHorizontalScrollOwner(expectOverflow: Boolean) {
        val owners = compose.onAllNodes(HAS_HORIZONTAL_SCROLL_RANGE).fetchSemanticsNodes()
        assertEquals("both rows must have exactly one shared horizontal scroll owner", 1, owners.size)
        val range = requireNotNull(
            owners.single().config.getOrNull(SemanticsProperties.HorizontalScrollAxisRange),
        )
        val maximum = range.maxValue()
        if (expectOverflow) {
            assertTrue("the deck must overflow at this width/font scale: range=$range", maximum > 0f)
        } else {
            assertTrue("the 356dp normal-font deck must not scroll: range=$range", maximum <= 1f)
        }
    }

    private fun assertTextDoesNotOverflow(label: String) {
        val results = mutableListOf<TextLayoutResult>()
        compose.onNodeWithText(label, useUnmergedTree = true)
            .performSemanticsAction(SemanticsActions.GetTextLayoutResult) { action -> action(results) }
        val keyIndex = VISIBLE_KEYS.indexOf(label)
        require(keyIndex >= 0) { "unknown deck label $label" }
        val bounds = keyBounds(SPOKEN_KEYS[keyIndex])
        val availableContentWidth = bounds.measuredWidth - KEY_HORIZONTAL_CONTENT_PADDING * 2
        compose.runOnIdle {
            val result = results.single()
            val keyWidthPixels = with(result.layoutInput.density) { bounds.measuredWidth.roundToPx() }
            val contentPaddingPixels = with(result.layoutInput.density) {
                KEY_HORIZONTAL_CONTENT_PADDING.roundToPx()
            }
            val availableContentPixels = keyWidthPixels - contentPaddingPixels * 2
            assertEquals(
                "$label must remain on exactly one line: measuredTextSize=${result.size} " +
                    "textConstraints=${result.layoutInput.constraints} keyBounds=$bounds",
                1,
                result.lineCount,
            )
            val lineWidth = result.getLineRight(0) - result.getLineLeft(0)
            val requiredLinePixels = ceil(lineWidth.toDouble()).toInt()
            val context = "$label at font scale $LARGE_FONT_SCALE: " +
                "didOverflowWidth=${result.didOverflowWidth} " +
                "didOverflowHeight=${result.didOverflowHeight} " +
                "hasVisualOverflow=${result.hasVisualOverflow} " +
                "lineWidth=$lineWidth requiredLinePixels=$requiredLinePixels " +
                "measuredTextSize=${result.size} textConstraints=${result.layoutInput.constraints} " +
                "keyBounds=$bounds availableContent=${availableContentWidth}x${bounds.measuredHeight} " +
                "availableContentPixels=$availableContentPixels"
            assertTrue(
                "rendered line exceeds the padded key content; $context",
                requiredLinePixels <= availableContentPixels,
            )
            assertFalse("text height overflowed; $context", result.didOverflowHeight)
            assertFalse("text was ellipsized; $context", result.isLineEllipsized(0))
            assertFalse(
                "Compose would clip the text canvas; $context",
                result.hasVisualOverflow,
            )
        }
    }

    private fun keyBounds(spokenName: String): DpRect =
        compose.onNodeWithContentDescription(spokenName).getUnclippedBoundsInRoot()

    private fun keyBounds(): List<DpRect> = SPOKEN_KEYS.map(::keyBounds)

    private val DpRect.measuredWidth: Dp
        get() = right - left

    private val DpRect.measuredHeight: Dp
        get() = bottom - top

    private fun contentDescription(node: SemanticsNode): String =
        requireNotNull(node.config.getOrNull(SemanticsProperties.ContentDescription)).single()

    private fun visibleText(node: SemanticsNode): String =
        requireNotNull(node.config.getOrNull(SemanticsProperties.Text)).single().text

    private fun stateDescription(value: String): SemanticsMatcher =
        SemanticsMatcher.expectValue(SemanticsProperties.StateDescription, value)

    private fun assertDpClose(message: String, expected: Dp, actual: Dp) {
        assertTrue(
            "$message: expected=$expected actual=$actual tolerance=$GEOMETRY_TOLERANCE",
            abs(expected.value - actual.value) <= GEOMETRY_TOLERANCE.value,
        )
    }

    private data class DeckLayout(
        val width: Dp,
        val fontScale: Float,
    )

    private companion object {
        const val COLUMN_COUNT = 7
        const val LARGE_FONT_SCALE = 2f
        const val DECK_HOST_TAG = "terminal-key-deck-test-host"
        val DECK_WIDTH = 356.dp
        val DECK_HEIGHT = 106.dp
        val NARROW_WIDTH = 320.dp
        val MINIMUM_TARGET = 48.dp
        val MINIMUM_GAP = 2.dp
        val OUTER_PADDING = 4.dp
        val KEY_HORIZONTAL_CONTENT_PADDING = 4.dp
        val GEOMETRY_TOLERANCE = 0.5.dp
        val VISIBLE_KEYS = listOf(
            "Esc", "/", "-", "Home", "↑", "End", "PgUp",
            "Tab", "Ctrl", "Alt", "←", "↓", "→", "PgDn",
        )
        val SPOKEN_KEYS = listOf(
            "Escape", "Slash", "Hyphen", "Home", "Up arrow", "End", "Page up",
            "Tab", "Control", "Alt", "Left arrow", "Down arrow", "Right arrow", "Page down",
        )
        val REVIEWED_ACCESSORIES = listOf(
            TerminalAccessory.Escape,
            TerminalAccessory.Slash,
            TerminalAccessory.Hyphen,
            TerminalAccessory.Home,
            TerminalAccessory.Up,
            TerminalAccessory.End,
            TerminalAccessory.PageUp,
            TerminalAccessory.Tab,
            TerminalAccessory.Control,
            TerminalAccessory.Alt,
            TerminalAccessory.Left,
            TerminalAccessory.Down,
            TerminalAccessory.Right,
            TerminalAccessory.PageDown,
        )
        val OFF_OFF = TerminalModifiers(TerminalModifierPhase.Off, TerminalModifierPhase.Off)
        val CONTROL_ARMED = TerminalModifiers(TerminalModifierPhase.Armed, TerminalModifierPhase.Off)
        val ALT_ARMED = TerminalModifiers(TerminalModifierPhase.Off, TerminalModifierPhase.Armed)
        val BOTH_ARMED = TerminalModifiers(TerminalModifierPhase.Armed, TerminalModifierPhase.Armed)
        val HAS_HORIZONTAL_SCROLL_RANGE = SemanticsMatcher("has horizontal scroll range") { node ->
            node.config.getOrNull(SemanticsProperties.HorizontalScrollAxisRange) != null
        }
    }
}
