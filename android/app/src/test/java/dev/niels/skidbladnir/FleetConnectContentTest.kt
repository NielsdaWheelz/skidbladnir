package dev.niels.skidbladnir

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class FleetConnectContentTest {
    @Test
    fun `install flow freezes truthful accessible copy for every state`() {
        val ready = fleetConnectContent(FleetConnectMode.Install, FleetConnectPhase.Ready)
        assertEquals("Connect your fleet", ready.title)
        assertEquals(
            "Sign in to Tailscale, then scan a fresh fleet invite from your MacBook.",
            ready.body,
        )
        assertEquals("Connect", ready.primaryAction)
        assertEquals(
            "Skíðblaðnir opens Tailscale but cannot sign in or control the VPN for you.",
            ready.externalBoundary,
        )
        assertEquals(null, ready.progress)
        assertEquals(null, ready.failure)

        assertEquals(
            "Scanning a fresh fleet invite…",
            fleetConnectContent(FleetConnectMode.Install, FleetConnectPhase.Scanning).progress,
        )
        assertEquals(
            "Connecting to 3 machines…",
            fleetConnectContent(FleetConnectMode.Install, FleetConnectPhase.Connecting).progress,
        )
        assertEquals(
            "Couldn’t connect the whole fleet. Nothing was saved. Create and scan a new fleet invite.",
            fleetConnectContent(FleetConnectMode.Install, FleetConnectPhase.Failed).failure,
        )
    }

    @Test
    fun `reconnect copy promises only exact fleet bearer rotation`() {
        val ready = fleetConnectContent(FleetConnectMode.Reconnect, FleetConnectPhase.Ready)

        assertEquals("Reconnect fleet", ready.title)
        assertEquals(
            "Scan a fresh fleet invite from your MacBook to reconnect the exact installed machines.",
            ready.body,
        )
        assertEquals("Reconnect fleet", ready.primaryAction)
        assertEquals(
            "Couldn’t connect the whole fleet. Nothing was saved. Create and scan a new fleet invite.",
            fleetConnectContent(FleetConnectMode.Reconnect, FleetConnectPhase.Failed).failure,
        )
    }

    @Test
    fun `reset-required state is truthful and cannot offer another scan`() {
        val content = fleetConnectContent(FleetConnectMode.Reconnect, FleetConnectPhase.ResetRequired)

        assertEquals("Fleet reset required", content.title)
        assertEquals(
            "Saved fleet credentials cannot be repaired in this app. Reset the app’s data, then connect again.",
            content.body,
        )
        assertNull(content.primaryAction)
        assertNull(content.externalBoundary)
        assertFalse(content.primaryEnabled)
        assertFalse(content.tailscaleActionVisible)
    }

    @Test
    fun `only a healthy invite mismatch may return to the installed fleet`() {
        assertTrue(
            fleetReconnectCanCancel(
                SkidbladnirUiState.FleetConnect(
                    FleetConnectMode.Reconnect,
                    FleetConnectPhase.ResetRequired,
                    FleetResetReason.InviteIdentityMismatch,
                ),
            ),
        )
        assertFalse(
            fleetReconnectCanCancel(
                SkidbladnirUiState.FleetConnect(
                    FleetConnectMode.Reconnect,
                    FleetConnectPhase.ResetRequired,
                    FleetResetReason.StoredFleetUnusable,
                ),
            ),
        )
    }
}
