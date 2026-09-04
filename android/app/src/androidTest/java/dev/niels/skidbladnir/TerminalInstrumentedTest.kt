package dev.niels.skidbladnir

import android.accessibilityservice.AccessibilityServiceInfo
import android.content.Context
import android.content.ContextWrapper
import android.content.pm.ActivityInfo
import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.Rect
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.InputDevice
import android.view.MotionEvent
import android.view.PixelCopy
import android.view.KeyEvent
import android.view.View
import android.view.ViewGroup
import android.view.ViewConfiguration
import android.view.ViewTreeObserver
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputConnection
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.core.net.toUri
import androidx.lifecycle.Lifecycle
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebMessagePortCompat
import androidx.webkit.WebViewCompat
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import org.json.JSONObject
import org.json.JSONTokener
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

private data class AccessoryExpectation(
    val key: String,
    val unmodified: String,
    val control: String,
    val alt: String,
    val controlAlt: String,
)

private val OFF_OFF = TerminalModifiers(TerminalModifierPhase.Off, TerminalModifierPhase.Off)
private val CONTROL_ARMED = TerminalModifiers(TerminalModifierPhase.Armed, TerminalModifierPhase.Off)
private val ALT_ARMED = TerminalModifiers(TerminalModifierPhase.Off, TerminalModifierPhase.Armed)
private val BOTH_ARMED = TerminalModifiers(TerminalModifierPhase.Armed, TerminalModifierPhase.Armed)

private const val TERMINAL_WHEEL_BACKWARD = "Terminal wheel backward"
private const val TERMINAL_WHEEL_FORWARD = "Terminal wheel forward"
private const val ACCESSIBILITY_STABILITY_MILLIS = 1_250L

private enum class TouchWheelDirection(
    val fingerRows: Float,
    val cursorSuffix: Char,
    val sgrButton: Int,
) {
    Backward(2.5f, 'A', 64),
    Forward(-2.5f, 'B', 65),
}

private data class TouchPoint(val x: Float, val y: Float)

private data class TerminalTouchGeometry(
    val screenLeft: Float,
    val screenTop: Float,
    val screenWidth: Float,
    val screenHeight: Float,
    val columns: Int,
    val rows: Int,
    val cssToScreenX: Float,
    val cssToScreenY: Float,
) {
    val rowHeight: Float get() = screenHeight / rows
    val touchSlopDistance: Float get() = 8f * cssToScreenY
    val belowSlopDistance: Float get() = touchSlopDistance / 2f
    val claimDistance: Float get() = maxOf(1.25f * rowHeight, touchSlopDistance + cssToScreenY)
    val claimWholeRows: Int get() = (claimDistance / rowHeight).toInt()
    val postCancelDistance: Float get() = 2.25f * rowHeight
    val belowSlopHorizontalDistance: Float get() = 4f * cssToScreenX
    val horizontalClaimDistance: Float get() = 9f * cssToScreenX
    private val verticalInset: Float get() = maxOf(1f, cssToScreenY)
    private val centeredStartDownRoom: Float
        get() = screenHeight - (rows / 2 + 0.5f) * rowHeight

    fun cell(column: Int, row: Int): TouchPoint = TouchPoint(
        x = screenLeft + screenWidth * (column + 0.5f) / columns,
        y = screenTop + screenHeight * (row + 0.5f) / rows,
    )

    fun move(point: TouchPoint, direction: TouchWheelDirection): TouchPoint {
        val end = point.copy(
            y = (point.y + direction.fingerRows * rowHeight).coerceIn(
                screenTop + verticalInset,
                screenTop + screenHeight - verticalInset,
            ),
        )
        assertTrue(
            "case=touch-geometry route=geometry ordinary-drag",
            kotlin.math.abs(end.y - point.y) > touchSlopDistance,
        )
        return end
    }

    fun claim(point: TouchPoint): TouchPoint = point.copy(y = point.y + claimDistance)

    fun postCancel(point: TouchPoint): TouchPoint =
        point.copy(y = point.y + postCancelDistance)

    fun contains(point: TouchPoint): Boolean =
        point.x > screenLeft &&
            point.x < screenLeft + screenWidth &&
            point.y > screenTop &&
            point.y < screenTop + screenHeight

    fun requireGestureScale(caseId: String) {
        assertTrue("case=$caseId route=geometry scale-x", cssToScreenX.isFinite() && cssToScreenX > 0f)
        assertTrue("case=$caseId route=geometry scale-y", cssToScreenY.isFinite() && cssToScreenY > 0f)
        assertTrue(
            "case=$caseId route=geometry slop-order",
            belowSlopDistance < touchSlopDistance &&
                claimDistance > touchSlopDistance &&
                claimWholeRows == 1 &&
                claimDistance < 2f * rowHeight &&
                postCancelDistance < centeredStartDownRoom - verticalInset &&
                TouchWheelDirection.Backward.fingerRows * rowHeight > touchSlopDistance &&
                belowSlopHorizontalDistance < 8f * cssToScreenX &&
                horizontalClaimDistance > 8f * cssToScreenX &&
                horizontalClaimDistance < screenWidth / 2f,
        )
    }
}

private data class TerminalContainmentState(
    val windowX: Int,
    val windowY: Int,
    val pageY: Int,
    val documentY: Int,
    val bodyY: Int,
    val terminalX: Int,
)

private data class TerminalAccessibilityAction(val id: Int, val label: String)

private data class TerminalAccessibilityActionOccurrence(
    val node: AccessibilityNodeInfo,
    val action: TerminalAccessibilityAction,
)

private data class TerminalRowKey(
    val windowId: Int,
    val left: Int,
    val top: Int,
    val right: Int,
    val bottom: Int,
    val rowIndex: Int,
    val rowSpan: Int,
    val columnIndex: Int,
    val columnSpan: Int,
    val packageName: String,
    val className: String,
    val viewIdResourceName: String?,
)

private data class TerminalNonRowKey(
    val windowId: Int,
    val left: Int,
    val top: Int,
    val right: Int,
    val bottom: Int,
    val packageName: String,
    val className: String,
    val viewIdResourceName: String?,
)

private enum class NativeTouchState {
    NotStarted,
    Active,
    Ended,
}

private data class NativeTargetSnapshot(
    val left: Int,
    val top: Int,
    val width: Int,
    val height: Int,
    val displayId: Int,
)

private data class NativeTargetObservation(
    val activityResumed: Boolean,
    val displayPresent: Boolean,
    val attached: Boolean,
    val shown: Boolean,
    val windowTokenPresent: Boolean,
    val windowFocused: Boolean,
    val positiveBounds: Boolean,
    val globalVisibleBounds: Boolean,
    val pointFinite: Boolean,
    val pointContained: Boolean,
    val activeRootPresent: Boolean,
    val activeRootPackageMatches: Boolean,
    val matchingWindowPresent: Boolean,
    val matchingWindowActive: Boolean,
    val matchingWindowFocused: Boolean,
    val snapshot: NativeTargetSnapshot?,
)

private data class TouchEventDiagnostics(
    val pointerDownCount: Int,
    val pointerMoveCount: Int,
    val pointerUpCount: Int,
    val pointerCancelCount: Int,
    val mouseDownCount: Int,
    val mouseMoveCount: Int,
    val mouseUpCount: Int,
    val clickCount: Int,
    val contextMenuCount: Int,
    val screenCompatibilityCount: Int,
    val pointerScreenTargetCount: Int,
    val pointerAccessibilityTargetCount: Int,
    val pointerOtherTargetCount: Int,
    val compatibilityScreenTargetCount: Int,
    val compatibilityAccessibilityTargetCount: Int,
    val compatibilityOtherTargetCount: Int,
    val contextTrustedCount: Int,
    val contextSourceCapabilitiesPresentCount: Int,
    val contextFiresTouchEventsCount: Int,
    val wheelCount: Int,
    val wheelTrustedCount: Int,
    val wheelDefaultPreventedCount: Int,
    val firstWheelOrder: Int,
    val lastPointerMoveOrder: Int,
    val firstMouseDownOrder: Int,
    val firstMouseMoveOrder: Int,
    val firstContextMenuOrder: Int,
    val scrollCount: Int,
    val firstScrollOrder: Int,
    val lastScrollPosition: Int,
    val scriptResourcePresent: Boolean,
    val scriptTransferSize: Int,
    val scriptDecodedBodySize: Int,
) {
    fun summary(): String =
        "pointerDownCount=$pointerDownCount pointerMoveCount=$pointerMoveCount " +
            "pointerUpCount=$pointerUpCount pointerCancelCount=$pointerCancelCount " +
            "mouseDownCount=$mouseDownCount mouseMoveCount=$mouseMoveCount " +
            "mouseUpCount=$mouseUpCount clickCount=$clickCount contextMenuCount=$contextMenuCount " +
            "screenCompatibilityCount=$screenCompatibilityCount " +
            "pointerScreenTargetCount=$pointerScreenTargetCount " +
            "pointerAccessibilityTargetCount=$pointerAccessibilityTargetCount " +
            "pointerOtherTargetCount=$pointerOtherTargetCount " +
            "compatibilityScreenTargetCount=$compatibilityScreenTargetCount " +
            "compatibilityAccessibilityTargetCount=$compatibilityAccessibilityTargetCount " +
            "compatibilityOtherTargetCount=$compatibilityOtherTargetCount " +
            "contextTrustedCount=$contextTrustedCount " +
            "contextSourceCapabilitiesPresentCount=$contextSourceCapabilitiesPresentCount " +
            "contextFiresTouchEventsCount=$contextFiresTouchEventsCount " +
            "wheelCount=$wheelCount wheelTrustedCount=$wheelTrustedCount " +
            "wheelDefaultPreventedCount=$wheelDefaultPreventedCount firstWheelOrder=$firstWheelOrder " +
            "lastPointerMoveOrder=$lastPointerMoveOrder firstMouseDownOrder=$firstMouseDownOrder " +
            "firstMouseMoveOrder=$firstMouseMoveOrder firstContextMenuOrder=$firstContextMenuOrder " +
            "scrollCount=$scrollCount firstScrollOrder=$firstScrollOrder " +
            "lastScrollPosition=$lastScrollPosition scriptResourcePresent=$scriptResourcePresent " +
            "scriptTransferSize=$scriptTransferSize scriptDecodedBodySize=$scriptDecodedBodySize"
}

private class NativeTouchStream(
    private val webView: WebView,
    private val caseId: String,
    private val primaryPointerId: Int = 0,
) : AutoCloseable {
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val automation = instrumentation.uiAutomation
    private var state = NativeTouchState.NotStarted
    private var downTime = 0L
    private var eventTime = 0L
    private var displayId = 0
    private var lastPoints = emptyList<TouchPoint>()

    init {
        require(primaryPointerId in 0..30)
    }

    fun down(point: TouchPoint) {
        assertEquals(
            "case=$caseId route=native-touch phase=down state",
            NativeTouchState.NotStarted,
            state,
        )
        assertTrue(
            "case=$caseId route=native-touch phase=down prior-stream-active",
            hasNoActiveStream(),
        )
        displayId = awaitDownPrecondition(point)
        assertTrue(
            "case=$caseId route=native-touch phase=down prior-stream-active-after-precondition",
            hasNoActiveStream(),
        )
        downTime = SystemClock.uptimeMillis()
        eventTime = downTime - 1
        val accepted = injectRaw(MotionEvent.ACTION_DOWN, listOf(point))
        if (!accepted) {
            throw AssertionError(
                "case=$caseId route=native-touch phase=down rejected " + rejectedDownDiagnostic(),
            )
        }
        if (accepted) {
            state = NativeTouchState.Active
            lastPoints = listOf(point)
            setActiveStream(this)
        }
    }

    fun move(point: TouchPoint) = injectTracked(
        MotionEvent.ACTION_MOVE,
        listOf(point),
        "move",
    )

    fun up(point: TouchPoint) = injectEnding(
        MotionEvent.ACTION_UP,
        listOf(point),
        "up",
    )

    fun cancel(point: TouchPoint) = injectEnding(
        MotionEvent.ACTION_CANCEL,
        listOf(point),
        "cancel",
    )

    fun secondDown(first: TouchPoint, second: TouchPoint) = injectTracked(
        MotionEvent.ACTION_POINTER_DOWN or (1 shl MotionEvent.ACTION_POINTER_INDEX_SHIFT),
        listOf(first, second),
        "second-down",
    )

    fun move(first: TouchPoint, second: TouchPoint) = injectTracked(
        MotionEvent.ACTION_MOVE,
        listOf(first, second),
        "move-two",
    )

    fun secondUp(first: TouchPoint, second: TouchPoint) = injectTracked(
        MotionEvent.ACTION_POINTER_UP or (1 shl MotionEvent.ACTION_POINTER_INDEX_SHIFT),
        listOf(first, second),
        "second-up",
        remainingPoints = listOf(first),
    )

    private fun awaitDownPrecondition(point: TouchPoint): Int {
        instrumentation.waitForIdleSync()
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var lastTarget: NativeTargetObservation? = null
        var previousReadySnapshot: NativeTargetSnapshot? = null
        while (System.nanoTime() < deadline) {
            val current = targetObservation(point)
            lastTarget = current
            val currentSnapshot = current.snapshot
            if (currentSnapshot != null && currentSnapshot == previousReadySnapshot) {
                return currentSnapshot.displayId
            }
            previousReadySnapshot = currentSnapshot
            instrumentation.waitForIdleSync()
        }
        val observed = requireNotNull(lastTarget)
        throw AssertionError(
            "case=$caseId route=native-touch phase=down precondition=false " +
                "activityResumed=${observed.activityResumed} " +
                "displayPresent=${observed.displayPresent} " +
                "attached=${observed.attached} shown=${observed.shown} " +
                "windowTokenPresent=${observed.windowTokenPresent} " +
                "windowFocused=${observed.windowFocused} " +
                "positiveBounds=${observed.positiveBounds} " +
                "globalVisibleBounds=${observed.globalVisibleBounds} " +
                "pointFinite=${observed.pointFinite} " +
                "pointContained=${observed.pointContained} " +
                "activeRootPresent=${observed.activeRootPresent} " +
                "activeRootPackageMatches=${observed.activeRootPackageMatches} " +
                "matchingWindowPresent=${observed.matchingWindowPresent} " +
                "matchingWindowActive=${observed.matchingWindowActive} " +
                "matchingWindowFocused=${observed.matchingWindowFocused}",
        )
    }

    private fun targetObservation(point: TouchPoint): NativeTargetObservation {
        val latch = CountDownLatch(1)
        var observation: NativeTargetObservation? = null
        Handler(Looper.getMainLooper()).post {
            val activity = terminalTestActivity(webView.context)
            val display = webView.display
            val activityResumed = activity?.lifecycle?.currentState == Lifecycle.State.RESUMED
            val attached = webView.isAttachedToWindow
            val shown = webView.isShown
            val windowTokenPresent = webView.windowToken != null
            val windowFocused = webView.hasWindowFocus()
            val positiveBounds = webView.width > 0 && webView.height > 0
            val visibleBounds = Rect()
            val globalVisibleBounds = webView.getGlobalVisibleRect(visibleBounds) &&
                !visibleBounds.isEmpty
            val pointFinite = point.x.isFinite() && point.y.isFinite()
            val pointContained = pointFinite &&
                globalVisibleBounds &&
                point.x > visibleBounds.left &&
                point.x < visibleBounds.right &&
                point.y > visibleBounds.top &&
                point.y < visibleBounds.bottom
            val ready = activityResumed &&
                display != null &&
                attached &&
                shown &&
                windowTokenPresent &&
                windowFocused &&
                positiveBounds &&
                pointContained
            observation = NativeTargetObservation(
                activityResumed = activityResumed,
                displayPresent = display != null,
                attached = attached,
                shown = shown,
                windowTokenPresent = windowTokenPresent,
                windowFocused = windowFocused,
                positiveBounds = positiveBounds,
                globalVisibleBounds = globalVisibleBounds,
                pointFinite = pointFinite,
                pointContained = pointContained,
                activeRootPresent = false,
                activeRootPackageMatches = false,
                matchingWindowPresent = false,
                matchingWindowActive = false,
                matchingWindowFocused = false,
                snapshot = if (ready) {
                    NativeTargetSnapshot(
                        left = visibleBounds.left,
                        top = visibleBounds.top,
                        width = visibleBounds.width(),
                        height = visibleBounds.height(),
                        displayId = requireNotNull(display).displayId,
                    )
                } else {
                    null
                },
            )
            latch.countDown()
        }
        assertTrue(
            "case=$caseId route=native-touch phase=down main-thread-timeout",
            latch.await(1, TimeUnit.SECONDS),
        )
        val observed = requireNotNull(observation)
        val targetPackage = instrumentation.targetContext.packageName
        val activeRoot = automation.rootInActiveWindow
        val activeRootPresent = activeRoot != null
        val activeRootPackageMatches = activeRoot?.packageName?.toString() == targetPackage
        val matchingWindow = activeRoot?.windowId?.let { id ->
            automation.windows.singleOrNull { it.id == id }
        }
        val windowReady = activeRootPackageMatches &&
            matchingWindow?.isActive == true &&
            matchingWindow?.isFocused == true
        return observed.copy(
            activeRootPresent = activeRootPresent,
            activeRootPackageMatches = activeRootPackageMatches,
            matchingWindowPresent = matchingWindow != null,
            matchingWindowActive = matchingWindow?.isActive == true,
            matchingWindowFocused = matchingWindow?.isFocused == true,
            snapshot = if (windowReady) observed.snapshot else null,
        )
    }

    private fun injectTracked(
        action: Int,
        points: List<TouchPoint>,
        phase: String,
        remainingPoints: List<TouchPoint> = points,
    ) {
        assertActive(phase)
        val accepted = injectRaw(action, points)
        assertTrue("case=$caseId route=native-touch phase=$phase rejected", accepted)
        if (accepted) lastPoints = remainingPoints
    }

    private fun injectEnding(action: Int, points: List<TouchPoint>, phase: String) {
        assertActive(phase)
        val accepted = injectRaw(action, points)
        assertTrue("case=$caseId route=native-touch phase=$phase rejected", accepted)
        if (accepted) {
            state = NativeTouchState.Ended
            clearActiveStream(this)
        }
    }

    private fun assertActive(phase: String) {
        assertEquals(
            "case=$caseId route=native-touch phase=$phase state",
            NativeTouchState.Active,
            state,
        )
    }

    private fun injectRaw(action: Int, points: List<TouchPoint>): Boolean {
        // UiAutomation otherwise submits consecutive samples roughly 1 ms apart.
        // Chromium resamples that impossible velocity past the requested endpoint,
        // so model one ordinary 60 Hz input frame between post-down packets.
        if (state == NativeTouchState.Active) SystemClock.sleep(16)
        eventTime = maxOf(SystemClock.uptimeMillis(), eventTime + 1)
        val properties = Array(points.size) { index ->
            MotionEvent.PointerProperties().apply {
                id = primaryPointerId + index
                toolType = MotionEvent.TOOL_TYPE_FINGER
            }
        }
        val coordinates = Array(points.size) { index ->
            MotionEvent.PointerCoords().apply {
                x = points[index].x
                y = points[index].y
                pressure = 1f
                size = 1f
            }
        }
        val event = requireNotNull(MotionEvent.obtain(
            downTime,
            eventTime,
            action,
            points.size,
            properties,
            coordinates,
            0,
            0,
            1f,
            1f,
            0,
            0,
            InputDevice.SOURCE_TOUCHSCREEN,
            displayId,
            0,
            MotionEvent.CLASSIFICATION_NONE,
        )) { "case=$caseId route=native-touch phase=event-create" }
        return try {
            automation.injectInputEvent(event, true)
        } finally {
            event.recycle()
        }
    }

    private fun rejectedDownDiagnostic(): String {
        val targetPackage = instrumentation.targetContext.packageName
        val root = automation.rootInActiveWindow
        val rootPresent = root != null
        val rootPackageMatches = root?.packageName?.toString() == targetPackage
        val rootWindowId = root?.windowId
        val matchingWindow = rootWindowId?.let { id ->
            automation.windows.singleOrNull { it.id == id }
        }
        return "activeRootPresent=$rootPresent activeRootPackageMatches=$rootPackageMatches " +
            "matchingWindowPresent=${matchingWindow != null} " +
            "matchingWindowActive=${matchingWindow?.isActive == true} " +
            "matchingWindowFocused=${matchingWindow?.isFocused == true}"
    }

    override fun close() {
        if (state != NativeTouchState.Active) return
        val accepted = injectRaw(MotionEvent.ACTION_CANCEL, lastPoints)
        state = NativeTouchState.Ended
        clearActiveStream(this)
        assertTrue("case=$caseId route=native-touch phase=close-cancel rejected", accepted)
    }

    companion object {
        private val activeLock = Any()
        private var active: NativeTouchStream? = null

        private fun hasNoActiveStream(): Boolean = synchronized(activeLock) {
            active == null
        }

        private fun setActiveStream(stream: NativeTouchStream) {
            synchronized(activeLock) {
                check(active == null)
                active = stream
            }
        }

        private fun clearActiveStream(stream: NativeTouchStream) {
            synchronized(activeLock) {
                if (active === stream) active = null
            }
        }

        fun resetTracking() {
            synchronized(activeLock) {
                active = null
            }
        }
    }
}

private fun terminalTestActivity(context: Context): TerminalTestActivity? {
    var current = context
    while (true) {
        when (current) {
            is TerminalTestActivity -> return current
            is ContextWrapper -> {
                val base = current.baseContext
                if (base === current) return null
                current = base
            }
            else -> return null
        }
    }
}

private val ACCESSORIES = listOf(
    AccessoryExpectation("Escape", "\u001b", "\u001b", "\u001b\u001b", "\u001b\u001b"),
    AccessoryExpectation("Slash", "/", "/", "\u001b/", "\u001b/"),
    AccessoryExpectation("Hyphen", "-", "-", "\u001b-", "\u001b-"),
    AccessoryExpectation("Home", "\u001b[H", "\u001b[1;5H", "\u001b[1;3H", "\u001b[1;7H"),
    AccessoryExpectation("Up", "\u001b[A", "\u001b[1;5A", "\u001b[1;3A", "\u001b[1;7A"),
    AccessoryExpectation("End", "\u001b[F", "\u001b[1;5F", "\u001b[1;3F", "\u001b[1;7F"),
    AccessoryExpectation("PageUp", "\u001b[5~", "\u001b[5;5~", "\u001b[5;3~", "\u001b[5;7~"),
    AccessoryExpectation("Tab", "\t", "\t", "\u001b\t", "\u001b\t"),
    AccessoryExpectation("Left", "\u001b[D", "\u001b[1;5D", "\u001b[1;3D", "\u001b[1;7D"),
    AccessoryExpectation("Down", "\u001b[B", "\u001b[1;5B", "\u001b[1;3B", "\u001b[1;7B"),
    AccessoryExpectation("Right", "\u001b[C", "\u001b[1;5C", "\u001b[1;3C", "\u001b[1;7C"),
    AccessoryExpectation("PageDown", "\u001b[6~", "\u001b[6;5~", "\u001b[6;3~", "\u001b[6;7~"),
)

@RunWith(AndroidJUnit4::class)
class TerminalInstrumentedTest {
    @Before
    fun resetProbe() {
        TerminalTestProbe.reset()
        NativeTouchStream.resetTracking()
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val serviceInfo = automation.serviceInfo
        if (serviceInfo.flags and AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS == 0) {
            serviceInfo.flags = serviceInfo.flags or
                AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS
            automation.serviceInfo = serviceInfo
        }
    }

    @Test
    fun productionTerminalLoadsOnlyPackagedAssets() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            assertEquals(
                "https://appassets.androidplatform.net/assets/terminal/index.html",
                onUi(scenario) { webView.url },
            )
            assertEquals("Skíðblaðnir terminal", evaluate(webView, "document.title"))
            assertEquals("1", evaluate(webView, "document.querySelectorAll('.xterm-helper-textarea').length"))
            assertEquals(
                "default-src 'none'; style-src 'self'; style-src-elem 'self' 'unsafe-inline'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'none'; connect-src 'none'; font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
                evaluate(webView, "document.querySelector('meta[http-equiv=\\\"Content-Security-Policy\\\"]').content"),
            )
            val settings = onUi(scenario) {
                listOf(
                    webView.settings.allowFileAccess,
                    webView.settings.allowContentAccess,
                    webView.settings.blockNetworkLoads,
                    webView.settings.useWideViewPort,
                    webView.settings.loadWithOverviewMode,
                )
            }
            assertEquals(listOf(false, false, true, false, false), settings)
            assertEquals(100, onUi(scenario) { webView.settings.textZoom })
            assertFalse(onUi(scenario) { webView.isHorizontalScrollBarEnabled })
            // Geometry reaches native only once the vendored font has settled,
            // so every sample published across page load must already conform.
            val size = awaitSettledSizeWithAllSamplesConforming()
            assertTrue("terminal published an out-of-range size: $size", size.first in 80..240 && size.second in 5..120)
        }
    }

    @Test
    fun trustedTouchRoutesThroughXtermWheelOwner() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val startColumn = minOf(7, geometry.columns - 1)
            val startRow = geometry.rows / 2
            val start = geometry.cell(startColumn, startRow)
            focusTerminal(scenario, webView)

            applyControlFixture(
                page,
                "selectable fixture\r\n".repeat(geometry.rows + 4) + "\u001b[?1003h\u001b[?1006h",
                "touch-mouse-enable",
            )
            installCompatibilityEventCounter(webView)
            val mouseLocalPosition = awaitAccessiblePosition(webView, "touch-mouse-position-before")
            for (direction in TouchWheelDirection.entries) {
                clearTerminalEvents()
                if (direction == TouchWheelDirection.Backward) {
                    armModifiers(scenario, webView, BOTH_ARMED)
                }
                injectDrag(webView, geometry, start, direction, "touch-mouse-${direction.name}")
                val expected = sgrWheel(direction, startColumn + 1, startRow + 1)
                assertOnlyInput(
                    caseId = "touch-mouse",
                    route = "mouse",
                    direction = direction,
                    expected = expected,
                )
                if (direction == TouchWheelDirection.Backward) {
                    assertRoutedInputConsumesModifiers("touch-mouse", expected)
                }
                assertEquals(
                    "case=touch-mouse route=mouse direction=${direction.name} local-position",
                    mouseLocalPosition,
                    awaitAccessiblePosition(webView, "touch-mouse-${direction.name}-position"),
                )
            }
            SystemClock.sleep(ViewConfiguration.getTapTimeout().toLong() * 2)
            assertEquals(
                "case=touch-mouse route=compatibility count",
                0,
                compatibilityEventCount(webView),
            )
            assertEquals(
                "case=touch-mouse route=compatibility synthetic-mouse",
                "true",
                evaluateSafely(
                    webView,
                    "document.querySelector('.xterm-screen').dispatchEvent(" +
                        "new MouseEvent('click', {bubbles:true,cancelable:true,composed:true}))",
                    "touch-mouse-synthetic-click",
                ),
            )
            assertEquals(
                "case=touch-mouse route=compatibility synthetic-count",
                1,
                compatibilityEventCount(webView),
            )
            clearTerminalEvents()

            applyControlFixture(page, "\u001b[?1003l\u001b[?1006l", "touch-mouse-disable")
            val tapPosition = awaitAccessiblePosition(webView, "touch-tap-position-before")
            val tapSetSize = accessibleSetSize(webView, "touch-tap-set-size")
            assertEquals(
                "case=touch-tap route=local bottom-position",
                maxOf(1, tapSetSize - geometry.rows + 1),
                tapPosition,
            )
            awaitNoTerminalSelection(webView, "touch-tap-selection-before")
            clearTerminalEvents()
            awaitNativeWebViewFocus(scenario, webView, "touch-tap-native-focus-before")
            awaitBooleanState(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "touch-tap-textarea-focus-before",
            )
            injectBelowSlopTap(webView, geometry, start, "touch-tap")
            awaitNativeWebViewFocus(scenario, webView, "touch-tap-native-focus")
            awaitBooleanState(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "touch-tap-focus",
            )
            assertEquals(
                "case=touch-tap route=local position",
                tapPosition,
                awaitAccessiblePosition(webView, "touch-tap-position-after"),
            )
            awaitNoTerminalSelection(webView, "touch-tap-selection-after")
            assertNoInput("touch-tap", "local", TouchWheelDirection.Forward)

            val bottom = tapPosition
            clearTerminalEvents()
            injectDrag(webView, geometry, start, TouchWheelDirection.Forward, "touch-local-bottom")
            assertNoInput("touch-local-bottom", "local", TouchWheelDirection.Forward)
            assertEquals(
                "case=touch-local-bottom route=local direction=Forward",
                bottom,
                awaitAccessiblePosition(webView, "touch-local-bottom-position"),
            )
            awaitNoTerminalSelection(webView, "touch-local-bottom-selection")

            armModifiers(scenario, webView, BOTH_ARMED)
            injectDrag(webView, geometry, start, TouchWheelDirection.Backward, "touch-local-backward")
            assertNoInput("touch-local-backward", "local", TouchWheelDirection.Backward)
            awaitAccessiblePosition(webView, "touch-local-backward") { it < bottom }
            assertNull(
                "case=touch-local-backward route=local expectedCount=0 index=0",
                pollEvent(),
            )
            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(ALT_ARMED))
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))

            repeat(2) { index ->
                clearTerminalEvents()
                injectLargeBackwardDrag(webView, geometry, start, "touch-local-top-$index")
                assertNoInput("touch-local-top-$index", "local", TouchWheelDirection.Backward)
            }
            val top = awaitAccessiblePosition(webView, "touch-local-top-position") { it == 1 }
            clearTerminalEvents()
            injectDrag(webView, geometry, start, TouchWheelDirection.Backward, "touch-local-top-outward")
            assertNoInput("touch-local-top-outward", "local", TouchWheelDirection.Backward)
            assertEquals(
                "case=touch-local-top-outward route=local direction=Backward",
                top,
                awaitAccessiblePosition(webView, "touch-local-top-outward-position"),
            )

            evaluateSafely(
                webView,
                """
                (function () {
                    var screen = document.querySelector('.xterm-screen');
                    var bounds = screen.getBoundingClientRect();
                    var options = {
                        bubbles: true,
                        cancelable: true,
                        composed: true,
                        pointerId: 91,
                        pointerType: 'touch',
                        isPrimary: true,
                        clientX: bounds.left + bounds.width / 2,
                        clientY: bounds.top + bounds.height / 2
                    };
                    screen.dispatchEvent(new PointerEvent('pointerdown', options));
                    options.clientY -= bounds.height / 3;
                    screen.dispatchEvent(new PointerEvent('pointermove', options));
                    screen.dispatchEvent(new PointerEvent('pointerup', options));
                }())
                """.trimIndent(),
                "script-pointer",
            )
            SystemClock.sleep(250)
            assertEquals(
                "case=script-pointer route=local position",
                top,
                awaitAccessiblePosition(webView, "script-pointer-position"),
            )
            assertNoInput("script-pointer", "none", TouchWheelDirection.Forward)

            applyControlFixture(page, "\u001b[?1049h\u001b[?1h", "touch-alternate")
            for (direction in TouchWheelDirection.entries) {
                clearTerminalEvents()
                if (direction == TouchWheelDirection.Backward) {
                    armModifiers(scenario, webView, BOTH_ARMED)
                }
                injectDrag(webView, geometry, start, direction, "touch-alternate-${direction.name}")
                val expected = "\u001bO${direction.cursorSuffix}".toByteArray()
                assertOnlyInput(
                    caseId = "touch-alternate",
                    route = "cursor",
                    direction = direction,
                    expected = expected,
                )
                if (direction == TouchWheelDirection.Backward) {
                    assertRoutedInputConsumesModifiers("touch-alternate", expected)
                }
            }

            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" +
                    "selectable fixture\r\n".repeat(geometry.rows + 1),
                "touch-selection",
            )
            awaitNoTerminalSelection(webView, "touch-selection-before")
            installTouchEventDiagnostics(webView, "touch-selection")
            injectLongPress(webView, start, "touch-selection")
            awaitBooleanState(
                webView,
                "document.querySelector('.xterm-selection').childElementCount > 0 || !window.getSelection().isCollapsed",
                "touch-selection-active",
                diagnostics = {
                    touchEventDiagnostics(webView, "touch-selection").summary()
                },
            )
            val selectedPosition = awaitAccessiblePosition(webView, "touch-selection-position")
            clearTerminalEvents()
            injectDrag(webView, geometry, start, TouchWheelDirection.Forward, "touch-selection-block")
            assertEquals(
                "case=touch-selection route=blocked position",
                selectedPosition,
                awaitAccessiblePosition(webView, "touch-selection-retained-position"),
            )
            awaitBooleanState(
                webView,
                "document.querySelector('.xterm-selection').childElementCount > 0 || !window.getSelection().isCollapsed",
                "touch-selection-retained",
            )
            assertNoInput("touch-selection", "blocked", TouchWheelDirection.Forward)
        }
    }

    @Test
    fun nativeWheelActionsRouteThroughFocusedChromiumNode() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            focusTerminal(scenario, webView)
            val firstFocusedNode = awaitFocusedTerminalRowNode(
                "actions-enabled-first",
                requireFocusAction = true,
            )
            val firstActions = assertFocusedActionOwnership(
                firstFocusedNode,
                "actions-enabled-first",
            )
            val firstFocusedKey = requireTerminalRowKey(
                firstFocusedNode,
                "actions-enabled-first-key",
            )
            val staleFirstAction = requireNotNull(
                firstActions.singleOrNull { it.label == TERMINAL_WHEEL_FORWARD },
            )
            val focusedNode = focusDistinctTerminalRowNode(
                firstFocusedNode,
                "actions-enabled-transfer",
            )
            assertFocusedActionOwnership(focusedNode, "actions-enabled-transfer")
            val previousNode = awaitTerminalRowNode(
                firstFocusedKey,
                "actions-enabled-transfer-previous",
            )
            assertFalse(
                "case=actions-enabled-transfer route=accessibility prior-focused-state",
                previousNode.isAccessibilityFocused,
            )
            clearTerminalEvents()
            assertFalse(
                "case=actions-enabled-transfer route=accessibility stale-action",
                previousNode.performAction(staleFirstAction.id),
            )
            assertNoInput(
                "actions-enabled-transfer-stale",
                "accessibility",
                TouchWheelDirection.Forward,
            )

            assertTrue(
                "case=actions-stock route=delegate",
                focusedNode.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS),
            )
            awaitNoAccessibilityFocus("actions-stock")
            focusTerminal(scenario, webView)
            awaitFocusedTerminalRowNode("actions-stock-restored", requireFocusAction = true)

            val nonRowNode = focusTerminalNonRowNode("actions-non-row")
            assertTrue(
                "case=actions-non-row route=accessibility custom-action-count",
                terminalWheelActions(nonRowNode).isEmpty(),
            )
            assertNoTerminalWheelActionLabels("actions-non-row")
            clearTerminalEvents()
            assertFalse(
                "case=actions-non-row route=accessibility custom-action-delegation",
                nonRowNode.performAction(staleFirstAction.id),
            )
            assertNoInput(
                "actions-non-row",
                "accessibility",
                TouchWheelDirection.Forward,
            )
            assertTrue(
                "case=actions-non-row route=accessibility focus-clear-rejected",
                nonRowNode.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS),
            )
            awaitNoAccessibilityFocus("actions-non-row-clear")
            focusTerminal(scenario, webView)
            awaitFocusedTerminalRowNode("actions-non-row-restored", requireFocusAction = true)

            applyControlFixture(
                page,
                "\r\n".repeat(geometry.rows * 5),
                "actions-scrollback",
            )
            val pageDelta = maxOf(1, geometry.rows - 1)
            val before = awaitAccessiblePosition(webView, "actions-local-before") {
                it - pageDelta > 1
            }
            assertEquals(
                "case=actions-local route=local direction=Backward bottom-position",
                maxOf(
                    1,
                    accessibleSetSize(webView, "actions-local-before-set-size") -
                        geometry.rows + 1,
                ),
                before,
            )
            clearTerminalEvents()
            performTerminalWheelAction(TERMINAL_WHEEL_BACKWARD, "actions-local-backward")
            val backward = awaitAccessiblePosition(webView, "actions-local-backward-after") {
                it == before - pageDelta
            }
            assertEquals(
                "case=actions-local route=local direction=Backward delta",
                pageDelta,
                before - backward,
            )
            assertNoInput("actions-local", "local", TouchWheelDirection.Backward)
            clearTerminalEvents()
            performTerminalWheelAction(TERMINAL_WHEEL_FORWARD, "actions-local-forward")
            val forward = awaitAccessiblePosition(webView, "actions-local-forward-after") {
                it == backward + pageDelta
            }
            assertEquals(
                "case=actions-local route=local direction=Forward delta",
                pageDelta,
                forward - backward,
            )
            assertNoInput("actions-local", "local", TouchWheelDirection.Forward)

            applyControlFixture(page, "\u001b[?1049h\u001b[?1h", "actions-alternate")
            val actionGestureStart = geometry.cell(
                minOf(7, geometry.columns - 1),
                geometry.rows / 2,
            )
            val actionGestureNode = awaitFocusedTerminalRowNode("actions-cancel-touch")
            val backwardAction = requireNotNull(
                terminalWheelActions(actionGestureNode).singleOrNull {
                    it.label == TERMINAL_WHEEL_BACKWARD
                },
            )
            clearTerminalEvents()
            NativeTouchStream(webView, "actions-cancel-touch").use { actionGesture ->
                actionGesture.down(actionGestureStart)
                actionGesture.move(
                    actionGestureStart.copy(
                        y = actionGestureStart.y + geometry.belowSlopDistance,
                    ),
                )
                actionGesture.move(geometry.claim(actionGestureStart))
                assertOnlyInput(
                    caseId = "actions-cancel-touch-prior",
                    route = "cursor",
                    direction = TouchWheelDirection.Backward,
                    expected = "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                assertTrue(
                    "case=actions-cancel-touch route=accessibility action-rejected",
                    actionGestureNode.performAction(backwardAction.id),
                )
                assertOnlyInput(
                    caseId = "actions-cancel-touch",
                    route = "cursor",
                    direction = TouchWheelDirection.Backward,
                    expected = "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                val oldGestureEnd = geometry.postCancel(actionGestureStart)
                actionGesture.move(oldGestureEnd)
                actionGesture.up(oldGestureEnd)
                assertNoInput("actions-cancel-touch-tail", "cursor", TouchWheelDirection.Backward)
            }

            for (direction in listOf(TouchWheelDirection.Forward)) {
                clearTerminalEvents()
                performTerminalWheelAction(actionLabel(direction), "actions-alternate-${direction.name}")
                assertOnlyInput(
                    caseId = "actions-alternate",
                    route = "cursor",
                    direction = direction,
                    expected = "\u001bO${direction.cursorSuffix}".toByteArray(),
                )
            }

            applyControlFixture(page, "\u001b[?1049l\u001b[?1003h\u001b[?1006h", "actions-mouse")
            val centerColumn = geometry.columns / 2 + 1
            val centerRow = geometry.rows / 2 + 1
            for (direction in TouchWheelDirection.entries) {
                clearTerminalEvents()
                performTerminalWheelAction(actionLabel(direction), "actions-mouse-${direction.name}")
                assertOnlyInput(
                    caseId = "actions-mouse",
                    route = "mouse",
                    direction = direction,
                    expected = sgrWheel(direction, centerColumn, centerRow),
                )
            }

            val enabledNode = awaitFocusedTerminalRowNode("actions-before-disable")
            val cachedForward = requireNotNull(
                terminalWheelActions(enabledNode).singleOrNull { it.label == TERMINAL_WHEEL_FORWARD },
            )
            val disabledEvents = recordTerminalAccessibilityEvents {
                onUi(scenario) { webView.isEnabled = false }
            }
            assertHasSubtreeRefresh(disabledEvents, "actions-disabled")
            assertFalse(
                "case=actions-disabled route=accessibility stale-action",
                enabledNode.performAction(cachedForward.id),
            )
            assertNoTerminalWheelActionLabels("actions-disabled")
            assertTrue(
                "case=actions-disabled route=accessibility scroll-event",
                disabledEvents.none { it.first == AccessibilityEvent.TYPE_VIEW_SCROLLED },
            )

            val enabledEvents = recordTerminalAccessibilityEvents {
                onUi(scenario) { webView.isEnabled = true }
            }
            assertHasSubtreeRefresh(enabledEvents, "actions-reenabled")
            assertTrue(
                "case=actions-reenabled route=accessibility scroll-event",
                enabledEvents.none { it.first == AccessibilityEvent.TYPE_VIEW_SCROLLED },
            )
            focusTerminal(scenario, webView)
            val reenabledActions = terminalWheelActions(
                awaitFocusedTerminalRowNode("actions-reenabled"),
            )
            assertEquals(
                "case=actions-reenabled route=accessibility labels",
                listOf(TERMINAL_WHEEL_BACKWARD, TERMINAL_WHEEL_FORWARD),
                reenabledActions.map { it.label }.sorted(),
            )
            postAccessory(scenario, webView, "PageUp")
            assertOnlyInput(
                "actions-page-up",
                "raw-key",
                TouchWheelDirection.Backward,
                "\u001b[5~".toByteArray(),
            )
            postAccessory(scenario, webView, "PageDown")
            assertOnlyInput(
                "actions-page-down",
                "raw-key",
                TouchWheelDirection.Forward,
                "\u001b[6~".toByteArray(),
            )

            focusTerminal(scenario, webView)
            val availableNode = awaitFocusedTerminalRowNode("actions-before-unavailable")
            val availableForward = requireNotNull(
                terminalWheelActions(availableNode).singleOrNull { it.label == TERMINAL_WHEEL_FORWARD },
            )
            val unavailableEvents = recordTerminalAccessibilityEvents {
                postRawNativeMessage(scenario, webView, "not-json")
                assertTrue(
                    "case=actions-unavailable route=accessibility count=1",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
            }
            assertHasSubtreeRefresh(unavailableEvents, "actions-unavailable")
            assertTrue(
                "case=actions-unavailable route=accessibility scroll-event",
                unavailableEvents.none { it.first == AccessibilityEvent.TYPE_VIEW_SCROLLED },
            )
            assertFalse(
                "case=actions-unavailable route=accessibility stale-action",
                availableNode.performAction(availableForward.id),
            )
            assertNoTerminalWheelActionLabels("actions-unavailable")
        }

        TerminalTestProbe.reset()
        val unreadyProbe = TerminalProbe()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val unreadyWebView = onUi(scenario) { activity ->
                createTestTerminal(
                    context = activity,
                    probe = unreadyProbe,
                    initialUrl = "https://appassets.androidplatform.net/assets/terminal/terminal.css",
                ).also(activity::setContentView)
            }
            awaitBooleanState(
                unreadyWebView,
                "document.readyState === 'complete'",
                "actions-unready-load-complete",
            )
            assertEquals("case=actions-unready route=lifecycle readyCount", 1L, unreadyProbe.ready.count)
            assertEquals("case=actions-unready route=lifecycle unavailableCount", 1L, unreadyProbe.unavailable.count)
            assertNoTerminalWheelActionLabels("actions-unready")
        }

        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            focusTerminal(scenario, webView)
            val node = awaitFocusedTerminalRowNode("actions-before-dispose", requireFocusAction = true)
            val action = requireNotNull(
                terminalWheelActions(node).singleOrNull { it.label == TERMINAL_WHEEL_FORWARD },
            )
            onUi(scenario) { (webView as LockedTerminalWebView).dispose() }
            assertFalse(
                "case=actions-disposed route=accessibility stale-action",
                node.performAction(action.id),
            )
            assertNoTerminalWheelActionLabels("actions-disposed")
        }

        val invalidMessages = listOf(
            "scroll-missing" to """{"kind":"Scroll"}""",
            "scroll-extra" to """{"kind":"Scroll","direction":"Backward","extra":true}""",
            "scroll-type" to """{"kind":"Scroll","direction":1}""",
            "scroll-case" to """{"kind":"scroll","direction":"Backward"}""",
            "scroll-direction" to """{"kind":"Scroll","direction":"Sideways"}""",
            "scroll-direction-case" to """{"kind":"Scroll","direction":"backward"}""",
            "reset-missing" to """{}""",
            "reset-extra" to """{"kind":"ResetInputState","extra":true}""",
            "reset-type" to """{"kind":1}""",
            "reset-case" to """{"kind":"resetInputState"}""",
            "reset-retired" to """{"kind":"ResetModifiers"}""",
        )
        for ((caseId, payload) in invalidMessages) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val webView = awaitTerminal(scenario)
                focusTerminal(scenario, webView)
                val page = requireNotNull(TerminalTestProbe.page)
                val geometry = terminalTouchGeometry(scenario, webView)
                val start = geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2)
                applyControlFixture(
                    page,
                    "\r\n".repeat(geometry.rows * 5),
                    "$caseId-route",
                )
                val pageDelta = maxOf(1, geometry.rows - 1)
                val bottom = awaitStableAccessiblePosition(webView, "$caseId-bottom")
                postRawNativeMessage(scenario, webView, """{"kind":"Scroll","direction":"Backward"}""")
                val middle = awaitAccessiblePosition(webView, "$caseId-middle") {
                    it == bottom - pageDelta
                }
                assertTrue(
                    "case=$caseId route=local unclipped",
                    middle - pageDelta > 1,
                )
                assertNoInput("$caseId-route", "local", TouchWheelDirection.Backward)
                clearTerminalEvents()
                installTouchEventDiagnostics(webView, caseId)
                NativeTouchStream(webView, "$caseId-pending").use { pending ->
                    pending.down(start)
                    assertEquals(
                        "case=$caseId route=local down-position",
                        middle,
                        awaitAccessiblePosition(webView, "$caseId-down-position"),
                    )
                    pending.move(start.copy(y = start.y + geometry.belowSlopDistance))
                    assertEquals(
                        "case=$caseId route=local below-slop-position",
                        middle,
                        awaitAccessiblePosition(webView, "$caseId-below-slop-position"),
                    )
                    pending.move(geometry.claim(start))
                    assertNoQueuedInput(
                        "$caseId-claimed",
                        "local",
                        TouchWheelDirection.Backward,
                    )
                    val claimed = awaitAccessiblePosition(
                        webView,
                        "$caseId-claimed-position",
                        diagnostics = {
                            touchEventDiagnostics(webView, caseId).summary()
                        },
                    ) {
                        it == middle - 1
                    }
                    assertEquals(
                        "case=$caseId route=local claimed-delta",
                        1,
                        middle - claimed,
                    )
                    assertNoInput("$caseId-claimed", "local", TouchWheelDirection.Backward)
                    clearTerminalEvents()
                    postRawNativeMessages(
                        scenario,
                        webView,
                        listOf(payload, """{"kind":"Scroll","direction":"Backward"}"""),
                    )
                    assertTrue(
                        "case=$caseId route=native-protocol unavailable",
                        TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                    )
                    val end = geometry.postCancel(start)
                    pending.move(end)
                    pending.up(end)
                    assertNoInput(caseId, "failed-page", TouchWheelDirection.Backward)
                    assertEquals(
                        "case=$caseId route=local failed-position",
                        claimed,
                        awaitAccessiblePosition(webView, "$caseId-failed-position"),
                    )
                }
            }
        }
    }

    @Test
    fun inputStateLifecycleCancelsTouchAndPreservesContainment() {
        val preReadyProbe = TerminalProbe()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            onUi(scenario) { activity ->
                createTestTerminal(activity, preReadyProbe).also { webView ->
                    webView.isEnabled = false
                    activity.setContentView(webView)
                }
            }
            assertTrue("case=pre-ready-disable route=lifecycle ready", preReadyProbe.ready.await(5, TimeUnit.SECONDS))
            assertEquals("case=pre-ready-disable route=lifecycle unavailable", 1L, preReadyProbe.unavailable.count)
            assertEquals(
                "case=pre-ready-disable route=lifecycle events",
                listOf(TerminalTestEvent.Modifiers(OFF_OFF), TerminalTestEvent.Ready),
                listOfNotNull(
                    preReadyProbe.events.poll(5, TimeUnit.SECONDS),
                    preReadyProbe.events.poll(5, TimeUnit.SECONDS),
                ),
            )
            assertNull(
                "case=pre-ready-disable route=lifecycle expectedCount=2 index=2",
                preReadyProbe.events.poll(350, TimeUnit.MILLISECONDS),
            )
            assertNull(
                "case=pre-ready-disable route=lifecycle expectedCount=0 index=0",
                preReadyProbe.input.poll(350, TimeUnit.MILLISECONDS),
            )
        }

        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            var webView = awaitTerminal(scenario)
            var page = requireNotNull(TerminalTestProbe.page)
            var geometry = terminalTouchGeometry(scenario, webView)
            var start = geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2)
            val outsideScreen = terminalDocumentPointOutsideScreen(
                scenario,
                webView,
                geometry,
                "second-pointer-outside-screen",
            )
            applyControlFixture(page, "\u001b[?1049h\u001b[?1h", "lifecycle-alternate")

            clearTerminalEvents()
            installTouchEventDiagnostics(webView, "horizontal-first")
            injectHorizontalFirst(webView, geometry, start, "horizontal-first")
            assertNoInput(
                "horizontal-first",
                "cursor",
                TouchWheelDirection.Forward,
            ) { touchEventDiagnostics(webView, "horizontal-first").summary() }

            clearTerminalEvents()
            NativeTouchStream(webView, "post-claim-reversal").use { reversalStream ->
                reversalStream.down(start)
                reversalStream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                reversalStream.move(geometry.claim(start))
                assertOnlyInput(
                    "post-claim-reversal-prior",
                    "cursor",
                    TouchWheelDirection.Backward,
                    "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                val reversedDebt = start.copy(y = start.y + 0.75f * geometry.rowHeight)
                val settledDebt = start.copy(y = start.y + geometry.rowHeight)
                reversalStream.move(reversedDebt)
                reversalStream.move(settledDebt)
                reversalStream.up(settledDebt)
                assertNoInput(
                    "post-claim-reversal-tail",
                    "cursor",
                    TouchWheelDirection.Backward,
                )
            }

            clearTerminalEvents()
            NativeTouchStream(webView, "second-pointer-before-claim").use { stream ->
                val belowSlop = start.copy(y = start.y + geometry.belowSlopDistance)
                val end = geometry.postCancel(start)
                stream.down(start)
                stream.move(belowSlop)
                stream.secondDown(belowSlop, outsideScreen)
                stream.move(end, outsideScreen)
                stream.secondUp(end, outsideScreen)
                stream.up(end)
            }
            assertNoInput(
                "second-pointer-before-claim",
                "cursor",
                TouchWheelDirection.Backward,
            )

            clearTerminalEvents()
            NativeTouchStream(webView, "second-pointer-after-claim").use { secondPointerStream ->
                val secondPointerClaim = geometry.claim(start)
                val secondPointerEnd = geometry.postCancel(start)
                secondPointerStream.down(start)
                secondPointerStream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                secondPointerStream.move(secondPointerClaim)
                assertOnlyInput(
                    "second-pointer-after-claim-prior",
                    "cursor",
                    TouchWheelDirection.Backward,
                    "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                secondPointerStream.secondDown(secondPointerClaim, outsideScreen)
                secondPointerStream.move(
                    secondPointerEnd,
                    outsideScreen,
                )
                secondPointerStream.secondUp(
                    secondPointerEnd,
                    outsideScreen,
                )
                secondPointerStream.up(secondPointerEnd)
                assertNoInput(
                    "second-pointer-after-claim-tail",
                    "cursor",
                    TouchWheelDirection.Backward,
                )
            }

            clearTerminalEvents()
            val backgroundFocusLost = CountDownLatch(1)
            val backgroundFocusRegained = CountDownLatch(1)
            var backgroundLossObserved = false
            val backgroundFocusListener = ViewTreeObserver.OnWindowFocusChangeListener { focused ->
                if (!focused) {
                    backgroundLossObserved = true
                    backgroundFocusLost.countDown()
                } else if (backgroundLossObserved) {
                    backgroundFocusRegained.countDown()
                }
            }
            onUi(scenario) {
                webView.viewTreeObserver.addOnWindowFocusChangeListener(backgroundFocusListener)
            }
            NativeTouchStream(
                webView,
                "background-active",
                primaryPointerId = 2,
            ).use { backgroundStream ->
                backgroundStream.down(start)
                backgroundStream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                backgroundStream.move(geometry.claim(start))
                assertOnlyInput(
                    "background-active-prior",
                    "cursor",
                    TouchWheelDirection.Backward,
                    "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                scenario.moveToState(Lifecycle.State.CREATED)
                assertTrue(
                    "case=background-active route=lifecycle focus-lost",
                    backgroundFocusLost.await(5, TimeUnit.SECONDS),
                )
                assertNoInput("background-active", "cursor", TouchWheelDirection.Backward)
                scenario.moveToState(Lifecycle.State.RESUMED)
                assertTrue(
                    "case=background-active route=lifecycle focus-regained",
                    backgroundFocusRegained.await(5, TimeUnit.SECONDS),
                )
                awaitAnimationFrame(webView, "background-resume-frame-1")
                awaitAnimationFrame(webView, "background-resume-frame-2")
                backgroundStream.cancel(geometry.claim(start))
                assertNoInput("background-active-tail", "cursor", TouchWheelDirection.Backward)
            }
            onUi(scenario) {
                if (webView.viewTreeObserver.isAlive) {
                    webView.viewTreeObserver.removeOnWindowFocusChangeListener(backgroundFocusListener)
                }
            }
            clearTerminalEvents()
            injectDrag(
                webView,
                geometry,
                start,
                TouchWheelDirection.Forward,
                "background-fresh",
                primaryPointerId = 3,
            )
            assertOnlyInput(
                "background-fresh",
                "cursor",
                TouchWheelDirection.Forward,
                "\u001bOB".toByteArray(),
            )

            clearTerminalEvents()
            NativeTouchStream(
                webView,
                "rotation-active",
                primaryPointerId = 4,
            ).use { rotationStream ->
                rotationStream.down(start)
                rotationStream.move(
                    start.copy(y = start.y + geometry.belowSlopDistance),
                )
                rotationStream.move(geometry.claim(start))
                assertOnlyInput(
                    "rotation-active-prior",
                    "cursor",
                    TouchWheelDirection.Backward,
                    "\u001bOA".toByteArray(),
                )
                clearTerminalEvents()
                TerminalTestProbe.sizes.clear()
                onUi(scenario) { it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE }
                awaitValue(webView, "window.innerWidth > window.innerHeight", "true")
                assertNoInput("rotation-active", "cursor", TouchWheelDirection.Backward)
                rotationStream.cancel(geometry.claim(start))
                TerminalTestProbe.sizes.clear()
                onUi(scenario) { it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_PORTRAIT }
                awaitValue(webView, "window.innerHeight > window.innerWidth", "true")
                geometry = terminalTouchGeometry(scenario, webView)
                assertNoInput("rotation-active-tail", "cursor", TouchWheelDirection.Backward)
            }
            start = geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2)
            clearTerminalEvents()
            injectDrag(
                webView,
                geometry,
                start,
                TouchWheelDirection.Forward,
                "rotation-fresh",
                primaryPointerId = 5,
            )
            assertOnlyInput(
                "rotation-fresh",
                "cursor",
                TouchWheelDirection.Forward,
                "\u001bOB".toByteArray(),
            )

            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" + "\r\n".repeat(geometry.rows * 5),
                "action-cancel-local",
            )
            val actionCancelBefore = awaitStableAccessiblePosition(webView, "action-cancel-before") {
                it - 2 > 1
            }
            assertTrue(
                "case=action-cancel route=local unclipped",
                actionCancelBefore - 2 > 1,
            )
            clearTerminalEvents()
            var actionCancelClaimed = -1
            NativeTouchStream(
                webView,
                "action-cancel",
                primaryPointerId = 6,
            ).use { stream ->
                val actionCancelClaim = geometry.claim(start)
                stream.down(start)
                stream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                stream.move(actionCancelClaim)
                actionCancelClaimed = awaitAccessiblePosition(
                    webView,
                    "action-cancel-claimed",
                    diagnostics = {
                        "before=$actionCancelBefore claimWholeRows=${geometry.claimWholeRows}"
                    },
                ) {
                    it == actionCancelBefore - 1
                }
                assertEquals(
                    "case=action-cancel route=local claimed-delta",
                    1,
                    actionCancelBefore - actionCancelClaimed,
                )
                assertNoInput("action-cancel-prior", "local", TouchWheelDirection.Backward)
                clearTerminalEvents()
                stream.cancel(actionCancelClaim)
                assertNoInput("action-cancel-after", "local", TouchWheelDirection.Backward)
                assertEquals(
                    "case=action-cancel route=local stable-position",
                    actionCancelClaimed,
                    awaitAccessiblePosition(webView, "action-cancel-stable-position"),
                )
            }
            clearTerminalEvents()
            NativeTouchStream(
                webView,
                "action-cancel-fresh",
                primaryPointerId = 7,
            ).use { stream ->
                val freshClaim = geometry.claim(start)
                stream.down(start)
                stream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                stream.move(freshClaim)
                val freshClaimed = awaitAccessiblePosition(
                    webView,
                    "action-cancel-fresh-claimed",
                    diagnostics = {
                        "before=$actionCancelClaimed claimWholeRows=${geometry.claimWholeRows}"
                    },
                ) {
                    it == actionCancelClaimed - 1
                }
                assertEquals(
                    "case=action-cancel-fresh route=local claimed-delta",
                    1,
                    actionCancelClaimed - freshClaimed,
                )
                stream.up(freshClaim)
            }
            val actionCancelFresh = awaitAccessiblePosition(
                webView,
                "action-cancel-fresh-position",
                diagnostics = { "before=$actionCancelClaimed" },
            ) { it == actionCancelClaimed - 1 }
            assertEquals(
                "case=action-cancel-fresh route=local exact-delta",
                1,
                actionCancelClaimed - actionCancelFresh,
            )
            assertNoInput("action-cancel-fresh", "local", TouchWheelDirection.Backward)

            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" + "\r\n".repeat(geometry.rows * 5),
                "pointer-up-residual-local",
            )
            val residualStart = start
            val residualBefore = awaitStableAccessiblePosition(webView, "pointer-up-residual-before") {
                it - 2 > 1
            }
            assertTrue(
                "case=pointer-up-residual route=local unclipped",
                residualBefore - 2 > 1,
            )
            clearTerminalEvents()
            var residualAfter = -1
            NativeTouchStream(webView, "pointer-up-residual").use { residualStream ->
                residualStream.down(residualStart)
                residualStream.move(
                    residualStart.copy(y = residualStart.y + geometry.belowSlopDistance),
                )
                residualStream.move(geometry.claim(residualStart))
                val residualFirst = awaitAccessiblePosition(
                    webView,
                    "pointer-up-residual-first",
                    diagnostics = {
                        "before=$residualBefore claimWholeRows=${geometry.claimWholeRows}"
                    },
                ) { it == residualBefore - 1 }
                assertEquals(
                    "case=pointer-up-residual route=local first-delta",
                    1,
                    residualBefore - residualFirst,
                )
                assertNoInput(
                    "pointer-up-residual-first",
                    "local",
                    TouchWheelDirection.Backward,
                )
                clearTerminalEvents()
                val residualEnd = residualStart.copy(
                    y = residualStart.y + geometry.postCancelDistance,
                )
                residualStream.move(residualEnd)
                residualStream.up(residualEnd)
                residualAfter = awaitAccessiblePosition(webView, "pointer-up-residual-after") {
                    it == residualFirst - 1
                }
                assertEquals(
                    "case=pointer-up-residual route=local residual-delta",
                    1,
                    residualFirst - residualAfter,
                )
                assertNoInput(
                    "pointer-up-residual-flush",
                    "local",
                    TouchWheelDirection.Backward,
                )
            }
            clearTerminalEvents()
            NativeTouchStream(webView, "pointer-up-residual-followup").use { stream ->
                val end = residualStart.copy(y = residualStart.y + geometry.rowHeight * 0.9f)
                stream.down(residualStart)
                stream.move(end)
                stream.up(end)
            }
            assertEquals(
                "case=pointer-up-residual-followup route=local position",
                residualAfter,
                awaitAccessiblePosition(webView, "pointer-up-residual-followup-position"),
            )
            assertNoInput(
                "pointer-up-residual-followup",
                "local",
                TouchWheelDirection.Backward,
            )

            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" + "\r\n".repeat(geometry.rows * 5),
                "in-display-reconciliation-local",
            )
            val reconciliationStart = start
            val reconciliationRows = maxOf(2, geometry.rows / 4)
            val overshootEnd = reconciliationStart.copy(
                y = reconciliationStart.y + (reconciliationRows + 1.2f) * geometry.rowHeight,
            )
            val finalEnd = reconciliationStart.copy(
                y = reconciliationStart.y + (reconciliationRows + 0.25f) * geometry.rowHeight,
            )
            assertTrue(
                "case=in-display-reconciliation route=local bounded-rows",
                reconciliationRows in 1 until geometry.rows &&
                    geometry.contains(overshootEnd) && geometry.contains(finalEnd),
            )
            assertEquals(
                "case=in-display-reconciliation route=local final-rows",
                reconciliationRows,
                ((finalEnd.y - reconciliationStart.y) / geometry.rowHeight).toInt(),
            )
            val reconciliationBefore = awaitStableAccessiblePosition(
                webView,
                "in-display-reconciliation-before",
            ) {
                it - reconciliationRows > 1
            }
            assertTrue(
                "case=in-display-reconciliation route=local unclipped",
                reconciliationBefore - reconciliationRows > 1,
            )
            clearTerminalEvents()
            NativeTouchStream(webView, "in-display-reconciliation").use { stream ->
                val claim = geometry.claim(reconciliationStart)
                stream.down(reconciliationStart)
                stream.move(
                    reconciliationStart.copy(
                        y = reconciliationStart.y + geometry.belowSlopDistance,
                    ),
                )
                stream.move(claim)
                val remainingDistance = overshootEnd.y - claim.y
                val travelFrames = maxOf(
                    1,
                    (remainingDistance / geometry.rowHeight).toInt() + 1,
                )
                for (frame in 1..travelFrames) {
                    val progress = frame.toFloat() / travelFrames
                    stream.move(
                        TouchPoint(
                            x = claim.x + (overshootEnd.x - claim.x) * progress,
                            y = claim.y + remainingDistance * progress,
                        ),
                    )
                }
                stream.move(finalEnd)
                stream.up(finalEnd)
            }
            val reconciliationAfter = awaitAccessiblePosition(
                webView,
                "in-display-reconciliation-after",
                diagnostics = {
                    "before=$reconciliationBefore rows=$reconciliationRows"
                },
            ) {
                it == reconciliationBefore - reconciliationRows
            }
            assertEquals(
                "case=in-display-reconciliation route=local displacement",
                reconciliationRows,
                reconciliationBefore - reconciliationAfter,
            )
            assertNoInput("in-display-reconciliation", "local", TouchWheelDirection.Backward)
            assertEquals(
                "case=in-display-reconciliation route=local no-debt",
                reconciliationAfter,
                awaitAccessiblePosition(webView, "in-display-reconciliation-settled"),
            )
            // Any two in-display coordinates are separated by fewer than rows
            // row heights, so the terminal.rows magnitude clamp has no honest
            // dynamic mutation point without off-screen injection or frame seams.
            focusTerminal(scenario, webView)
            val disposeBefore = awaitStableAccessiblePosition(webView, "dispose-before") {
                it - 1 > 1
            }
            assertTrue(
                "case=dispose-pending route=local unclipped",
                disposeBefore - 1 > 1,
            )
            clearTerminalEvents()
            NativeTouchStream(
                webView,
                "dispose-pending",
                primaryPointerId = 10,
            ).use { disposeStream ->
                disposeStream.down(start)
                disposeStream.move(start.copy(y = start.y + geometry.belowSlopDistance))
                disposeStream.move(geometry.claim(start))
                val disposeClaimed = awaitAccessiblePosition(webView, "dispose-claimed") {
                    it == disposeBefore - 1
                }
                assertEquals(
                    "case=dispose-pending route=local claimed-delta",
                    1,
                    disposeBefore - disposeClaimed,
                )
                assertNoInput("dispose-claimed", "local", TouchWheelDirection.Backward)
                clearTerminalEvents()
                onUi(scenario) { (webView as LockedTerminalWebView).dispose() }
                disposeStream.cancel(geometry.claim(start))
                assertNoInput("dispose-pending", "none", TouchWheelDirection.Backward)
            }

            TerminalTestProbe.reset()
            scenario.recreate()
            webView = awaitTerminal(scenario)
            page = requireNotNull(TerminalTestProbe.page)
            geometry = terminalTouchGeometry(scenario, webView)
            applyControlFixture(page, "\u001b[?1049h\u001b[?1h", "recreation-fresh")
            clearTerminalEvents()
            injectDrag(
                webView,
                geometry,
                geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2),
                TouchWheelDirection.Forward,
                "recreation-fresh",
                primaryPointerId = 11,
            )
            assertOnlyInput(
                "recreation-fresh",
                "cursor",
                TouchWheelDirection.Forward,
                "\u001bOB".toByteArray(),
            )
            assertEquals("case=recreation-fresh route=containment webview-x", 0, onUi(scenario) { webView.scrollX })
            assertEquals("case=recreation-fresh route=containment webview-y", 0, onUi(scenario) { webView.scrollY })
            assertTrue(
                "case=recreation-fresh route=geometry",
                geometry.columns >= 80 && geometry.rows >= 5,
            )
        }
    }

    @Test
    fun disabledTerminalRejectsTouchAndAccessibilityUntilReenabled() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val start = geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2)
            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" + "\r\n".repeat(geometry.rows * 5),
                "enabled-disable-local",
            )
            installTouchEventDiagnostics(webView, "enabled-disable")
            val disabledBefore = awaitStableAccessiblePosition(
                webView,
                "enabled-disable-before",
            ) { it > 2 }
            focusTerminal(scenario, webView)
            val disableNode = awaitFocusedTerminalRowNode(
                "enabled-disable-before",
                requireFocusAction = true,
            )
            val disableAction = requireNotNull(
                terminalWheelActions(disableNode).singleOrNull {
                    it.label == TERMINAL_WHEEL_BACKWARD
                },
            )
            armModifiers(scenario, webView, BOTH_ARMED)
            clearTerminalEvents()
            var disabledClaimed = -1
            NativeTouchStream(webView, "enabled-disable").use { pendingDisable ->
                pendingDisable.down(start)
                pendingDisable.move(start.copy(y = start.y + geometry.belowSlopDistance))
                pendingDisable.move(geometry.claim(start))
                disabledClaimed = awaitAccessiblePosition(webView, "enabled-disable-claimed") {
                    it == disabledBefore - 1
                }
                assertEquals(
                    "case=enabled-disable route=local claimed-delta",
                    1,
                    disabledBefore - disabledClaimed,
                )
                assertNoInput("enabled-disable-claimed", "local", TouchWheelDirection.Backward)
                clearTerminalEvents()
                onUi(scenario) { webView.isEnabled = false }
                assertEvent(TerminalTestEvent.Modifiers(OFF_OFF), "enabled-disable-reset")
                val disabledEnd = geometry.postCancel(start)
                pendingDisable.move(disabledEnd)
                pendingDisable.up(disabledEnd)
                assertNoInput("enabled-disable", "local", TouchWheelDirection.Backward)
                assertEquals(
                    "case=enabled-disable route=local position",
                    disabledClaimed,
                    awaitAccessiblePosition(webView, "enabled-disable-position"),
                )
            }
            assertFalse(
                "case=enabled-disable route=accessibility stale-action",
                disableNode.performAction(disableAction.id),
            )
            assertNoTerminalWheelActionLabels("enabled-disable")
            clearTerminalEvents()
            injectDrag(webView, geometry, start, TouchWheelDirection.Backward, "disabled-fresh-touch")
            assertNoInput("disabled-fresh-touch", "local", TouchWheelDirection.Backward)
            assertEquals(
                "case=disabled-fresh-touch route=local position",
                disabledClaimed,
                awaitAccessiblePosition(webView, "disabled-fresh-position"),
            )
            onUi(scenario) { webView.isEnabled = true }
            focusTerminal(scenario, webView)
            clearTerminalEvents()
            injectDrag(
                webView,
                geometry,
                start,
                TouchWheelDirection.Backward,
                "reenabled-fresh-touch",
                primaryPointerId = 1,
            )
            awaitAccessiblePosition(
                webView,
                "reenabled-fresh-position",
                diagnostics = {
                    "before=$disabledClaimed " +
                        touchEventDiagnostics(webView, "reenabled-fresh-touch").summary()
                },
            ) { it < disabledClaimed }
            assertNoInput("reenabled-fresh-touch", "local", TouchWheelDirection.Backward)
        }
    }

    @Test
    fun activeCompositionSurvivesTrustedTouchScrollAndPreservesContainment() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" + "\r\n".repeat(geometry.rows * 5),
                "composition-local",
            )
            val compositionBefore = awaitStableAccessiblePosition(
                webView,
                "composition-local-before",
            ) { it > 2 }
            focusTerminal(scenario, webView)
            val compositionStarted = withTerminalInputConnection(scenario, webView) { connection ->
                connection.setComposingText("active composition", 1)
            }
            assertTrue("case=composition-local route=ime start", compositionStarted)
            awaitBooleanState(
                webView,
                "document.querySelector('.composition-view').classList.contains('active')",
                "composition-active-before",
            )
            val compositionGeometry = terminalTouchGeometry(
                scenario,
                webView,
                geometry.columns to geometry.rows,
            )
            val compositionStart = compositionGeometry.cell(
                minOf(7, compositionGeometry.columns - 1),
                compositionGeometry.rows / 2,
            )
            val containmentBefore = terminalContainmentState(webView, "composition-containment-before")
            clearTerminalEvents()
            injectDrag(
                webView,
                compositionGeometry,
                compositionStart,
                TouchWheelDirection.Backward,
                "composition-local",
            )
            val compositionAfter = awaitAccessiblePosition(webView, "composition-local-after") {
                it < compositionBefore
            }
            assertTrue(
                "case=composition-local route=local displacement",
                compositionAfter < compositionBefore,
            )
            awaitBooleanState(
                webView,
                "document.querySelector('.composition-view').classList.contains('active')",
                "composition-active-after",
            )
            awaitBooleanState(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "composition-focus-after",
            )
            assertNoInput("composition-local", "local", TouchWheelDirection.Backward)
            assertEquals(
                "case=composition-local route=containment page",
                containmentBefore,
                terminalContainmentState(webView, "composition-containment-after"),
            )
            assertEquals(
                "case=composition-local route=containment webview-x",
                0,
                onUi(scenario) { webView.scrollX },
            )
            assertEquals(
                "case=composition-local route=containment webview-y",
                0,
                onUi(scenario) { webView.scrollY },
            )
            val compositionFinished = withTerminalInputConnection(scenario, webView) { connection ->
                connection.finishComposingText()
            }
            assertTrue("case=composition-local route=ime finish", compositionFinished)
        }
    }

    @Test
    fun nativeOutputRendersAnsiAndUnicodeWithoutJavascriptInterface() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            assertNotNull(TerminalTestProbe.page)

            TerminalTestProbe.page?.write("\u001b[32mSkíðblaðnir\u001b[0m ".toByteArray())
            TerminalTestProbe.page?.write("北極星".toByteArray())
            awaitValue(
                webView,
                "document.querySelector('.xterm-rows').textContent.includes('Skíðblaðnir 北極星')",
                "true",
            )
            assertEquals("undefined", evaluate(webView, "typeof window.Android"))
        }
    }

    @Test
    fun modifierSnapshotBindsOffOffBeforeReady() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            awaitTerminal(scenario, discardBindEvents = false)

            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
            assertEvent(TerminalTestEvent.Ready)
            assertNull("page-port bind emitted an extra protocol event", pollEvent())
        }
    }

    @Test
    fun exactBaseValuesHonorNormalAndApplicationCursorModes() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)

            for (value in ACCESSORIES) {
                postAccessory(scenario, webView, value.key)
                assertEvent(TerminalTestEvent.Input(value.unmodified.toByteArray()))
            }

            dispatchHardwareKey(scenario, webView, KeyEvent.KEYCODE_ENTER)
            assertEvent(TerminalTestEvent.Input("\r".toByteArray()))

            page.write("\u001b[?1hAPPLICATION MODE".toByteArray())
            awaitValue(
                webView,
                "document.querySelector('.xterm-rows').textContent.includes('APPLICATION MODE')",
                "true",
            )
            for ((key, value) in listOf(
                "Home" to "\u001bOH",
                "Up" to "\u001bOA",
                "End" to "\u001bOF",
                "Left" to "\u001bOD",
                "Down" to "\u001bOB",
                "Right" to "\u001bOC",
            )) {
                postAccessory(scenario, webView, key)
                assertEvent(TerminalTestEvent.Input(value.toByteArray()))
            }
            awaitValue(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "true",
            )

            assertNull("base-value matrix emitted an extra protocol event", pollEvent())
        }
    }

    @Test
    fun ctrlAltAndCombinedModifierTablesAreExactAndAtomic() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(CONTROL_ARMED))
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(BOTH_ARMED))
            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(ALT_ARMED))
            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(BOTH_ARMED))
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(CONTROL_ARMED))
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(BOTH_ARMED))
            postAccessory(scenario, webView, "Slash")
            assertConsumedInput("\u001b/")

            for (accessory in ACCESSORIES) {
                for ((modifiers, expected) in listOf(
                    CONTROL_ARMED to accessory.control,
                    ALT_ARMED to accessory.alt,
                    BOTH_ARMED to accessory.controlAlt,
                )) {
                    armModifiers(scenario, webView, modifiers)
                    postAccessory(scenario, webView, accessory.key)
                    assertConsumedInput(expected)
                }
            }

            requireNotNull(TerminalTestProbe.page).write("\u001b[?1hMODIFIED APPLICATION MODE".toByteArray())
            awaitValue(
                webView,
                "document.querySelector('.xterm-rows').textContent.includes('MODIFIED APPLICATION MODE')",
                "true",
            )
            for ((modifiers, expected) in listOf(
                CONTROL_ARMED to "\u001b[1;5A",
                ALT_ARMED to "\u001b[1;3A",
                BOTH_ARMED to "\u001b[1;7A",
            )) {
                armModifiers(scenario, webView, modifiers)
                postAccessory(scenario, webView, "Up")
                assertConsumedInput(expected)
            }

            assertNull("modifier matrix emitted an extra protocol event", pollEvent())
        }
    }

    @Test
    fun trustedPrintableAsciiUsesCtrlThenAltExactlyOnce() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val mappings = listOf(
                Triple(KeyEvent.KEYCODE_C, 0, "\u0003"),
                Triple(KeyEvent.KEYCODE_A, KeyEvent.META_SHIFT_ON, "\u0001"),
                Triple(KeyEvent.KEYCODE_2, KeyEvent.META_SHIFT_ON, "\u0000"),
                Triple(KeyEvent.KEYCODE_LEFT_BRACKET, 0, "\u001b"),
                Triple(KeyEvent.KEYCODE_MINUS, KeyEvent.META_SHIFT_ON, "\u001f"),
                Triple(KeyEvent.KEYCODE_SLASH, KeyEvent.META_SHIFT_ON, "\u007f"),
                Triple(KeyEvent.KEYCODE_1, 0, "1"),
            )

            for ((keyCode, metaState, expected) in mappings) {
                armModifiers(scenario, webView, CONTROL_ARMED)
                dispatchHardwareKey(scenario, webView, keyCode, metaState)
                assertConsumedInput(expected)
            }

            armModifiers(scenario, webView, ALT_ARMED)
            dispatchHardwareKey(scenario, webView, KeyEvent.KEYCODE_C)
            assertConsumedInput("\u001bc")

            armModifiers(scenario, webView, BOTH_ARMED)
            dispatchHardwareKey(scenario, webView, KeyEvent.KEYCODE_C)
            assertConsumedInput("\u001b\u0003")
        }
    }

    @Test
    fun uncertainImeAndCompositionStayLiteralAndConsumeBothModifiers() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            for (value in listOf("c", "北", "dictated words")) {
                armModifiers(scenario, webView, BOTH_ARMED)
                commitText(scenario, webView, value)
                assertConsumedInput(value)
            }

            armModifiers(scenario, webView, BOTH_ARMED)
            composeText(scenario, webView, "c")
            assertConsumedInput("c")
        }
    }

    @Test
    fun deckFocusTransferPreservesModifiersWhileExplicitAndWindowBoundariesReset() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            armModifiers(scenario, webView, BOTH_ARMED)
            onUi(scenario) { webView.clearFocus() }
            assertNull(
                "intra-surface focus transfer changed modifiers",
                pollEvent(),
            )
            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(ALT_ARMED))
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))

            armModifiers(scenario, webView, BOTH_ARMED)
            onUi(scenario) { requireNotNull(TerminalTestProbe.page).resetInputState() }
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))

            armModifiers(scenario, webView, BOTH_ARMED)
            onUi(scenario) { webView.onWindowFocusChanged(false) }
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))

            armModifiers(scenario, webView, BOTH_ARMED)
            onUi(scenario) { webView.reload() }
            assertTrue(
                "same-page reload kept the stale page port connected",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
            assertNull("a reset boundary emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun pageFailureClearsBothModifiersBeforeBecomingUnavailable() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            armModifiers(scenario, webView, BOTH_ARMED)
            postRawNativeMessage(scenario, webView, "not-json")

            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
            assertTrue(
                "page failure did not make the terminal unavailable",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
            assertNull("page failure emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun backgroundingAndRecreationDoNotRetainModifiers() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)

            armModifiers(scenario, webView, BOTH_ARMED)
            scenario.moveToState(Lifecycle.State.CREATED)
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
            assertNull("background reset emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
            scenario.moveToState(Lifecycle.State.RESUMED)

            val resumedWebView = onUi(scenario) {
                requireNotNull(findWebView(it.window.decorView)) { "resumed terminal activity has no WebView" }
            }
            armModifiers(scenario, resumedWebView, BOTH_ARMED)
            TerminalTestProbe.reset()
            scenario.recreate()
            awaitTerminal(scenario, discardBindEvents = false)
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
            assertEvent(TerminalTestEvent.Ready)
            assertNull("recreated page inherited terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun exactNativeProtocolRejectsUnsupportedExtraMalformedAndUnknownMessages() {
        for ((index, payload) in listOf(
            "not-json",
            """{"kind":"Accessory","key":"LineFeed"}""",
            """{"kind":"ResetControl"}""",
            """{"kind":"Accessory","key":"Control","extra":true}""",
            """{"kind":"Accessory","key":1}""",
            """{"kind":"Accessory","key":"Meta"}""",
            """{"kind":"Unknown"}""",
        ).withIndex()) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val webView = awaitTerminal(scenario)
                postRawNativeMessage(scenario, webView, payload)
                assertTrue(
                    "case=native-invalid-$index route=protocol expectedCount=1 index=0",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
            }
        }
    }

    @Test
    fun exactPageProtocolRejectsUnsupportedExtraMalformedAndUnknownModifierStates() {
        for ((index, payload) in listOf(
            """{"kind":"ControlState","state":"Armed"}""",
            """{"kind":"ModifierState","control":"Armed","alt":"Off","extra":true}""",
            """{"kind":"ModifierState","control":1,"alt":"Off"}""",
            """{"kind":"ModifierState","control":"Locked","alt":"Off"}""",
            """{"kind":"ModifierState","control":"Armed"}""",
            """{"kind":"Unknown"}""",
        ).withIndex()) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val webView = awaitTerminal(scenario)
                replaceNextModifierState(webView, payload)
                postAccessory(scenario, webView, "Control")
                assertTrue(
                    "case=page-invalid-$index route=protocol expectedCount=1 index=0",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
            }
        }
    }

    @Test
    fun pagePortHandshakeIsExactVersionOne() {
        val payload = """{"kind":"PagePort","version":1,"extra":true}"""
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            postRawHandshake(webView, payload)
            assertTrue(
                "case=handshake-invalid route=protocol expectedCount=1 index=0",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
        }
    }

    @Test
    fun missingRequiredDomReportsFailureWhenTheHostPortArrives() {
        val loaded = CountDownLatch(1)
        val messages = LinkedBlockingQueue<String>()
        val terminalSource = InstrumentationRegistry.getInstrumentation()
            .targetContext.assets.open("terminal/terminal.js").bufferedReader().use { it.readText() }

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = onUi(scenario) { activity ->
                WebView(activity).also { view ->
                    view.settings.javaScriptEnabled = true
                    view.webViewClient = object : WebViewClient() {
                        override fun onPageFinished(view: WebView, url: String) {
                            loaded.countDown()
                        }
                    }
                    activity.setContentView(view)
                    view.loadDataWithBaseURL(
                        "https://appassets.androidplatform.net/assets/terminal/malformed.html",
                        "<html><body></body></html>",
                        "text/html",
                        "UTF-8",
                        null,
                    )
                }
            }
            assertTrue("malformed terminal fixture did not load", loaded.await(5, TimeUnit.SECONDS))
            evaluate(webView, terminalSource)
            onUi(scenario) {
                val ports = WebViewCompat.createWebMessageChannel(webView)
                ports[0].setWebMessageCallback(
                    Handler(Looper.getMainLooper()),
                    object : WebMessagePortCompat.WebMessageCallbackCompat() {
                        override fun onMessage(port: WebMessagePortCompat, message: WebMessageCompat?) {
                            message?.data?.let(messages::add)
                        }
                    },
                )
                WebViewCompat.postWebMessage(
                    webView,
                    WebMessageCompat("{\"kind\":\"PagePort\",\"version\":1}", arrayOf(ports[1])),
                    "https://appassets.androidplatform.net".toUri(),
                )
            }
            assertEquals(
                "{\"kind\":\"PageFailure\"}",
                messages.poll(5, TimeUnit.SECONDS),
            )
        }
    }

    @Test
    fun trueColorEscapeSequenceProducesColoredPixels() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            TerminalTestProbe.page?.write(
                ("\u001b[31mINDEXED RED\u001b[0m " +
                    "\u001b[38;2;97;175;239mTRUECOLOR BLUE\u001b[0m " +
                    "\u001b[48;2;97;175;239m BACKGROUND BLUE \u001b[0m").toByteArray(),
            )
            awaitValue(
                webView,
                "document.querySelector('.xterm-rows').textContent.includes('BACKGROUND BLUE')",
                "true",
            )
            awaitValue(
                webView,
                """
                (function () {
                    var spans = Array.from(document.querySelectorAll('.xterm-rows span'));
                    var indexed = spans.find(function (node) { return node.textContent === 'INDEXED RED'; });
                    var foreground = spans.find(function (node) { return node.textContent === 'TRUECOLOR BLUE'; });
                    var background = spans.find(function (node) { return node.textContent === ' BACKGROUND BLUE '; });
                    return indexed && foreground && background &&
                        getComputedStyle(indexed).color === 'rgb(215, 78, 51)' &&
                        getComputedStyle(foreground).color === 'rgb(97, 175, 239)' &&
                        getComputedStyle(background).backgroundColor === 'rgb(97, 175, 239)';
                }())
                """.trimIndent(),
                "true",
            )
            awaitVisualState(webView)

            val rendered = copyWebView(scenario, webView)
            assertTrue(
                "case=true-color route=render expectedCount=1 index=0",
                rendered.containsPixel { pixel ->
                    Color.red(pixel) in 75..130 &&
                        Color.green(pixel) in 145..205 &&
                        Color.blue(pixel) in 210..255
                },
            )
        }
    }

    @Test
    fun brightAnsiAndCursorPaintGoldWhileGrayscaleRemapsAndTheCubeStaysDefault() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            focusTerminal(scenario, webView)
            requireNotNull(TerminalTestProbe.page).write(
                ("\u001b[93mBRIGHT YELLOW\u001b[0m " +
                    "\u001b[38;5;244mGRAYSCALE 244\u001b[0m " +
                    "\u001b[38;5;21mCUBE 21\u001b[0m").toByteArray(),
            )
            // The focused block cursor blinks, so poll until its lit phase.
            // Cube 21 keeps the library default #0000ff, which is 2.3:1 on Ink
            // and therefore lifted to #4646ff by minimumContrastRatio 3; a
            // contiguous-24 extendedAnsi array would paint it from the
            // grayscale ramp instead.
            awaitValue(
                webView,
                """
                (function () {
                    var spans = Array.from(document.querySelectorAll('.xterm-rows span'));
                    var bright = spans.find(function (node) { return node.textContent === 'BRIGHT YELLOW'; });
                    var grayscale = spans.find(function (node) { return node.textContent === 'GRAYSCALE 244'; });
                    var cube = spans.find(function (node) { return node.textContent === 'CUBE 21'; });
                    var cursor = document.querySelector('.xterm-rows .xterm-cursor');
                    return bright && grayscale && cube && cursor &&
                        getComputedStyle(bright).color === 'rgb(214, 168, 95)' &&
                        getComputedStyle(grayscale).color === 'rgb(133, 131, 128)' &&
                        getComputedStyle(cube).color === 'rgb(70, 70, 255)' &&
                        getComputedStyle(cursor).backgroundColor === 'rgb(214, 168, 95)';
                }())
                """.trimIndent(),
                "true",
            )
        }
    }

    @Test
    fun imeCompositionAtTheRightEdgeDoesNotPanTheTerminal() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            TerminalTestProbe.page?.write("\u001b[999C".toByteArray())
            awaitValue(
                webView,
                "parseFloat(document.querySelector('.xterm-helper-textarea').style.left) > " +
                    "document.querySelector('.xterm-screen').clientWidth / 2",
                "true",
            )

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.focus({ preventScroll: true });
                    input.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', {
                        bubbles: true,
                        data: 'the quick brown fox jumps over the lazy dwarf'
                    }));
                    input.value = 'the quick brown fox jumps over the lazy dwarf 0123456789 !@#\u0024%^&*()';
                    input.dispatchEvent(new InputEvent('beforeinput', {
                        bubbles: true,
                        data: input.value,
                        inputType: 'insertCompositionText'
                    }));
                    input.dispatchEvent(new InputEvent('input', {
                        bubbles: true,
                        data: input.value,
                        inputType: 'insertCompositionText'
                    }));
                }())
                """.trimIndent(),
            )
            awaitVisualState(webView)
            TerminalTestProbe.page?.write("\u001b[999C".toByteArray())
            awaitVisualState(webView)

            onUi(scenario) {
                webView.scrollTo(240, 0)
                assertEquals(0, webView.scrollX)
            }

            awaitValue(
                webView,
                """
                (function () {
                    var screen = document.querySelector('.xterm-screen').getBoundingClientRect();
                    var compositionNode = document.querySelector('.composition-view');
                    var composition = compositionNode.getBoundingClientRect();
                    var input = document.querySelector('.xterm-helper-textarea').getBoundingClientRect();
                    return window.scrollX === 0 &&
                        document.documentElement.scrollWidth <= document.documentElement.clientWidth + 1 &&
                        document.body.scrollWidth <= document.body.clientWidth + 1 &&
                        (!window.visualViewport || window.visualViewport.offsetLeft === 0) &&
                        compositionNode.textContent.charCodeAt(0) === 0x200e &&
                        compositionNode.textContent.charCodeAt(compositionNode.textContent.length - 1) === 0x200e &&
                        composition.right <= screen.right + 0.5 &&
                        input.right <= screen.right + 0.5;
                }())
                """.trimIndent(),
                "true",
            )
        }
    }

    @Test
    fun portraitScaleReturnsAndModifiersResetAfterAFullRotation() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val initialSize = requireNotNull(TerminalTestProbe.sizes.poll(5, TimeUnit.SECONDS))
            val initialScreenWidth = evaluate(
                webView,
                "document.querySelector('.xterm-screen').getBoundingClientRect().width",
            ).toDouble()
            TerminalTestProbe.sizes.clear()

            armModifiers(scenario, webView, BOTH_ARMED)
            onUi(scenario) { it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE }
            awaitValue(webView, "window.innerWidth > window.innerHeight", "true")
            assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
            assertNull(
                "orientation reset emitted terminal input",
                TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS),
            )
            val landscapeSize = awaitSettledSizeWithAllSamplesConforming()
            assertTrue("landscape terminal dropped below 80 columns: $landscapeSize", landscapeSize.first >= 80)
            TerminalTestProbe.sizes.clear()

            onUi(scenario) { it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_PORTRAIT }
            awaitValue(webView, "window.innerHeight > window.innerWidth", "true")
            val finalSize = awaitSettledSizeWithAllSamplesConforming()
            val finalScreenWidth = evaluate(
                webView,
                "document.querySelector('.xterm-screen').getBoundingClientRect().width",
            ).toDouble()

            assertEquals(
                "portrait cell scale drifted after rotation",
                initialScreenWidth / initialSize.first,
                finalScreenWidth / finalSize.first,
                0.2,
            )
            assertEquals(0, onUi(scenario) { webView.scrollX })
        }
    }

    @Test
    fun clipboardPasteRemovesTerminalControlsBeforeInputLeavesThePage() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            armModifiers(scenario, webView, BOTH_ARMED)
            evaluate(
                webView,
                """
                (function () {
                    var clipboard = new DataTransfer();
                    clipboard.setData('text/plain', 'one\u0000two\r\nthree\u001b[201~\u0085\u0001\t');
                    document.querySelector('.xterm-helper-textarea').dispatchEvent(new ClipboardEvent('paste', {
                        clipboardData: clipboard,
                        bubbles: true,
                        cancelable: true
                    }));
                }())
                """.trimIndent(),
            )

            assertConsumedInput("onetwo\nthree[201~\t")
        }
    }

    @Test
    fun packagedResourceFailureLeavesPreparingForReconnect() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            evaluate(
                webView,
                """
                (function () {
                    var script = document.createElement('script');
                    script.src = '/assets/terminal/definitely-missing.js';
                    document.head.appendChild(script);
                }())
                """.trimIndent(),
            )

            assertTrue(
                "packaged resource failure did not surface terminal unavailability",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
        }
    }

    @Test
    fun packagedPageThatNeverSignalsReadyHitsTheReadinessDeadline() {
        val probe = TerminalProbe()
        var deadlineStartedAt = 0L

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                deadlineStartedAt = System.nanoTime()
                activity.setContentView(
                    createTestTerminal(
                        context = activity,
                        probe = probe,
                        initialUrl = "https://appassets.androidplatform.net/assets/terminal/terminal.css",
                        readinessTimeoutMillis = 250,
                    ),
                )
            }

            assertTrue("never-ready packaged page did not time out", probe.unavailable.await(5, TimeUnit.SECONDS))
            assertTrue(
                "never-ready packaged page failed before its deadline",
                TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - deadlineStartedAt) >= 200,
            )
            assertEquals("never-ready packaged page signaled Ready", 1L, probe.ready.count)
        }
    }

    private fun terminalTouchGeometry(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        currentSize: Pair<Int, Int>? = null,
    ): TerminalTouchGeometry {
        val size = currentSize ?: awaitLatestTerminalSize("touch-geometry")
        val bounds = JSONObject(
            evaluateSafely(
                webView,
                """
                (function () {
                    var screen = document.querySelector('.xterm-screen').getBoundingClientRect();
                    return JSON.stringify({
                        left: screen.left,
                        top: screen.top,
                        width: screen.width,
                        height: screen.height,
                        viewportWidth: window.innerWidth,
                        viewportHeight: window.innerHeight
                    });
                }())
                """.trimIndent(),
                "touch-geometry",
            ),
        )
        return onUi(scenario) {
            val location = IntArray(2)
            webView.getLocationOnScreen(location)
            val scaleX = webView.width.toFloat() / bounds.getDouble("viewportWidth").toFloat()
            val scaleY = webView.height.toFloat() / bounds.getDouble("viewportHeight").toFloat()
            TerminalTouchGeometry(
                screenLeft = location[0] + bounds.getDouble("left").toFloat() * scaleX,
                screenTop = location[1] + bounds.getDouble("top").toFloat() * scaleY,
                screenWidth = bounds.getDouble("width").toFloat() * scaleX,
                screenHeight = bounds.getDouble("height").toFloat() * scaleY,
                columns = size.first,
                rows = size.second,
                cssToScreenX = scaleX,
                cssToScreenY = scaleY,
            ).also { it.requireGestureScale("touch-geometry") }
        }
    }

    private fun terminalDocumentPointOutsideScreen(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        geometry: TerminalTouchGeometry,
        caseId: String,
    ): TouchPoint = onUi(scenario) {
        val webViewBounds = Rect()
        assertTrue(
            "case=$caseId route=geometry webview-visible",
            webView.getGlobalVisibleRect(webViewBounds) && !webViewBounds.isEmpty,
        )
        val insetX = minOf(maxOf(1f, geometry.cssToScreenX), webViewBounds.width() / 4f)
        val insetY = minOf(maxOf(1f, geometry.cssToScreenY), webViewBounds.height() / 4f)
        val screenCenterX = geometry.screenLeft + geometry.screenWidth / 2f
        val screenCenterY = geometry.screenTop + geometry.screenHeight / 2f
        val candidate = listOf(
            TouchPoint(webViewBounds.left + insetX, screenCenterY),
            TouchPoint(webViewBounds.right - insetX, screenCenterY),
            TouchPoint(screenCenterX, webViewBounds.top + insetY),
            TouchPoint(screenCenterX, webViewBounds.bottom - insetY),
        ).firstOrNull { point ->
            point.x > webViewBounds.left &&
                point.x < webViewBounds.right &&
                point.y > webViewBounds.top &&
                point.y < webViewBounds.bottom &&
                !geometry.contains(point)
        }
        assertNotNull("case=$caseId route=geometry outside-screen-point", candidate)
        requireNotNull(candidate)
    }

    private fun awaitLatestTerminalSize(caseId: String): Pair<Int, Int> {
        var latest = awaitTerminalSize { it.first in 80..240 && it.second in 5..120 }
        val hardDeadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
        var quietDeadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(250)
        while (System.nanoTime() < hardDeadline && System.nanoTime() < quietDeadline) {
            val sample = TerminalTestProbe.sizes.poll(50, TimeUnit.MILLISECONDS) ?: continue
            assertTrue(
                "case=$caseId route=geometry sample-range",
                sample.first in 80..240 && sample.second in 5..120,
            )
            latest = sample
            quietDeadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(250)
        }
        return latest
    }

    private fun applyControlFixture(page: TerminalPage, control: String, caseId: String) {
        clearTerminalEvents()
        page.write((control + "\u001b[5n").toByteArray())
        val actual = TerminalTestProbe.input.poll(5, TimeUnit.SECONDS)
        assertNotNull("case=$caseId route=fixture count=0 index=0", actual)
        assertTrue(
            "case=$caseId route=fixture count=1 index=0 expectedLength=4 actualLength=${actual?.size ?: -1}",
            actual != null && actual.contentEquals("\u001b[0n".toByteArray()),
        )
        clearTerminalEvents()
    }

    private fun clearTerminalEvents() {
        TerminalTestProbe.input.clear()
        TerminalTestProbe.events.clear()
    }

    private fun assertOnlyInput(
        caseId: String,
        route: String,
        direction: TouchWheelDirection,
        expected: ByteArray,
    ) {
        val actual = TerminalTestProbe.input.poll(5, TimeUnit.SECONDS)
        assertNotNull("case=$caseId route=$route direction=${direction.name} count=0 index=0", actual)
        assertTrue(
            "case=$caseId route=$route direction=${direction.name} count=1 index=0 " +
                "expectedLength=${expected.size} actualLength=${actual?.size ?: -1}",
            actual != null && actual.contentEquals(expected),
        )
        assertNull(
            "case=$caseId route=$route direction=${direction.name} count>1 index=1",
            TerminalTestProbe.input.poll(350, TimeUnit.MILLISECONDS),
        )
    }

    private fun assertNoInput(
        caseId: String,
        route: String,
        direction: TouchWheelDirection,
        diagnostics: (() -> String)? = null,
    ) {
        val actual = TerminalTestProbe.input.poll(350, TimeUnit.MILLISECONDS)
        assertNull(
            "case=$caseId route=$route direction=${direction.name} expectedCount=0 index=0 " +
                "actualLength=${actual?.size ?: 0}" +
                (diagnostics?.let { " ${it()}" } ?: ""),
            actual,
        )
    }

    private fun assertNoQueuedInput(
        caseId: String,
        route: String,
        direction: TouchWheelDirection,
    ) {
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(350)
        while (TerminalTestProbe.input.peek() == null && System.nanoTime() < deadline) {
            Thread.sleep(25)
        }
        val first = TerminalTestProbe.input.peek()
        assertNull(
            "case=$caseId route=$route direction=${direction.name} expectedCount=0 " +
                "actualCount=${TerminalTestProbe.input.size} firstLength=${first?.size ?: 0}",
            first,
        )
    }

    private fun assertRoutedInputConsumesModifiers(caseId: String, expected: ByteArray) {
        assertEvent(TerminalTestEvent.Modifiers(OFF_OFF), "$caseId-modifiers")
        assertEvent(TerminalTestEvent.Input(expected), "$caseId-input")
    }

    private fun sgrWheel(direction: TouchWheelDirection, column: Int, row: Int): ByteArray =
        "\u001b[<${direction.sgrButton};$column;${row}M".toByteArray()

    private fun actionLabel(direction: TouchWheelDirection): String = when (direction) {
        TouchWheelDirection.Backward -> TERMINAL_WHEEL_BACKWARD
        TouchWheelDirection.Forward -> TERMINAL_WHEEL_FORWARD
    }

    private fun installCompatibilityEventCounter(webView: WebView) {
        evaluateSafely(
            webView,
            """
            (function () {
                var screen = document.querySelector('.xterm-screen');
                screen.dataset.compatibilityEventCount = '0';
                ['mousedown', 'mousemove', 'mouseup', 'click', 'contextmenu'].forEach(function (name) {
                    screen.addEventListener(name, function () {
                        screen.dataset.compatibilityEventCount =
                            String(Number(screen.dataset.compatibilityEventCount) + 1);
                    }, true);
                });
            }())
            """.trimIndent(),
            "compatibility-counter-install",
        )
    }

    private fun compatibilityEventCount(webView: WebView): Int =
        evaluateSafely(
            webView,
            "Number(document.querySelector('.xterm-screen').dataset.compatibilityEventCount || '-1')",
            "compatibility-counter-read",
        ).toInt()

    private fun installTouchEventDiagnostics(webView: WebView, caseId: String) {
        assertEquals(
            "case=$caseId route=diagnostic install",
            "true",
            evaluateSafely(
                webView,
                """
                (function () {
                    var terminal = document.querySelector('#terminal .xterm');
                    var screen = document.querySelector('.xterm-screen');
                    if (!terminal || !screen) return false;
                    var state = {
                        order: 0,
                        pointerDownCount: 0,
                        pointerMoveCount: 0,
                        pointerUpCount: 0,
                        pointerCancelCount: 0,
                        mouseDownCount: 0,
                        mouseMoveCount: 0,
                        mouseUpCount: 0,
                        clickCount: 0,
                        contextMenuCount: 0,
                        screenCompatibilityCount: 0,
                        pointerScreenTargetCount: 0,
                        pointerAccessibilityTargetCount: 0,
                        pointerOtherTargetCount: 0,
                        compatibilityScreenTargetCount: 0,
                        compatibilityAccessibilityTargetCount: 0,
                        compatibilityOtherTargetCount: 0,
                        contextTrustedCount: 0,
                        contextSourceCapabilitiesPresentCount: 0,
                        contextFiresTouchEventsCount: 0,
                        wheelCount: 0,
                        wheelTrustedCount: 0,
                        wheelDefaultPreventedCount: 0,
                        firstWheelOrder: 0,
                        lastPointerMoveOrder: 0,
                        firstMouseDownOrder: 0,
                        firstMouseMoveOrder: 0,
                        firstContextMenuOrder: 0,
                        scrollCount: 0,
                        firstScrollOrder: 0,
                        lastScrollPosition: -1,
                        scriptResourcePresent: false,
                        scriptTransferSize: -1,
                        scriptDecodedBodySize: -1
                    };
                    var script = Array.from(document.scripts).find(function (node) {
                        return node.src.indexOf('xterm-6.0.0-skidbladnir-wheel.js') >= 0;
                    });
                    var resource = script ? performance.getEntriesByName(script.src).slice(-1)[0] : null;
                    if (resource) {
                        state.scriptResourcePresent = true;
                        state.scriptTransferSize = Math.round(resource.transferSize || 0);
                        state.scriptDecodedBodySize = Math.round(resource.decodedBodySize || 0);
                    }
                    function position() {
                        var row = document.querySelector('.xterm-accessibility-tree [aria-posinset]');
                        return row ? Number(row.getAttribute('aria-posinset') || '-1') : -1;
                    }
                    function category(target) {
                        var node = target && target.nodeType === Node.ELEMENT_NODE ?
                            target : target && target.parentElement;
                        if (!node) return 3;
                        if (node === screen || screen.contains(node)) return 1;
                        if (node.closest && node.closest('.xterm-accessibility')) return 2;
                        return 3;
                    }
                    function recordTarget(prefix, target) {
                        var targetCategory = category(target);
                        if (targetCategory === 1) state[prefix + 'ScreenTargetCount'] += 1;
                        else if (targetCategory === 2) state[prefix + 'AccessibilityTargetCount'] += 1;
                        else state[prefix + 'OtherTargetCount'] += 1;
                    }
                    function commit() {
                        terminal.dataset.touchEventDiagnostics = JSON.stringify(state);
                    }
                    ['pointerdown', 'pointermove', 'pointerup', 'pointercancel'].forEach(function (name) {
                        terminal.addEventListener(name, function (event) {
                            state.order += 1;
                            if (name === 'pointerdown') state.pointerDownCount += 1;
                            else if (name === 'pointermove') {
                                state.pointerMoveCount += 1;
                                state.lastPointerMoveOrder = state.order;
                            } else if (name === 'pointerup') state.pointerUpCount += 1;
                            else state.pointerCancelCount += 1;
                            recordTarget('pointer', event.target);
                            commit();
                        }, true);
                    });
                    ['mousedown', 'mousemove', 'mouseup', 'click', 'contextmenu'].forEach(function (name) {
                        terminal.addEventListener(name, function (event) {
                            state.order += 1;
                            if (name === 'mousedown') {
                                state.mouseDownCount += 1;
                                if (state.firstMouseDownOrder === 0) state.firstMouseDownOrder = state.order;
                            } else if (name === 'mousemove') {
                                state.mouseMoveCount += 1;
                                if (state.firstMouseMoveOrder === 0) state.firstMouseMoveOrder = state.order;
                            } else if (name === 'mouseup') state.mouseUpCount += 1;
                            else if (name === 'click') state.clickCount += 1;
                            else {
                                state.contextMenuCount += 1;
                                if (state.firstContextMenuOrder === 0) state.firstContextMenuOrder = state.order;
                                if (event.isTrusted === true) state.contextTrustedCount += 1;
                                if (event.sourceCapabilities) {
                                    state.contextSourceCapabilitiesPresentCount += 1;
                                    if (event.sourceCapabilities.firesTouchEvents === true) {
                                        state.contextFiresTouchEventsCount += 1;
                                    }
                                }
                            }
                            recordTarget('compatibility', event.target);
                            commit();
                        }, true);
                        screen.addEventListener(name, function () {
                            state.screenCompatibilityCount += 1;
                            commit();
                        }, true);
                    });
                    terminal.addEventListener('wheel', function (event) {
                        state.order += 1;
                        state.wheelCount += 1;
                        if (event.isTrusted === true) state.wheelTrustedCount += 1;
                        if (event.defaultPrevented) state.wheelDefaultPreventedCount += 1;
                        if (state.firstWheelOrder === 0) state.firstWheelOrder = state.order;
                        commit();
                    }, true);
                    terminal.addEventListener('scroll', function (event) {
                        if (!terminal.contains(event.target)) return;
                        state.order += 1;
                        state.scrollCount += 1;
                        if (state.firstScrollOrder === 0) state.firstScrollOrder = state.order;
                        state.lastScrollPosition = position();
                        commit();
                    }, true);
                    commit();
                    return true;
                }())
                """.trimIndent(),
                "$caseId-install",
            ),
        )
    }

    private fun touchEventDiagnostics(webView: WebView, caseId: String): TouchEventDiagnostics {
        val state = JSONObject(
            evaluateSafely(
                webView,
                "document.querySelector('#terminal .xterm').dataset.touchEventDiagnostics",
                "$caseId-read",
            ),
        )
        return TouchEventDiagnostics(
            pointerDownCount = state.getInt("pointerDownCount"),
            pointerMoveCount = state.getInt("pointerMoveCount"),
            pointerUpCount = state.getInt("pointerUpCount"),
            pointerCancelCount = state.getInt("pointerCancelCount"),
            mouseDownCount = state.getInt("mouseDownCount"),
            mouseMoveCount = state.getInt("mouseMoveCount"),
            mouseUpCount = state.getInt("mouseUpCount"),
            clickCount = state.getInt("clickCount"),
            contextMenuCount = state.getInt("contextMenuCount"),
            screenCompatibilityCount = state.getInt("screenCompatibilityCount"),
            pointerScreenTargetCount = state.getInt("pointerScreenTargetCount"),
            pointerAccessibilityTargetCount = state.getInt("pointerAccessibilityTargetCount"),
            pointerOtherTargetCount = state.getInt("pointerOtherTargetCount"),
            compatibilityScreenTargetCount = state.getInt("compatibilityScreenTargetCount"),
            compatibilityAccessibilityTargetCount = state.getInt("compatibilityAccessibilityTargetCount"),
            compatibilityOtherTargetCount = state.getInt("compatibilityOtherTargetCount"),
            contextTrustedCount = state.getInt("contextTrustedCount"),
            contextSourceCapabilitiesPresentCount =
                state.getInt("contextSourceCapabilitiesPresentCount"),
            contextFiresTouchEventsCount = state.getInt("contextFiresTouchEventsCount"),
            wheelCount = state.getInt("wheelCount"),
            wheelTrustedCount = state.getInt("wheelTrustedCount"),
            wheelDefaultPreventedCount = state.getInt("wheelDefaultPreventedCount"),
            firstWheelOrder = state.getInt("firstWheelOrder"),
            lastPointerMoveOrder = state.getInt("lastPointerMoveOrder"),
            firstMouseDownOrder = state.getInt("firstMouseDownOrder"),
            firstMouseMoveOrder = state.getInt("firstMouseMoveOrder"),
            firstContextMenuOrder = state.getInt("firstContextMenuOrder"),
            scrollCount = state.getInt("scrollCount"),
            firstScrollOrder = state.getInt("firstScrollOrder"),
            lastScrollPosition = state.getInt("lastScrollPosition"),
            scriptResourcePresent = state.getBoolean("scriptResourcePresent"),
            scriptTransferSize = state.getInt("scriptTransferSize"),
            scriptDecodedBodySize = state.getInt("scriptDecodedBodySize"),
        )
    }

    private fun awaitAccessiblePosition(
        webView: WebView,
        caseId: String,
        diagnostics: (() -> String)? = null,
        predicate: (Int) -> Boolean = { it > 0 },
    ): Int {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var lastValue = -1
        while (System.nanoTime() < deadline) {
            val value = evaluateSafely(
                webView,
                "Number(document.querySelector('.xterm-accessibility-tree [aria-posinset]')?.getAttribute('aria-posinset') || '-1')",
                caseId,
            ).toIntOrNull() ?: -1
            lastValue = value
            if (predicate(value)) return value
            Thread.sleep(50)
        }
        val diagnostic = diagnostics?.invoke()?.let { " $it" }.orEmpty()
        throw AssertionError(
            "case=$caseId route=local numeric-position lastValue=$lastValue$diagnostic",
        )
    }

    private fun awaitStableAccessiblePosition(
        webView: WebView,
        caseId: String,
        predicate: (Int) -> Boolean = { it > 0 },
    ): Int {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var previous: Int? = null
        var stableSince = System.nanoTime()
        var lastValue = -1
        while (System.nanoTime() < deadline) {
            lastValue = evaluateSafely(
                webView,
                "Number(document.querySelector('.xterm-accessibility-tree [aria-posinset]')" +
                    "?.getAttribute('aria-posinset') || '-1')",
                caseId,
            ).toIntOrNull() ?: -1
            when {
                !predicate(lastValue) -> stableSince = System.nanoTime()
                previous != lastValue -> stableSince = System.nanoTime()
                TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - stableSince) >=
                    ACCESSIBILITY_STABILITY_MILLIS -> return lastValue
            }
            previous = lastValue
            Thread.sleep(50)
        }
        throw AssertionError(
            "case=$caseId route=local stable-numeric-position lastValue=$lastValue",
        )
    }

    private fun accessibleSetSize(webView: WebView, caseId: String): Int =
        evaluateSafely(
            webView,
            "Number(document.querySelector('.xterm-accessibility-tree [aria-setsize]')?.getAttribute('aria-setsize') || '-1')",
            caseId,
        ).toIntOrNull() ?: -1

    private fun awaitNoTerminalSelection(webView: WebView, caseId: String) {
        awaitBooleanState(
            webView,
            "document.querySelector('.xterm-selection').childElementCount === 0 && " +
                "window.getSelection().isCollapsed",
            caseId,
        )
    }

    private fun terminalContainmentState(webView: WebView, caseId: String): TerminalContainmentState {
        val state = JSONObject(
            evaluateSafely(
                webView,
                """
                (function () {
                    return JSON.stringify({
                        windowX: window.scrollX,
                        windowY: window.scrollY,
                        pageY: document.scrollingElement ? document.scrollingElement.scrollTop : 0,
                        documentY: document.documentElement.scrollTop,
                        bodyY: document.body.scrollTop,
                        terminalX: document.querySelector('.xterm-viewport').scrollLeft
                    });
                }())
                """.trimIndent(),
                caseId,
            ),
        )
        return TerminalContainmentState(
            windowX = state.getDouble("windowX").toInt(),
            windowY = state.getDouble("windowY").toInt(),
            pageY = state.getDouble("pageY").toInt(),
            documentY = state.getDouble("documentY").toInt(),
            bodyY = state.getDouble("bodyY").toInt(),
            terminalX = state.getDouble("terminalX").toInt(),
        )
    }

    private fun awaitBooleanState(
        webView: WebView,
        expression: String,
        caseId: String,
        diagnostics: (() -> String)? = null,
    ) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (evaluateSafely(webView, expression, caseId) == "true") return
            Thread.sleep(50)
        }
        val diagnostic = diagnostics?.invoke()?.let { " $it" }.orEmpty()
        throw AssertionError("case=$caseId route=webview state=false$diagnostic")
    }

    private fun awaitAnimationFrame(webView: WebView, caseId: String) {
        assertEquals(
            "case=$caseId route=webview frame-scheduled",
            "true",
            evaluateSafely(
                webView,
                """
                (function () {
                    document.documentElement.dataset.terminalTestFrameReady = 'false';
                    requestAnimationFrame(function () {
                        document.documentElement.dataset.terminalTestFrameReady = 'true';
                    });
                    return true;
                }())
                """.trimIndent(),
                "$caseId-schedule",
            ),
        )
        awaitBooleanState(
            webView,
            "document.documentElement.dataset.terminalTestFrameReady === 'true'",
            caseId,
        )
    }

    private fun evaluateSafely(webView: WebView, expression: String, caseId: String): String {
        val latch = CountDownLatch(1)
        var result: String? = null
        webView.post {
            webView.evaluateJavascript(expression) {
                result = JSONTokener(it).nextValue()?.toString() ?: "null"
                latch.countDown()
            }
        }
        assertTrue("case=$caseId route=webview javascript-timeout", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }

    private fun awaitFocusedTerminalRowNode(
        caseId: String,
        requireFocusAction: Boolean = false,
    ): AccessibilityNodeInfo {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        if (requireFocusAction) {
            val focusedRows = visibleTerminalRows().filter { it.isAccessibilityFocused }
            assertTrue(
                "case=$caseId route=accessibility prior-focused-row-count",
                focusedRows.size <= 1,
            )
            val existing = focusedRows.singleOrNull()
                ?: automation.rootInActiveWindow
                    ?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY)
            if (existing != null) {
                assertTrue(
                    "case=$caseId route=accessibility prior-focus-clear-rejected",
                    existing.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS),
                )
            }
            awaitNoAccessibilityFocus("$caseId-prior-focus")
        }
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val rows = visibleTerminalRows()
            val focusedRows = rows.filter { it.isAccessibilityFocused }
            assertTrue(
                "case=$caseId route=accessibility focused-row-count",
                focusedRows.size <= 1,
            )
            if (!requireFocusAction && focusedRows.size == 1) {
                return focusedRows.single()
            }
            val candidate = rows.getOrNull(rows.size / 2)
            if (candidate != null) return focusTerminalRow(candidate, caseId)
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=accessibility visible-row-missing")
    }

    private fun focusDistinctTerminalRowNode(
        previous: AccessibilityNodeInfo,
        caseId: String,
    ): AccessibilityNodeInfo {
        val previousKey = requireTerminalRowKey(previous, "$caseId-previous")
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val candidates = visibleTerminalRows().filter {
                val key = terminalRowKeyOrNull(it)
                key != null &&
                    key.windowId == previousKey.windowId &&
                    key != previousKey
            }
            val candidate = candidates.getOrNull(candidates.size / 2)
            if (candidate != null) {
                val focused = focusTerminalRow(candidate, caseId)
                assertTrue(
                    "case=$caseId route=accessibility distinct-source",
                    requireTerminalRowKey(focused, "$caseId-focused") != previousKey,
                )
                return focused
            }
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=accessibility distinct-row-missing")
    }

    private fun focusTerminalRow(
        candidate: AccessibilityNodeInfo,
        caseId: String,
    ): AccessibilityNodeInfo {
        val expectedKey = requireTerminalRowKey(candidate, "$caseId-candidate")
        assertFalse(
            "case=$caseId route=accessibility candidate-already-focused",
            candidate.isAccessibilityFocused,
        )
        val accepted = candidate.performAction(
            AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS,
        )
        assertTrue("case=$caseId route=accessibility row-focus-rejected", accepted)
        assertTrue(
            "case=$caseId route=accessibility cache-clear-rejected",
            InstrumentationRegistry.getInstrumentation().uiAutomation.clearCache(),
        )
        return awaitSingleFocusedTerminalRow(expectedKey, caseId)
    }

    private fun awaitSingleFocusedTerminalRow(
        expectedKey: TerminalRowKey,
        caseId: String,
    ): AccessibilityNodeInfo {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var visibleCount = 0
        var focusedRowCount = 0
        var expectedPresent = false
        var anyFocusedNode = false
        while (System.nanoTime() < deadline) {
            automation.clearCache()
            val rows = visibleTerminalRows()
            val focused = rows.filter { it.isAccessibilityFocused }
            visibleCount = rows.size
            focusedRowCount = focused.size
            expectedPresent = rows.any { terminalRowKeyOrNull(it) == expectedKey }
            anyFocusedNode = automation.rootInActiveWindow
                ?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY) != null
            if (focused.size == 1 &&
                terminalRowKeyOrNull(focused.single()) == expectedKey
            ) {
                return focused.single()
            }
            Thread.sleep(25)
        }
        throw AssertionError(
            "case=$caseId route=accessibility focused-row-missing " +
                "visibleCount=$visibleCount focusedRowCount=$focusedRowCount " +
                "expectedPresent=$expectedPresent anyFocusedNode=$anyFocusedNode",
        )
    }

    private fun awaitTerminalRowNode(
        expectedKey: TerminalRowKey,
        caseId: String,
    ): AccessibilityNodeInfo {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val match = visibleTerminalRows().singleOrNull {
                terminalRowKeyOrNull(it) == expectedKey
            }
            if (match != null) return match
            Thread.sleep(25)
        }
        throw AssertionError("case=$caseId route=accessibility row-key-missing")
    }

    private fun focusTerminalNonRowNode(caseId: String): AccessibilityNodeInfo {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val focusedRows = visibleTerminalRows().filter { it.isAccessibilityFocused }
        assertTrue(
            "case=$caseId route=accessibility prior-focused-row-count",
            focusedRows.size <= 1,
        )
        val existing = focusedRows.singleOrNull()
            ?: automation.rootInActiveWindow
                ?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY)
        if (existing != null) {
            assertTrue(
                "case=$caseId route=accessibility prior-focus-clear-rejected",
                existing.performAction(AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS),
            )
        }
        awaitNoAccessibilityFocus("$caseId-prior-focus")

        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var candidate: AccessibilityNodeInfo? = null
        while (System.nanoTime() < deadline && candidate == null) {
            candidate = visibleChromiumNonRowNodes(requireFocusAction = true).firstOrNull()
            if (candidate == null) Thread.sleep(50)
        }
        val target = candidate
        assertNotNull("case=$caseId route=accessibility non-row-missing", target)
        val confirmedTarget = requireNotNull(target)
        val expectedKey = terminalNonRowKeyOrNull(confirmedTarget)
        assertNotNull("case=$caseId route=accessibility non-row-key-missing", expectedKey)
        val confirmedKey = requireNotNull(expectedKey)
        assertFalse(
            "case=$caseId route=accessibility candidate-already-focused",
            confirmedTarget.isAccessibilityFocused,
        )
        assertTrue(
            "case=$caseId route=accessibility non-row-focus-rejected",
            confirmedTarget.performAction(AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS),
        )
        assertTrue(
            "case=$caseId route=accessibility non-row-cache-clear-rejected",
            automation.clearCache(),
        )

        val focusDeadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < focusDeadline) {
            val focused = visibleChromiumNonRowNodes(requireFocusAction = false)
                .filter { it.isAccessibilityFocused }
            if (focused.size == 1 && terminalNonRowKeyOrNull(focused.single()) == confirmedKey) {
                return focused.single()
            }
            Thread.sleep(25)
        }
        throw AssertionError("case=$caseId route=accessibility focused-non-row-missing")
    }

    private fun visibleTerminalRows(): List<AccessibilityNodeInfo> {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val root = automation.rootInActiveWindow ?: return emptyList()
        val rows = mutableListOf<AccessibilityNodeInfo>()
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.add(root)
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            if (isVisibleTerminalRow(node, targetPackage)) rows.add(node)
            for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
        }
        return rows
    }

    private fun visibleChromiumNonRowNodes(
        requireFocusAction: Boolean,
    ): List<AccessibilityNodeInfo> {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val root = automation.rootInActiveWindow ?: return emptyList()
        val nodes = mutableListOf<AccessibilityNodeInfo>()
        val queue = ArrayDeque<Pair<AccessibilityNodeInfo, Boolean>>()
        queue.add(root to false)
        while (queue.isNotEmpty()) {
            val (node, insideTerminalWebView) = queue.removeFirst()
            val terminalWebView = node.packageName?.toString() == targetPackage &&
                node.className?.toString() == WebView::class.java.name
            if (insideTerminalWebView && terminalNonRowKeyOrNull(node) != null &&
                (!requireFocusAction || node.actionList.any {
                    it.id == AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS
                })
            ) {
                nodes.add(node)
            }
            val childInsideTerminalWebView = insideTerminalWebView || terminalWebView
            for (index in 0 until node.childCount) {
                node.getChild(index)?.let { child ->
                    queue.addLast(child to childInsideTerminalWebView)
                }
            }
        }
        return nodes
    }

    private fun isVisibleTerminalRow(node: AccessibilityNodeInfo, targetPackage: String): Boolean {
        val bounds = Rect()
        node.getBoundsInScreen(bounds)
        val parentCollection = node.parent?.collectionInfo
        return node.packageName?.toString() == targetPackage &&
            node.isVisibleToUser &&
            !bounds.isEmpty &&
            node.collectionItemInfo != null &&
            parentCollection != null &&
            node.className?.toString() != WebView::class.java.name
    }

    private fun terminalRowKeyOrNull(node: AccessibilityNodeInfo?): TerminalRowKey? {
        node ?: return null
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val packageName = node.packageName?.toString() ?: return null
        val className = node.className?.toString()?.takeIf { it.isNotBlank() } ?: return null
        val item = node.collectionItemInfo ?: return null
        if (packageName != targetPackage || className == WebView::class.java.name) return null
        if (node.parent?.collectionInfo == null) return null
        val bounds = Rect()
        node.getBoundsInScreen(bounds)
        if (bounds.isEmpty) return null
        return TerminalRowKey(
            windowId = node.windowId,
            left = bounds.left,
            top = bounds.top,
            right = bounds.right,
            bottom = bounds.bottom,
            rowIndex = item.rowIndex,
            rowSpan = item.rowSpan,
            columnIndex = item.columnIndex,
            columnSpan = item.columnSpan,
            packageName = packageName,
            className = className,
            viewIdResourceName = node.viewIdResourceName,
        )
    }

    private fun terminalNonRowKeyOrNull(node: AccessibilityNodeInfo?): TerminalNonRowKey? {
        node ?: return null
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val packageName = node.packageName?.toString() ?: return null
        val className = node.className?.toString()?.takeIf { it.isNotBlank() } ?: return null
        if (packageName != targetPackage || className == WebView::class.java.name ||
            node.collectionItemInfo != null || !node.isVisibleToUser
        ) return null
        val bounds = Rect()
        node.getBoundsInScreen(bounds)
        if (bounds.isEmpty) return null
        return TerminalNonRowKey(
            windowId = node.windowId,
            left = bounds.left,
            top = bounds.top,
            right = bounds.right,
            bottom = bounds.bottom,
            packageName = packageName,
            className = className,
            viewIdResourceName = node.viewIdResourceName,
        )
    }

    private fun requireTerminalRowKey(
        node: AccessibilityNodeInfo,
        caseId: String,
    ): TerminalRowKey {
        val key = terminalRowKeyOrNull(node)
        assertNotNull("case=$caseId route=accessibility row-key-missing", key)
        return requireNotNull(key)
    }

    private fun awaitNoAccessibilityFocus(caseId: String) {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            automation.clearCache()
            if (automation.rootInActiveWindow
                    ?.findFocus(AccessibilityNodeInfo.FOCUS_ACCESSIBILITY) == null &&
                visibleTerminalRows().none { it.isAccessibilityFocused }
            ) {
                return
            }
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=accessibility focus-clear=false")
    }

    private fun terminalWheelActions(node: AccessibilityNodeInfo): List<TerminalAccessibilityAction> =
        node.actionList.mapNotNull { action ->
            action.label?.toString()?.let { label ->
                if (label == TERMINAL_WHEEL_BACKWARD || label == TERMINAL_WHEEL_FORWARD) {
                    TerminalAccessibilityAction(action.id, label)
                } else {
                    null
                }
            }
        }

    private fun allTerminalWheelActionOccurrences(
        caseId: String,
    ): List<TerminalAccessibilityActionOccurrence> {
        val root = InstrumentationRegistry.getInstrumentation().uiAutomation.rootInActiveWindow
        assertNotNull("case=$caseId route=accessibility root-missing", root)
        val occurrences = mutableListOf<TerminalAccessibilityActionOccurrence>()
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.add(requireNotNull(root))
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            terminalWheelActions(node).mapTo(occurrences) {
                TerminalAccessibilityActionOccurrence(node = node, action = it)
            }
            for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
        }
        return occurrences
    }

    private fun assertFocusedActionOwnership(
        focusedNode: AccessibilityNodeInfo,
        caseId: String,
    ): List<TerminalAccessibilityAction> {
        val focusedKey = requireTerminalRowKey(focusedNode, "$caseId-focused")
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
        var occurrences = emptyList<TerminalAccessibilityActionOccurrence>()
        var focusedRows = emptyList<AccessibilityNodeInfo>()
        while (System.nanoTime() < deadline) {
            occurrences = allTerminalWheelActionOccurrences(caseId)
            focusedRows = visibleTerminalRows().filter { it.isAccessibilityFocused }
            if (occurrences.size == 2 &&
                focusedRows.size == 1 &&
                terminalRowKeyOrNull(focusedRows.single()) == focusedKey &&
                occurrences.all {
                    it.node.isAccessibilityFocused &&
                        terminalRowKeyOrNull(it.node) == focusedKey
                }
            ) {
                break
            }
            Thread.sleep(50)
        }
        assertEquals(
            "case=$caseId route=accessibility occurrence-count",
            2,
            occurrences.size,
        )
        assertEquals(
            "case=$caseId route=accessibility focused-row-count",
            1,
            focusedRows.size,
        )
        assertEquals(
            "case=$caseId route=accessibility focused-row-identity",
            focusedKey,
            focusedRows.singleOrNull()?.let(::terminalRowKeyOrNull),
        )
        val actions = occurrences.map { it.action }
        assertEquals(
            "case=$caseId route=accessibility labels",
            listOf(TERMINAL_WHEEL_BACKWARD, TERMINAL_WHEEL_FORWARD),
            actions.map { it.label }.sorted(),
        )
        assertEquals(
            "case=$caseId route=accessibility distinct-id-count",
            2,
            actions.map { it.id }.distinct().size,
        )
        for ((index, occurrence) in occurrences.withIndex()) {
            assertEquals(
                "case=$caseId route=accessibility window-identity index=$index",
                focusedKey.windowId,
                terminalRowKeyOrNull(occurrence.node)?.windowId,
            )
            assertEquals(
                "case=$caseId route=accessibility source-identity index=$index",
                focusedKey,
                terminalRowKeyOrNull(occurrence.node),
            )
            assertTrue(
                "case=$caseId route=accessibility focused-state index=$index",
                occurrence.node.isAccessibilityFocused,
            )
            assertResourceBackedAccessibilityAction(occurrence.action, caseId, index)
        }
        return actions
    }

    private fun assertResourceBackedAccessibilityAction(
        action: TerminalAccessibilityAction,
        caseId: String,
        index: Int,
    ) {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val packageName = runCatching {
            targetContext.resources.getResourcePackageName(action.id)
        }.getOrNull()
        val typeName = runCatching {
            targetContext.resources.getResourceTypeName(action.id)
        }.getOrNull()
        assertEquals(
            "case=$caseId route=accessibility resource-package index=$index",
            targetContext.packageName,
            packageName,
        )
        assertEquals(
            "case=$caseId route=accessibility resource-type index=$index",
            "id",
            typeName,
        )
    }

    private fun performTerminalWheelAction(label: String, caseId: String) {
        val node = awaitFocusedTerminalRowNode(caseId)
        val action = terminalWheelActions(node).singleOrNull { it.label == label }
        assertNotNull("case=$caseId route=accessibility action-missing", action)
        assertTrue("case=$caseId route=accessibility action-rejected", node.performAction(requireNotNull(action).id))
    }

    private fun allAccessibilityActionLabels(): List<String>? {
        val root = InstrumentationRegistry.getInstrumentation().uiAutomation.rootInActiveWindow ?: return null
        val labels = mutableListOf<String>()
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.add(root)
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            node.actionList.mapNotNullTo(labels) { it.label?.toString() }
            for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
        }
        return labels
    }

    private fun assertNoTerminalWheelActionLabels(caseId: String) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
        while (System.nanoTime() < deadline) {
            val labels = allAccessibilityActionLabels()
            if (labels != null && labels.none {
                    it == TERMINAL_WHEEL_BACKWARD || it == TERMINAL_WHEEL_FORWARD
                }
            ) {
                return
            }
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=accessibility expectedCount=0 index=0")
    }

    private fun recordTerminalAccessibilityEvents(block: () -> Unit): List<Pair<Int, Int>> {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val events = LinkedBlockingQueue<Pair<Int, Int>>()
        automation.setOnAccessibilityEventListener { event ->
            if (event.packageName?.toString() == targetPackage &&
                event.source?.packageName?.toString() == targetPackage
            ) {
                events.add(event.eventType to event.contentChangeTypes)
            }
        }
        return try {
            block()
            val discoveryDeadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
            while (System.nanoTime() < discoveryDeadline && events.none(::isSubtreeRefresh)) {
                Thread.sleep(50)
            }
            if (events.any(::isSubtreeRefresh)) {
                val tailDeadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(1)
                var quietDeadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(350)
                var observedCount = events.size
                while (System.nanoTime() < tailDeadline && System.nanoTime() < quietDeadline) {
                    Thread.sleep(25)
                    if (events.size != observedCount) {
                        observedCount = events.size
                        quietDeadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(350)
                    }
                }
            }
            events.toList()
        } finally {
            automation.setOnAccessibilityEventListener(null)
        }
    }

    private fun assertHasSubtreeRefresh(events: List<Pair<Int, Int>>, caseId: String) {
        assertTrue(
            "case=$caseId route=accessibility subtree-refresh",
            events.any(::isSubtreeRefresh),
        )
    }

    private fun isSubtreeRefresh(event: Pair<Int, Int>): Boolean =
        event.first == AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED &&
            event.second and AccessibilityEvent.CONTENT_CHANGE_TYPE_SUBTREE != 0

    private fun injectBelowSlopTap(
        webView: WebView,
        geometry: TerminalTouchGeometry,
        point: TouchPoint,
        caseId: String,
    ) {
        NativeTouchStream(webView, caseId).use { stream ->
            val moved = point.copy(y = point.y + geometry.belowSlopDistance)
            stream.down(point)
            stream.move(moved)
            stream.up(moved)
        }
    }

    private fun injectDrag(
        webView: WebView,
        geometry: TerminalTouchGeometry,
        start: TouchPoint,
        direction: TouchWheelDirection,
        caseId: String,
        primaryPointerId: Int = 0,
    ) {
        NativeTouchStream(webView, caseId, primaryPointerId).use { stream ->
            stream.down(start)
            val end = geometry.move(start, direction)
            stream.move(end)
            stream.up(end)
        }
    }

    private fun injectLargeBackwardDrag(
        webView: WebView,
        geometry: TerminalTouchGeometry,
        start: TouchPoint,
        caseId: String,
    ) {
        NativeTouchStream(webView, caseId).use { stream ->
            val end = start.copy(y = geometry.screenTop + geometry.screenHeight - 1f)
            stream.down(start)
            stream.move(end)
            stream.up(end)
        }
    }

    private fun injectHorizontalFirst(
        webView: WebView,
        geometry: TerminalTouchGeometry,
        start: TouchPoint,
        caseId: String,
    ) {
        NativeTouchStream(webView, caseId).use { stream ->
            stream.down(start)
            stream.move(
                start.copy(
                    x = start.x + geometry.horizontalClaimDistance,
                    y = start.y - geometry.belowSlopDistance,
                ),
            )
            stream.move(geometry.move(start, TouchWheelDirection.Forward))
            stream.up(geometry.move(start, TouchWheelDirection.Forward))
        }
    }

    private fun injectLongPress(webView: WebView, point: TouchPoint, caseId: String) {
        NativeTouchStream(webView, caseId).use { stream ->
            stream.down(point)
            SystemClock.sleep(ViewConfiguration.getLongPressTimeout().toLong() + 250)
            stream.up(point)
        }
    }

    private fun postRawNativeMessages(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        payloads: List<String>,
    ) {
        onUi(scenario) {
            val field = LockedTerminalWebView::class.java.getDeclaredField("pagePort")
            field.isAccessible = true
            val port = requireNotNull(field.get(webView) as? WebMessagePortCompat)
            payloads.forEach { port.postMessage(WebMessageCompat(it)) }
        }
    }

    private fun awaitTerminal(
        scenario: ActivityScenario<TerminalTestActivity>,
        discardBindEvents: Boolean = true,
    ): WebView {
        assertTrue("native page port did not become ready", TerminalTestProbe.ready.await(5, TimeUnit.SECONDS))
        val webView = onUi(scenario) {
            requireNotNull(findWebView(it.window.decorView)) { "terminal activity has no WebView" }
        }
        if (discardBindEvents) {
            TerminalTestProbe.events.clear()
        }
        return webView
    }

    private fun assertEvent(expected: TerminalTestEvent, caseId: String = "terminal-protocol") {
        val actual = TerminalTestProbe.events.poll(5, TimeUnit.SECONDS)
        assertNotNull("case=$caseId route=event expectedCount=1 index=0", actual)
        if (expected is TerminalTestEvent.Input) {
            assertTrue("case=$caseId route=event index=0 expectedType=input", actual is TerminalTestEvent.Input)
            val actualBytes = (actual as TerminalTestEvent.Input).bytes
            assertTrue(
                "case=$caseId route=event index=0 expectedLength=${expected.bytes.size} " +
                    "actualLength=${actualBytes.size}",
                actualBytes.contentEquals(expected.bytes),
            )
        } else {
            assertTrue("case=$caseId route=event index=0 expectedType=state", actual !is TerminalTestEvent.Input)
            assertEquals("case=$caseId route=event index=0", expected, actual)
        }
    }

    private fun pollEvent(): TerminalTestEvent? =
        TerminalTestProbe.events.poll(250, TimeUnit.MILLISECONDS)

    private fun assertConsumedInput(expected: String) {
        assertEvent(TerminalTestEvent.Modifiers(OFF_OFF))
        assertEvent(TerminalTestEvent.Input(expected.toByteArray()))
    }

    private fun armModifiers(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        target: TerminalModifiers,
    ) {
        require(target != OFF_OFF)
        if (target.control == TerminalModifierPhase.Armed) {
            postAccessory(scenario, webView, "Control")
            assertEvent(TerminalTestEvent.Modifiers(CONTROL_ARMED))
        }
        if (target.alt == TerminalModifierPhase.Armed) {
            postAccessory(scenario, webView, "Alt")
            assertEvent(TerminalTestEvent.Modifiers(target))
        }
    }

    private fun postAccessory(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        key: String,
    ) {
        val accessory = TerminalAccessory.entries.singleOrNull { it.name == key }
        assertNotNull("the typed TerminalAccessory boundary is missing $key", accessory)
        onUi(scenario) {
            val page = requireNotNull(TerminalTestProbe.page)
            assertTrue("the test probe exposed a different terminal page", page === webView)
            page.sendAccessory(requireNotNull(accessory))
        }
    }

    private fun dispatchHardwareKey(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        keyCode: Int,
        metaState: Int = 0,
    ) {
        val eventTime = SystemClock.uptimeMillis()
        val accepted = onUi(scenario) {
            webView.dispatchKeyEvent(
                KeyEvent(eventTime, eventTime, KeyEvent.ACTION_DOWN, keyCode, 0, metaState),
            )
        }
        assertTrue("WebView rejected hardware key $keyCode", accepted)
        onUi(scenario) {
            webView.dispatchKeyEvent(
                KeyEvent(eventTime, SystemClock.uptimeMillis(), KeyEvent.ACTION_UP, keyCode, 0, metaState),
            )
        }
    }

    private fun commitText(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        value: String,
    ) {
        val accepted = withTerminalInputConnection(scenario, webView) { connection ->
            connection.commitText(value, 1)
        }
        assertTrue("IME commit was rejected", accepted)
    }

    private fun composeText(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        value: String,
    ) {
        val accepted = withTerminalInputConnection(scenario, webView) { connection ->
            connection.setComposingText(value, 1) && connection.finishComposingText()
        }
        assertTrue("IME composition was rejected", accepted)
    }

    private fun focusTerminal(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
    ) {
        assertTrue(
            "WebView could not take terminal input focus",
            onUi(scenario) {
                webView.requestFocus()
                webView.hasFocus()
            },
        )
        requireNotNull(TerminalTestProbe.page).focus()
        awaitValue(
            webView,
            "document.hasFocus() && document.activeElement === document.querySelector('.xterm-helper-textarea')",
            "true",
        )
    }

    private fun awaitNativeWebViewFocus(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (onUi(scenario) { webView.hasFocus() }) return
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=native focus=false")
    }

    private fun withTerminalInputConnection(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        block: (InputConnection) -> Boolean,
    ): Boolean {
        focusTerminal(scenario, webView)
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val (available, accepted) = onUi(scenario) {
                val connection = webView.onCreateInputConnection(EditorInfo())
                if (connection == null) false to false else true to block(connection)
            }
            if (available) return accepted
            Thread.sleep(50)
        }
        throw AssertionError("focused WebView did not expose a terminal InputConnection")
    }

    private fun postRawNativeMessage(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        payload: String,
    ) {
        onUi(scenario) {
            val field = LockedTerminalWebView::class.java.getDeclaredField("pagePort")
            field.isAccessible = true
            val port = requireNotNull(field.get(webView) as? WebMessagePortCompat)
            port.postMessage(WebMessageCompat(payload))
        }
    }

    private fun postRawHandshake(webView: WebView, payload: String) {
        evaluate(
            webView,
            """
            (function () {
                var channel = new MessageChannel();
                window.dispatchEvent(new MessageEvent('message', {
                    data: ${JSONObject.quote(payload)},
                    ports: [channel.port2]
                }));
            }())
            """.trimIndent(),
        )
    }

    private fun replaceNextModifierState(webView: WebView, replacement: String) {
        evaluate(
            webView,
            """
            (function () {
                var original = MessagePort.prototype.postMessage;
                MessagePort.prototype.postMessage = function (value) {
                    var payload = JSON.parse(value);
                    if (payload.kind === 'ModifierState') {
                        MessagePort.prototype.postMessage = original;
                        return original.call(this, ${JSONObject.quote(replacement)});
                    }
                    return original.apply(this, arguments);
                };
            }())
            """.trimIndent(),
        )
    }

    private fun findWebView(root: View): WebView? {
        if (root is WebView) return root
        if (root is ViewGroup) {
            for (index in 0 until root.childCount) {
                findWebView(root.getChildAt(index))?.let { return it }
            }
        }
        return null
    }

    private fun <Value> onUi(
        scenario: ActivityScenario<TerminalTestActivity>,
        block: (TerminalTestActivity) -> Value,
    ): Value {
        val latch = CountDownLatch(1)
        var result: Value? = null
        scenario.onActivity { activity ->
            result = block(activity)
            latch.countDown()
        }
        assertTrue("UI-thread operation timed out", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }

    private fun evaluate(webView: WebView, expression: String): String {
        val latch = CountDownLatch(1)
        var result: String? = null
        webView.post {
            webView.evaluateJavascript(expression) {
                result = JSONTokener(it).nextValue()?.toString() ?: "null"
                latch.countDown()
            }
        }
        assertTrue("case=legacy-evaluate route=javascript expectedCount=1 index=0", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }

    private fun awaitValue(webView: WebView, expression: String, expected: String) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (evaluate(webView, expression) == expected) return
            Thread.sleep(50)
        }
        throw AssertionError("case=legacy-await route=javascript expectedCount=1 index=0")
    }

    private fun awaitTerminalSize(predicate: (Pair<Int, Int>) -> Boolean): Pair<Int, Int> {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val size = TerminalTestProbe.sizes.poll(100, TimeUnit.MILLISECONDS) ?: continue
            if (predicate(size)) return size
        }
        throw AssertionError("terminal did not publish the required size")
    }

    // Every published sample resizes the shared PTY, so a single transitional
    // sample below 80 columns is already the regression, not noise to skip.
    private fun awaitSettledSizeWithAllSamplesConforming(): Pair<Int, Int> {
        var latest = awaitTerminalSize { true }
        assertTrue("terminal published below 80 columns: $latest", latest.first >= 80)
        val settleDeadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(2)
        while (System.nanoTime() < settleDeadline) {
            val size = TerminalTestProbe.sizes.poll(100, TimeUnit.MILLISECONDS) ?: continue
            assertTrue("terminal published below 80 columns: $size", size.first >= 80)
            latest = size
        }
        return latest
    }

    private fun awaitVisualState(webView: WebView) {
        val committed = CountDownLatch(1)
        webView.post {
            webView.postVisualStateCallback(
                1L,
                object : WebView.VisualStateCallback() {
                    override fun onComplete(requestId: Long) {
                        committed.countDown()
                    }
                },
            )
        }
        assertTrue("terminal render did not reach a visual state", committed.await(5, TimeUnit.SECONDS))
    }

    @Suppress("DEPRECATION")
    private fun copyWebView(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
    ): Bitmap {
        val copied = CountDownLatch(1)
        var result: Bitmap? = null
        var status: Int? = null
        scenario.onActivity { activity ->
            val location = IntArray(2)
            webView.getLocationInWindow(location)
            val bitmap = Bitmap.createBitmap(webView.width, webView.height, Bitmap.Config.ARGB_8888)
            PixelCopy.request(
                activity.window,
                Rect(location[0], location[1], location[0] + webView.width, location[1] + webView.height),
                bitmap,
                {
                    status = it
                    result = bitmap
                    copied.countDown()
                },
                Handler(Looper.getMainLooper()),
            )
        }
        assertTrue("terminal screenshot timed out", copied.await(5, TimeUnit.SECONDS))
        assertEquals(PixelCopy.SUCCESS, status)
        return requireNotNull(result)
    }

    private fun Bitmap.containsPixel(predicate: (Int) -> Boolean): Boolean {
        val pixels = IntArray(width * height)
        getPixels(pixels, 0, width, 0, 0, width, height)
        return pixels.any(predicate)
    }

}
