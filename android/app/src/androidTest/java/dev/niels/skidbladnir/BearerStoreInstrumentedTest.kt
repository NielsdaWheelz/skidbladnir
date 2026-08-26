package dev.niels.skidbladnir

import android.content.Context
import android.content.ContextWrapper
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class BearerStoreInstrumentedTest {
    @Test
    fun validatedBearerRoundTripsWithoutPlaintextPreferences() {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val context = object : ContextWrapper(targetContext) {
            override fun getSharedPreferences(name: String?, mode: Int) =
                targetContext.getSharedPreferences("skidbladnir.pairing.instrumented-test", mode)
        }
        val store = BearerStore(context)
        val sentinel = "instrumentation-only-bearer-sentinel"

        try {
            store.clear()
            store.clear()
            store.write(sentinel)

            assertEquals(sentinel, store.read())
            val persisted = context
                .getSharedPreferences("skidbladnir.pairing", Context.MODE_PRIVATE)
                .all
                .values
                .joinToString()
            assertFalse("encrypted preferences contained the plaintext bearer", persisted.contains(sentinel))
        } finally {
            store.clear()
            store.clear()
        }
    }
}
