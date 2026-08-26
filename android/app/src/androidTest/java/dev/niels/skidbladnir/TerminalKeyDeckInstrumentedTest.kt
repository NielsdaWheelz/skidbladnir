package dev.niels.skidbladnir

import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.SemanticsNode
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.getUnclippedBoundsInRoot
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlin.math.abs
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TerminalKeyDeckInstrumentedTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun showsAndDispatchesOnlyTheReviewedTerminalKeysInStableOrder() {
        val sent = mutableListOf<TerminalAccessory>()
        compose.setContent {
            MaterialTheme {
                TerminalKeyDeck(
                    controlState = TerminalControlState.Off,
                    enabled = true,
                    onAccessory = sent::add,
                )
            }
        }

        val controls = compose.onAllNodes(hasClickAction()).fetchSemanticsNodes()
        assertEquals(SPOKEN_KEYS, controls.map(::contentDescription))
        assertEquals(
            (0 until REVIEWED_KEYS.size).map(Int::toFloat),
            controls.map { it.config.getOrNull(SemanticsProperties.TraversalIndex) },
        )

        VISIBLE_KEYS.forEach { compose.onNodeWithText(it).assertExists() }
        compose.onNodeWithText("Agents").assertDoesNotExist()
        compose.onNodeWithText("Detach").assertDoesNotExist()
        compose.onNodeWithText("Ctrl-C").assertDoesNotExist()

        SPOKEN_KEYS.forEach { spoken ->
            compose.onNodeWithContentDescription(spoken).performScrollTo().performClick()
        }
        compose.runOnIdle { assertEquals(REVIEWED_KEYS, sent) }
    }

    @Test
    fun controlDispatchesWithoutReducingAndRendersBoundaryOwnedState() {
        var state by mutableStateOf(TerminalControlState.Off)
        val sent = mutableListOf<TerminalAccessory>()
        compose.setContent {
            MaterialTheme {
                TerminalKeyDeck(
                    controlState = state,
                    enabled = true,
                    onAccessory = sent::add,
                )
            }
        }

        val control = compose.onNodeWithContentDescription("Control")
        val restingBounds = listOf("Control", "Tab").map {
            compose.onNodeWithContentDescription(it).fetchSemanticsNode().boundsInRoot
        }
        control.assertIsOff().assert(stateDescription("Off"))
        control.performClick()
        control.assertIsOff().assert(stateDescription("Off"))
        compose.runOnIdle { state = TerminalControlState.Armed }
        control.assertIsOn().assert(stateDescription("Armed"))
        assertEquals(
            "arming Control moved the stable key row",
            restingBounds,
            listOf("Control", "Tab").map {
                compose.onNodeWithContentDescription(it).fetchSemanticsNode().boundsInRoot
            },
        )
        control.performClick()
        control.assertIsOn().assert(stateDescription("Armed"))
        compose.runOnIdle { state = TerminalControlState.Off }
        control.assertIsOff().assert(stateDescription("Off"))
        compose.runOnIdle {
            assertEquals(listOf(TerminalAccessory.Control, TerminalAccessory.Control), sent)
        }
    }

    @Test
    fun frozenDeckDisablesEveryFullSizedTargetInOneSpacedRow() {
        compose.setContent {
            MaterialTheme {
                TerminalKeyDeck(
                    controlState = TerminalControlState.Armed,
                    enabled = false,
                    onAccessory = { error("a frozen deck accepted input") },
                )
            }
        }

        val minimumTarget = 48.dp
        val minimumSpacing = 8.dp
        val geometryTolerance = 0.5.dp
        SPOKEN_KEYS.forEach { spokenName ->
            val control = compose.onNodeWithContentDescription(spokenName)
            control.performScrollTo().assertIsNotEnabled()
            val bounds = control.getUnclippedBoundsInRoot()
            assertTrue(
                "$spokenName target is narrower than $minimumTarget: bounds=$bounds",
                bounds.right - bounds.left >= minimumTarget,
            )
            assertTrue(
                "$spokenName target is shorter than $minimumTarget: bounds=$bounds",
                bounds.bottom - bounds.top >= minimumTarget,
            )
        }

        val control = compose.onNodeWithContentDescription("Control")
        control.assertIsOff().assert(stateDescription("Off"))

        SPOKEN_KEYS.zipWithNext().forEach { (leftName, rightName) ->
            val left = compose.onNodeWithContentDescription(leftName).getUnclippedBoundsInRoot()
            val right = compose.onNodeWithContentDescription(rightName).getUnclippedBoundsInRoot()
            val verticalDelta = abs(left.top.value - right.top.value).dp
            val gap = right.left - left.right
            assertTrue(
                "$leftName and $rightName are not in one horizontal row: " +
                    "left=$left right=$right verticalDelta=$verticalDelta",
                verticalDelta <= geometryTolerance,
            )
            assertTrue(
                "$leftName and $rightName are separated by less than $minimumSpacing: " +
                    "left=$left right=$right gap=$gap",
                gap >= minimumSpacing - geometryTolerance,
            )
        }
    }

    private fun contentDescription(node: SemanticsNode): String =
        requireNotNull(node.config.getOrNull(SemanticsProperties.ContentDescription)).single()

    private fun stateDescription(value: String): SemanticsMatcher =
        SemanticsMatcher.expectValue(SemanticsProperties.StateDescription, value)

    private companion object {
        val REVIEWED_KEYS = listOf(
            TerminalAccessory.Escape,
            TerminalAccessory.Control,
            TerminalAccessory.Tab,
            TerminalAccessory.LineFeed,
            TerminalAccessory.Left,
            TerminalAccessory.Up,
            TerminalAccessory.Down,
            TerminalAccessory.Right,
            TerminalAccessory.Home,
            TerminalAccessory.End,
        )
        val VISIBLE_KEYS = listOf("Esc", "Ctrl", "Tab", "Line break", "←", "↑", "↓", "→", "Home", "End")
        val SPOKEN_KEYS = listOf(
            "Escape",
            "Control",
            "Tab",
            "Line break; sends line feed",
            "Left arrow",
            "Up arrow",
            "Down arrow",
            "Right arrow",
            "Home",
            "End",
        )
    }
}
