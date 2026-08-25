package dev.niels.skidbladnir

import android.webkit.WebView
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
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
            assertEquals("https://appassets.androidplatform.net/assets/terminal/index.html", webView.url)
            assertEquals("Skíðblaðnir terminal platform harness", evaluate(webView, "document.title"))
            assertEquals("ready", evaluate(webView, "window.__skidbladnirHarness.state"))
            assertTrue(evaluate(webView, "window.__skidbladnirHarness.ansiUnicode") == "PASS")

            evaluate(webView, "window.__skidbladnirHarness.resize(80, 24)")
            assertEquals("80x24", evaluate(webView, "window.__skidbladnirHarness.viewport"))
        }
    }

    @Test
    fun compositionPasteAndDictationRemainEditableAndSanitized() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            evaluate(webView, "window.__skidbladnirHarness.compose('北極星')")
            evaluate(webView, "window.__skidbladnirHarness.paste('one\\u0000two\\r\\nthree')")
            evaluate(webView, "window.__skidbladnirHarness.dictation(' reviewed')")

            assertEquals(
                "北極星onetwo\\nthree reviewed",
                evaluate(webView, "window.__skidbladnirHarness.editorValue"),
            )
            assertEquals("false", evaluate(webView, "window.__skidbladnirHarness.autoSubmitted"))
            assertEquals("PASS", evaluate(webView, "window.__skidbladnirHarness.ime"))
        }
    }

    @Test
    fun lockedWebViewUsesWebMessagePortInsteadOfJavascriptInterface() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)
            assertEquals(false, webView.settings.allowFileAccess)
            assertEquals(false, webView.settings.allowContentAccess)
            assertEquals(true, webView.settings.blockNetworkLoads)
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
            result = activity.terminalWebView
            awaitLoaded(result!!, latch)
        }
        assertTrue("local harness did not load", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }

    private fun awaitLoaded(webView: WebView, latch: CountDownLatch) {
        if (webView.url?.endsWith("/terminal/index.html") == true) {
            latch.countDown()
        } else {
            webView.postDelayed({ awaitLoaded(webView, latch) }, 50)
        }
    }

    private fun evaluate(webView: WebView, expression: String): String {
        val latch = CountDownLatch(1)
        var result: String? = null
        webView.post {
            webView.evaluateJavascript("JSON.stringify($expression)") {
                result = it.removeSurrounding("\"").replace("\\n", "\n").replace("\\\"", "\"")
                latch.countDown()
            }
        }
        assertTrue("JavaScript evaluation timed out: $expression", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }
}
