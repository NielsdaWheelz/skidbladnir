package dev.niels.skidbladnir

import android.content.Context
import android.content.ContextWrapper
import android.content.SharedPreferences
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MachineStoreInstrumentedTest {
    @Test
    fun concurrentRemovalsAcrossStoreInstancesLeaveNoMachines() {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val testPreferences = "skidbladnir.machines.concurrent-instrumented-test"
        val preferences = CoordinatedPreferences(
            targetContext.getSharedPreferences(testPreferences, Context.MODE_PRIVATE),
        )
        val context = object : ContextWrapper(targetContext) {
            override fun getSharedPreferences(name: String?, mode: Int) = preferences
        }
        val firstStore = MachineStore(context, "skidbladnir.machine-bearers.concurrent-instrumented-test")
        val secondStore = MachineStore(context, "skidbladnir.machine-bearers.concurrent-instrumented-test")
        val devbox = credential(
            "mh-0123456789abcdef0123456789abcdef",
            "Devbox",
            "https://devbox.example.ts.net:8443",
            "A".repeat(43),
        )
        val macBook = credential(
            "mh-fedcba9876543210fedcba9876543210",
            "MacBook",
            "https://macbook.example.ts.net:8443",
            "B" + "A".repeat(42),
        )
        val executor = Executors.newFixedThreadPool(2)

        try {
            firstStore.resetAll()
            firstStore.add(devbox)
            firstStore.add(macBook)
            preferences.coordinateTwoCollectionReads()

            val first = executor.submit { firstStore.remove(devbox.machine.handle) }
            assertTrue("first removal did not read storage", preferences.firstRead.await(5, TimeUnit.SECONDS))

            val secondThread = AtomicReference<Thread>()
            val secondAttempting = CountDownLatch(1)
            val second = executor.submit {
                secondThread.set(Thread.currentThread())
                secondAttempting.countDown()
                secondStore.remove(macBook.machine.handle)
            }
            assertTrue("second removal did not start", secondAttempting.await(5, TimeUnit.SECONDS))

            // Activity replacement can create another store while the old controller still commits.
            // BLOCKED only proves that the second removal reached the shared serialization boundary.
            val deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (
                preferences.secondRead.count != 0L &&
                secondThread.get().state != Thread.State.BLOCKED &&
                System.nanoTime() < deadline
            ) {
                Thread.yield()
            }
            assertTrue(
                "second removal neither read concurrently nor blocked on store serialization",
                preferences.secondRead.count == 0L || secondThread.get().state == Thread.State.BLOCKED,
            )
            preferences.releaseFirst.countDown()
            first.get(5, TimeUnit.SECONDS)
            second.get(5, TimeUnit.SECONDS)

            assertTrue(firstStore.readAll().isEmpty())
        } finally {
            preferences.releaseFirst.countDown()
            executor.shutdownNow()
            firstStore.resetAll()
        }
    }

    @Test
    fun twoMachineBearersRoundTripEncryptedWithFreshNonceAndBoundAad() {
        val targetContext = InstrumentationRegistry.getInstrumentation().targetContext
        val testPreferences = "skidbladnir.machines.instrumented-test"
        val context = object : ContextWrapper(targetContext) {
            override fun getSharedPreferences(name: String?, mode: Int) =
                targetContext.getSharedPreferences(testPreferences, mode)
        }
        val store = MachineStore(context, "skidbladnir.machine-bearers.instrumented-test")
        val devbox = credential(
            "mh-0123456789abcdef0123456789abcdef",
            "Devbox",
            "https://devbox.example.ts.net:8443",
            "A".repeat(43),
        )
        val macBook = credential(
            "mh-fedcba9876543210fedcba9876543210",
            "MacBook",
            "https://macbook.example.ts.net:8443",
            "B" + "A".repeat(42),
        )

        try {
            store.resetAll()
            store.add(devbox)
            store.add(macBook)

            val restored = store.readAll()
            assertEquals(listOf(devbox.machine.handle, macBook.machine.handle), restored.map { it.machine.handle })
            assertTrue(restored.zip(listOf(devbox, macBook)).all { (actual, expected) -> actual == expected })
            val preferences = context.getSharedPreferences(testPreferences, Context.MODE_PRIVATE)
            val persisted = preferences.all.values.joinToString()
            assertFalse(persisted.contains(devbox.bearer.encoded))
            assertFalse(persisted.contains(macBook.bearer.encoded))

            val nonceKey = "machine.${devbox.machine.handle.encoded}.nonce"
            val firstNonce = preferences.getString(nonceKey, null)
            val rotatedBearer = requireNotNull(GatewayBearer.parse("C" + "A".repeat(42)))
            store.rotateBearer(devbox.copy(bearer = rotatedBearer))
            assertTrue(firstNonce != preferences.getString(nonceKey, null))

            val renamedLabel = requireNotNull(MachineLabel.parse("Build Mac"))
            store.rename(devbox.machine.handle, renamedLabel)
            val renamed = store.readAll().single { it.machine.handle == devbox.machine.handle }
            assertTrue(
                renamed.machine.label == renamedLabel &&
                    renamed.machine.handle == devbox.machine.handle &&
                    renamed.machine.origin == devbox.machine.origin &&
                    renamed.bearer == rotatedBearer,
            )
            assertThrows(IllegalArgumentException::class.java) {
                store.rename(devbox.machine.handle, requireNotNull(MachineLabel.parse("macbook")))
            }

            store.remove(devbox.machine.handle)
            val remaining = store.readAll()
            assertTrue(remaining == listOf(macBook))

            preferences.edit()
                .putString(
                    "machine.${macBook.machine.handle.encoded}.origin",
                    "https://changed.example.ts.net:8443/",
                )
                .commit()
            assertThrows(Exception::class.java) { store.readAll() }
        } finally {
            store.resetAll()
        }
    }

    private fun credential(handle: String, label: String, origin: String, bearer: String): MachineCredential =
        MachineCredential(
            PairedMachine(
                requireNotNull(MachineHandle.parse(handle)),
                requireNotNull(MachineLabel.parse(label)),
                requireNotNull(MachineOrigin.parse(origin)),
            ),
            requireNotNull(GatewayBearer.parse(bearer)),
        )

    private class CoordinatedPreferences(
        private val delegate: SharedPreferences,
    ) : SharedPreferences by delegate {
        val firstRead = CountDownLatch(1)
        val releaseFirst = CountDownLatch(1)
        val secondRead = CountDownLatch(1)
        private val coordinatedReads = AtomicInteger(0)

        fun coordinateTwoCollectionReads() {
            coordinatedReads.set(2)
        }

        override fun getStringSet(key: String?, defValues: MutableSet<String>?): MutableSet<String>? {
            val snapshot = delegate.getStringSet(key, defValues)?.toMutableSet()
            when (coordinatedReads.getAndUpdate { remaining -> if (remaining > 0) remaining - 1 else 0 }) {
                2 -> {
                    firstRead.countDown()
                    check(releaseFirst.await(5, TimeUnit.SECONDS)) { "first index read was not released" }
                }
                1 -> secondRead.countDown()
            }
            return snapshot
        }
    }
}
