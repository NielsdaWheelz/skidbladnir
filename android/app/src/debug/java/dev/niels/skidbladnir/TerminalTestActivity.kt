package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.ComponentActivity
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue

internal object TerminalTestProbe : TerminalPageListener {
    var ready = CountDownLatch(1)
    var unavailable = CountDownLatch(1)
    var page: TerminalPage? = null
    val input = LinkedBlockingQueue<ByteArray>()
    val sizes = LinkedBlockingQueue<Pair<Int, Int>>()

    fun reset() {
        ready = CountDownLatch(1)
        unavailable = CountDownLatch(1)
        page = null
        input.clear()
        sizes.clear()
    }

    override fun onReady(page: TerminalPage) {
        this.page = page
        ready.countDown()
    }

    override fun onInput(bytes: ByteArray) {
        input.add(bytes)
    }

    override fun onResize(columns: Int, rows: Int) {
        sizes.add(columns to rows)
    }

    override fun onUnavailable() {
        unavailable.countDown()
    }
}

internal class TerminalTestActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(LockedTerminalWebView(this, TerminalTestProbe))
    }
}
