package dev.niels.skidbladnir

import android.content.pm.ActivityInfo
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.json.JSONTokener
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TerminalHarnessInstrumentedTest {
    @Test
    fun localTerminalAssetRendersAndReportsViewport() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            assertEquals(
                "https://appassets.androidplatform.net/assets/terminal/index.html",
                onUi(scenario) { webView.url },
            )
            assertEquals("Skíðblaðnir terminal platform harness", evaluate(webView, "document.title"))
            assertEquals("ready", evaluate(webView, "window.__skidbladnirHarness.state"))
            assertTrue(evaluate(webView, "window.__skidbladnirHarness.ansiUnicode") == "PASS")
            assertEquals("true", evaluate(webView, "window.__skidbladnirHarness.actualInputElement"))
            assertEquals("true", evaluate(webView, "window.__skidbladnirHarness.screenReaderMode"))
            assertEquals("1", evaluate(webView, "document.querySelectorAll('.xterm-helper-textarea').length"))
            assertEquals(
                "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'none'; connect-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
                evaluate(webView, "document.querySelector('meta[http-equiv=\\\"Content-Security-Policy\\\"]').content"),
            )

            evaluate(webView, "window.__skidbladnirHarness.resize(80, 24)")
            assertEquals("80x24", evaluate(webView, "window.__skidbladnirHarness.viewport"))

            evaluate(webView, "window.__skidbladnirHarness.probeAutomaticReplies()")
            Thread.sleep(200)
            val replies = evaluate(webView, "window.__skidbladnirHarness.automaticReplies.join(',')")
            assertTrue("missing DA1 reply: $replies", replies.contains("DA1"))
            assertTrue("missing DA2 reply: $replies", replies.contains("DA2"))
            assertTrue("missing DSR reply: $replies", replies.contains("DSR"))
            assertTrue("missing CPR reply: $replies", replies.contains("CPR"))
        }
    }

    @Test
    fun editableDraftAndVisualViewportAreVisible() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            assertEquals("Empty draft", evaluate(webView, "document.getElementById('draft-value').textContent"))
            assertTrue(
                evaluate(webView, "document.getElementById('viewport-status').textContent.includes('Visual viewport:')") == "true",
            )

            evaluate(webView, "window.__skidbladnirHarness.compose('北極星')")
            evaluate(webView, "window.__skidbladnirHarness.paste('one')")
            evaluate(webView, "window.__skidbladnirHarness.dictation(' reviewed')")
            evaluate(webView, "window.__skidbladnirHarness.backspace()")

            assertEquals(
                "北極星one reviewe",
                evaluate(webView, "document.getElementById('draft-value').textContent"),
            )
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.autoSubmitted"))
            assertTrue(
                evaluate(webView, "window.__skidbladnirHarness.visualViewport.width > 0 && window.__skidbladnirHarness.visualViewport.height > 0") == "true",
            )
        }
    }

    @Test
    fun compositionPasteAndDictationRemainEditableAndSanitized() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            evaluate(webView, "window.__skidbladnirHarness.compose('北極星')")
            evaluate(webView, "window.__skidbladnirHarness.paste('one\\u0000two\\r\\nthree\\u001b[201~\\u0085\\u0001\\t')")
            evaluate(webView, "window.__skidbladnirHarness.dictation(' reviewed')")

            assertEquals(
                "北極星onetwo\nthree[201~\t reviewed",
                evaluate(webView, "window.__skidbladnirHarness.editorValue"),
            )
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.autoSubmitted"))
            assertEquals("PASS", evaluate(webView, "window.__skidbladnirHarness.ime"))
            assertEquals("3", evaluate(webView, "window.__skidbladnirHarness.inputHistory.length"))
            assertEquals("-1", evaluate(webView, "window.__skidbladnirHarness.editorValue.indexOf('\\u001b')"))
            assertEquals("-1", evaluate(webView, "window.__skidbladnirHarness.editorValue.indexOf('\\u0085')"))
            assertEquals("-1", evaluate(webView, "window.__skidbladnirHarness.editorValue.indexOf('\\u0001')"))
            assertEquals("-1", evaluate(webView, "window.__skidbladnirHarness.inputHistory.join('').indexOf('\\u001b[201~')"))
        }
    }

    @Test
    fun editableUnicodeDraftSurvivesRotationAndRemainsEditable() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            var webView = awaitWebView(scenario)
            evaluate(webView, "window.__skidbladnirHarness.compose('北極星')")
            val beforeRotation = evaluate(webView, "window.visualViewport.width + 'x' + window.visualViewport.height")
            var initialOrientation = 0
            scenario.onActivity { activity ->
                initialOrientation = activity.resources.configuration.orientation
            }
            val firstOrientation = if (
                initialOrientation == android.content.res.Configuration.ORIENTATION_PORTRAIT
            ) {
                android.content.res.Configuration.ORIENTATION_LANDSCAPE
            } else {
                android.content.res.Configuration.ORIENTATION_PORTRAIT
            }
            val firstRequestedOrientation = if (
                firstOrientation == android.content.res.Configuration.ORIENTATION_PORTRAIT
            ) {
                ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
            } else {
                ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
            }

            scenario.onActivity { activity ->
                activity.requestedOrientation = firstRequestedOrientation
            }
            webView = awaitWebViewWithViewportChange(
                scenario,
                beforeRotation,
                firstOrientation,
            )
            assertEquals("北極星", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            evaluate(webView, "window.__skidbladnirHarness.compose('追加')")
            assertEquals("北極星追加", evaluate(webView, "window.__skidbladnirHarness.editorValue"))

            val secondOrientation = if (
                firstOrientation == android.content.res.Configuration.ORIENTATION_PORTRAIT
            ) {
                android.content.res.Configuration.ORIENTATION_LANDSCAPE
            } else {
                android.content.res.Configuration.ORIENTATION_PORTRAIT
            }
            val secondRequestedOrientation = if (
                secondOrientation == android.content.res.Configuration.ORIENTATION_PORTRAIT
            ) {
                ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
            } else {
                ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
            }
            scenario.onActivity { activity ->
                activity.requestedOrientation = secondRequestedOrientation
            }
            webView = awaitWebViewWithViewportChange(
                scenario,
                evaluate(webView, "window.visualViewport.width + 'x' + window.visualViewport.height"),
                secondOrientation,
            )
            assertEquals("北極星追加", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("true", evaluate(webView, "window.__skidbladnirHarness.actualInputElement"))
            evaluate(webView, "window.__skidbladnirHarness.compose('!')")
            assertEquals("北極星追加!", evaluate(webView, "window.__skidbladnirHarness.editorValue"))

            scenario.onActivity { activity ->
                activity.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
            }
        }
    }

    @Test
    fun lockedWebViewUsesWebMessagePortInsteadOfJavascriptInterface() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            val settings = onUi(scenario) {
                listOf(
                    webView.settings.allowFileAccess,
                    webView.settings.allowContentAccess,
                    webView.settings.blockNetworkLoads,
                )
            }
            assertEquals(false, settings[0])
            assertEquals(false, settings[1])
            assertEquals(true, settings[2])
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.networkEnabled"))
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.fileAccess"))
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.contentAccess"))
            assertEquals("true", evaluate(webView, "window.__skidbladnirHarness.webMessagePort"))

            evaluate(webView, "window.__skidbladnirHarness.send('input', 'draft')")
            assertEquals("input", evaluate(webView, "window.__skidbladnirHarness.lastAck"))
        }
    }

    private fun awaitWebView(scenario: ActivityScenario<MainActivity>): WebView {
        val latch = CountDownLatch(1)
        var result: WebView? = null
        scenario.onActivity { activity ->
            result = findWebView(activity.window.decorView)
            requireNotNull(result) { "MainActivity view tree has no WebView" }
            awaitLoaded(requireNotNull(result), latch)
        }
        assertTrue("local harness did not load", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
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

    private fun awaitLoaded(webView: WebView, latch: CountDownLatch) {
        if (webView.url?.endsWith("/terminal/index.html") == true) {
            webView.evaluateJavascript(
                "typeof window.__skidbladnirHarness === 'object' && window.__skidbladnirHarness.state === 'ready'",
            ) { ready ->
                if (ready == "true") {
                    latch.countDown()
                } else {
                    webView.postDelayed({ awaitLoaded(webView, latch) }, 50)
                }
            }
        } else {
            webView.postDelayed({ awaitLoaded(webView, latch) }, 50)
        }
    }

    private fun awaitWebViewWithViewportChange(
        scenario: ActivityScenario<MainActivity>,
        previousViewport: String,
        expectedOrientation: Int,
    ): WebView {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (System.nanoTime() < deadline) {
            var orientation = 0
            var webView: WebView? = null
            scenario.onActivity { activity ->
                orientation = activity.resources.configuration.orientation
                webView = findWebView(activity.window.decorView)
            }
            val candidate = webView
            if (candidate != null && orientation == expectedOrientation) {
                val ready = evaluate(candidate, "window.__skidbladnirHarness.state === 'ready'")
                val viewport = evaluate(candidate, "window.visualViewport.width + 'x' + window.visualViewport.height")
                if (ready == "true" && viewport != previousViewport) return candidate
            }
            Thread.sleep(100)
        }
        throw AssertionError("rotation did not produce a ready WebView with a changed viewport")
    }

    private fun <T> onUi(scenario: ActivityScenario<MainActivity>, block: () -> T): T {
        val latch = CountDownLatch(1)
        var result: T? = null
        scenario.onActivity {
            result = block()
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
        assertTrue("JavaScript evaluation timed out: $expression", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }
}
