package dev.niels.skidbladnir

import android.content.pm.ActivityInfo
import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.Rect
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.PixelCopy
import android.view.KeyEvent
import android.view.View
import android.view.ViewGroup
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputConnection
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.core.net.toUri
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
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class TerminalInstrumentedTest {
    @Before
    fun resetProbe() {
        TerminalTestProbe.reset()
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
                "default-src 'none'; style-src 'self'; style-src-elem 'self' 'unsafe-inline'; style-src-attr 'unsafe-inline'; script-src 'self'; img-src 'none'; connect-src 'none'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
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
            val size = TerminalTestProbe.sizes.poll(5, TimeUnit.SECONDS)
            assertNotNull("terminal did not publish its initial size", size)
            assertTrue(requireNotNull(size).let { it.first in 80..240 && it.second in 5..120 })
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
    fun deckLineFeedAndTrustedEnterStayLfCrDistinctWhileCursorKeysHonorMode() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)

            for ((accessory, value) in listOf(
                TerminalAccessory.Escape to "\u001b",
                TerminalAccessory.Tab to "\u0009",
                TerminalAccessory.LineFeed to "\u000a",
                TerminalAccessory.Left to "\u001b[D",
                TerminalAccessory.Up to "\u001b[A",
                TerminalAccessory.Down to "\u001b[B",
                TerminalAccessory.Right to "\u001b[C",
                TerminalAccessory.Home to "\u001b[H",
                TerminalAccessory.End to "\u001b[F",
            )) {
                page.sendAccessory(accessory)
                assertInput(value)
            }
            dispatchHardwareKey(scenario, webView, KeyEvent.KEYCODE_ENTER)
            assertInput(
                "\r",
                "trusted Enter must emit CR (0x0d), distinct from deck LineFeed LF (0x0a)",
            )

            page.write("\u001b[?1hAPPLICATION MODE".toByteArray())
            awaitValue(
                webView,
                "document.querySelector('.xterm-rows').textContent.includes('APPLICATION MODE')",
                "true",
            )
            for ((accessory, value) in listOf(
                TerminalAccessory.Left to "\u001bOD",
                TerminalAccessory.Up to "\u001bOA",
                TerminalAccessory.Down to "\u001bOB",
                TerminalAccessory.Right to "\u001bOC",
                TerminalAccessory.Home to "\u001bOH",
                TerminalAccessory.End to "\u001bOF",
            )) {
                page.sendAccessory(accessory)
                assertInput(value)
            }
            awaitValue(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "true",
            )

            TerminalTestProbe.events.clear()
            page.sendAccessory(TerminalAccessory.Control)
            page.sendAccessory(TerminalAccessory.LineFeed)
            page.sendAccessory(TerminalAccessory.Control)
            page.sendAccessory(TerminalAccessory.Escape)
            page.sendAccessory(TerminalAccessory.Control)
            page.sendAccessory(TerminalAccessory.Control)
            for (expected in listOf(
                TerminalTestEvent.ControlState(TerminalControlState.Armed),
                TerminalTestEvent.ControlState(TerminalControlState.Off),
                TerminalTestEvent.Input("\n".toByteArray()),
                TerminalTestEvent.ControlState(TerminalControlState.Armed),
                TerminalTestEvent.ControlState(TerminalControlState.Off),
                TerminalTestEvent.Input("\u001b".toByteArray()),
                TerminalTestEvent.ControlState(TerminalControlState.Armed),
                TerminalTestEvent.ControlState(TerminalControlState.Off),
            )) {
                assertEvent(expected)
            }
            assertNull("burst emitted an extra callback", TerminalTestProbe.events.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun controlMapsProvenAsciiOnceAndSecondTapCancels() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
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
                page.sendAccessory(TerminalAccessory.Control)
                assertControlState(TerminalControlState.Armed)
                dispatchHardwareKey(scenario, webView, keyCode, metaState)
                assertInput(expected)
                assertControlState(TerminalControlState.Off)
            }

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Off)
            assertNull("second Control emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun uncertainImeAndCompositionStayLiteralAndConsumeControl() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)

            for (value in listOf("c", "北", "dictated words")) {
                page.sendAccessory(TerminalAccessory.Control)
                assertControlState(TerminalControlState.Armed)
                commitText(scenario, webView, value)
                assertInput(value)
                assertControlState(TerminalControlState.Off)
            }

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            composeText(scenario, webView, "c")
            assertInput("c")
            assertControlState(TerminalControlState.Off)
        }
    }

    @Test
    fun deckFocusTransferPreservesControlWhileExplicitBoundariesReset() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            onUi(scenario) { webView.clearFocus() }
            assertNull(
                "intra-surface focus transfer changed Control",
                TerminalTestProbe.controlStates.poll(250, TimeUnit.MILLISECONDS),
            )
            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Off)

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            onUi(scenario) { webView.clearFocus() }
            page.focus()
            awaitValue(
                webView,
                "document.activeElement === document.querySelector('.xterm-helper-textarea')",
                "true",
            )
            dispatchHardwareKey(scenario, webView, KeyEvent.KEYCODE_C)
            assertInput("\u0003")
            assertControlState(TerminalControlState.Off)

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            page.resetControl()
            assertControlState(TerminalControlState.Off)
            assertNull("Control reset emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            onUi(scenario) { webView.onWindowFocusChanged(false) }
            assertControlState(TerminalControlState.Off)

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            onUi(scenario) { webView.reload() }
            assertTrue(
                "same-page reload kept the stale page port connected",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
            assertNull("reload emitted terminal input", TerminalTestProbe.input.poll(250, TimeUnit.MILLISECONDS))
        }
    }

    @Test
    fun exactControlProtocolRejectsRetiredExtraAndMalformedMessages() {
        for (payload in listOf(
            "not-json",
            """{"kind":"Accessory","key":"CtrlC"}""",
            """{"kind":"Accessory","key":"Control","extra":true}""",
        )) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val webView = awaitTerminal(scenario)
                postRawNativeMessage(scenario, webView, payload)
                assertTrue(
                    "page accepted invalid native message $payload",
                    TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
                )
            }
        }

        TerminalTestProbe.reset()
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            mutateNextControlState(webView, "payload.state = 1;")
            requireNotNull(TerminalTestProbe.page).sendAccessory(TerminalAccessory.Control)
            assertTrue(
                "native accepted a non-string ControlState",
                TerminalTestProbe.unavailable.await(5, TimeUnit.SECONDS),
            )
        }
    }

    @Test
    fun pagePortHandshakeIsExactVersionOne() {
        val payload = """{"kind":"PagePort","version":1,"extra":true}"""
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            postRawHandshake(webView, payload)
            assertTrue(
                "page accepted invalid PagePort handshake $payload",
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
                        getComputedStyle(indexed).color === 'rgb(224, 108, 117)' &&
                        getComputedStyle(foreground).color === 'rgb(97, 175, 239)' &&
                        getComputedStyle(background).backgroundColor === 'rgb(97, 175, 239)';
                }())
                """.trimIndent(),
                "true",
            )
            awaitVisualState(webView)

            val rendered = copyWebView(scenario, webView)
            val renderState = evaluate(
                webView,
                """
                JSON.stringify({
                    canvases: document.querySelectorAll('canvas').length,
                    spans: Array.from(document.querySelectorAll('.xterm-rows span')).map(function (node) {
                        var style = getComputedStyle(node);
                        return {
                            html: node.outerHTML,
                            color: style.color,
                            background: style.backgroundColor
                        };
                    })
                })
                """.trimIndent(),
            )
            assertTrue(
                "true-color terminal output rendered without a blue pixel; " +
                    "renderState=$renderState dominantPixels=${rendered.dominantPixels()}",
                rendered.containsPixel { pixel ->
                    Color.red(pixel) in 75..130 &&
                        Color.green(pixel) in 145..205 &&
                        Color.blue(pixel) in 210..255
                },
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
    fun portraitScaleReturnsAndControlResetsAfterAFullRotation() {
        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            val webView = awaitTerminal(scenario)
            val page = requireNotNull(TerminalTestProbe.page)
            val initialSize = requireNotNull(TerminalTestProbe.sizes.poll(5, TimeUnit.SECONDS))
            val initialScreenWidth = evaluate(
                webView,
                "document.querySelector('.xterm-screen').getBoundingClientRect().width",
            ).toDouble()
            TerminalTestProbe.sizes.clear()

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            onUi(scenario) { it.requestedOrientation = ActivityInfo.SCREEN_ORIENTATION_LANDSCAPE }
            awaitValue(webView, "window.innerWidth > window.innerHeight", "true")
            assertControlState(TerminalControlState.Off)
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
            val page = requireNotNull(TerminalTestProbe.page)
            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
            pasteText(webView, "c")
            assertInput("c")
            assertControlState(TerminalControlState.Off)

            page.sendAccessory(TerminalAccessory.Control)
            assertControlState(TerminalControlState.Armed)
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

            val bytes = TerminalTestProbe.input.poll(5, TimeUnit.SECONDS)
            assertNotNull("sanitized paste did not reach the native message port", bytes)
            val value = requireNotNull(bytes).toString(Charsets.UTF_8)
            assertEquals("onetwo\nthree[201~\t", value)
            assertFalse(value.contains('\u001b'))
            assertFalse(value.contains('\u0000'))
            assertFalse(value.contains('\u0001'))
            assertFalse(value.contains('\u0085'))
            assertControlState(TerminalControlState.Off)
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
        val ready = CountDownLatch(1)
        val unavailable = CountDownLatch(1)
        var deadlineStartedAt = 0L
        val listener = object : TerminalPageListener {
            override fun onReady(page: TerminalPage) {
                ready.countDown()
            }

            override fun onInput(bytes: ByteArray) = Unit

            override fun onResize(columns: Int, rows: Int) = Unit

            override fun onControlStateChanged(state: TerminalControlState) = Unit

            override fun onUnavailable() {
                unavailable.countDown()
            }
        }

        ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                deadlineStartedAt = System.nanoTime()
                activity.setContentView(
                    LockedTerminalWebView(
                        context = activity,
                        listener = listener,
                        initialUrl = "https://appassets.androidplatform.net/assets/terminal/terminal.css",
                        readinessTimeoutMillis = 250,
                    ),
                )
            }

            assertTrue("never-ready packaged page did not time out", unavailable.await(5, TimeUnit.SECONDS))
            assertTrue(
                "never-ready packaged page failed before its deadline",
                TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - deadlineStartedAt) >= 200,
            )
            assertEquals("never-ready packaged page signaled Ready", 1L, ready.count)
        }
    }

    private fun awaitTerminal(scenario: ActivityScenario<TerminalTestActivity>): WebView {
        assertTrue("native page port did not become ready", TerminalTestProbe.ready.await(5, TimeUnit.SECONDS))
        val webView = onUi(scenario) {
            requireNotNull(findWebView(it.window.decorView)) { "terminal activity has no WebView" }
        }
        assertControlState(TerminalControlState.Off)
        return webView
    }

    private fun assertInput(
        expected: String,
        failureMessage: String = "terminal input did not reach the native message port",
    ) {
        val actual = TerminalTestProbe.input.poll(5, TimeUnit.SECONDS)
        assertNotNull(failureMessage, actual)
        assertArrayEquals(failureMessage, expected.toByteArray(), requireNotNull(actual))
    }

    private fun assertControlState(expected: TerminalControlState) {
        assertEquals(expected, TerminalTestProbe.controlStates.poll(5, TimeUnit.SECONDS))
    }

    private fun assertEvent(expected: TerminalTestEvent) {
        when (val actual = TerminalTestProbe.events.poll(5, TimeUnit.SECONDS)) {
            is TerminalTestEvent.Input -> {
                assertTrue("expected Input event, got $expected", expected is TerminalTestEvent.Input)
                assertArrayEquals((expected as TerminalTestEvent.Input).bytes, actual.bytes)
            }
            else -> assertEquals(expected, actual)
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

    private fun withTerminalInputConnection(
        scenario: ActivityScenario<TerminalTestActivity>,
        webView: WebView,
        block: (InputConnection) -> Boolean,
    ): Boolean {
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

    private fun pasteText(webView: WebView, value: String) {
        evaluate(
            webView,
            """
            (function () {
                var clipboard = new DataTransfer();
                clipboard.setData('text/plain', ${JSONObject.quote(value)});
                return document.querySelector('.xterm-helper-textarea').dispatchEvent(
                    new ClipboardEvent('paste', {
                        clipboardData: clipboard,
                        bubbles: true,
                        cancelable: true
                    })
                );
            }())
            """.trimIndent(),
        )
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

    private fun mutateNextControlState(webView: WebView, mutation: String) {
        evaluate(
            webView,
            """
            (function () {
                var original = MessagePort.prototype.postMessage;
                MessagePort.prototype.postMessage = function (value) {
                    var payload = JSON.parse(value);
                    if (payload.kind === 'ControlState') {
                        MessagePort.prototype.postMessage = original;
                        $mutation
                        return original.call(this, JSON.stringify(payload));
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
        assertTrue("JavaScript evaluation timed out: $expression", latch.await(5, TimeUnit.SECONDS))
        return requireNotNull(result)
    }

    private fun awaitValue(webView: WebView, expression: String, expected: String) {
        val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
        while (System.nanoTime() < deadline) {
            if (evaluate(webView, expression) == expected) return
            Thread.sleep(50)
        }
        throw AssertionError("JavaScript value did not become $expected: $expression")
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

    private fun Bitmap.dominantPixels(): String {
        val counts = mutableMapOf<Int, Int>()
        for (y in 0 until height step 4) {
            for (x in 0 until width step 4) {
                val pixel = getPixel(x, y)
                counts[pixel] = (counts[pixel] ?: 0) + 1
            }
        }
        return counts.entries
            .sortedByDescending { it.value }
            .take(12)
            .joinToString { (pixel, count) -> "#%08X:%d".format(pixel, count) }
    }
}
