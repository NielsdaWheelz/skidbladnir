package dev.niels.skidbladnir

import java.io.IOException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class FleetPersistenceTest {
    private val installed = credentials('A')
    private val rotated = credentials('D')
    private val emptyRead = MachineStoreRead(emptyList(), emptyList())
    private val installedRead = MachineStoreRead(installed, emptyList())
    private val quarantinedRead = MachineStoreRead(emptyList(), List(3) { UnreadableStoredMachine() })

    @Test
    fun `checked keystore load failure becomes an ordinary unavailable seal`() {
        assertNull(
            sealFleetOrNull(installed) { _, _ ->
                throw IOException("fixture keystore load failure")
            },
        )
    }

    @Test
    fun `every installation and reconnection result has one finite product disposition`() {
        assertEquals(
            FleetPersistenceDisposition.Connected,
            fleetInstallationDisposition(FleetInstallation.Installed, emptyRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            fleetInstallationDisposition(FleetInstallation.StoreNotEmpty, installedRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.RetryWithFreshInvite,
            fleetInstallationDisposition(FleetInstallation.StorageUnavailable, emptyRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            fleetInstallationDisposition(FleetInstallation.StorageUnavailable, quarantinedRead, rotated),
        )
        assertThrows(IllegalStateException::class.java) {
            fleetInstallationDisposition(FleetInstallation.InvalidFleet, emptyRead, rotated)
        }

        assertEquals(
            FleetPersistenceDisposition.Connected,
            fleetReconnectionDisposition(FleetReconnection.Reconnected, installedRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            fleetReconnectionDisposition(FleetReconnection.FleetMismatch, installedRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.RetryWithFreshInvite,
            fleetReconnectionDisposition(FleetReconnection.StorageUnavailable, installedRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            fleetReconnectionDisposition(FleetReconnection.StorageUnavailable, quarantinedRead, rotated),
        )
        assertEquals(
            FleetPersistenceDisposition.Connected,
            fleetInstallationDisposition(
                FleetInstallation.StorageUnavailable,
                MachineStoreRead(rotated, emptyList()),
                rotated,
            ),
        )
        assertEquals(
            FleetPersistenceDisposition.Connected,
            fleetReconnectionDisposition(
                FleetReconnection.StorageUnavailable,
                MachineStoreRead(rotated, emptyList()),
                rotated,
            ),
        )
    }

    @Test
    fun `resume distinguishes a durable commit from pre-mutation cancellation and quarantine`() {
        assertEquals(
            FleetPersistenceDisposition.Connected,
            resumedFleetPersistenceDisposition(
                FleetConnectMode.Reconnect,
                rotated,
                MachineStoreRead(rotated, emptyList()),
            ),
        )
        assertEquals(
            FleetPersistenceDisposition.RetryWithFreshInvite,
            resumedFleetPersistenceDisposition(FleetConnectMode.Reconnect, rotated, installedRead),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            resumedFleetPersistenceDisposition(FleetConnectMode.Reconnect, rotated, quarantinedRead),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            resumedFleetPersistenceDisposition(FleetConnectMode.Install, rotated, installedRead),
        )
        assertEquals(
            FleetPersistenceDisposition.ResetRequired,
            resumedFleetPersistenceDisposition(
                FleetConnectMode.Reconnect,
                rotated,
                MachineStoreRead(installed.take(1), emptyList()),
            ),
        )
    }

    @Test
    fun `reconnect rejects changed identities before consuming invitations`() {
        val invite = FleetInvite(
            installed.mapIndexed { index, credential ->
                FleetInviteMachine(
                    credential.machine,
                    requireNotNull(PairingInviteToken.parse(('A'.code + index).toChar() + "A".repeat(42))),
                )
            },
        )
        assertTrue(reconnectInviteMatchesInstalled(invite, installed))
        assertFalse(
            reconnectInviteMatchesInstalled(
                invite.copy(
                    machines = invite.machines.toMutableList().also { machines ->
                        machines[0] = machines[0].copy(
                            machine = machines[0].machine.copy(
                                origin = requireNotNull(MachineOrigin.parse("https://replacement.example.ts.net:8443/")),
                            ),
                        )
                    },
                ),
                installed,
            ),
        )
    }

    private fun credentials(firstBearerPrefix: Char): List<MachineCredential> =
        listOf("Arch", "Devbox", "MacBook").mapIndexed { index, label ->
            MachineCredential(
                PairedMachine(
                    requireNotNull(MachineHandle.parse("mh-${(index + 1).toString().repeat(32)}")),
                    requireNotNull(MachineLabel.parse(label)),
                    requireNotNull(MachineOrigin.parse("https://${label.lowercase()}.example.ts.net:8443/")),
                ),
                requireNotNull(GatewayBearer.parse((firstBearerPrefix.code + index).toChar() + "A".repeat(42))),
            )
        }
}
