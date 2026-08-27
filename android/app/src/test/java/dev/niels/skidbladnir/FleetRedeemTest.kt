package dev.niels.skidbladnir

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class FleetRedeemTest {
    @Test
    fun `all three redeems start concurrently settle once and never yield a partial fleet`() {
        val invite = fleetInvite()
        val executor = Executors.newFixedThreadPool(3)
        val started = CountDownLatch(3)
        val release = CountDownLatch(1)
        val calls = ConcurrentHashMap<MachineHandle, AtomicInteger>()
        try {
            val result = redeemFleetInvite(invite, executor) { invited ->
                calls.computeIfAbsent(invited.machine.handle) { AtomicInteger() }.incrementAndGet()
                started.countDown()
                check(release.await(2, TimeUnit.SECONDS))
                if (invited.machine.label.text == "Devbox") {
                    GatewayResult.Failure(GatewayFailure.Transport)
                } else {
                    GatewayResult.Success(
                        PairingResponse(
                            MachineSummary(
                                invited.machine.handle,
                                if (invited.machine.label.text == "MacBook") {
                                    MachinePlatform.Darwin
                                } else {
                                    MachinePlatform.Linux
                                },
                            ),
                            requireNotNull(
                                GatewayBearer.parse(
                                    if (invited.machine.label.text == "Arch") {
                                        "D" + "A".repeat(42)
                                    } else {
                                        "F" + "A".repeat(42)
                                    },
                                ),
                            ),
                        ),
                    )
                }
            }

            assertTrue("redeems were serialized", started.await(2, TimeUnit.SECONDS))
            assertFalse("a partial fleet settled before every host", result.isDone)
            release.countDown()

            assertNull(result.get(2, TimeUnit.SECONDS))
            assertEquals(3, calls.size)
            assertTrue(calls.values.all { it.get() == 1 })
        } finally {
            release.countDown()
            executor.shutdownNow()
        }
    }

    private fun fleetInvite(): FleetInvite {
        val labels = listOf("Arch", "Devbox", "MacBook")
        return FleetInvite(
            labels.mapIndexed { index, label ->
                FleetInviteMachine(
                    PairedMachine(
                        requireNotNull(MachineHandle.parse("mh-${(index + 1).toString().repeat(32)}")),
                        requireNotNull(MachineLabel.parse(label)),
                        requireNotNull(MachineOrigin.parse("https://${label.lowercase()}.example.ts.net:8443/")),
                    ),
                    requireNotNull(
                        PairingInviteToken.parse(('A'.code + index).toChar() + "A".repeat(42)),
                    ),
                )
            },
        )
    }
}
