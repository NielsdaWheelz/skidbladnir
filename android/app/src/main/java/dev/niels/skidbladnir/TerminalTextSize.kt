package dev.niels.skidbladnir

import android.content.Context
import androidx.datastore.core.CorruptionException
import androidx.datastore.core.DataStore
import androidx.datastore.core.Serializer
import androidx.datastore.dataStore
import java.io.IOException
import java.io.InputStream
import java.io.OutputStream
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

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

/** The at-rest format: exactly two ASCII digits in 12..24 followed by EOF. */
private object TerminalTextSizeSerializer : Serializer<Int> {
    override val defaultValue: Int = TERMINAL_TEXT_SIZE_DEFAULT_SP

    override suspend fun readFrom(input: InputStream): Int {
        val bytes = input.readNBytes(3)
        if (bytes.size != 2 || bytes.any { it !in '0'.code.toByte()..'9'.code.toByte() }) {
            throw CorruptionException("terminal text size is not two ASCII digits")
        }
        val nominalSp = (bytes[0] - '0'.code.toByte()) * 10 + (bytes[1] - '0'.code.toByte())
        if (nominalSp !in TERMINAL_TEXT_SIZE_RANGE_SP) throw CorruptionException("terminal text size is out of range")
        return nominalSp
    }

    override suspend fun writeTo(nominalSp: Int, output: OutputStream) {
        // justify-service-invariant-check: Int cannot carry the two-digit at-rest range, and this
        // serializer owns both ends of the format its readFrom enforces.
        require(nominalSp in TERMINAL_TEXT_SIZE_RANGE_SP)
        output.write(nominalSp.toString().toByteArray(Charsets.US_ASCII))
    }
}

private val Context.terminalTextSizeDataStore: DataStore<Int> by dataStore(
    fileName = "terminal-text-size",
    serializer = TerminalTextSizeSerializer,
)

/** The app-scoped saved text size; callbacks land on the main looper. */
internal class TerminalTextSizeStore(context: Context) {
    private val dataStore = context.applicationContext.terminalTextSizeDataStore
    private val scope = CoroutineScope(Dispatchers.Main.immediate + SupervisorJob())

    // DataStore reports every read and write failure, corruption included, as an
    // IOException; the product models those as Unavailable and a failed write.
    fun read(onReady: (nominalSp: Int) -> Unit, onUnavailable: () -> Unit) {
        scope.launch {
            val nominalSp = try {
                dataStore.data.first()
            } catch (_: IOException) { // justify-ignore-error: modeled as Unavailable with an explicit reread.
                onUnavailable()
                return@launch
            }
            onReady(nominalSp)
        }
    }

    fun write(nominalSp: Int, onCommitted: () -> Unit, onFailed: () -> Unit) {
        scope.launch {
            try {
                dataStore.updateData { nominalSp }
            } catch (_: IOException) { // justify-ignore-error: modeled as a failed write; the prior value stays committed.
                onFailed()
                return@launch
            }
            onCommitted()
        }
    }

    fun close() {
        scope.cancel()
    }
}
