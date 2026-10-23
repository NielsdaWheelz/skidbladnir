package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.content.res.Configuration
import android.graphics.Bitmap
import android.graphics.Color
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.HapticFeedbackConstants
import android.view.MotionEvent
import android.view.ViewConfiguration
import android.view.ViewGroup
import android.view.WindowInsets
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.widget.Toast
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebMessagePortCompat
import androidx.webkit.WebResourceErrorCompat
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewClientCompat
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature
import androidx.core.net.toUri
import org.json.JSONException
import org.json.JSONObject

private const val LOCAL_ASSET_HOST = "appassets.androidplatform.net"
private const val TERMINAL_URL = "https://$LOCAL_ASSET_HOST/assets/terminal/index.html"
private const val MAXIMUM_PAGE_OUTPUT_BYTES = 1024 * 1024
private const val TERMINAL_PAGE_READY_TIMEOUT_MILLIS = 10_000L

internal interface TerminalPage {
    fun write(bytes: ByteArray)
    fun sendAccessory(accessory: TerminalAccessory)
    fun resetInputState()
    fun setTextSize(nominalSp: Int)
}

internal enum class TerminalAccessory {
    Escape,
    Slash,
    Shift,
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
    val shift: TerminalModifierPhase,
)

internal interface TerminalPageListener {
    fun onReady(page: TerminalPage)
    // xterm has written an output frame.
    fun onOutputApplied(page: TerminalPage)
    fun onInput(bytes: ByteArray)
    fun onResize(columns: Int, rows: Int)
    fun onViewportTooSmall()
    fun onModifiersChanged(modifiers: TerminalModifiers)
    fun onUnavailable()
}

@Suppress("DEPRECATION") // The deprecated file-URL switches remain explicitly disabled as defense in depth.
@SuppressLint(
    "RequiresFeature",
    "SetJavaScriptEnabled", // justify-override: JavaScript is required only for the packaged asset-only xterm runtime.
    "ViewConstructor", // Code-only construction requires the terminal listener.
)
// Main-thread only: every entry (view and WebViewClient callbacks, the port
// callback registered on the main looper, TerminalConnection's posted writes,
// controller calls) runs on the UI thread, so its state needs no lock.
internal class LockedTerminalWebView(
    context: Context,
    nominalTextSizeSp: Int,
    private val listener: TerminalPageListener,
) : WebView(context), TerminalPage {
    private val assetLoader = WebViewAssetLoader.Builder()
        .setDomain(LOCAL_ASSET_HOST)
        .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(context))
        .build()
    private val main = Handler(Looper.getMainLooper())
    private val selectionToolbar = TerminalSelectionToolbar(this) {
        postCommand(JSONObject().put("kind", "ClearSelection"))
    }
    private var pagePort: WebMessagePortCompat? = null
    private var ready = false
    private var unavailable = false
    private var disposed = false
    private var orientation = resources.configuration.orientation
    private var textSizeSp = nominalTextSizeSp
    // Output posted to the page that xterm has not yet written. Frames still
    // queued on a busy main looper count only once write runs.
    private var unappliedOutputBytes = 0
    private val pageReadinessDeadline = Runnable(::markUnavailable)
    private var activeTouchEvent: MotionEvent? = null
    private val live: Boolean get() = ready && !unavailable

    init {
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
        main.postDelayed(pageReadinessDeadline, TERMINAL_PAGE_READY_TIMEOUT_MILLIS)
        loadUrl(TERMINAL_URL)
    }

    override fun scrollTo(x: Int, y: Int) {
        super.scrollTo(0, 0)
    }

    override fun scrollBy(x: Int, y: Int) {
        super.scrollTo(0, 0)
    }

    // Each frame goes to the page at once; the page acknowledges it when xterm
    // has written it, and more than the budget unwritten fails the page.
    override fun write(bytes: ByteArray) {
        if (!live) return
        if (unappliedOutputBytes + bytes.size > MAXIMUM_PAGE_OUTPUT_BYTES) {
            markUnavailable()
            return
        }
        unappliedOutputBytes += bytes.size
        postToPage(WebMessageCompat(bytes))
    }

    override fun sendAccessory(accessory: TerminalAccessory) {
        postCommand(JSONObject().put("kind", "Accessory").put("key", accessory.name))
    }

    override fun resetInputState() {
        selectionToolbar.hide()
        postCommand(JSONObject().put("kind", "ResetInputState"))
    }

    override fun setTextSize(nominalSp: Int) {
        textSizeSp = nominalSp
        postCommand(JSONObject().put("kind", "FontSize").put("fontSizeCssPx", fontSizeCssPx()))
    }

    // Android text scaling is applied exactly once, here; the page keeps
    // textZoom at 100 and zoom disabled, so CSS px map 1:1 onto dp.
    private fun fontSizeCssPx(): Double {
        val metrics = resources.displayMetrics
        return TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_SP, textSizeSp.toFloat(), metrics)
            .toDouble() / metrics.density
    }

    override fun setEnabled(enabled: Boolean) {
        if (!enabled && isEnabled) {
            cancelActiveTouch()
            resetInputState()
        }
        super.setEnabled(enabled)
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

    override fun onWindowFocusChanged(hasWindowFocus: Boolean) {
        super.onWindowFocusChanged(hasWindowFocus)
        if (!hasWindowFocus) resetInputState()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        if (newConfig.orientation == orientation) return
        orientation = newConfig.orientation
        resetInputState()
    }

    override fun onSizeChanged(width: Int, height: Int, oldWidth: Int, oldHeight: Int) {
        super.onSizeChanged(width, height, oldWidth, oldHeight)
        selectionToolbar.contentRectChanged()
    }

    override fun onDetachedFromWindow() {
        markUnavailable()
        super.onDetachedFromWindow()
    }

    fun dispose() {
        if (disposed) return
        disposed = true
        cancelActiveTouch()
        closePage()
        stopLoading()
        clearHistory()
        (parent as? ViewGroup)?.removeView(this)
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
        ): Boolean = request.isForMainFrame && request.url.toString() != TERMINAL_URL

        override fun onPageFinished(view: WebView, url: String) {
            super.onPageFinished(view, url)
            if (url == TERMINAL_URL) attachPagePort(view)
        }

        override fun onPageStarted(view: WebView, url: String, favicon: Bitmap?) {
            super.onPageStarted(view, url, favicon)
            if (ready) markUnavailable()
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

    // The packaged page port (docs/architecture.md, page port, lists every
    // message). Commands and output go out through postToPage; page messages
    // are decoded here.
    private fun attachPagePort(view: WebView) {
        if (unavailable || pagePort != null) return
        val ports = WebViewCompat.createWebMessageChannel(view)
        val nativePort = ports[0]
        nativePort.setWebMessageCallback(
            main,
            object : WebMessagePortCompat.WebMessageCallbackCompat() {
                // Same-APK messages, read only for what native uses. A missing or mistyped field, an
                // unknown kind or phase, or a message native cannot parse (left to throw, it would
                // crash the app) ends the page.
                override fun onMessage(
                    port: WebMessagePortCompat,
                    message: WebMessageCompat?,
                ) {
                    if (unavailable) return
                    val data = message?.takeIf { it.type == WebMessageCompat.TYPE_STRING }?.data
                        ?: return markUnavailable()
                    val decoded = try {
                        JSONObject(data)
                    } catch (_: JSONException) {
                        return markUnavailable()
                    }
                    when (decoded.opt("kind")) {
                        "Ready" -> {
                            ready = true
                            main.removeCallbacks(pageReadinessDeadline)
                            listener.onReady(this@LockedTerminalWebView)
                        }
                        "Input" -> {
                            val value = decoded.opt("value") as? String ?: return markUnavailable()
                            listener.onInput(value.toByteArray(Charsets.UTF_8))
                        }
                        "Resize" -> {
                            val columns = decoded.opt("columns") as? Int ?: return markUnavailable()
                            val rows = decoded.opt("rows") as? Int ?: return markUnavailable()
                            listener.onResize(columns, rows)
                        }
                        "ViewportTooSmall" -> listener.onViewportTooSmall()
                        "OutputApplied" -> {
                            val bytes = decoded.opt("bytes") as? Int ?: return markUnavailable()
                            unappliedOutputBytes -= bytes
                            listener.onOutputApplied(this@LockedTerminalWebView)
                        }
                        "ModifierState" -> listener.onModifiersChanged(
                            TerminalModifiers(
                                control = decoded.modifierPhase("control") ?: return markUnavailable(),
                                alt = decoded.modifierPhase("alt") ?: return markUnavailable(),
                                shift = decoded.modifierPhase("shift") ?: return markUnavailable(),
                            ),
                        )
                        "ImeRequested" -> requestTerminalIme()
                        "SelectionStarted" -> performHapticFeedback(HapticFeedbackConstants.LONG_PRESS)
                        "SelectionAvailable" -> {
                            val anchorX = (decoded.opt("anchorX") as? Number)?.toDouble() ?: return markUnavailable()
                            val anchorY = (decoded.opt("anchorY") as? Number)?.toDouble() ?: return markUnavailable()
                            val text = decoded.opt("text") as? String ?: return markUnavailable()
                            if (isEnabled && hasWindowFocus()) {
                                selectionToolbar.show(text, anchorX, anchorY)
                            } else {
                                postCommand(JSONObject().put("kind", "ClearSelection"))
                            }
                        }
                        "SelectionCleared" -> selectionToolbar.hide()
                        "SelectionTooLarge" ->
                            Toast.makeText(context, R.string.terminal_selection_too_large, Toast.LENGTH_SHORT).show()
                        else -> markUnavailable() // PageFailure and anything unknown
                    }
                }
            },
        )
        pagePort = nativePort
        WebViewCompat.postWebMessage(
            view,
            WebMessageCompat(
                JSONObject()
                    .put("longPressMilliseconds", ViewConfiguration.getLongPressTimeout())
                    .put("fontSizeCssPx", fontSizeCssPx())
                    .toString(),
                arrayOf(ports[1]),
            ),
            "https://$LOCAL_ASSET_HOST".toUri(),
        )
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
            WebViewFeature.WEB_MESSAGE_ARRAY_BUFFER,
        )
        check(required.all(WebViewFeature::isFeatureSupported)) // justify-service-invariant-check: the bearer-free native terminal boundary cannot be implemented without every message-port operation.
    }

    private fun postCommand(command: JSONObject) {
        postToPage(WebMessageCompat(command.toString()))
    }

    private fun postToPage(message: WebMessageCompat) {
        if (!live) return
        val port = checkNotNull(pagePort) // every path that clears the port sets `unavailable` first
        try {
            port.postMessage(message)
        } catch (_: IllegalStateException) {
            markUnavailable()
        }
    }

    // The IME rises for a page tap only while the terminal can take input.
    private fun requestTerminalIme() {
        if (live && isEnabled && hasWindowFocus() && isShown && requestFocus()) {
            windowInsetsController?.show(WindowInsets.Type.ime())
        }
    }

    private fun markUnavailable() {
        if (unavailable) return
        closePage()
        listener.onUnavailable()
    }

    // Ends the page for good; idempotent.
    private fun closePage() {
        unavailable = true
        main.removeCallbacks(pageReadinessDeadline)
        selectionToolbar.hide()
        pagePort?.close()
        pagePort = null
    }

    private fun JSONObject.modifierPhase(name: String): TerminalModifierPhase? =
        TerminalModifierPhase.entries.firstOrNull { it.name == opt(name) }

    private fun WebResourceRequest.isTerminalAsset(): Boolean =
        url.scheme == "https" && url.host == LOCAL_ASSET_HOST && url.path?.startsWith("/assets/terminal/") == true
}
