package dev.niels.skidbladnir

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Color
import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.ViewGroup
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.AndroidView
import androidx.webkit.WebMessageCompat
import androidx.webkit.WebMessagePortCompat
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewCompat
import org.json.JSONObject

private const val LOCAL_ASSET_HOST = "appassets.androidplatform.net"
private const val TERMINAL_URL = "https://$LOCAL_ASSET_HOST/assets/terminal/index.html"

class MainActivity : ComponentActivity() {
    internal lateinit var terminalWebView: LockedTerminalWebView
        private set

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            MaterialTheme {
                TerminalHarnessScreen { view -> terminalWebView = view }
            }
        }
    }
}

@Composable
private fun TerminalHarnessScreen(onViewReady: (LockedTerminalWebView) -> Unit) {
    AndroidView(
        modifier = Modifier.fillMaxSize(),
        factory = { context ->
            LockedTerminalWebView(context).also(onViewReady)
        },
        update = { view -> onViewReady(view) },
    )
}

@SuppressLint("SetJavaScriptEnabled")
internal class LockedTerminalWebView(context: Context) : WebView(context) {
    private val assetLoader = WebViewAssetLoader.Builder()
        .setDomain(LOCAL_ASSET_HOST)
        .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(context))
        .build()
    private var bridgePort: WebMessagePortCompat? = null

    init {
        setBackgroundColor(Color.rgb(16, 17, 20))
        isFocusable = true
        isFocusableInTouchMode = true
        layoutParams = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT,
        )
        configureSettings()
        webViewClient = localAssetClient()
        loadUrl(TERMINAL_URL)
    }

    private fun configureSettings() {
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
            mediaPlaybackRequiresUserGesture = true
        }
    }

    private fun localAssetClient(): WebViewClient {
        return object : WebViewClient() {
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest,
            ): WebResourceResponse? {
                val uri = request.url
                if (uri.scheme != "https" || uri.host != LOCAL_ASSET_HOST) {
                    return blockedResponse()
                }
                return assetLoader.shouldInterceptRequest(uri) ?: blockedResponse()
            }

            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest,
            ): Boolean = request.url.host != LOCAL_ASSET_HOST

            override fun onPageFinished(view: WebView, url: String) {
                super.onPageFinished(view, url)
                if (url == TERMINAL_URL) attachNativePort(view)
            }
        }
    }

    private fun attachNativePort(view: WebView) {
        bridgePort?.close()
        val ports = WebViewCompat.createWebMessageChannel(view)
        val nativePort = ports[0]
        val pagePort = ports[1]
        nativePort.setWebMessageCallback(
            Handler(Looper.getMainLooper()),
            object : WebMessagePortCompat.WebMessageCallbackCompat() {
                override fun onMessage(
                    port: WebMessagePortCompat,
                    message: WebMessageCompat?,
                ) {
                    val payload = message?.data ?: return
                    val kind = JSONObject(payload).optString("kind")
                    if (kind !in setOf("ready", "input", "resize")) return
                    port.postMessage(
                        WebMessageCompat(
                            JSONObject()
                                .put("kind", "ack")
                                .put("for", kind)
                                .toString(),
                        ),
                    )
                }
            },
        )
        bridgePort = nativePort
        WebViewCompat.postWebMessage(
            view,
            WebMessageCompat("{\"kind\":\"bridge\",\"version\":1}", arrayOf(pagePort)),
            Uri.parse("https://$LOCAL_ASSET_HOST"),
        )
    }

    override fun onDetachedFromWindow() {
        bridgePort?.close()
        bridgePort = null
        stopLoading()
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
}
