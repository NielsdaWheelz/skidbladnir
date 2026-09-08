package dev.niels.skidbladnir

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.content.Context
import android.content.ContextWrapper
import android.content.pm.ActivityInfo
import android.content.res.Configuration
import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.Rect
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.ActionMode
import android.view.InputDevice
import android.view.MotionEvent
import android.view.PixelCopy
import android.view.KeyEvent
import android.view.View
import android.view.ViewGroup
import android.view.ViewConfiguration
import android.view.ViewTreeObserver
import android.view.WindowInsets
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityWindowInfo
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputConnection
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.OnBackPressedCallback
import androidx.activity.compose.setContent
import androidx.core.net.toUri
import androidx.lifecycle.Lifecycle
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebMessagePortCompat
import androidx.webkit.WebViewCompat
import java.io.ByteArrayOutputStream
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
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
private const val TERMINAL_SELECTION_COPY = "Copy"
private const val TERMINAL_SELECTION_TOO_LARGE = "Selection is too large to copy."
private const val ACCESSIBILITY_STABILITY_MILLIS = 1_250L
private const val IME_STABILITY_MILLIS = 1_000L

private enum class TerminalSelectionMouseMode(val control: String) {
    Off("\u001b[?1000l\u001b[?1002l\u001b[?1003l\u001b[?1006l"),
    Sgr("\u001b[?1003h\u001b[?1006h"),
}

private enum class TerminalSelectionLifecycleBoundary {
    PrimaryTap,
    SecondPointer,
    TouchCancel,
    Disable,
    Background,
    Rotation,
    PageFailure,
    Disposal,
    Recreation,
}

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

private sealed interface PreservedClipboard {
    data object KnownEmpty : PreservedClipboard

    data class KnownClip(val snapshot: ClipData) : PreservedClipboard
}

private data class SgrMouseReport(
    val button: Int,
    val column: Int,
    val row: Int,
    val release: Boolean,
)

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
    fun testActivityDisposesOwnedWebViewsBeforeReplacementAndDestroy() {
        val initialUnavailable = TerminalTestProbe.unavailable
        val replacementProbe = TerminalProbe()
        val finalProbe = TerminalProbe()
        val scenario = ActivityScenario.launch(TerminalTestActivity::class.java)
        try {
            awaitTerminal(scenario)
            onUi(scenario) { activity ->
                activity.setContentView(createTestTerminal(activity, replacementProbe))
            }
            assertTrue(
                "case=test-owner-replacement route=lifecycle ready=false",
                replacementProbe.ready.await(5, TimeUnit.SECONDS),
            )
            InstrumentationRegistry.getInstrumentation().waitForIdleSync()
            assertEquals(
                "case=test-owner-webview-replacement route=lifecycle unavailable=false",
                1L,
                initialUnavailable.count,
            )
            onUi(scenario) { activity -> activity.setContent {} }
            InstrumentationRegistry.getInstrumentation().waitForIdleSync()
            assertEquals(
                "case=test-owner-compose-replacement route=lifecycle unavailable=false",
                1L,
                replacementProbe.unavailable.count,
            )
            onUi(scenario) { activity ->
                activity.setContentView(createTestTerminal(activity, finalProbe))
            }
            assertTrue(
                "case=test-owner-final route=lifecycle ready=false",
                finalProbe.ready.await(5, TimeUnit.SECONDS),
            )
        } finally {
            scenario.close()
        }
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        assertEquals(
            "case=test-owner-destroy route=lifecycle unavailable=false",
            1L,
            finalProbe.unavailable.count,
        )
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
    fun unfocusedTrustedTerminalTapShowsTheSoftwareKeyboard() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val geometry = terminalTouchGeometry(scenario, webView)
            val tap = geometry.cell(minOf(7, geometry.columns - 1), geometry.rows / 2)
            unfocusTerminal(scenario, webView, "terminal-drag-before")
            awaitNoTerminalSelection(webView, "terminal-tap-selection-before")

            try {
                clearTerminalEvents()
                injectDrag(
                    webView,
                    geometry,
                    tap,
                    TouchWheelDirection.Forward,
                    "terminal-drag-ime",
                )
                assertTerminalImeRemainsHidden(scenario, webView, "terminal-drag-ime")
                awaitNoTerminalSelection(webView, "terminal-drag-selection-after")
                assertNoInput("terminal-drag", "local", TouchWheelDirection.Forward)

                unfocusTerminal(scenario, webView, "terminal-tap-before")
                val origin = onUi(scenario) { webView.x to webView.y }
                clearTerminalEvents()
                injectBelowSlopTap(webView, geometry, tap, "terminal-tap-ime")

                awaitTerminalIme(
                    scenario,
                    webView,
                    visible = true,
                    caseId = "terminal-tap-ime",
                )
                awaitNativeWebViewFocus(scenario, webView, "terminal-tap-native-focus-after")
                awaitBooleanState(
                    webView,
                    "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                    "terminal-tap-textarea-focus-after",
                )
                assertEquals(
                    "case=terminal-tap route=native webview-origin",
                    origin,
                    onUi(scenario) { webView.x to webView.y },
                )
                awaitNoTerminalSelection(webView, "terminal-tap-selection-after")
                assertNoInput("terminal-tap", "local", TouchWheelDirection.Forward)
                assertEquals(
                    "case=terminal-tap route=page live",
                    1L,
                    TerminalTestProbe.unavailable.count,
                )
            } finally {
                hideTerminalIme(scenario, webView, "terminal-tap-cleanup")
            }
        }
    }

    @Test
    fun imeRequestRequiresAnIdleEnabledTerminal() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "ime-authorization-selection"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            focusTerminal(scenario, webView)
            applyControlFixture(
                page,
                "\u001bc" + "\r\n".repeat(row) + "seed",
                "$caseId-fixture",
            )
            injectSelectionHoldDrag(
                webView,
                geometry.cell(0, row),
                geometry.cell(3, row),
                "$caseId-selection",
            )
            awaitSelectionCopyAction("$caseId-selection-action")
            awaitNativeXtermSelection(webView, "$caseId-selection-range")
            hideTerminalIme(scenario, webView, "$caseId-before")
            replaceNextPageMessage(
                webView,
                "OutputApplied",
                """{"kind":"ImeRequested"}""",
            )
            clearTerminalEvents()

            page.write("\r\nx".toByteArray())

            assertTerminalImeRemainsHidden(scenario, webView, "$caseId-ime")
            awaitSelectionCopyAction("$caseId-action-after")
            awaitNativeXtermSelection(webView, "$caseId-range-after")
            assertEquals(
                "case=$caseId route=selection page-live",
                1L,
                TerminalTestProbe.unavailable.count,
            )
            assertNull(
                "case=$caseId route=selection terminal-event",
                pollEvent(),
            )
        }

        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "ime-authorization-disabled"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            unfocusTerminal(scenario, webView, "$caseId-before")
            onUi(scenario) { webView.isEnabled = false }
            replaceNextPageMessage(
                webView,
                "OutputApplied",
                """{"kind":"ImeRequested"}""",
            )
            clearTerminalEvents()
            try {
                page.write("x".toByteArray())

                assertTerminalImeRemainsHidden(scenario, webView, "$caseId-ime")
                assertEquals(
                    "case=$caseId route=disabled page-live",
                    1L,
                    TerminalTestProbe.unavailable.count,
                )
                assertNull(
                    "case=$caseId route=disabled terminal-event",
                    pollEvent(),
                )
            } finally {
                onUi(scenario) { webView.isEnabled = true }
                hideTerminalIme(scenario, webView, "$caseId-cleanup")
            }
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

            injectBelowSlopTap(webView, geometry, start, "touch-mouse-tap")
            assertEvent(
                TerminalTestEvent.Input(
                    "\u001b[<0;${startColumn + 1};${startRow + 1}M".toByteArray(),
                ),
                "touch-mouse-tap-press",
            )
            assertEvent(
                TerminalTestEvent.Input(
                    "\u001b[<0;${startColumn + 1};${startRow + 1}m".toByteArray(),
                ),
                "touch-mouse-tap-release",
            )
            assertNull(
                "case=touch-mouse-tap route=event expectedCount=2 index=2",
                pollEvent(),
            )
            awaitNoTerminalSelection(webView, "touch-mouse-tap-selection")

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

            assertEquals(
                "case=script-touch route=containment dispatch-not-consumed",
                "false",
                evaluateSafely(
                    webView,
                    """
                (function () {
                    var screen = document.querySelector('.xterm-screen');
                    var bounds = screen.getBoundingClientRect();
                    screen.dataset.untrustedTouchLeaks = '0';
                    screen.addEventListener('touchstart', function () {
                        screen.dataset.untrustedTouchLeaks =
                            String(Number(screen.dataset.untrustedTouchLeaks) + 1);
                    });
                    function touch(y) {
                        return new Touch({
                            identifier: 91,
                            target: screen,
                            clientX: bounds.left + bounds.width / 2,
                            clientY: y
                        });
                    }
                    var startTouch = touch(bounds.top + bounds.height / 2);
                    var movedTouch = touch(bounds.top + bounds.height / 3);
                    screen.dispatchEvent(new TouchEvent('touchstart', {
                        bubbles: true, cancelable: true, composed: true,
                        touches: [startTouch], targetTouches: [startTouch],
                        changedTouches: [startTouch]
                    }));
                    screen.dispatchEvent(new TouchEvent('touchmove', {
                        bubbles: true, cancelable: true, composed: true,
                        touches: [movedTouch], targetTouches: [movedTouch],
                        changedTouches: [movedTouch]
                    }));
                    return screen.dispatchEvent(new TouchEvent('touchend', {
                        bubbles: true, cancelable: true, composed: true,
                        touches: [], targetTouches: [], changedTouches: [movedTouch]
                    }));
                }())
                    """.trimIndent(),
                    "script-touch",
                ),
            )
            SystemClock.sleep(250)
            assertEquals(
                "case=script-touch route=local position",
                top,
                awaitAccessiblePosition(webView, "script-touch-position"),
            )
            assertNoInput("script-touch", "none", TouchWheelDirection.Forward)
            assertEquals(
                "case=script-touch route=containment lower-listener-count",
                "0",
                evaluateSafely(
                    webView,
                    "document.querySelector('.xterm-screen').dataset.untrustedTouchLeaks",
                    "script-touch-leak-count",
                ),
            )

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
    fun trustedTouchSelectionCopiesExactPhoneLocalClipboardWithMouseOff() {
        trustedTouchSelectionCopyJourney(TerminalSelectionMouseMode.Off)
    }

    @Test
    fun trustedTouchSelectionCopiesExactPhoneLocalClipboardWithSgrMouse() {
        trustedTouchSelectionCopyJourney(TerminalSelectionMouseMode.Sgr)
    }

    private fun trustedTouchSelectionCopyJourney(mouseMode: TerminalSelectionMouseMode) {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-copy-${mouseMode.name.lowercase()}"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            val selectedText = "phone local selection"
            val start = geometry.cell(0, row)
            val end = geometry.cell(selectedText.lastIndex, row)
            val fallbackBackCount = AtomicInteger()
            focusTerminal(scenario, webView)
            withRestoredClipboard(scenario) { clipboard ->
                applyControlFixture(
                    page,
                    "\u001bc" + "\r\n".repeat(row) + selectedText + mouseMode.control,
                    "$caseId-fixture",
                )
                setClipboardBaseline(clipboard)

                injectTap(webView, start, "$caseId-prethreshold-tap")
                awaitNoTerminalSelection(webView, "$caseId-prethreshold-selection")
                awaitNoSelectionCopyAction("$caseId-prethreshold-action")
                assertPrethresholdTapInput(mouseMode, 0, row, "$caseId-prethreshold-tap")
                clearTerminalEvents()

                onUi(scenario) { activity ->
                    activity.onBackPressedDispatcher.addCallback(
                        activity,
                        object : OnBackPressedCallback(true) {
                            override fun handleOnBackPressed() {
                                fallbackBackCount.incrementAndGet()
                            }
                        },
                    )
                }

                injectSelectionHoldDrag(webView, start, end, "$caseId-back-selection")
                awaitNativeXtermSelection(webView, "$caseId-back-selection-visible")
                val backCopy = awaitSelectionCopyAction("$caseId-back-action")
                assertFloatingSelectionCopyWindow(scenario, webView, backCopy, "$caseId-back-window")
                sendSystemBack()
                awaitNoSelectionCopyAction("$caseId-back-dismissed-action")
                awaitNoTerminalSelection(webView, "$caseId-back-dismissed-selection")
                assertEquals("case=$caseId route=back-owned", 0, fallbackBackCount.get())
                sendSystemBack()
                awaitCounter(fallbackBackCount, 1, "$caseId-back-fallback")
                assertSelectionEmittedNoInput("$caseId-back")

                injectSelectionHoldDragWithDistinctEnd(
                    webView,
                    start,
                    geometry.cell(selectedText.lastIndex - 1, row),
                    end,
                    "$caseId-copy-selection",
                )
                awaitNativeXtermSelection(webView, "$caseId-copy-selection-visible")
                val beforeRedraw = awaitSelectionCopyAction("$caseId-copy-action-before-redraw")
                assertFloatingSelectionCopyWindow(
                    scenario,
                    webView,
                    beforeRedraw,
                    "$caseId-copy-window-before-redraw",
                )
                applyControlFixture(
                    page,
                    "\rdisplay changed cells",
                    "$caseId-release-snapshot",
                )
                awaitBooleanState(
                    webView,
                    "document.querySelector('.xterm-rows').textContent.includes('display changed cells')",
                    "$caseId-release-snapshot-rendered",
                )
                awaitNativeXtermSelection(webView, "$caseId-release-snapshot-selection")
                val copyAction = awaitSelectionCopyAction("$caseId-copy-action-after-redraw")
                assertFloatingSelectionCopyWindow(
                    scenario,
                    webView,
                    copyAction,
                    "$caseId-copy-window-after-redraw",
                )
                assertClipboardBaseline(clipboard, "$caseId-before-copy")
                InstrumentationRegistry.getInstrumentation().uiAutomation.clearCache()
                val activationAction = awaitSelectionCopyAction("$caseId-copy-action-activation")
                injectSelectionCopyAction(webView, activationAction, "$caseId-copy-activation")
                awaitExactSelectionClipboard(clipboard, selectedText, "$caseId-clipboard")
                awaitNoSelectionCopyAction("$caseId-copy-cleared-action")
                awaitNoTerminalSelection(webView, "$caseId-copy-cleared-selection")
                assertSelectionEmittedNoInput("$caseId-copy")
            }
        }
    }

    @Test
    fun selectionClipboardAcceptsExactUtf8ByteLimit() {
        val limit = 262_144
        val cases = listOf(
            "ascii" to "a".repeat(limit),
            "multibyte" to "é".repeat(limit / 2),
        )
        for ((phase, selectedText) in cases) {
            assertEquals(
                "case=selection-limit-$phase route=fixture byte-count",
                limit,
                selectedText.toByteArray(Charsets.UTF_8).size,
            )
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val caseId = "selection-limit-$phase"
                val webView = awaitTerminal(scenario)
                val page = requireNotNull(TerminalTestProbe.page)
                val geometry = terminalTouchGeometry(scenario, webView)
                val row = geometry.rows / 2
                focusTerminal(scenario, webView)
                withRestoredClipboard(scenario) { clipboard ->
                    applyControlFixture(
                        page,
                        "\u001bc" + "\r\n".repeat(row) + "seed",
                        "$caseId-fixture",
                    )
                    setClipboardBaseline(clipboard)
                    replaceNextPageMessage(
                        webView,
                        "SelectionAvailable",
                        JSONObject()
                            .put("kind", "SelectionAvailable")
                            .put("anchorX", 0.5)
                            .put("anchorY", 0.5)
                            .put("text", selectedText)
                            .toString(),
                    )
                    clearTerminalEvents()
                    injectSelectionHoldDrag(
                        webView,
                        geometry.cell(0, row),
                        geometry.cell(3, row),
                        "$caseId-selection",
                    )
                    val copyAction = awaitSelectionCopyAction("$caseId-action")
                    assertClipboardBaseline(clipboard, "$caseId-before-copy")
                    injectSelectionCopyAction(webView, copyAction, "$caseId-activation")
                    awaitExactSelectionClipboard(clipboard, selectedText, "$caseId-clipboard")
                    awaitNoSelectionCopyAction("$caseId-cleared-action")
                    awaitNoTerminalSelection(webView, "$caseId-cleared-selection")
                    assertSelectionEmittedNoInput(caseId)
                }
            }
        }
    }

    @Test
    fun selectionLifecycleClearsWithoutClipboardOrTerminalInput() {
        for (boundary in TerminalSelectionLifecycleBoundary.entries) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val caseId = "selection-lifecycle-${boundary.name.lowercase()}"
                var webView = awaitTerminal(scenario)
                var page = requireNotNull(TerminalTestProbe.page)
                val originalRequestedOrientation = if (boundary == TerminalSelectionLifecycleBoundary.Rotation) {
                    onUi(scenario) { activity ->
                        val original = activity.requestedOrientation
                        activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
                        original
                    }.also {
                        awaitValue(webView, "window.innerHeight > window.innerWidth", "true")
                        assertEquals(
                            "case=$caseId route=lifecycle orientation-normalized",
                            Configuration.ORIENTATION_PORTRAIT,
                            onUi(scenario) { activity -> activity.resources.configuration.orientation },
                        )
                    }
                } else {
                    null
                }
                val geometry = terminalTouchGeometry(scenario, webView)
                val row = geometry.rows / 2
                val start = geometry.cell(0, row)
                val end = geometry.cell(3, row)
                focusTerminal(scenario, webView)
                withRestoredClipboard(scenario) { clipboard ->
                    applyControlFixture(
                        page,
                        "\u001bc" + "\r\n".repeat(row) + "seed",
                        "$caseId-fixture",
                    )
                    setClipboardBaseline(clipboard)
                    clearTerminalEvents()

                    if (boundary == TerminalSelectionLifecycleBoundary.SecondPointer ||
                        boundary == TerminalSelectionLifecycleBoundary.TouchCancel ||
                        boundary == TerminalSelectionLifecycleBoundary.Disable ||
                        boundary == TerminalSelectionLifecycleBoundary.Background ||
                        boundary == TerminalSelectionLifecycleBoundary.Rotation
                    ) {
                        NativeTouchStream(webView, caseId).use { stream ->
                            stream.down(start)
                            SystemClock.sleep(ViewConfiguration.getLongPressTimeout().toLong() + 250)
                            awaitNativeXtermSelection(webView, "$caseId-active")
                            when (boundary) {
                                TerminalSelectionLifecycleBoundary.SecondPointer -> {
                                    val second = geometry.cell(4, minOf(row + 1, geometry.rows - 1))
                                    stream.secondDown(start, second)
                                    stream.secondUp(start, second)
                                    stream.up(start)
                                }
                                TerminalSelectionLifecycleBoundary.TouchCancel -> stream.cancel(start)
                                TerminalSelectionLifecycleBoundary.Disable -> {
                                    onUi(scenario) { webView.isEnabled = false }
                                    awaitNoTerminalSelection(webView, "$caseId-disabled-selection")
                                    onUi(scenario) { webView.isEnabled = true }
                                    awaitResumedActivityWindowFocus(scenario, "$caseId-reenabled")
                                    stream.move(end)
                                    stream.up(end)
                                }
                                TerminalSelectionLifecycleBoundary.Background -> {
                                    scenario.moveToState(Lifecycle.State.CREATED)
                                    assertFalse(
                                        "case=$caseId route=lifecycle created-window-focus",
                                        onUi(scenario) { webView.hasWindowFocus() },
                                    )
                                    awaitNoTerminalSelection(webView, "$caseId-created-selection")
                                    scenario.moveToState(Lifecycle.State.RESUMED)
                                    awaitResumedActivityWindowFocus(scenario, "$caseId-resumed")
                                    stream.move(end)
                                    stream.up(end)
                                }
                                TerminalSelectionLifecycleBoundary.Rotation -> {
                                    onUi(scenario) {
                                        it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
                                    }
                                    awaitValue(webView, "window.innerWidth > window.innerHeight", "true")
                                    assertEquals(
                                        "case=$caseId route=lifecycle orientation-toggled",
                                        Configuration.ORIENTATION_LANDSCAPE,
                                        onUi(scenario) { activity ->
                                            activity.resources.configuration.orientation
                                        },
                                    )
                                    awaitNoTerminalSelection(webView, "$caseId-landscape-selection")
                                    onUi(scenario) {
                                        it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
                                    }
                                    awaitValue(webView, "window.innerHeight > window.innerWidth", "true")
                                    assertEquals(
                                        "case=$caseId route=lifecycle orientation-returned",
                                        Configuration.ORIENTATION_PORTRAIT,
                                        onUi(scenario) { activity ->
                                            activity.resources.configuration.orientation
                                        },
                                    )
                                    onUi(scenario) {
                                        it.requestedOrientation = requireNotNull(originalRequestedOrientation)
                                    }
                                    assertEquals(
                                        "case=$caseId route=lifecycle orientation-restored",
                                        originalRequestedOrientation,
                                        onUi(scenario) { it.requestedOrientation },
                                    )
                                    stream.move(end)
                                    stream.up(end)
                                }
                                else -> error("released selection boundary reached active branch")
                            }
                        }
                    } else {
                        injectSelectionHoldDrag(webView, start, end, "$caseId-selection")
                        awaitNativeXtermSelection(webView, "$caseId-selection-visible")
                        awaitSelectionCopyAction("$caseId-action")
                        assertClipboardBaseline(clipboard, "$caseId-selected")
                        when (boundary) {
                            TerminalSelectionLifecycleBoundary.PrimaryTap ->
                                injectBelowSlopTap(
                                    webView,
                                    geometry,
                                    geometry.cell(6, row),
                                    "$caseId-boundary",
                                )
                            TerminalSelectionLifecycleBoundary.PageFailure -> {
                                postRawNativeMessage(scenario, webView, "not-json")
                                assertTrue(
                                    "case=$caseId route=lifecycle unavailable=false",
                                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                                )
                            }
                            TerminalSelectionLifecycleBoundary.Disposal ->
                                onUi(scenario) { (webView as LockedTerminalWebView).dispose() }
                            TerminalSelectionLifecycleBoundary.Recreation -> {
                                val priorInput = TerminalTestProbe.input
                                TerminalTestProbe.reset()
                                scenario.recreate()
                                webView = awaitTerminal(scenario)
                                page = requireNotNull(TerminalTestProbe.page)
                                assertNull(
                                    "case=$caseId route=terminal-input prior-owner-count>0",
                                    priorInput.poll(350, TimeUnit.MILLISECONDS),
                                )
                            }
                            TerminalSelectionLifecycleBoundary.SecondPointer,
                            TerminalSelectionLifecycleBoundary.TouchCancel,
                            TerminalSelectionLifecycleBoundary.Disable,
                            TerminalSelectionLifecycleBoundary.Background,
                            TerminalSelectionLifecycleBoundary.Rotation,
                            -> error("active selection boundary reached released branch")
                        }
                    }

                    awaitNoSelectionCopyAction("$caseId-cleared-action")
                    if (boundary != TerminalSelectionLifecycleBoundary.Disposal) {
                        awaitNoTerminalSelection(webView, "$caseId-cleared-selection")
                    }
                    assertSelectionAndCopyRemainAbsent(
                        if (boundary == TerminalSelectionLifecycleBoundary.Disposal) null else webView,
                        "$caseId-stable-absence",
                    )
                    assertClipboardBaseline(clipboard, "$caseId-cleared-clipboard")
                    assertSelectionEmittedNoInput(caseId)

                    if (boundary != TerminalSelectionLifecycleBoundary.PageFailure &&
                        boundary != TerminalSelectionLifecycleBoundary.Disposal
                    ) {
                        awaitResumedActivityWindowFocus(scenario, "$caseId-fresh-resumed")
                        val freshGeometry = terminalTouchGeometry(
                            scenario,
                            webView,
                            if (boundary == TerminalSelectionLifecycleBoundary.Recreation) {
                                null
                            } else {
                                geometry.columns to geometry.rows
                            },
                        )
                        val freshRow = freshGeometry.rows / 2
                        applyControlFixture(
                            page,
                            "\u001bc" + "\r\n".repeat(freshRow) + "seed",
                            "$caseId-fresh-fixture",
                        )
                        focusTerminal(scenario, webView)
                        clearTerminalEvents()
                        injectSelectionHoldDrag(
                            webView,
                            freshGeometry.cell(0, freshRow),
                            freshGeometry.cell(3, freshRow),
                            "$caseId-fresh-selection",
                        )
                        awaitNativeXtermSelection(webView, "$caseId-fresh-visible")
                        awaitSelectionCopyAction("$caseId-fresh-action")
                        assertEquals(
                            "case=$caseId route=protocol fresh-unavailable-count",
                            1L,
                            TerminalTestProbe.unavailable.count,
                        )
                        injectBelowSlopTap(
                            webView,
                            freshGeometry,
                            freshGeometry.cell(6, freshRow),
                            "$caseId-fresh-clear",
                        )
                        awaitNoSelectionCopyAction("$caseId-fresh-cleared-action")
                        awaitNoTerminalSelection(webView, "$caseId-fresh-cleared-selection")
                        assertSelectionEmittedNoInput("$caseId-fresh")
                    }
                }
            }
        }
    }

    @Test
    fun backgroundSelectionClearAcknowledgementIsGenerationBounded() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-clear-generation-background"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            focusTerminal(scenario, webView)
            applyControlFixture(
                page,
                "\u001bc" + "\r\n".repeat(row) + "seed",
                "$caseId-fixture",
            )
            injectSelectionHoldDrag(
                webView,
                geometry.cell(0, row),
                geometry.cell(3, row),
                "$caseId-selection",
            )
            awaitNativeXtermSelection(webView, "$caseId-selection-visible")
            awaitSelectionCopyAction("$caseId-action")
            val firstGeneration = terminalSelectionGeneration(scenario, webView, "$caseId-first")
            clearTerminalEvents()

            scenario.moveToState(Lifecycle.State.CREATED)
            awaitNoTerminalSelection(webView, "$caseId-background-selection")
            scenario.moveToState(Lifecycle.State.RESUMED)
            awaitResumedActivityWindowFocus(scenario, "$caseId-resumed")
            awaitNoSelectionCopyAction("$caseId-cleared-action")
            assertSelectionAndCopyRemainAbsent(webView, "$caseId-stable-absence")
            assertEquals(
                "case=$caseId route=protocol unavailable-count",
                1L,
                TerminalTestProbe.unavailable.count,
            )
            postRawNativeMessage(
                scenario,
                webView,
                JSONObject()
                    .put("kind", "ClearSelection")
                    .put("generation", firstGeneration)
                    .toString(),
            )
            SystemClock.sleep(350)
            assertEquals(
                "case=$caseId route=protocol duplicate-clear-unavailable-count",
                1L,
                TerminalTestProbe.unavailable.count,
            )

            val resumedGeometry = terminalTouchGeometry(
                scenario,
                webView,
                geometry.columns to geometry.rows,
            )
            val resumedRow = resumedGeometry.rows / 2
            injectSelectionHoldDrag(
                webView,
                resumedGeometry.cell(0, resumedRow),
                resumedGeometry.cell(3, resumedRow),
                "$caseId-fresh-selection",
            )
            awaitNativeXtermSelection(webView, "$caseId-fresh-selection-visible")
            awaitSelectionCopyAction("$caseId-fresh-action")
            val secondGeneration = terminalSelectionGeneration(scenario, webView, "$caseId-second")
            assertEquals(
                "case=$caseId route=protocol generation-monotonic",
                firstGeneration.toLong() + 1,
                secondGeneration.toLong(),
            )
            postRawNativeMessage(
                scenario,
                webView,
                JSONObject()
                    .put("kind", "ClearSelection")
                    .put("generation", firstGeneration)
                    .toString(),
            )
            SystemClock.sleep(350)
            awaitNativeXtermSelection(webView, "$caseId-stale-clear-selection")
            awaitSelectionCopyAction("$caseId-stale-clear-action")
            assertEquals(
                "case=$caseId route=protocol fresh-unavailable-count",
                1L,
                TerminalTestProbe.unavailable.count,
            )
            injectBelowSlopTap(
                webView,
                resumedGeometry,
                resumedGeometry.cell(6, resumedRow),
                "$caseId-fresh-clear",
            )
            awaitNoSelectionCopyAction("$caseId-fresh-cleared-action")
            awaitNoTerminalSelection(webView, "$caseId-fresh-cleared-selection")
            assertSelectionEmittedNoInput(caseId)
        }
    }

    @Test
    fun releasedSelectionCancellationSuppressesRemainingPrimaryTail() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-released-cancel-tail"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            val first = geometry.cell(5, row)
            val second = geometry.cell(6, minOf(row + 1, geometry.rows - 1))
            focusTerminal(scenario, webView)
            applyControlFixture(
                page,
                "\u001bc" + "\r\n".repeat(row) + "seed" + TerminalSelectionMouseMode.Sgr.control,
                "$caseId-fixture",
            )
            injectSelectionHoldDrag(
                webView,
                geometry.cell(0, row),
                geometry.cell(3, row),
                "$caseId-selection",
            )
            awaitNativeXtermSelection(webView, "$caseId-selection-visible")
            awaitSelectionCopyAction("$caseId-action")
            clearTerminalEvents()

            NativeTouchStream(webView, caseId, primaryPointerId = 17).use { stream ->
                stream.down(first)
                stream.secondDown(first, second)
                stream.secondUp(first, second)
                stream.up(first)
            }

            awaitNoSelectionCopyAction("$caseId-cleared-action")
            awaitNoTerminalSelection(webView, "$caseId-cleared-selection")
            assertEquals(
                "case=$caseId route=protocol unavailable-count",
                1L,
                TerminalTestProbe.unavailable.count,
            )
            assertSelectionEmittedNoInput(caseId)
            injectSelectionHoldDrag(
                webView,
                geometry.cell(0, row),
                geometry.cell(3, row),
                "$caseId-fresh",
                primaryPointerId = 21,
            )
            awaitNativeXtermSelection(webView, "$caseId-fresh-selection")
            awaitSelectionCopyAction("$caseId-fresh-action")
            injectBelowSlopTap(webView, geometry, first, "$caseId-fresh-clear")
            awaitNoTerminalSelection(webView, "$caseId-fresh-cleared-selection")
            awaitNoSelectionCopyAction("$caseId-fresh-cleared-action")
        }
    }

    @Test
    fun accessibilityScrollClearsActiveSelectionAndDrainsTouchTail() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-accessibility-scroll"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            val start = geometry.cell(0, row)
            val end = geometry.cell(3, row)
            focusTerminal(scenario, webView)
            applyControlFixture(
                page,
                "\u001bc" + "\r\n".repeat(row) + "seed" + TerminalSelectionMouseMode.Sgr.control,
                "$caseId-fixture",
            )
            val rowKey = requireTerminalRowKey(
                awaitFocusedTerminalRowNode(caseId, requireFocusAction = true),
                "$caseId-initial",
            )
            clearTerminalEvents()

            NativeTouchStream(webView, caseId, primaryPointerId = 23).use { stream ->
                stream.down(start)
                SystemClock.sleep(ViewConfiguration.getLongPressTimeout().toLong() + 250)
                awaitNativeXtermSelection(webView, "$caseId-active")
                val currentNode = awaitSingleFocusedTerminalRow(
                    rowKey,
                    "$caseId-current",
                )
                val currentAction = requireNotNull(
                    terminalWheelActions(currentNode).singleOrNull {
                        it.label == TERMINAL_WHEEL_FORWARD
                    },
                )
                assertTrue(
                    "case=$caseId route=accessibility action-rejected",
                    currentNode.performAction(currentAction.id),
                )
                awaitNoTerminalSelection(webView, "$caseId-cleared-selection")
                stream.move(end)
                stream.up(end)
            }

            awaitNoSelectionCopyAction("$caseId-cleared-action")
            assertEquals(
                "case=$caseId route=protocol unavailable-count",
                1L,
                TerminalTestProbe.unavailable.count,
            )
            assertSelectionEmittedNoInput(caseId)

            focusTerminal(scenario, webView)
            injectSelectionHoldDrag(
                webView,
                start,
                end,
                "$caseId-fresh-selection",
                primaryPointerId = 25,
            )
            awaitNativeXtermSelection(webView, "$caseId-fresh-visible")
            awaitSelectionCopyAction("$caseId-fresh-action")
            injectBelowSlopTap(webView, geometry, geometry.cell(6, row), "$caseId-fresh-clear")
            awaitNoTerminalSelection(webView, "$caseId-fresh-cleared-selection")
            awaitNoSelectionCopyAction("$caseId-fresh-cleared-action")
            assertSelectionEmittedNoInput("$caseId-fresh")
        }
    }

    @Test
    fun availabilityRevocationPrecedesQueuedCopyAction() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-revocation-before-copy"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            focusTerminal(scenario, webView)
            withRestoredClipboard(scenario) { clipboard ->
                applyControlFixture(
                    page,
                    "\u001bc" + "\r\n".repeat(row) + "seed",
                    "$caseId-fixture",
                )
                setClipboardBaseline(clipboard)
                injectSelectionHoldDrag(
                    webView,
                    geometry.cell(0, row),
                    geometry.cell(3, row),
                    "$caseId-selection",
                )
                awaitSelectionCopyAction("$caseId-action")
                // A public ActionMode tap cannot establish this ordering: its input delivery may
                // race the main-thread unavailable callback. Queue the framework's real callback
                // itself so the output-lock commit is proven to occur before the clipboard gate;
                // no fake callback, clipboard, or production test seam participates.
                val invokeCopyAction = onUi(scenario) {
                    val controllerField = LockedTerminalWebView::class.java
                        .getDeclaredField("selectionController")
                        .apply { isAccessible = true }
                    val controller = controllerField.get(webView)
                    val callback = controller.javaClass.getDeclaredField("actionModeCallback")
                        .apply { isAccessible = true }
                        .get(controller) as ActionMode.Callback
                    val state = controller.javaClass.getDeclaredField("state")
                        .apply { isAccessible = true }
                        .get(controller)
                    val mode = state.javaClass.getDeclaredField("actionMode")
                        .apply { isAccessible = true }
                        .get(state) as ActionMode
                    val item = mode.menu.findItem(R.id.terminal_selection_copy_action)
                    val invocation: () -> Boolean = {
                        callback.onActionItemClicked(mode, item)
                    }
                    invocation
                }
                val mainEntered = CountDownLatch(1)
                val releaseMain = CountDownLatch(1)
                webView.post {
                    mainEntered.countDown()
                    releaseMain.await(5, TimeUnit.SECONDS)
                }
                assertTrue(
                    "case=$caseId route=ordering main-blocked=false",
                    mainEntered.await(5, TimeUnit.SECONDS),
                )
                val copyFinished = CountDownLatch(1)
                val copyResult = AtomicInteger(-1)
                try {
                    assertTrue(
                        "case=$caseId route=ordering action-queued=false",
                        webView.post {
                            copyResult.set(if (invokeCopyAction()) 1 else 0)
                            copyFinished.countDown()
                        },
                    )
                    page.write(ByteArray(1024 * 1024 + 1))
                    val unavailableCommitted = run {
                        val monitorField = LockedTerminalWebView::class.java
                            .getDeclaredField("outputMonitor")
                            .apply { isAccessible = true }
                        val unavailableField = LockedTerminalWebView::class.java
                            .getDeclaredField("unavailable")
                            .apply { isAccessible = true }
                        val monitor = requireNotNull(monitorField.get(webView))
                        synchronized(monitor) { unavailableField.getBoolean(webView) }
                    }
                    assertTrue(
                        "case=$caseId route=ordering unavailable-not-committed",
                        unavailableCommitted,
                    )
                } finally {
                    releaseMain.countDown()
                }
                assertTrue(
                    "case=$caseId route=ordering copy-callback-timeout",
                    copyFinished.await(5, TimeUnit.SECONDS),
                )
                assertTrue(
                    "case=$caseId route=protocol unavailable=false",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
                assertClipboardBaseline(clipboard, "$caseId-clipboard")
                assertEquals(
                    "case=$caseId route=ordering copy-result",
                    1,
                    copyResult.get(),
                )
                awaitNoSelectionCopyAction("$caseId-action-cleared")
                assertSelectionEmittedNoInput(caseId)
            }
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
    fun activeCompositionArbitratesTrustedTouchBeforeFreshScroll() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            applyControlFixture(
                page,
                "\u001b[?1049l\u001b[?1l" +
                    TerminalSelectionMouseMode.Sgr.control +
                    "\r\n".repeat(geometry.rows * 5),
                "composition-local",
            )
            val compositionBefore = awaitStableAccessiblePosition(
                webView,
                "composition-local-before",
            ) { it > 2 }
            focusTerminal(scenario, webView)
            val compositionText = "active composition"
            val compositionBytes = compositionText.toByteArray()
            val compositionStarted = withTerminalInputConnection(scenario, webView) { connection ->
                connection.setComposingText(compositionText, 1)
            }
            assertTrue("case=composition-local route=ime start", compositionStarted)
            awaitBooleanState(
                webView,
                "document.querySelector('.composition-view').classList.contains('active')",
                "composition-active-before",
            )
            assertNoInput(
                "composition-baseline",
                "ime",
                TouchWheelDirection.Backward,
            )
            awaitBooleanState(
                webView,
                "document.querySelector('.composition-view').classList.contains('active')",
                "composition-active-after-baseline",
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
            assertAccessiblePositionRemains(
                webView,
                "composition-local-after",
                compositionBefore,
            )
            assertOnlyInput(
                "composition-local",
                "ime",
                TouchWheelDirection.Backward,
                compositionBytes,
            )
            awaitBooleanState(
                webView,
                "!document.querySelector('.composition-view').classList.contains('active')",
                "composition-inactive-after",
            )
            assertSelectionAndCopyRemainAbsent(webView, "composition-local")
            awaitBooleanState(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "composition-focus-after",
            )
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
            assertNoInput(
                "composition-finished",
                "ime",
                TouchWheelDirection.Backward,
            )

            applyControlFixture(
                page,
                TerminalSelectionMouseMode.Off.control,
                "composition-fresh-local",
            )
            injectDrag(
                webView,
                compositionGeometry,
                compositionStart,
                TouchWheelDirection.Backward,
                "composition-fresh-scroll",
            )
            val freshPosition = awaitAccessiblePosition(webView, "composition-fresh-scroll-after") {
                it < compositionBefore
            }
            assertTrue(
                "case=composition-fresh-scroll route=local displacement",
                freshPosition < compositionBefore,
            )
            assertNoInput(
                "composition-fresh-scroll",
                "local",
                TouchWheelDirection.Backward,
            )
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
    fun exactPageProtocolRejectsMalformedAndOutOfOrderSelectionMessages() {
        val payloads = listOf(
            """{"kind":"ControlState","state":"Armed"}""",
            """{"kind":"ModifierState","control":"Armed","alt":"Off","extra":true}""",
            """{"kind":"ModifierState","control":1,"alt":"Off"}""",
            """{"kind":"ModifierState","control":"Locked","alt":"Off"}""",
            """{"kind":"ModifierState","control":"Armed"}""",
            """{"kind":"ImeRequested","extra":true}""",
            """{"kind":"SelectionAvailable","anchorX":0.5,"anchorY":0.5,"text":"x"}""",
            """{"kind":"SelectionStarted","generation":1}""",
            """{"kind":"SelectionStarted","generation":"01"}""",
            """{"kind":"SelectionStarted","generation":"9007199254740992"}""",
            """{"kind":"SelectionStarted","generation":"1","extra":true}""",
            """{"kind":"Unknown"}""",
        )
        for ((index, payload) in payloads.withIndex()) {
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
    fun selectionGenerationRejectsReplayDecreaseAndSkip() {
        data class Case(val phase: String, val acceptedSelections: Int, val invalid: (Long) -> Long)
        val cases = listOf(
            Case("replay", 1) { last -> last },
            Case("decrease", 2) { last -> last - 1 },
            Case("skip", 1) { last -> last + 2 },
        )
        for (case in cases) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val caseId = "selection-generation-${case.phase}"
                val webView = awaitTerminal(scenario)
                val page = requireNotNull(TerminalTestProbe.page)
                val geometry = terminalTouchGeometry(scenario, webView)
                val row = geometry.rows / 2
                focusTerminal(scenario, webView)
                applyControlFixture(
                    page,
                    "\u001bc" + "\r\n".repeat(row) + "seed",
                    "$caseId-fixture",
                )
                var lastGeneration = 0L
                repeat(case.acceptedSelections) { index ->
                    injectSelectionHoldDrag(
                        webView,
                        geometry.cell(0, row),
                        geometry.cell(3, row),
                        "$caseId-selection-$index",
                    )
                    awaitSelectionCopyAction("$caseId-action-$index")
                    val generation = terminalSelectionGeneration(
                        scenario,
                        webView,
                        "$caseId-generation-$index",
                    ).toLong()
                    assertEquals(
                        "case=$caseId route=protocol accepted-successor index=$index",
                        lastGeneration + 1,
                        generation,
                    )
                    lastGeneration = generation
                    injectBelowSlopTap(
                        webView,
                        geometry,
                        geometry.cell(6, row),
                        "$caseId-clear-$index",
                    )
                    awaitNoTerminalSelection(webView, "$caseId-cleared-selection-$index")
                    awaitNoSelectionCopyAction("$caseId-cleared-action-$index")
                }
                replaceNextModifierState(
                    webView,
                    JSONObject()
                        .put("kind", "SelectionStarted")
                        .put("generation", case.invalid(lastGeneration).toString())
                        .toString(),
                )
                clearTerminalEvents()
                postAccessory(scenario, webView, "Control")
                assertTrue(
                    "case=$caseId route=protocol unavailable=false",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
                awaitNoSelectionCopyAction("$caseId-action")
                awaitNoTerminalSelection(webView, "$caseId-selection")
                assertSelectionEmittedNoInput(caseId)
            }
        }
    }

    @Test
    fun selectionDecoderRejectsInvalidSnapshotAfterSelectionStarted() {
        val cases = listOf(
            "wrong-shape" to
                """{"kind":"SelectionAvailable","anchorX":0.5,"anchorY":0.5}""",
            "oversize" to JSONObject()
                .put("kind", "SelectionAvailable")
                .put("anchorX", 0.5)
                .put("anchorY", 0.5)
                .put("text", "a".repeat(262_145))
                .toString(),
            "invalid-scalar" to
                "{\"kind\":\"SelectionAvailable\",\"anchorX\":0.5," +
                "\"anchorY\":0.5,\"text\":\"\\uD800\"}",
        )
        for ((phase, replacement) in cases) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val caseId = "selection-decoder-$phase"
                val webView = awaitTerminal(scenario)
                val page = requireNotNull(TerminalTestProbe.page)
                val geometry = terminalTouchGeometry(scenario, webView)
                val row = geometry.rows / 2
                focusTerminal(scenario, webView)
                withRestoredClipboard(scenario) { clipboard ->
                    applyControlFixture(
                        page,
                        "\u001bc" + "\r\n".repeat(row) + "seed",
                        "$caseId-fixture",
                    )
                    setClipboardBaseline(clipboard)
                    replaceNextPageMessage(webView, "SelectionAvailable", replacement)
                    clearTerminalEvents()
                    injectSelectionHoldDrag(
                        webView,
                        geometry.cell(0, row),
                        geometry.cell(3, row),
                        "$caseId-selection",
                    )
                    assertTrue(
                        "case=$caseId route=protocol unavailable=false",
                        TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                    )
                    awaitNoSelectionCopyAction("$caseId-action")
                    assertClipboardBaseline(clipboard, "$caseId-clipboard")
                    assertSelectionEmittedNoInput(caseId)
                }
            }
        }
    }

    @Test
    fun selectionTooLargeRejectionClearsAndShowsFailureWithoutClipboardWrite() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-rejected-too-large"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            focusTerminal(scenario, webView)
            withRestoredClipboard(scenario) { clipboard ->
                applyControlFixture(
                    page,
                    "\u001bc" + "\r\n".repeat(row) + "seed",
                    "$caseId-fixture",
                )
                setClipboardBaseline(clipboard)
                replaceNextPageMessage(
                    webView,
                    "SelectionAvailable",
                    "{\"kind\":\"SelectionCopyRejected\",\"reason\":\"TooLarge\"}",
                )
                clearTerminalEvents()
                assertSingleTooLargeToast("$caseId-feedback") {
                    injectSelectionHoldDrag(
                        webView,
                        geometry.cell(0, row),
                        geometry.cell(3, row),
                        "$caseId-selection",
                    )
                }
                assertEquals(
                    "case=$caseId route=protocol unavailable-count",
                    1L,
                    TerminalTestProbe.unavailable.count,
                )
                awaitNoSelectionCopyAction("$caseId-action")
                awaitNoTerminalSelection(webView, "$caseId-selection-cleared")
                assertClipboardBaseline(clipboard, "$caseId-clipboard")
                assertSelectionEmittedNoInput(caseId)
            }
        }
    }

    @Test
    fun duplicateSelectionSnapshotFailsClosed() {
        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val caseId = "selection-duplicate"
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val geometry = terminalTouchGeometry(scenario, webView)
            val row = geometry.rows / 2
            focusTerminal(scenario, webView)
            withRestoredClipboard(scenario) { clipboard ->
                applyControlFixture(
                    page,
                    "\u001bc" + "\r\n".repeat(row) + "seed",
                    "$caseId-fixture",
                )
                setClipboardBaseline(clipboard)
                injectSelectionHoldDrag(
                    webView,
                    geometry.cell(0, row),
                    geometry.cell(3, row),
                    "$caseId-selection",
                )
                awaitSelectionCopyAction("$caseId-action")
                val generation = terminalSelectionGeneration(scenario, webView, caseId)
                replaceNextModifierState(
                    webView,
                    JSONObject()
                        .put("kind", "SelectionAvailable")
                        .put("generation", generation)
                        .put("anchorX", 0.5)
                        .put("anchorY", 0.5)
                        .put("text", "x")
                        .toString(),
                )
                clearTerminalEvents()
                postAccessory(scenario, webView, "Control")
                assertTrue(
                    "case=$caseId route=protocol unavailable=false",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
                awaitNoSelectionCopyAction("$caseId-cleared-action")
                assertClipboardBaseline(clipboard, "$caseId-clipboard")
                assertSelectionEmittedNoInput(caseId)
            }
        }
    }

    @Test
    fun pagePortHandshakeIsExactVersionThree() {
        assertEquals(
            "case=handshake-v3 route=page valid",
            "{\"kind\":\"PageFailure\"}",
            missingDomHandshake(
                """{"kind":"PagePort","version":3,"longPressMilliseconds":500}""",
                "handshake-v3",
            ),
        )
        for ((index, payload) in listOf(
            """{"kind":"PagePort","version":1}""",
            """{"kind":"PagePort","version":2,"longPressMilliseconds":500}""",
            """{"kind":"PagePort","version":3}""",
            """{"kind":"PagePort","version":3,"longPressMilliseconds":0}""",
            """{"kind":"PagePort","version":3,"longPressMilliseconds":1.5}""",
            """{"kind":"PagePort","version":3,"longPressMilliseconds":"500"}""",
            """{"kind":"PagePort","version":3,"longPressMilliseconds":500,"extra":true}""",
        ).withIndex()) {
            assertNull(
                "case=handshake-invalid-$index route=page expectedCount=0 index=0",
                missingDomHandshake(payload, "handshake-invalid-$index"),
            )
        }
    }

    private fun missingDomHandshake(payload: String, caseId: String): String? {
        TerminalTestProbe.reset()
        val loaded = CountDownLatch(1)
        val messages = LinkedBlockingQueue<String>()
        var nativePort: WebMessagePortCompat? = null
        val terminalSource = InstrumentationRegistry.getInstrumentation()
            .targetContext.assets.open("terminal/terminal.js").bufferedReader().use { it.readText() }

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = onUi(scenario) { activity ->
                (findWebView(activity.window.decorView) as? LockedTerminalWebView)?.dispose()
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
            var primaryFailure: Throwable? = null
            var result: String? = null
            try {
                assertTrue(
                    "case=$caseId route=page load-timeout",
                    loaded.await(5, TimeUnit.SECONDS),
                )
                evaluate(webView, terminalSource)
                onUi(scenario) {
                    val ports = WebViewCompat.createWebMessageChannel(webView)
                    nativePort = ports[0]
                    ports[0].setWebMessageCallback(
                        Handler(Looper.getMainLooper()),
                        object : WebMessagePortCompat.WebMessageCallbackCompat() {
                            override fun onMessage(
                                port: WebMessagePortCompat,
                                message: WebMessageCompat?,
                            ) {
                                message?.data?.let(messages::add)
                            }
                        },
                    )
                    WebViewCompat.postWebMessage(
                        webView,
                        WebMessageCompat(payload, arrayOf(ports[1])),
                        "https://appassets.androidplatform.net".toUri(),
                    )
                }
                result = messages.poll(1, TimeUnit.SECONDS)
            } catch (failure: Throwable) {
                primaryFailure = failure
            }
            val cleanupFailures = mutableListOf<Throwable>()
            runCatching {
                onUi(scenario) {
                    nativePort?.close()
                    Unit
                }
            }.exceptionOrNull()?.let(cleanupFailures::add)
            runCatching {
                onUi(scenario) {
                    val webViewCleanupFailures = mutableListOf<Throwable>()
                    try {
                        runCatching(webView::stopLoading)
                            .exceptionOrNull()
                            ?.let(webViewCleanupFailures::add)
                        runCatching {
                            (webView.parent as? ViewGroup)?.removeView(webView)
                        }.exceptionOrNull()?.let(webViewCleanupFailures::add)
                        runCatching(webView::removeAllViews)
                            .exceptionOrNull()
                            ?.let(webViewCleanupFailures::add)
                    } finally {
                        runCatching(webView::destroy)
                            .exceptionOrNull()
                            ?.let(webViewCleanupFailures::add)
                    }
                    webViewCleanupFailures.firstOrNull()?.let { firstFailure ->
                        webViewCleanupFailures.drop(1).forEach(firstFailure::addSuppressed)
                        throw firstFailure
                    }
                    Unit
                }
            }.exceptionOrNull()?.let(cleanupFailures::add)
            val failure = primaryFailure
            if (failure != null) {
                cleanupFailures.forEach(failure::addSuppressed)
                throw failure
            }
            cleanupFailures.firstOrNull()?.let { cleanupFailure ->
                cleanupFailures.drop(1).forEach(cleanupFailure::addSuppressed)
                throw cleanupFailure
            }
            return result
        }
    }

    @Test
    fun duplicatePagePortFailsClosed() {
        val payload = """{"kind":"PagePort","version":3,"longPressMilliseconds":500}"""
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            postRawHandshake(webView, payload)
            assertTrue(
                "case=handshake-duplicate route=protocol expectedCount=1 index=0",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
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

    private fun assertSelectionEmittedNoInput(caseId: String) {
        val actual = TerminalTestProbe.input.poll(350, TimeUnit.MILLISECONDS)
        assertNull(
            "case=$caseId route=terminal-input expectedCount=0 actualLength=${actual?.size ?: 0}",
            actual,
        )
    }

    private fun assertPrethresholdTapInput(
        mouseMode: TerminalSelectionMouseMode,
        column: Int,
        row: Int,
        caseId: String,
    ) {
        if (mouseMode == TerminalSelectionMouseMode.Off) {
            assertSelectionEmittedNoInput(caseId)
            return
        }
        val wire = ByteArrayOutputStream()
        var chunkCount = 0
        var bounded = true
        var chunk = TerminalTestProbe.input.poll(5, TimeUnit.SECONDS)
        while (chunk != null) {
            chunkCount += 1
            if (chunkCount > 16 || wire.size() + chunk.size > 512) {
                bounded = false
                break
            }
            wire.write(chunk)
            chunk = TerminalTestProbe.input.poll(350, TimeUnit.MILLISECONDS)
        }

        val reports = if (bounded) parseSgrMouseReports(wire.toByteArray()) else null
        val expectedColumn = column + 1
        val expectedRow = row + 1
        val coordinateMismatchCount = reports?.count {
            it.column != expectedColumn || it.row != expectedRow
        } ?: -1
        val pressIndices = reports?.indices?.filter {
            reports[it].button == 0 && !reports[it].release
        }.orEmpty()
        val releaseIndices = reports?.indices?.filter {
            reports[it].button == 0 && reports[it].release
        }.orEmpty()
        val pressIndex = pressIndices.singleOrNull()
        val releaseIndex = releaseIndices.singleOrNull()
        val validHoverPrefix = pressIndex != null && reports != null &&
            reports.take(pressIndex).all { it.button == 35 && !it.release }
        val validOrder = pressIndex != null && releaseIndex != null && reports != null &&
            releaseIndex == pressIndex + 1 && releaseIndex == reports.lastIndex
        assertTrue(
            "case=$caseId route=mouse-report chunks=$chunkCount length=${wire.size()} " +
                "bounded=$bounded parsed=${reports?.size ?: -1} " +
                "coordinateMismatches=$coordinateMismatchCount presses=${pressIndices.size} " +
                "releases=${releaseIndices.size} hoverPrefix=$validHoverPrefix order=$validOrder",
            bounded &&
                chunk == null &&
                reports != null &&
                reports.size >= 2 &&
                coordinateMismatchCount == 0 &&
                validHoverPrefix &&
                validOrder,
        )
    }

    private fun parseSgrMouseReports(wire: ByteArray): List<SgrMouseReport>? {
        var index = 0
        val reports = mutableListOf<SgrMouseReport>()

        fun readUnsignedInteger(delimiter: Int): Int? {
            if (index >= wire.size || wire[index].toInt() !in '0'.code..'9'.code) return null
            var value = 0
            while (index < wire.size && wire[index].toInt() in '0'.code..'9'.code) {
                val digit = wire[index].toInt() - '0'.code
                if (value > (Int.MAX_VALUE - digit) / 10) return null
                value = value * 10 + digit
                index += 1
            }
            if (index >= wire.size || wire[index].toInt() != delimiter) return null
            index += 1
            return value
        }

        while (index < wire.size) {
            if (
                index + 3 > wire.size ||
                wire[index].toInt() != 0x1b ||
                wire[index + 1].toInt() != '['.code ||
                wire[index + 2].toInt() != '<'.code
            ) {
                return null
            }
            index += 3
            val button = readUnsignedInteger(';'.code) ?: return null
            val column = readUnsignedInteger(';'.code) ?: return null
            if (index >= wire.size || wire[index].toInt() !in '0'.code..'9'.code) return null
            var row = 0
            while (index < wire.size && wire[index].toInt() in '0'.code..'9'.code) {
                val digit = wire[index].toInt() - '0'.code
                if (row > (Int.MAX_VALUE - digit) / 10) return null
                row = row * 10 + digit
                index += 1
            }
            if (index >= wire.size) return null
            val suffix = wire[index].toInt()
            if (suffix != 'M'.code && suffix != 'm'.code) return null
            index += 1
            if (column <= 0 || row <= 0) return null
            reports += SgrMouseReport(
                button = button,
                column = column,
                row = row,
                release = suffix == 'm'.code,
            )
        }
        return reports.takeIf { it.isNotEmpty() }
    }

    private fun setClipboardBaseline(clipboard: ClipboardManager) {
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            clipboard.setPrimaryClip(
                ClipData.newPlainText("Terminal selection test baseline", "not selected text"),
            )
        }
    }

    private fun withRestoredClipboard(
        scenario: ActivityScenario<TerminalTestActivity>,
        block: (ClipboardManager) -> Unit,
    ) {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val clipboard = instrumentation.targetContext.getSystemService(ClipboardManager::class.java)
        awaitResumedActivityWindowFocus(scenario, "clipboard-preservation-capture")
        val preserved = onUi(scenario) { activity ->
            assertTrue(
                "case=clipboard-preservation-capture route=lifecycle foreground=false",
                activity.lifecycle.currentState == Lifecycle.State.RESUMED &&
                    activity.hasWindowFocus(),
            )
            val hadBefore = clipboard.hasPrimaryClip()
            val snapshot = clipboard.primaryClip?.let(::ClipData)
            val hadAfter = clipboard.hasPrimaryClip()
            assertTrue(
                "case=clipboard-preservation-capture route=clipboard stable=false",
                hadBefore == hadAfter && hadBefore == (snapshot != null),
            )
            if (snapshot == null) {
                PreservedClipboard.KnownEmpty
            } else {
                PreservedClipboard.KnownClip(snapshot)
            }
        }
        var primaryFailure: Throwable? = null
        try {
            block(clipboard)
        } catch (failure: Throwable) {
            primaryFailure = failure
        }
        val restorationFailure = runCatching {
            scenario.moveToState(Lifecycle.State.RESUMED)
            awaitResumedActivityWindowFocus(scenario, "clipboard-preservation-restore")
            // Re-setting the prior payload is best-effort: Android cannot restore
            // its timestamp, source attribution, classifier state, URI grants,
            // synchronization state, or any system UI already shown by this test.
            onUi(scenario) { activity ->
                assertTrue(
                    "case=clipboard-preservation-restore route=lifecycle foreground=false",
                    activity.lifecycle.currentState == Lifecycle.State.RESUMED &&
                        activity.hasWindowFocus(),
                )
                when (preserved) {
                    PreservedClipboard.KnownEmpty -> clipboard.clearPrimaryClip()
                    is PreservedClipboard.KnownClip ->
                        clipboard.setPrimaryClip(ClipData(preserved.snapshot))
                }
                val hadBefore = clipboard.hasPrimaryClip()
                val restored = clipboard.primaryClip?.let(::ClipData)
                val hadAfter = clipboard.hasPrimaryClip()
                val expectedPresent = preserved is PreservedClipboard.KnownClip
                assertTrue(
                    "case=clipboard-preservation-restore route=clipboard stable=false",
                    hadBefore == hadAfter &&
                        hadBefore == (restored != null) &&
                        hadBefore == expectedPresent,
                )
            }
        }.exceptionOrNull()
        val failure = primaryFailure
        if (failure != null) {
            restorationFailure?.let { failure.addSuppressed(it) }
            throw failure
        }
        restorationFailure?.let { throw it }
    }

    private fun awaitResumedActivityWindowFocus(
        scenario: ActivityScenario<TerminalTestActivity>,
        caseId: String,
    ) {
        scenario.moveToState(Lifecycle.State.RESUMED)
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            val foreground = onUi(scenario) { activity ->
                activity.lifecycle.currentState == Lifecycle.State.RESUMED &&
                    activity.hasWindowFocus()
            }
            if (foreground) return
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=lifecycle foreground=false")
    }

    private fun assertClipboardBaseline(clipboard: ClipboardManager, caseId: String) {
        var unchanged = false
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            val clip = clipboard.primaryClip
            unchanged = clip != null &&
                clip.itemCount == 1 &&
                clip.getItemAt(0).text == "not selected text"
        }
        assertTrue("case=$caseId route=clipboard baseline=false", unchanged)
    }

    private fun awaitExactSelectionClipboard(
        clipboard: ClipboardManager,
        expectedText: String,
        caseId: String,
    ) {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            var exact = false
            instrumentation.runOnMainSync {
                val clip = clipboard.primaryClip
                val description = clip?.description
                exact = clip != null &&
                    description != null &&
                    clip.itemCount == 1 &&
                    clip.getItemAt(0).text == expectedText &&
                    description.label?.toString() == "Terminal selection" &&
                    description.mimeTypeCount == 1 &&
                    description.getMimeType(0) == ClipDescription.MIMETYPE_TEXT_PLAIN &&
                    description.extras?.containsKey(ClipDescription.EXTRA_IS_REMOTE_DEVICE) == true &&
                    description.extras?.getBoolean(ClipDescription.EXTRA_IS_REMOTE_DEVICE) == true &&
                    description.extras?.containsKey(ClipDescription.EXTRA_IS_SENSITIVE) != true
            }
            if (exact) return
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=clipboard exact=false")
    }

    private fun awaitCounter(counter: AtomicInteger, expected: Int, caseId: String) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (counter.get() == expected) return
            Thread.sleep(50)
        }
        throw AssertionError(
            "case=$caseId route=platform-counter expected=$expected actual=${counter.get()}",
        )
    }

    private fun sendSystemBack() {
        assertTrue(
            "case=system-back route=accessibility action-rejected",
            InstrumentationRegistry.getInstrumentation().uiAutomation.performGlobalAction(
                AccessibilityService.GLOBAL_ACTION_BACK,
            ),
        )
    }

    private fun injectSelectionCopyAction(
        webView: WebView,
        action: AccessibilityNodeInfo,
        caseId: String,
    ) {
        assertTrue("case=$caseId route=action-mode phase=stale", action.refresh())
        val bounds = Rect()
        action.getBoundsInScreen(bounds)
        assertFalse("case=$caseId route=action-mode phase=empty-bounds", bounds.isEmpty)
        val displayId = requireNotNull(webView.display) {
            "case=$caseId route=action-mode phase=display-missing"
        }.displayId
        val point = TouchPoint(bounds.exactCenterX(), bounds.exactCenterY())
        val downTime = SystemClock.uptimeMillis()
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation

        fun inject(actionCode: Int, eventTime: Long, phase: String) {
            val properties = arrayOf(MotionEvent.PointerProperties().apply {
                id = 0
                toolType = MotionEvent.TOOL_TYPE_FINGER
            })
            val coordinates = arrayOf(MotionEvent.PointerCoords().apply {
                x = point.x
                y = point.y
                pressure = 1f
                size = 1f
            })
            val event = requireNotNull(MotionEvent.obtain(
                downTime,
                eventTime,
                actionCode,
                1,
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
            )) { "case=$caseId route=action-mode phase=$phase-create" }
            try {
                assertTrue(
                    "case=$caseId route=action-mode phase=$phase-rejected",
                    automation.injectInputEvent(event, true),
                )
            } finally {
                event.recycle()
            }
        }

        inject(MotionEvent.ACTION_DOWN, downTime, "down")
        SystemClock.sleep(16)
        inject(MotionEvent.ACTION_UP, maxOf(SystemClock.uptimeMillis(), downTime + 1), "up")
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
                        return node.src.indexOf('xterm-6.0.0-skidbladnir.js') >= 0;
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
            val value = accessiblePosition(webView, caseId)
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
            lastValue = accessiblePosition(webView, caseId)
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

    private fun assertAccessiblePositionRemains(
        webView: WebView,
        caseId: String,
        expected: Int,
    ) {
        val deadline = System.nanoTime() +
            TimeUnit.MILLISECONDS.toNanos(ACCESSIBILITY_STABILITY_MILLIS)
        var sample = 0
        while (System.nanoTime() < deadline) {
            assertEquals(
                "case=$caseId route=local stable-numeric-position sample=$sample",
                expected,
                accessiblePosition(webView, caseId),
            )
            sample += 1
            Thread.sleep(50)
        }
    }

    private fun accessiblePosition(webView: WebView, caseId: String): Int =
        evaluateSafely(
            webView,
            "Number(document.querySelector('.xterm-accessibility-tree [aria-posinset]')" +
                "?.getAttribute('aria-posinset') || '-1')",
            caseId,
        ).toIntOrNull() ?: -1

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

    private fun awaitNativeXtermSelection(
        webView: WebView,
        caseId: String,
        diagnostics: (() -> String)? = null,
    ) {
        awaitBooleanState(
            webView,
            "document.querySelector('.xterm-selection').childElementCount > 0 && " +
                "window.getSelection().isCollapsed",
            caseId,
            diagnostics,
        )
    }

    private fun terminalSelectionGeneration(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ): String {
        val generation = onUi(scenario) {
            val controllerField = LockedTerminalWebView::class.java
                .getDeclaredField("selectionController")
                .apply { isAccessible = true }
            val controller = controllerField.get(webView)
            val stateField = controller.javaClass.getDeclaredField("state")
                .apply { isAccessible = true }
            val state = stateField.get(controller)
            runCatching {
                state.javaClass.getDeclaredField("generation")
                    .apply { isAccessible = true }
                    .get(state) as? String
            }.getOrNull().orEmpty()
        }
        assertTrue("case=$caseId route=protocol generation-missing", generation.isNotEmpty())
        assertTrue(
            "case=$caseId route=protocol generation-noncanonical",
            generation.matches(Regex("[1-9][0-9]*")) &&
                generation.toLongOrNull() in 1..9_007_199_254_740_991L,
        )
        return generation
    }

    private fun assertSelectionAndCopyRemainAbsent(webView: WebView?, caseId: String) {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        repeat(5) { sample ->
            Thread.sleep(75)
            automation.clearCache()
            val actions = selectionCopyActions()
            assertNotNull("case=$caseId route=action-mode sample=$sample roots=missing", actions)
            assertEquals(
                "case=$caseId route=action-mode sample=$sample count",
                0,
                actions?.size,
            )
            if (webView != null) {
                assertEquals(
                    "case=$caseId route=webview sample=$sample selection-present",
                    "true",
                    evaluateSafely(
                        webView,
                        "document.querySelector('.xterm-selection').childElementCount === 0 && " +
                            "window.getSelection().isCollapsed",
                        "$caseId-$sample",
                    ),
                )
            }
        }
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

    private fun selectionCopyActions(): List<AccessibilityNodeInfo>? {
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val windows = instrumentation.uiAutomation.windows
        val rootedWindows = windows.mapNotNull { window -> window.root?.let { window to it } }
        if (rootedWindows.isEmpty()) return null
        val activityWindowIds = rootedWindows.mapNotNullTo(mutableSetOf()) { (window, root) ->
            val queue = ArrayDeque<AccessibilityNodeInfo>()
            queue.add(root)
            var containsWebView = false
            while (queue.isNotEmpty()) {
                val node = queue.removeFirst()
                containsWebView = containsWebView || node.className?.toString() == WebView::class.java.name
                for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
            }
            window.id.takeIf { containsWebView }
        }
        val actions = mutableListOf<AccessibilityNodeInfo>()
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        rootedWindows.filter { (window, _) ->
            window.type == AccessibilityWindowInfo.TYPE_APPLICATION &&
                window.id !in activityWindowIds
        }.forEach { (_, root) -> queue.addLast(root) }
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            if (node.isVisibleToUser &&
                node.isEnabled &&
                node.isClickable &&
                (node.text?.toString() == TERMINAL_SELECTION_COPY ||
                    node.contentDescription?.toString() == TERMINAL_SELECTION_COPY) &&
                node.actionList.any { it.id == AccessibilityNodeInfo.ACTION_CLICK }
            ) {
                actions.add(node)
            }
            for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
        }
        return actions
    }

    private fun awaitSelectionCopyAction(
        caseId: String,
        diagnostics: (() -> String)? = null,
    ): AccessibilityNodeInfo {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var observedCount = -1
        while (System.nanoTime() < deadline) {
            automation.clearCache()
            val actions = selectionCopyActions()
            if (actions != null) {
                observedCount = actions.size
                if (actions.size == 1) return actions.single()
            }
            Thread.sleep(50)
        }
        val diagnostic = diagnostics?.invoke()?.let { " $it" }.orEmpty()
        throw AssertionError(
            "case=$caseId route=action-mode expectedCount=1 actualCount=$observedCount$diagnostic",
        )
    }

    private fun awaitNoSelectionCopyAction(caseId: String) {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var observedCount = -1
        while (System.nanoTime() < deadline) {
            automation.clearCache()
            val actions = selectionCopyActions()
            if (actions != null) {
                observedCount = actions.size
                if (actions.isEmpty()) return
            }
            Thread.sleep(50)
        }
        throw AssertionError(
            "case=$caseId route=action-mode expectedCount=0 actualCount=$observedCount",
        )
    }

    private fun assertFloatingSelectionCopyWindow(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        copyAction: AccessibilityNodeInfo,
        caseId: String,
    ) {
        val activityWindowId = onUi(scenario) { webView.createAccessibilityNodeInfo().windowId }
        assertTrue("case=$caseId route=window activity-id", activityWindowId >= 0)
        assertTrue("case=$caseId route=window popup-id", copyAction.windowId >= 0)
        assertTrue(
            "case=$caseId route=window separate=false",
            copyAction.windowId != activityWindowId,
        )
        val popup = InstrumentationRegistry.getInstrumentation().uiAutomation.windows
            .singleOrNull { it.id == copyAction.windowId }
        assertNotNull("case=$caseId route=window popup-missing", popup)
        assertEquals(
            "case=$caseId route=window popup-type",
            AccessibilityWindowInfo.TYPE_APPLICATION,
            requireNotNull(popup).type,
        )
        val root = popup.root
        assertNotNull("case=$caseId route=window popup-root-missing", root)
        val queue = ArrayDeque<AccessibilityNodeInfo>()
        queue.add(requireNotNull(root))
        var containsCopyAction = false
        var containsWebView = false
        while (queue.isNotEmpty()) {
            val node = queue.removeFirst()
            containsCopyAction = containsCopyAction ||
                (node.windowId == copyAction.windowId &&
                    (node.text?.toString() == TERMINAL_SELECTION_COPY ||
                        node.contentDescription?.toString() == TERMINAL_SELECTION_COPY) &&
                    node.isVisibleToUser &&
                    node.isEnabled &&
                    node.isClickable)
            containsWebView = containsWebView || node.className?.toString() == WebView::class.java.name
            for (index in 0 until node.childCount) node.getChild(index)?.let(queue::addLast)
        }
        assertTrue("case=$caseId route=window popup-action-missing", containsCopyAction)
        assertFalse("case=$caseId route=window popup-contains-webview", containsWebView)
    }

    private fun assertSingleTooLargeToast(caseId: String, trigger: () -> Unit) {
        val automation = InstrumentationRegistry.getInstrumentation().uiAutomation
        val targetPackage = InstrumentationRegistry.getInstrumentation().targetContext.packageName
        val matchingCount = AtomicInteger()
        val candidateCount = AtomicInteger()
        val firstMatch = CountDownLatch(1)
        automation.setOnAccessibilityEventListener { event ->
            val isCandidate = event.eventType == AccessibilityEvent.TYPE_NOTIFICATION_STATE_CHANGED &&
                event.className?.toString() == "android.widget.Toast" &&
                event.packageName?.toString() == targetPackage
            if (isCandidate) {
                candidateCount.incrementAndGet()
                val hasExactText = event.text.size == 1 &&
                    event.text.single().toString() == TERMINAL_SELECTION_TOO_LARGE
                val hasNoParcelableData = event.parcelableData == null
                if (hasExactText && hasNoParcelableData) {
                    matchingCount.incrementAndGet()
                    firstMatch.countDown()
                }
            }
        }
        try {
            trigger()
            assertTrue(
                "case=$caseId route=toast first-match=false candidates=${candidateCount.get()}",
                firstMatch.await(5, TimeUnit.SECONDS),
            )
            assertEquals(
                "case=$caseId route=toast matching-count",
                1,
                matchingCount.get(),
            )
            Thread.sleep(350)
            assertEquals(
                "case=$caseId route=toast delayed-matching-count",
                1,
                matchingCount.get(),
            )
            assertEquals(
                "case=$caseId route=toast candidate-count",
                1,
                candidateCount.get(),
            )
        } finally {
            automation.setOnAccessibilityEventListener(null)
        }
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

    private fun injectTap(webView: WebView, point: TouchPoint, caseId: String) {
        NativeTouchStream(webView, caseId).use { stream ->
            stream.down(point)
            stream.up(point)
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

    private fun injectSelectionHoldDrag(
        webView: WebView,
        start: TouchPoint,
        end: TouchPoint,
        caseId: String,
        primaryPointerId: Int = 0,
    ) {
        NativeTouchStream(webView, caseId, primaryPointerId).use { stream ->
            stream.down(start)
            SystemClock.sleep(ViewConfiguration.getLongPressTimeout().toLong() + 250)
            stream.move(end)
            stream.up(end)
        }
    }

    private fun injectSelectionHoldDragWithDistinctEnd(
        webView: WebView,
        start: TouchPoint,
        penultimate: TouchPoint,
        end: TouchPoint,
        caseId: String,
    ) {
        NativeTouchStream(webView, caseId).use { stream ->
            stream.down(start)
            SystemClock.sleep(ViewConfiguration.getLongPressTimeout().toLong() + 250)
            stream.move(penultimate)
            stream.up(end)
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

    private fun awaitNativeWebViewWithoutFocus(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (!onUi(scenario) { webView.hasFocus() }) return
            Thread.sleep(50)
        }
        throw AssertionError("case=$caseId route=native focus=true")
    }

    private fun unfocusTerminal(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ) {
        assertEquals(
            "case=$caseId route=webview textarea-blur",
            "true",
            evaluateSafely(
                webView,
                "(document.querySelector('.xterm-helper-textarea').blur(), true)",
                "$caseId-textarea-blur",
            ),
        )
        onUi(scenario) { webView.clearFocus() }
        hideTerminalIme(scenario, webView, caseId)
        awaitNativeWebViewWithoutFocus(scenario, webView, "$caseId-native-focus")
        awaitBooleanState(
            webView,
            "document.activeElement !== document.querySelector('.xterm-helper-textarea')",
            "$caseId-textarea-focus",
        )
    }

    private fun hideTerminalIme(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ) {
        onUi(scenario) {
            requireNotNull(webView.windowInsetsController) {
                "case=$caseId route=native insets-controller=missing"
            }.hide(WindowInsets.Type.ime())
        }
        awaitTerminalIme(scenario, webView, visible = false, caseId = caseId)
    }

    private fun awaitTerminalIme(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        visible: Boolean,
        caseId: String,
    ) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        var lastState: Pair<Boolean?, Int?> = null to null
        while (System.nanoTime() < deadline) {
            lastState = onUi(scenario) {
                val insets = webView.rootWindowInsets
                insets?.isVisible(WindowInsets.Type.ime()) to
                    insets?.getInsets(WindowInsets.Type.ime())?.bottom
            }
            val hasExpectedState = lastState.first == visible &&
                (!visible || requireNotNull(lastState.second) > 0)
            if (hasExpectedState) return
            Thread.sleep(50)
        }
        throw AssertionError(
            "case=$caseId route=native expectedImeVisible=$visible " +
                "actualImeVisible=${lastState.first} actualImeBottom=${lastState.second}",
        )
    }

    private fun assertTerminalImeRemainsHidden(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        caseId: String,
    ) {
        val deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(IME_STABILITY_MILLIS)
        var observedInsets = false
        while (System.nanoTime() < deadline) {
            val state = onUi(scenario) {
                val insets = webView.rootWindowInsets
                insets?.isVisible(WindowInsets.Type.ime()) to
                    insets?.getInsets(WindowInsets.Type.ime())?.bottom
            }
            if (state.first != null) observedInsets = true
            if (state.first == true || (state.second ?: 0) > 0) {
                throw AssertionError(
                    "case=$caseId route=native expectedImeVisible=false " +
                        "actualImeVisible=${state.first} actualImeBottom=${state.second}",
                )
            }
            Thread.sleep(50)
        }
        assertTrue("case=$caseId route=native root-insets=missing", observedInsets)
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
        replaceNextPageMessage(webView, "ModifierState", replacement)
    }

    private fun replaceNextPageMessage(
        webView: WebView,
        kind: String,
        replacement: String,
    ) {
        evaluate(
            webView,
            """
            (function () {
                var original = MessagePort.prototype.postMessage;
                MessagePort.prototype.postMessage = function (value) {
                    var payload = JSON.parse(value);
                    if (payload.kind === ${JSONObject.quote(kind)}) {
                        MessagePort.prototype.postMessage = original;
                        var replaced = JSON.parse(${JSONObject.quote(replacement)});
                        if (Object.prototype.hasOwnProperty.call(payload, 'generation') &&
                            !Object.prototype.hasOwnProperty.call(replaced, 'generation')) {
                            replaced.generation = payload.generation;
                        }
                        return original.call(this, JSON.stringify(replaced));
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
