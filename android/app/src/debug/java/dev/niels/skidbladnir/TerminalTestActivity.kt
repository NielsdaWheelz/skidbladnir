package dev.niels.skidbladnir

import android.content.Context
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import androidx.activity.ComponentActivity
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue

internal sealed interface TerminalTestEvent {
    data object Ready : TerminalTestEvent
    data class Input(val bytes: ByteArray) : TerminalTestEvent
    data class Modifiers(val value: TerminalModifiers) : TerminalTestEvent
}

internal class TerminalProbe {
    val ready = CountDownLatch(1)
    val unavailable = CountDownLatch(1)
    @Volatile
    var page: TerminalPage? = null
    val input = LinkedBlockingQueue<ByteArray>()
    val sizes = LinkedBlockingQueue<Pair<Int, Int>>()
    val viewportTooSmall = LinkedBlockingQueue<Unit>()
    val events = LinkedBlockingQueue<TerminalTestEvent>()
    @Volatile
    var resizedBeforeReady = false

    fun listener(): TerminalPageListener = object : TerminalPageListener {
        override fun onReady(page: TerminalPage) {
            this@TerminalProbe.page = page
            events.add(TerminalTestEvent.Ready)
            ready.countDown()
        }

        override fun onInput(bytes: ByteArray) {
            input.add(bytes)
            events.add(TerminalTestEvent.Input(bytes))
        }

        override fun onResize(columns: Int, rows: Int) {
            if (ready.count != 0L) resizedBeforeReady = true
            sizes.add(columns to rows)
        }

        override fun onViewportTooSmall() {
            viewportTooSmall.add(Unit)
        }

        override fun onModifiersChanged(modifiers: TerminalModifiers) {
            events.add(TerminalTestEvent.Modifiers(modifiers))
        }

        override fun onUnavailable() {
            unavailable.countDown()
        }
    }
}

internal object TerminalTestProbe {
    @Volatile
    private var active = TerminalProbe()

    val ready: CountDownLatch get() = active.ready
    val unavailable: CountDownLatch get() = active.unavailable
    val page: TerminalPage? get() = active.page
    val input: LinkedBlockingQueue<ByteArray> get() = active.input
    val sizes: LinkedBlockingQueue<Pair<Int, Int>> get() = active.sizes
    val viewportTooSmall: LinkedBlockingQueue<Unit> get() = active.viewportTooSmall
    val events: LinkedBlockingQueue<TerminalTestEvent> get() = active.events
    val resizedBeforeReady: Boolean get() = active.resizedBeforeReady

    @Synchronized
    fun reset() {
        active = TerminalProbe()
    }

    fun createTerminal(context: Context): LockedTerminalWebView = createTestTerminal(context, active)
}

internal fun createTestTerminal(
    context: Context,
    probe: TerminalProbe,
    initialUrl: String = "https://appassets.androidplatform.net/assets/terminal/index.html",
    readinessTimeoutMillis: Long = 10_000L,
    nominalTextSizeSp: Int = TERMINAL_TEXT_SIZE_DEFAULT_SP,
): LockedTerminalWebView = LockedTerminalWebView(
    context = context,
    nominalTextSizeSp = nominalTextSizeSp,
    listener = probe.listener(),
    initialUrl = initialUrl,
    readinessTimeoutMillis = readinessTimeoutMillis,
)

internal class TerminalTestActivity : ComponentActivity() {
    private var ownedContentView: View? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(TerminalTestProbe.createTerminal(this))
    }

    override fun setContentView(view: View?) {
        val contentView = requireNotNull(view) { "TerminalTestActivity content is required" }
        replaceOwnedContentView(contentView) { super.setContentView(contentView) }
    }

    override fun setContentView(view: View?, params: ViewGroup.LayoutParams?) {
        val contentView = requireNotNull(view) { "TerminalTestActivity content is required" }
        val contentParams = requireNotNull(params) { "TerminalTestActivity layout parameters are required" }
        replaceOwnedContentView(contentView) {
            super.setContentView(contentView, contentParams)
        }
    }

    override fun setContentView(layoutResID: Int) {
        error("TerminalTestActivity supports code-owned content only")
    }

    override fun addContentView(view: View?, params: ViewGroup.LayoutParams?) {
        error("TerminalTestActivity supports one owned content root")
    }

    private inline fun replaceOwnedContentView(view: View, install: () -> Unit) {
        disposeOwnedContentView()
        ownedContentView = view
        try {
            install()
        } catch (failure: Throwable) {
            ownedContentView = null
            disposeContentView(view)
            throw failure
        }
    }

    override fun onDestroy() {
        disposeOwnedContentView()
        super.onDestroy()
    }

    private fun disposeOwnedContentView() {
        val view = ownedContentView ?: return
        ownedContentView = null
        disposeContentView(view)
    }

    private fun disposeContentView(view: View) {
        if (view is LockedTerminalWebView) {
            view.dispose()
            return
        }
        if (view is WebView) view.stopLoading()
        (view.parent as? ViewGroup)?.removeView(view)
        if (view is WebView) {
            view.removeAllViews()
            view.destroy()
        }
    }
}
