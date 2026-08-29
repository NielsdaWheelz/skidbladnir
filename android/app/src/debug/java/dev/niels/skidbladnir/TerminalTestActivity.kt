package dev.niels.skidbladnir

import android.content.Context
import android.os.Bundle
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
    val events = LinkedBlockingQueue<TerminalTestEvent>()

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
            sizes.add(columns to rows)
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
    val events: LinkedBlockingQueue<TerminalTestEvent> get() = active.events

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
): LockedTerminalWebView = LockedTerminalWebView(
    context = context,
    listener = probe.listener(),
    initialUrl = initialUrl,
    readinessTimeoutMillis = readinessTimeoutMillis,
)

internal class TerminalTestActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(TerminalTestProbe.createTerminal(this))
    }
}
