package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.ComponentActivity
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue

internal sealed interface TerminalTestEvent {
    data class Input(val bytes: ByteArray) : TerminalTestEvent
    data class ControlState(val state: TerminalControlState) : TerminalTestEvent
}

internal object TerminalTestProbe {
    @Volatile
    private var generation = 0
    var ready = CountDownLatch(1)
    var unavailable = CountDownLatch(1)
    var page: TerminalPage? = null
    val input = LinkedBlockingQueue<ByteArray>()
    val sizes = LinkedBlockingQueue<Pair<Int, Int>>()
    val controlStates = LinkedBlockingQueue<TerminalControlState>()
    val events = LinkedBlockingQueue<TerminalTestEvent>()

    @Synchronized
    fun reset() {
        generation += 1
        ready = CountDownLatch(1)
        unavailable = CountDownLatch(1)
        page = null
        input.clear()
        sizes.clear()
        controlStates.clear()
        events.clear()
    }

    fun listener(): TerminalPageListener {
        val listenerGeneration = synchronized(this) { generation }
        return object : TerminalPageListener {
            override fun onReady(page: TerminalPage) {
                accept(listenerGeneration) {
                    this@TerminalTestProbe.page = page
                    ready.countDown()
                }
            }

            override fun onInput(bytes: ByteArray) {
                accept(listenerGeneration) {
                    input.add(bytes)
                    events.add(TerminalTestEvent.Input(bytes))
                }
            }

            override fun onResize(columns: Int, rows: Int) {
                accept(listenerGeneration) { sizes.add(columns to rows) }
            }

            override fun onControlStateChanged(state: TerminalControlState) {
                accept(listenerGeneration) {
                    controlStates.add(state)
                    events.add(TerminalTestEvent.ControlState(state))
                }
            }

            override fun onUnavailable() {
                accept(listenerGeneration) { unavailable.countDown() }
            }
        }
    }

    private fun accept(listenerGeneration: Int, block: () -> Unit) {
        synchronized(this) {
            if (listenerGeneration == generation) block()
        }
    }
}

internal class TerminalTestActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(LockedTerminalWebView(this, TerminalTestProbe.listener()))
    }
}
