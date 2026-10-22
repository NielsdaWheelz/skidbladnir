package dev.niels.skidbladnir

import android.util.AtomicFile
import java.io.IOException

/** The phone's one durable replace of a private file: startWrite, write, fsync, rename. A failed sync or write leaves the old file. */
internal fun AtomicFile.writeAtomically(bytes: ByteArray) {
    val stream = startWrite()
    try {
        stream.write(bytes)
        stream.fd.sync()
        finishWrite(stream)
    } catch (failure: IOException) {
        failWrite(stream)
        throw failure
    }
}
