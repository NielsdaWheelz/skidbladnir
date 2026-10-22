package dev.niels.skidbladnir

import android.content.Context
import android.util.AtomicFile
import java.io.File
import java.io.IOException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

internal const val TERMINAL_TEXT_SIZE_DEFAULT_SP = 16
internal val TERMINAL_TEXT_SIZE_RANGE_SP = 12..24

internal enum class TerminalTextSizeWrite { Idle, Pending, Failed }

internal sealed interface TerminalTextSizeState {
    data object Reading : TerminalTextSizeState
    data object Unavailable : TerminalTextSizeState

    /** The sheet edits the committed value, so its openness lives with that value. */
    data class Ready(
        val nominalSp: Int,
        val write: TerminalTextSizeWrite,
        val sheetOpen: Boolean,
    ) : TerminalTextSizeState
}

/**
 * The phone-wide terminal text size. Only a committed value is shown or applied: the saved value is read once
 * per owner and again only after a failed read, and a write shows once it is durable; a failed write keeps the
 * prior value. The file holds exactly two ASCII digits in 12..24, at the path and in the format the earlier
 * DataStore store used. Main-confined: [changed] runs after every transition.
 */
internal class TerminalTextSize(context: Context, private val scope: CoroutineScope, private val changed: () -> Unit) {
    private val file = AtomicFile(File(context.filesDir, "datastore/terminal-text-size"))

    var state: TerminalTextSizeState = TerminalTextSizeState.Reading
        private set

    fun read() {
        set(TerminalTextSizeState.Reading)
        scope.launch {
            set(try {
                TerminalTextSizeState.Ready(withContext(Dispatchers.IO) { load() }, TerminalTextSizeWrite.Idle, sheetOpen = false)
            } catch (_: IOException) {
                TerminalTextSizeState.Unavailable
            })
        }
    }

    /** A failed write is reported for that write only, so a new attempt or sheet session opens clean. */
    fun sheet(open: Boolean) {
        val ready = state as? TerminalTextSizeState.Ready ?: return
        set(ready.copy(sheetOpen = open, write = if (ready.write == TerminalTextSizeWrite.Failed) TerminalTextSizeWrite.Idle else ready.write))
    }

    /** Callers admit the step; [apply] receives the value once it is committed. */
    fun write(nominalSp: Int, apply: (Int) -> Unit) {
        val ready = state as? TerminalTextSizeState.Ready ?: return
        set(ready.copy(write = TerminalTextSizeWrite.Pending))
        scope.launch {
            // The sheet may open or close meanwhile; a pending write keeps the value Ready (only Unavailable is re-read).
            set(try {
                withContext(Dispatchers.IO) { file.writeAtomically(nominalSp.toString().toByteArray(Charsets.US_ASCII)) }
                apply(nominalSp)
                (state as TerminalTextSizeState.Ready).copy(nominalSp = nominalSp, write = TerminalTextSizeWrite.Idle)
            } catch (_: IOException) {
                (state as TerminalTextSizeState.Ready).copy(write = TerminalTextSizeWrite.Failed)
            })
        }
    }

    private fun load(): Int {
        if (!file.baseFile.exists()) return TERMINAL_TEXT_SIZE_DEFAULT_SP
        val bytes = file.openRead().use { it.readNBytes(3) }
        if (bytes.size != 2 || bytes.any { it !in '0'.code.toByte()..'9'.code.toByte() }) {
            throw IOException("terminal text size is not two ASCII digits")
        }
        val nominalSp = (bytes[0] - '0'.code.toByte()) * 10 + (bytes[1] - '0'.code.toByte())
        if (nominalSp !in TERMINAL_TEXT_SIZE_RANGE_SP) throw IOException("terminal text size is out of range")
        return nominalSp
    }

    private fun set(next: TerminalTextSizeState) {
        state = next
        changed()
    }
}
