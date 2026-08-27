package dev.niels.skidbladnir

internal enum class FleetPersistenceDisposition {
    Connected,
    RetryWithFreshInvite,
    ResetRequired,
}

internal fun fleetInstallationDisposition(
    result: FleetInstallation,
    durable: MachineStoreRead,
    pending: List<MachineCredential>,
): FleetPersistenceDisposition = when (result) {
    FleetInstallation.Installed -> FleetPersistenceDisposition.Connected
    FleetInstallation.StoreNotEmpty -> FleetPersistenceDisposition.ResetRequired
    FleetInstallation.InvalidFleet -> throw IllegalStateException("validated fleet was rejected by storage")
    FleetInstallation.StorageUnavailable ->
        resumedFleetPersistenceDisposition(FleetConnectMode.Install, pending, durable)
}

internal fun fleetReconnectionDisposition(
    result: FleetReconnection,
    durable: MachineStoreRead,
    pending: List<MachineCredential>,
): FleetPersistenceDisposition = when (result) {
    FleetReconnection.Reconnected -> FleetPersistenceDisposition.Connected
    FleetReconnection.FleetMismatch -> FleetPersistenceDisposition.ResetRequired
    FleetReconnection.StorageUnavailable ->
        resumedFleetPersistenceDisposition(FleetConnectMode.Reconnect, pending, durable)
}

internal fun resumedFleetPersistenceDisposition(
    mode: FleetConnectMode,
    pending: List<MachineCredential>,
    durable: MachineStoreRead,
): FleetPersistenceDisposition {
    if (durable.unreadable.isNotEmpty()) return FleetPersistenceDisposition.ResetRequired
    if (durable.credentials.size == pending.size &&
        durable.credentials.toSet() == pending.toSet()
    ) return FleetPersistenceDisposition.Connected
    return when (mode) {
        FleetConnectMode.Install -> if (durable.credentials.isEmpty()) {
            FleetPersistenceDisposition.RetryWithFreshInvite
        } else {
            FleetPersistenceDisposition.ResetRequired
        }
        FleetConnectMode.Reconnect -> if (
            durable.credentials.size == 3 && pending.size == 3 &&
            durable.credentials.map { it.machine }.toSet() == pending.map { it.machine }.toSet()
        ) {
            FleetPersistenceDisposition.RetryWithFreshInvite
        } else {
            FleetPersistenceDisposition.ResetRequired
        }
    }
}

internal fun reconnectInviteMatchesInstalled(
    invite: FleetInvite,
    installed: Collection<MachineCredential>,
): Boolean {
    val labelOrder = mapOf("Arch" to 0, "Devbox" to 1, "MacBook" to 2)
    val ordered = installed.sortedBy { labelOrder[it.machine.label.text] ?: Int.MAX_VALUE }
    return ordered.size == 3 && ordered.map { it.machine } == invite.machines.map { it.machine }
}
