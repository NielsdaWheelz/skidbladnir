package dev.niels.skidbladnir

import android.content.pm.ActivityInfo
import android.view.View
import android.view.ViewGroup
import android.view.WindowInsets
import android.view.inputmethod.InputMethodManager
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
    fun typingControlFocusesTerminalAndLiveCompositionIsVisible() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            val webView = awaitWebView(scenario)

            assertEquals("1", evaluate(webView, "document.querySelectorAll('#focus-terminal').length"))
            evaluate(webView, "document.getElementById('focus-terminal').click()")
            assertEquals(
                "true",
                evaluate(webView, "document.activeElement.classList.contains('xterm-helper-textarea')"),
            )

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: 'é北' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北", evaluate(webView, "document.getElementById('draft-value').textContent"))
            assertEquals("", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals(
                "true",
                evaluate(webView, "document.getElementById('draft-status').textContent.includes('composing')"),
            )

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    window.__skidbladnirHarness.paste('é北');
                    input.dispatchEvent(new CompositionEvent('compositionend', { data: 'é北' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北", evaluate(webView, "document.getElementById('draft-value').textContent"))

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: '追加' }));
                    input.dispatchEvent(new CompositionEvent('compositionend', { data: '追加' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北追加", evaluate(webView, "document.getElementById('draft-value').textContent"))
            assertEquals("é北", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            evaluate(webView, "window.__skidbladnirHarness.paste('追加')")
            assertEquals("é北追加", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加", evaluate(webView, "document.getElementById('draft-value').textContent"))

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: '甲' }));
                    input.dispatchEvent(new CompositionEvent('compositionend', { data: '甲' }));
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: '乙' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北追加甲乙", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(webView, "window.__skidbladnirHarness.paste('甲')")
            assertEquals("é北追加甲", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加甲乙", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(
                webView,
                "document.querySelector('.xterm-helper-textarea').dispatchEvent(new CompositionEvent('compositionend', { data: '乙' }))",
            )
            assertEquals("é北追加甲乙", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(webView, "window.__skidbladnirHarness.paste('乙')")
            assertEquals("é北追加甲乙", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加甲乙", evaluate(webView, "document.getElementById('draft-value').textContent"))

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: '丙' }));
                    input.dispatchEvent(new CompositionEvent('compositionend', { data: '丙' }));
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: '丁' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北追加甲乙丙丁", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(webView, "window.__skidbladnirHarness.paste('丁')")
            assertEquals("é北追加甲乙丁", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加甲乙丁丙", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(
                webView,
                "document.querySelector('.xterm-helper-textarea').dispatchEvent(new CompositionEvent('compositionend', { data: '丁' }))",
            )
            assertEquals("é北追加甲乙丁丙", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(webView, "window.__skidbladnirHarness.paste('丙')")
            assertEquals("é北追加甲乙丁丙", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加甲乙丁丙", evaluate(webView, "document.getElementById('draft-value').textContent"))

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.value = 'existing screen-reader text';
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.value += '語';
                    input.dispatchEvent(new InputEvent('input', {
                        data: '語',
                        inputType: 'insertCompositionText',
                        isComposing: true
                    }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北追加甲乙丁丙語", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(
                webView,
                "document.querySelector('.xterm-helper-textarea').dispatchEvent(new CompositionEvent('compositionend', { data: '語' }))",
            )
            awaitValue(webView, "window.__skidbladnirHarness.editorValue", "é北追加甲乙丁丙語")

            evaluate(
                webView,
                """
                (function () {
                    var input = document.querySelector('.xterm-helper-textarea');
                    input.dispatchEvent(new CompositionEvent('compositionstart', { data: '' }));
                    input.dispatchEvent(new CompositionEvent('compositionupdate', { data: 'cancelled' }));
                }())
                """.trimIndent(),
            )
            assertEquals("é北追加甲乙丁丙語cancelled", evaluate(webView, "document.getElementById('draft-value').textContent"))
            evaluate(
                webView,
                "document.querySelector('.xterm-helper-textarea').dispatchEvent(new CompositionEvent('compositionend', { data: '' }))",
            )
            assertEquals("é北追加甲乙丁丙語", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            assertEquals("é北追加甲乙丁丙語", evaluate(webView, "document.getElementById('draft-value').textContent"))
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
    fun imeOpenRoundTripRotationRetainsScaleAndFitsViewport() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            var webView = awaitWebView(scenario)
            evaluate(webView, "document.getElementById('focus-terminal').click()")
            showIme(scenario, webView)
            awaitImeVisible(scenario)
            evaluate(webView, "window.__skidbladnirHarness.compose('北極星')")
            awaitTerminalFit(webView)

            val initialScale = evaluate(webView, "window.visualViewport.scale")
            val initialWidth = evaluate(webView, "window.innerWidth")
            val initialTerminalViewport = evaluate(webView, "window.__skidbladnirHarness.viewport")
            var initialOrientation = 0
            scenario.onActivity { activity ->
                initialOrientation = activity.resources.configuration.orientation
            }
            val otherOrientation = if (
                initialOrientation == android.content.res.Configuration.ORIENTATION_PORTRAIT
            ) {
                android.content.res.Configuration.ORIENTATION_LANDSCAPE
            } else {
                android.content.res.Configuration.ORIENTATION_PORTRAIT
            }

            scenario.onActivity { activity ->
                activity.requestedOrientation = requestedOrientation(otherOrientation)
            }
            webView = awaitWebViewWithViewportChange(
                scenario,
                initialWidth,
                otherOrientation,
                "window.innerWidth",
            )
            awaitImeVisible(scenario)
            awaitTerminalFit(webView)
            assertEquals("true", evaluate(webView, "document.documentElement.scrollWidth <= window.innerWidth"))
            assertEquals(initialScale, evaluate(webView, "window.visualViewport.scale"))
            assertTrue(
                "terminal viewport did not refit after rotation",
                evaluate(webView, "window.__skidbladnirHarness.viewport") != initialTerminalViewport,
            )

            val otherWidth = evaluate(webView, "window.innerWidth")
            scenario.onActivity { activity ->
                activity.requestedOrientation = requestedOrientation(initialOrientation)
            }
            webView = awaitWebViewWithViewportChange(
                scenario,
                otherWidth,
                initialOrientation,
                "window.innerWidth",
            )
            awaitImeVisible(scenario)
            awaitTerminalFit(webView)

            assertEquals(initialScale, evaluate(webView, "window.visualViewport.scale"))
            assertEquals(initialWidth, evaluate(webView, "window.innerWidth"))
            assertEquals(initialTerminalViewport, evaluate(webView, "window.__skidbladnirHarness.viewport"))
            assertEquals("true", evaluate(webView, "document.documentElement.scrollWidth <= window.innerWidth"))
            assertEquals("北極星", evaluate(webView, "window.__skidbladnirHarness.editorValue"))
            evaluate(webView, "window.__skidbladnirHarness.compose('!')")
            assertEquals("北極星!", evaluate(webView, "window.__skidbladnirHarness.editorValue"))

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
                "typeof window.__skidbladnirHarness === 'object' && window.__skidbladnirHarness.state === 'ready' && window.__skidbladnirHarness.viewport !== 'unknown'",
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
        viewportExpression: String = "window.visualViewport.width + 'x' + window.visualViewport.height",
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
                val viewport = evaluate(candidate, viewportExpression)
                if (ready == "true" && viewport != previousViewport) return candidate
            }
            Thread.sleep(100)
        }
        throw AssertionError("rotation did not produce a ready WebView with a changed viewport")
    }

    private fun requestedOrientation(orientation: Int): Int =
        if (orientation == android.content.res.Configuration.ORIENTATION_PORTRAIT) {
            ActivityInfo.SCREEN_ORIENTATION_PORTRAIT
        } else {
            ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE
        }

    private fun showIme(scenario: ActivityScenario<MainActivity>, webView: WebView) {
        scenario.onActivity { activity ->
            webView.requestFocus()
            activity.getSystemService(InputMethodManager::class.java).showSoftInput(
                webView,
                InputMethodManager.SHOW_IMPLICIT,
            )
        }
    }

    private fun awaitImeVisible(scenario: ActivityScenario<MainActivity>) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (System.nanoTime() < deadline) {
            var visible = false
            scenario.onActivity { activity ->
                visible = activity.window.decorView.rootWindowInsets
                    ?.isVisible(WindowInsets.Type.ime()) == true
            }
            if (visible) return
            Thread.sleep(100)
        }
        throw AssertionError("Gboard did not become visible")
    }

    private fun awaitTerminalFit(webView: WebView) {
        val expression = """
            (function () {
                var host = document.getElementById('terminal');
                var terminal = host.querySelector('.xterm');
                var screen = host.querySelector('.xterm-screen');
                if (!terminal || !screen || window.__skidbladnirHarness.viewport === 'unknown') return false;
                var bounds = screen.getBoundingClientRect();
                return bounds.width > 0 && bounds.height > 0 &&
                    bounds.width <= terminal.clientWidth && bounds.height <= terminal.clientHeight;
            }())
        """.trimIndent()
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(8)
        while (System.nanoTime() < deadline) {
            if (evaluate(webView, expression) == "true") return
            Thread.sleep(100)
        }
        throw AssertionError("xterm did not fit its current container")
    }

    private fun awaitValue(webView: WebView, expression: String, expected: String) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (evaluate(webView, expression) == expected) return
            Thread.sleep(50)
        }
        throw AssertionError("JavaScript value did not become $expected: $expression")
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
