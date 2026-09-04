package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.content.res.Configuration
import android.graphics.Bitmap
import android.graphics.Color
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Base64
import android.view.MotionEvent
import android.view.ViewGroup
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityNodeProvider
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebMessagePortCompat
import androidx.webkit.WebResourceErrorCompat
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewClientCompat
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import androidx.core.net.toUri
import java.util.ArrayDeque
import org.json.JSONException
import org.json.JSONObject

private const val LOCAL_ASSET_HOST = "appassets.androidplatform.net"
private const val TERMINAL_URL = "https://$LOCAL_ASSET_HOST/assets/terminal/index.html"
private const val MAXIMUM_PAGE_OUTPUT_BYTES = 1024 * 1024L
private const val MAXIMUM_PAGE_INPUT_BYTES = 1024 * 1024

// The gateway's published geometry bounds; the page's glyph scaling guarantees
// at least 80 columns, so geometry outside these bounds is a page defect.
private const val MINIMUM_COLUMNS = 20
private const val MAXIMUM_COLUMNS = 240
private const val MINIMUM_ROWS = 5
private const val MAXIMUM_ROWS = 120
private const val TERMINAL_PAGE_READY_TIMEOUT_MILLIS = 10_000L

private data class PendingPageOutput(
    val sequence: String,
    val bytes: ByteArray,
)

private enum class PageOutputAdvance {
    Complete,
    Continue,
    Invalid,
}

private enum class TerminalScrollDirection {
    Backward,
    Forward,
}

private sealed interface TerminalPageCommand {
    data object Focus : TerminalPageCommand
    data class Accessory(val accessory: TerminalAccessory) : TerminalPageCommand
    data object ResetInputState : TerminalPageCommand
    data class Scroll(val direction: TerminalScrollDirection) : TerminalPageCommand
}

internal interface TerminalPage {
    fun write(bytes: ByteArray)
    fun focus()
    fun sendAccessory(accessory: TerminalAccessory)
    fun resetInputState()
}

internal enum class TerminalAccessory {
    Escape,
    Slash,
    Hyphen,
    Home,
    Up,
    End,
    PageUp,
    Tab,
    Control,
    Alt,
    Left,
    Down,
    Right,
    PageDown,
}

internal enum class TerminalModifierPhase {
    Off,
    Armed,
}

internal data class TerminalModifiers(
    val control: TerminalModifierPhase,
    val alt: TerminalModifierPhase,
)

internal interface TerminalPageListener {
    fun onReady(page: TerminalPage)
    fun onInput(bytes: ByteArray)
    fun onResize(columns: Int, rows: Int)
    fun onModifiersChanged(modifiers: TerminalModifiers)
    fun onUnavailable()
}

@Suppress("DEPRECATION") // The deprecated file-URL switches remain explicitly disabled as defense in depth.
@SuppressLint(
    "RequiresFeature",
    "SetJavaScriptEnabled", // justify-override: JavaScript is required only for the packaged asset-only xterm runtime.
    "ViewConstructor", // Code-only construction requires the terminal listener.
)
internal class LockedTerminalWebView(
    context: Context,
    private val listener: TerminalPageListener,
    private val initialUrl: String = TERMINAL_URL,
    readinessTimeoutMillis: Long = TERMINAL_PAGE_READY_TIMEOUT_MILLIS,
) : WebView(context), TerminalPage {
    private val assetLoader = WebViewAssetLoader.Builder()
        .setDomain(LOCAL_ASSET_HOST)
        .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(context))
        .build()
    private val main = Handler(Looper.getMainLooper())
    private var pagePort: WebMessagePortCompat? = null
    private var disposed = false
    private var unavailable = false
    private var pageReady = false
    private var orientation = resources.configuration.orientation
    private val pageReadinessDeadline = Runnable(::markUnavailable)
    private val outputMonitor = Any()
    private val pendingOutput = ArrayDeque<PendingPageOutput>()
    private var pendingOutputBytes = 0L
    private var outputInFlight = false
    private var nextOutputSequence = 1L
    private var accessibilityDelegate: AccessibilityNodeProvider? = null
    private var accessibilityWrapper: AccessibilityNodeProvider? = null
    private var accessibilityActionsWereAvailable = false
    private var activeTouchEvent: MotionEvent? = null
    private val backwardWheelAction by lazy {
        AccessibilityNodeInfo.AccessibilityAction(
            R.id.terminal_wheel_backward,
            context.getString(R.string.terminal_wheel_backward_label),
        )
    }
    private val forwardWheelAction by lazy {
        AccessibilityNodeInfo.AccessibilityAction(
            R.id.terminal_wheel_forward,
            context.getString(R.string.terminal_wheel_forward_label),
        )
    }

    init {
        require(initialUrl.startsWith("https://$LOCAL_ASSET_HOST/assets/terminal/"))
        require(readinessTimeoutMillis in 1..TERMINAL_PAGE_READY_TIMEOUT_MILLIS)
        requireWebMessagePort()
        setBackgroundColor(Color.rgb(12, 13, 15))
        isFocusable = true
        isFocusableInTouchMode = true
        isHorizontalScrollBarEnabled = false
        isVerticalScrollBarEnabled = false
        overScrollMode = OVER_SCROLL_NEVER
        layoutParams = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT,
        )
        settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = false
            databaseEnabled = false
            allowFileAccess = false
            allowContentAccess = false
            allowFileAccessFromFileURLs = false
            allowUniversalAccessFromFileURLs = false
            blockNetworkLoads = true
            blockNetworkImage = true
            mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            setSupportMultipleWindows(false)
            javaScriptCanOpenWindowsAutomatically = false
            builtInZoomControls = false
            displayZoomControls = false
            setSupportZoom(false)
            useWideViewPort = false
            loadWithOverviewMode = false
            textZoom = 100
            mediaPlaybackRequiresUserGesture = true
        }
        webViewClient = localAssetClient()
        main.postDelayed(pageReadinessDeadline, readinessTimeoutMillis)
        loadUrl(initialUrl)
    }

    override fun scrollTo(x: Int, y: Int) {
        super.scrollTo(0, 0)
    }

    override fun scrollBy(x: Int, y: Int) {
        super.scrollTo(0, 0)
    }

    override fun write(bytes: ByteArray) {
        val shouldDrain = synchronized(outputMonitor) {
            if (disposed || unavailable) return
            if (pendingOutputBytes + bytes.size > MAXIMUM_PAGE_OUTPUT_BYTES) {
                clearOutputLocked()
                unavailable = true
                main.post(::reportUnavailable)
                return
            }
            pendingOutput.addLast(PendingPageOutput((nextOutputSequence++).toString(), bytes))
            pendingOutputBytes += bytes.size
            if (outputInFlight) {
                false
            } else {
                outputInFlight = true
                true
            }
        }
        if (shouldDrain) post(::sendNextOutput)
    }

    override fun focus() {
        sendPageCommand(TerminalPageCommand.Focus)
    }

    override fun sendAccessory(accessory: TerminalAccessory) {
        sendPageCommand(TerminalPageCommand.Accessory(accessory))
    }

    override fun resetInputState() {
        sendPageCommand(TerminalPageCommand.ResetInputState)
    }

    override fun setEnabled(enabled: Boolean) {
        val changed = enabled != isEnabled
        if (changed && !enabled) {
            cancelActiveTouch()
            if (pageIsLive()) sendPageCommand(TerminalPageCommand.ResetInputState)
        }
        super.setEnabled(enabled)
        if (changed) refreshAccessibilityActionAvailability()
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (!isEnabled) return false
        val handled = super.onTouchEvent(event)
        when (event.actionMasked) {
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> clearActiveTouch()
            MotionEvent.ACTION_DOWN -> if (handled) retainActiveTouch(event)
            MotionEvent.ACTION_MOVE,
            MotionEvent.ACTION_POINTER_DOWN,
            MotionEvent.ACTION_POINTER_UP,
            -> if (activeTouchEvent != null) retainActiveTouch(event)
        }
        return handled
    }

    override fun getAccessibilityNodeProvider(): AccessibilityNodeProvider? {
        val delegate = super.getAccessibilityNodeProvider()
        if (delegate == null) {
            accessibilityDelegate = null
            accessibilityWrapper = null
            return null
        }
        if (delegate !== accessibilityDelegate) {
            accessibilityDelegate = delegate
            accessibilityWrapper = TerminalAccessibilityNodeProvider(delegate)
        }
        return accessibilityWrapper
    }

    override fun onWindowFocusChanged(hasWindowFocus: Boolean) {
        super.onWindowFocusChanged(hasWindowFocus)
        if (!hasWindowFocus && synchronized(outputMonitor) { pageReady && !disposed && !unavailable }) {
            resetInputState()
        }
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        val previousOrientation = orientation
        val nextOrientation = newConfig.orientation
        val previousOrientationWasKnown = previousOrientation == Configuration.ORIENTATION_PORTRAIT ||
            previousOrientation == Configuration.ORIENTATION_LANDSCAPE
        val nextOrientationIsKnown = nextOrientation == Configuration.ORIENTATION_PORTRAIT ||
            nextOrientation == Configuration.ORIENTATION_LANDSCAPE
        val rotated = previousOrientation != nextOrientation &&
            previousOrientationWasKnown &&
            nextOrientationIsKnown
        orientation = nextOrientation
        if (rotated && synchronized(outputMonitor) { pageReady && !disposed && !unavailable }) {
            resetInputState()
        }
    }

    fun dispose() {
        main.removeCallbacks(pageReadinessDeadline)
        cancelActiveTouch()
        if (pageIsLive()) sendPageCommand(TerminalPageCommand.ResetInputState)
        synchronized(outputMonitor) {
            if (disposed) return
            disposed = true
            clearOutputLocked()
            unavailable = true
        }
        refreshAccessibilityActionAvailability()
        pagePort?.close()
        pagePort = null
        stopLoading()
        clearHistory()
        (parent as? android.view.ViewGroup)?.removeView(this)
        removeAllViews()
        destroy()
    }

    private fun retainActiveTouch(event: MotionEvent) {
        activeTouchEvent?.recycle()
        activeTouchEvent = MotionEvent.obtain(event)
    }

    private fun clearActiveTouch() {
        activeTouchEvent?.recycle()
        activeTouchEvent = null
    }

    private fun cancelActiveTouch() {
        val active = activeTouchEvent ?: return
        activeTouchEvent = null
        val cancellation = MotionEvent.obtain(active)
        active.recycle()
        cancellation.action = MotionEvent.ACTION_CANCEL
        try {
            super.onTouchEvent(cancellation)
        } finally {
            cancellation.recycle()
        }
    }

    @SuppressLint("MissingOnRenderProcessGone") // The detector does not recognize the WebViewClientCompat override directly below.
    private fun localAssetClient(): WebViewClientCompat = object : WebViewClientCompat() {
        override fun shouldInterceptRequest(
            view: WebView,
            request: WebResourceRequest,
        ): WebResourceResponse? {
            val uri = request.url
            if (uri.scheme != "https" || uri.host != LOCAL_ASSET_HOST) return blockedResponse()
            return assetLoader.shouldInterceptRequest(uri) ?: blockedResponse()
        }

        override fun shouldOverrideUrlLoading(
            view: WebView,
            request: WebResourceRequest,
        ): Boolean = request.isForMainFrame && request.url.toString() != initialUrl

        override fun onPageFinished(view: WebView, url: String) {
            super.onPageFinished(view, url)
            if (url == initialUrl) attachPagePort(view)
        }

        override fun onPageStarted(view: WebView, url: String, favicon: Bitmap?) {
            super.onPageStarted(view, url, favicon)
            if (synchronized(outputMonitor) { pageReady && !disposed && !unavailable }) {
                markUnavailable()
            }
        }

        override fun onReceivedError(
            view: WebView,
            request: WebResourceRequest,
            error: WebResourceErrorCompat,
        ) {
            super.onReceivedError(view, request, error)
            if (request.isTerminalAsset()) markUnavailable()
        }

        override fun onReceivedHttpError(
            view: WebView,
            request: WebResourceRequest,
            errorResponse: WebResourceResponse,
        ) {
            super.onReceivedHttpError(view, request, errorResponse)
            if (request.isTerminalAsset()) markUnavailable()
        }

        override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
            markUnavailable()
            post(::dispose)
            return true
        }
    }

    private fun attachPagePort(view: WebView) {
        if (synchronized(outputMonitor) { disposed || unavailable }) return
        if (pagePort != null) return
        val ports = WebViewCompat.createWebMessageChannel(view)
        val nativePort = ports[0]
        nativePort.setWebMessageCallback(
            main,
            object : WebMessagePortCompat.WebMessageCallbackCompat() {
                override fun onMessage(
                    port: WebMessagePortCompat,
                    message: WebMessageCompat?,
                ) {
                    if (synchronized(outputMonitor) { disposed || unavailable }) return
                    val payload = message?.data
                    if (payload == null) {
                        markUnavailable()
                        return
                    }
                    val objectValue = try {
                        JSONObject(payload)
                    } catch (_: JSONException) {
                        markUnavailable()
                        return
                    }
                    when (objectValue.stringField("kind")) {
                        "Ready" -> if (objectValue.hasExactKeys("kind")) {
                            val firstReady = synchronized(outputMonitor) {
                                if (disposed || unavailable || pageReady) {
                                    false
                                } else {
                                    pageReady = true
                                    true
                                }
                            }
                            if (firstReady) {
                                main.removeCallbacks(pageReadinessDeadline)
                                refreshAccessibilityActionAvailability()
                                listener.onReady(this@LockedTerminalWebView)
                            } else {
                                markUnavailable()
                            }
                        } else {
                            markUnavailable()
                        }
                        "Input" -> if (objectValue.hasExactKeys("kind", "value")) {
                            val value = objectValue.stringField("value")
                            val byteCount = value?.utf8ByteCountWithin(MAXIMUM_PAGE_INPUT_BYTES)
                            if (value == null || byteCount == null) {
                                markUnavailable()
                            } else {
                                listener.onInput(value.toByteArray(Charsets.UTF_8))
                            }
                        } else {
                            markUnavailable()
                        }
                        "Resize" -> if (objectValue.hasExactKeys("kind", "columns", "rows")) {
                            val columns = objectValue.intField("columns")
                            val rows = objectValue.intField("rows")
                            if (columns == null || rows == null ||
                                columns !in MINIMUM_COLUMNS..MAXIMUM_COLUMNS ||
                                rows !in MINIMUM_ROWS..MAXIMUM_ROWS
                            ) {
                                markUnavailable()
                            } else {
                                listener.onResize(columns, rows)
                            }
                        } else {
                            markUnavailable()
                        }
                        "OutputApplied" -> if (objectValue.hasExactKeys("kind", "sequence")) {
                            val sequence = objectValue.stringField("sequence")
                            if (sequence == null) markUnavailable() else outputApplied(sequence)
                        } else {
                            markUnavailable()
                        }
                        "ModifierState" -> if (objectValue.hasExactKeys("kind", "control", "alt")) {
                            val control = objectValue.modifierPhase("control")
                            val alt = objectValue.modifierPhase("alt")
                            if (control == null || alt == null) {
                                markUnavailable()
                            } else {
                                listener.onModifiersChanged(TerminalModifiers(control = control, alt = alt))
                            }
                        } else {
                            markUnavailable()
                        }
                        "PageFailure" -> markUnavailable()
                        else -> markUnavailable()
                    }
                }
            },
        )
        pagePort = nativePort
        WebViewCompat.postWebMessage(
            view,
            WebMessageCompat("{\"kind\":\"PagePort\",\"version\":1}", arrayOf(ports[1])),
            "https://$LOCAL_ASSET_HOST".toUri(),
        )
    }

    override fun onDetachedFromWindow() {
        if (!disposed) markUnavailable()
        pagePort?.close()
        pagePort = null
        super.onDetachedFromWindow()
    }

    private fun blockedResponse(): WebResourceResponse =
        WebResourceResponse(
            "text/plain",
            "UTF-8",
            403,
            "Forbidden",
            mapOf("Cache-Control" to "no-store"),
            "".byteInputStream(),
        )

    private fun requireWebMessagePort() {
        val required = listOf(
            WebViewFeature.CREATE_WEB_MESSAGE_CHANNEL,
            WebViewFeature.POST_WEB_MESSAGE,
            WebViewFeature.WEB_MESSAGE_PORT_POST_MESSAGE,
            WebViewFeature.WEB_MESSAGE_PORT_SET_MESSAGE_CALLBACK,
            WebViewFeature.WEB_MESSAGE_PORT_CLOSE,
        )
        check(required.all(WebViewFeature::isFeatureSupported)) // justify-service-invariant-check: the bearer-free native terminal boundary cannot be implemented without every message-port operation.
    }

    private fun sendPageCommand(command: TerminalPageCommand): Boolean {
        if (Looper.myLooper() == Looper.getMainLooper()) return sendPageCommandNow(command)
        if (!pageCommandAllowed(command)) return false
        return post { sendPageCommandNow(command) }
    }

    private fun sendPageCommandNow(command: TerminalPageCommand): Boolean {
        if (!pageCommandAllowed(command)) return false
        val port = pagePort
        if (port == null) {
            markUnavailable()
            return false
        }
        return try {
            port.postMessage(WebMessageCompat(command.toPayload()))
            true
        } catch (_: IllegalStateException) {
            markUnavailable()
            false
        }
    }

    private fun pageCommandAllowed(command: TerminalPageCommand): Boolean =
        pageIsLive() && (command !is TerminalPageCommand.Scroll || isEnabled)

    private fun pageIsLive(): Boolean = synchronized(outputMonitor) {
        pageReady && !disposed && !unavailable
    }

    private fun TerminalPageCommand.toPayload(): String = when (this) {
        TerminalPageCommand.Focus -> "{\"kind\":\"Focus\"}"
        is TerminalPageCommand.Accessory -> JSONObject()
            .put("kind", "Accessory")
            .put("key", accessory.name)
            .toString()
        TerminalPageCommand.ResetInputState -> "{\"kind\":\"ResetInputState\"}"
        is TerminalPageCommand.Scroll -> JSONObject()
            .put("kind", "Scroll")
            .put("direction", direction.name)
            .toString()
    }

    private fun accessibilityActionsAvailable(): Boolean = pageIsLive() && isEnabled

    private fun refreshAccessibilityActionAvailability() {
        val available = accessibilityActionsAvailable()
        if (available == accessibilityActionsWereAvailable) return
        accessibilityActionsWereAvailable = available
        parent?.notifySubtreeAccessibilityStateChanged(
            this,
            this,
            AccessibilityEvent.CONTENT_CHANGE_TYPE_SUBTREE,
        )
    }

    private inner class TerminalAccessibilityNodeProvider(
        private val delegate: AccessibilityNodeProvider,
    ) : AccessibilityNodeProvider() {
        override fun createAccessibilityNodeInfo(virtualViewId: Int): AccessibilityNodeInfo? =
            delegate.createAccessibilityNodeInfo(virtualViewId)?.also { node ->
                decorate(node, virtualViewId)
            }

        override fun findAccessibilityNodeInfosByText(
            text: String?,
            virtualViewId: Int,
        ): MutableList<AccessibilityNodeInfo>? =
            delegate.findAccessibilityNodeInfosByText(text, virtualViewId)?.also { nodes ->
                nodes.forEach { node -> decorate(node) }
            }

        override fun findFocus(focus: Int): AccessibilityNodeInfo? =
            delegate.findFocus(focus)?.also { node ->
                decorate(node)
            }

        override fun performAction(
            virtualViewId: Int,
            action: Int,
            arguments: Bundle?,
        ): Boolean {
            val direction = when (action) {
                R.id.terminal_wheel_backward -> TerminalScrollDirection.Backward
                R.id.terminal_wheel_forward -> TerminalScrollDirection.Forward
                else -> return delegate.performAction(virtualViewId, action, arguments)
            }
            val current = delegate.createAccessibilityNodeInfo(virtualViewId)
            if (current == null || !isTerminalRowNode(current, virtualViewId)) {
                return delegate.performAction(virtualViewId, action, arguments)
            }
            if (!accessibilityActionsAvailable()) return false
            if (!current.isAccessibilityFocused) return false
            return sendPageCommand(TerminalPageCommand.Scroll(direction))
        }

        override fun addExtraDataToAccessibilityNodeInfo(
            virtualViewId: Int,
            info: AccessibilityNodeInfo,
            extraDataKey: String,
            arguments: Bundle?,
        ) {
            delegate.addExtraDataToAccessibilityNodeInfo(
                virtualViewId,
                info,
                extraDataKey,
                arguments,
            )
        }

        private fun decorate(
            node: AccessibilityNodeInfo,
            virtualViewId: Int? = null,
        ) {
            if (!isTerminalRowNode(node, virtualViewId) ||
                !node.isAccessibilityFocused ||
                !accessibilityActionsAvailable()
            ) return
            if (node.actionList.none { it.id == backwardWheelAction.id }) {
                node.addAction(backwardWheelAction)
            }
            if (node.actionList.none { it.id == forwardWheelAction.id }) {
                node.addAction(forwardWheelAction)
            }
        }

        private fun isTerminalRowNode(
            node: AccessibilityNodeInfo,
            virtualViewId: Int? = null,
        ): Boolean {
            // Chromium's row nodes carry CollectionItemInfo. Provider-returned
            // nodes are still mutable here, so Android's public parent query is
            // unavailable until the framework seals the node; instrumentation
            // also requires its delivered parent to carry CollectionInfo.
            val item = node.collectionItemInfo ?: return false
            return virtualViewId != HOST_VIEW_ID &&
                node.packageName?.toString() == context.packageName &&
                node.className?.toString() != WebView::class.java.name &&
                item.rowSpan > 0 &&
                item.columnSpan > 0
        }
    }

    private fun sendNextOutput() {
        val output = synchronized(outputMonitor) {
            if (disposed || unavailable) return
            pendingOutput.firstOrNull() ?: run {
                outputInFlight = false
                return
            }
        }
        val port = pagePort
        if (port == null) {
            markUnavailable()
            return
        }
        try {
            port.postMessage(
                WebMessageCompat(
                    JSONObject()
                        .put("kind", "Output")
                        .put("sequence", output.sequence)
                        .put("data", Base64.encodeToString(output.bytes, Base64.NO_WRAP))
                        .toString(),
                ),
            )
        } catch (_: IllegalStateException) {
            markUnavailable()
        }
    }

    private fun outputApplied(sequence: String) {
        val advance = synchronized(outputMonitor) {
            if (disposed || unavailable) return
            val applied = pendingOutput.firstOrNull()
            if (applied?.sequence != sequence) {
                PageOutputAdvance.Invalid
            } else {
                pendingOutput.removeFirst()
                pendingOutputBytes -= applied.bytes.size
                if (pendingOutput.isEmpty()) {
                    outputInFlight = false
                    PageOutputAdvance.Complete
                } else {
                    PageOutputAdvance.Continue
                }
            }
        }
        when (advance) {
            PageOutputAdvance.Complete -> Unit
            PageOutputAdvance.Continue -> post(::sendNextOutput)
            PageOutputAdvance.Invalid -> markUnavailable()
        }
    }

    private fun markUnavailable() {
        main.removeCallbacks(pageReadinessDeadline)
        val shouldReport = synchronized(outputMonitor) {
            if (disposed || unavailable) {
                false
            } else {
                unavailable = true
                clearOutputLocked()
                true
            }
        }
        if (shouldReport) main.post(::reportUnavailable)
    }

    private fun reportUnavailable() {
        refreshAccessibilityActionAvailability()
        if (disposed) return
        pagePort?.close()
        pagePort = null
        listener.onUnavailable()
    }

    private fun clearOutputLocked() {
        pendingOutput.clear()
        pendingOutputBytes = 0
        outputInFlight = false
    }

    private fun JSONObject.hasExactKeys(vararg expected: String): Boolean {
        val actual = buildSet {
            val iterator = keys()
            while (iterator.hasNext()) add(iterator.next())
        }
        return actual == expected.toSet()
    }

    private fun JSONObject.stringField(name: String): String? = opt(name) as? String

    private fun JSONObject.modifierPhase(name: String): TerminalModifierPhase? =
        stringField(name)?.let { value -> TerminalModifierPhase.entries.singleOrNull { it.name == value } }

    private fun JSONObject.intField(name: String): Int? = opt(name) as? Int

    private fun WebResourceRequest.isTerminalAsset(): Boolean =
        url.scheme == "https" && url.host == LOCAL_ASSET_HOST && url.path?.startsWith("/assets/terminal/") == true

}
