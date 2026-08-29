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
import org.junit.Assert.assertArrayEquals
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
            onUi(scenario) { requireNotNull(TerminalTestProbe.page).resetModifiers() }
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
        for (payload in listOf(
            "not-json",
            """{"kind":"Accessory","key":"LineFeed"}""",
            """{"kind":"ResetControl"}""",
            """{"kind":"Accessory","key":"Control","extra":true}""",
            """{"kind":"Accessory","key":1}""",
            """{"kind":"Accessory","key":"Meta"}""",
            """{"kind":"Unknown"}""",
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
    }

    @Test
    fun exactPageProtocolRejectsUnsupportedExtraMalformedAndUnknownModifierStates() {
        for (payload in listOf(
            """{"kind":"ControlState","state":"Armed"}""",
            """{"kind":"ModifierState","control":"Armed","alt":"Off","extra":true}""",
            """{"kind":"ModifierState","control":1,"alt":"Off"}""",
            """{"kind":"ModifierState","control":"Locked","alt":"Off"}""",
            """{"kind":"ModifierState","control":"Armed"}""",
            """{"kind":"Unknown"}""",
        )) {
            TerminalTestProbe.reset()
            ActivityScenario.launch(TerminalTestActivity::class.java).use { scenario ->
                val webView = awaitTerminal(scenario)
                replaceNextModifierState(webView, payload)
                postAccessory(scenario, webView, "Control")
                assertTrue(
                    "native accepted invalid page message $payload",
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
                        getComputedStyle(indexed).color === 'rgb(215, 78, 51)' &&
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

    private fun assertEvent(expected: TerminalTestEvent) {
        when (val actual = TerminalTestProbe.events.poll(5, TimeUnit.SECONDS)) {
            is TerminalTestEvent.Input -> {
                assertTrue("expected Input event, got $expected", expected is TerminalTestEvent.Input)
                assertArrayEquals(
                    "terminal input bytes differed; expected event=$expected",
                    (expected as TerminalTestEvent.Input).bytes,
                    actual.bytes,
                )
            }
            else -> assertEquals("terminal protocol event differed", expected, actual)
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
