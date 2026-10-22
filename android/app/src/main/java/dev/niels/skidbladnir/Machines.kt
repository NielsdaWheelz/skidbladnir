package dev.niels.skidbladnir

import java.util.Locale
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.cancel
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeoutOrNull

internal sealed interface ExecutionContext {
    data class Local(val cwd: String?, val agent: AgentRuntime?) : ExecutionContext
    data class Remote(val machine: PairedMachine, val cwd: String?, val agent: RemoteAgent?) : ExecutionContext
    data object RemoteUnknown : ExecutionContext
}

internal data class InventorySnapshot(val inventory: SessionsResponse)

internal sealed interface InventoryState {
    data object Reading : InventoryState
    data class Fresh(val snapshot: InventorySnapshot) : InventoryState
    /** A mutation landed on this machine and the snapshot has not caught up with it yet. */
    data class Superseded(val snapshot: InventorySnapshot, val requiredMutationFence: Long) : InventoryState
    data class Stale(val snapshot: InventorySnapshot, val cause: GatewayFailure) : InventoryState
    data class Unreachable(val cause: GatewayFailure) : InventoryState
}

internal sealed interface MachineAccess {
    data object Ready : MachineAccess
    data object AuthRequired : MachineAccess
    data object IdentityChanged : MachineAccess
}

internal fun InventoryState.lastSnapshot(): InventorySnapshot? = when (this) {
    is InventoryState.Fresh -> snapshot
    is InventoryState.Superseded -> snapshot
    is InventoryState.Stale -> snapshot
    InventoryState.Reading, is InventoryState.Unreachable -> null
}

/** The last observed state of [session]'s lifetime, if a snapshot holds it. */
internal fun InventoryState.observedSession(session: TmuxSession): TmuxSession? =
    lastSnapshot()?.inventory?.sessions?.singleOrNull { it.sameLifetime(session) }

/** Single owner of the read-failure downgrade: a failed read never discards another machine's facts. */
internal fun InventoryState.downgraded(cause: GatewayFailure): InventoryState = when (this) {
    is InventoryState.Fresh -> InventoryState.Stale(snapshot, cause)
    is InventoryState.Superseded -> InventoryState.Stale(snapshot, cause)
    is InventoryState.Stale -> InventoryState.Stale(snapshot, cause)
    InventoryState.Reading, is InventoryState.Unreachable -> InventoryState.Unreachable(cause)
}

internal data class MachineState(
    val machine: PairedMachine,
    val access: MachineAccess,
    val inventory: InventoryState,
    val pressure: PressureState,
    val remoteContexts: Map<String, ExecutionContext.Remote> = emptyMap(),
    val notifications: Map<NotificationKey, NotificationPresentation> = emptyMap(),
) {
    val canMutate: Boolean get() = when (access) {
        MachineAccess.Ready -> inventory is InventoryState.Fresh
        MachineAccess.AuthRequired, MachineAccess.IdentityChanged -> false
    }

    fun inventoryFailed(cause: GatewayFailure): MachineState = copy(inventory = inventory.downgraded(cause))
}

internal fun MachineState.executionContext(session: TmuxSession): ExecutionContext {
    val current = (inventory as? InventoryState.Fresh)?.snapshot?.inventory?.sessions?.singleOrNull {
        it.sameLifetime(session)
    }
    val observed = current ?: session
    return if (observed.connection == null) ExecutionContext.Local(observed.cwd, observed.agent)
    else if (access == MachineAccess.Ready && current != null)
        remoteContexts[session.identityToken] ?: ExecutionContext.RemoteUnknown
    else ExecutionContext.RemoteUnknown
}

/** Same bearer to the same origin: machine state, forge drafts and the open terminal survive a fleet change. */
internal fun sameAuthority(a: MachineCredential?, b: MachineCredential?): Boolean =
    a != null && b != null && a.bearer == b.bearer && a.machine.origin == b.machine.origin

/** What a mutation reserves at submit: nothing (ordered only), a fresh inventory, or that plus a metadata fence. */
internal enum class Reservation { None, Inventory, Metadata }

// tmux and host pressure expose no push; the product fixes a five-second foreground cadence.
private const val CADENCE_MILLIS = 5_000L

/** One machine's serial order of mutations and inventory reads. kotlinx Mutex is fair, so the lane is FIFO. */
private class Lane {
    val mutex = Mutex()
    var reserved = 0L
    var completed = 0L
}

/** One foreground span's polling of one credential; cancelling [scope] drops every read and completion it owns. */
private class Poll(val credential: MachineCredential, val scope: CoroutineScope) {
    var reading = false
    var trailing = false
    var started = 0L
    var awaited: Long? = null
    var pressureReading = false
    var contexts: Job? = null
}

/**
 * The phone's machine runtime: per-machine polling, serial mutation lanes and their fences,
 * awaited reads and remote contexts. Main-confined: [app] dispatches on the main thread and
 * nothing here leaves it except suspended http.
 *
 * Two lifetimes: a dispatched mutation's http call lives in [app] and holds its lane until the
 * gateway answers, so no later read overtakes it; its completion, and every read, lives in the
 * poll's scope, which background, credential change and access loss cancel.
 */
internal class Machines(
    private val app: CoroutineScope,
    private val listener: Listener,
) {
    interface Listener {
        /** Any machine fact changed: republish derived ui state. */
        fun changed()
        fun inventoryReadStarting()
        /** A read was applied as Fresh. */
        fun inventoryRead(handle: MachineHandle)
        /** The machine is already downgraded and unpolled. */
        fun accessLost(handle: MachineHandle, failure: GatewayFailure.Api)
    }

    private val credentials = mutableMapOf<MachineHandle, MachineCredential>()
    private val states = mutableMapOf<MachineHandle, MachineState>()
    private val polls = mutableMapOf<MachineHandle, Poll>()
    /** Never dropped: the serial order and its fences span foreground spans and credential changes. */
    private val lanes = mutableMapOf<MachineHandle, Lane>()
    private val metadataFences = mutableMapOf<MachineHandle, Long>()
    private var foreground: CoroutineScope? = null

    val all: List<MachineState> get() = states.values.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) }

    /** An awaited read is outstanding. */
    val refreshing: Boolean get() = polls.values.any { it.awaited != null }

    /** No Ready machine is still on its first read. */
    val settled: Boolean get() = states.values.none {
        it.access == MachineAccess.Ready && it.inventory == InventoryState.Reading
    }

    operator fun get(handle: MachineHandle): MachineState? = states[handle]
    fun credential(handle: MachineHandle): MachineCredential? = credentials[handle]
    fun credentials(): List<MachineCredential> = credentials.values.toList()
    fun polled(handle: MachineHandle): Boolean = handle in polls
    fun canMutate(handle: MachineHandle): Boolean = handle in polls && states[handle]?.canMutate == true

    /** Polls every Ready machine with an origin. */
    fun start(foreground: CoroutineScope) {
        this.foreground = foreground
        for (credential in credentials.values.toList()) {
            if (states[credential.machine.handle]?.access == MachineAccess.Ready) startPoll(credential)
        }
        listener.changed()
    }

    fun stop() {
        foreground = null
        polls.values.forEach { it.scope.cancel() }
        polls.clear()
        metadataFences.clear()
        listener.changed()
    }

    /** Unchanged authority keeps its state; a changed credential restarts its poll. */
    fun install(stored: List<MachineCredential>) {
        val next = stored.sortedBy { it.machine.label.text.lowercase(Locale.ROOT) }.associate { credential ->
            val handle = credential.machine.handle
            val current = states[handle]
            handle to when {
                current != null && sameAuthority(credentials[handle], credential) -> current.copy(machine = credential.machine)
                // Shown, never reachable: no request is ever made without an origin.
                credential.machine.origin == null -> MachineState(credential.machine, MachineAccess.Ready,
                    InventoryState.Unreachable(GatewayFailure.Transport), PressureState.Unavailable(GatewayFailure.Transport))
                else -> MachineState(credential.machine, MachineAccess.Ready, InventoryState.Reading, PressureState.Reading)
            }
        }
        for ((handle, poll) in polls.toList()) {
            if (stored.none { it == poll.credential }) stopPoll(handle)
        }
        credentials.clear()
        stored.forEach { credentials[it.machine.handle] = it }
        states.clear()
        states.putAll(next)
        if (foreground != null) {
            for (credential in stored) {
                val handle = credential.machine.handle
                if (handle !in polls && states.getValue(handle).access == MachineAccess.Ready) startPoll(credential)
            }
        }
        listener.changed()
    }

    /** A fleet (re)connect: every machine starts over. */
    fun replace(connected: List<MachineCredential>) {
        polls.keys.toList().forEach(::stopPoll)
        credentials.clear()
        states.clear()
        install(connected)
    }

    /** Requests a read that starts after now and keeps [refreshing] until it lands. */
    fun refresh(handle: MachineHandle): Boolean {
        val poll = polls[handle] ?: return false
        poll.awaited = requestRead(handle, poll, awaited = true)
        listener.changed()
        return true
    }

    fun refreshAll(): Boolean {
        val handles = polls.keys.toList()
        handles.forEach(::refresh)
        return handles.isNotEmpty()
    }

    /** Callers admit through [canMutate] in the same main-thread step, so the poll exists. */
    fun <T> mutate(
        handle: MachineHandle,
        reservation: Reservation,
        call: suspend (MachineCredential) -> GatewayResult<T>,
        complete: (fence: Long, result: GatewayResult<T>) -> Unit,
    ) {
        val poll = checkNotNull(polls[handle])
        val lane = lanes.getOrPut(handle, ::Lane)
        val fence = ++lane.reserved
        when (reservation) {
            Reservation.None -> Unit
            Reservation.Inventory -> supersede(handle, fence)
            Reservation.Metadata -> {
                check(metadataFences.put(handle, fence) == null)
                supersede(handle, fence)
            }
        }
        val outcome = app.async {
            lane.mutex.withLock {
                try { call(poll.credential) } finally { lane.completed = fence }
            }
        }
        poll.scope.launch {
            val result = outcome.await()
            if (result is GatewayResult.Failure && accessFailure(handle, result.failure)) return@launch
            complete(fence, result)
        }
    }

    /** One lane read outside coalescing, applied like a poll read; access loss is routed instead of [complete]. */
    fun read(handle: MachineHandle, complete: (GatewayResult<SessionsResponse>) -> Unit) {
        val poll = polls[handle] ?: return
        poll.scope.launch { complete(readInLane(handle, poll)) }
    }

    /** A poll-scoped request outside the lane; access loss is routed before [complete]. Null when unpolled. */
    fun <T> request(
        handle: MachineHandle,
        call: suspend (MachineCredential) -> GatewayResult<T>,
        complete: (GatewayResult<T>) -> Unit,
    ): Job? {
        val poll = polls[handle] ?: return null
        return poll.scope.launch {
            val result = call(poll.credential)
            if (result is GatewayResult.Failure && accessFailure(handle, result.failure)) return@launch
            complete(result)
        }
    }

    /** Unauthenticated and machine identity mismatch end the machine's access and its polling. */
    fun accessFailure(handle: MachineHandle, failure: GatewayFailure): Boolean {
        val api = failure as? GatewayFailure.Api ?: return false
        val access = when (api.code) {
            ApiErrorCode.Unauthenticated -> MachineAccess.AuthRequired
            ApiErrorCode.MachineIdentityMismatch -> MachineAccess.IdentityChanged
            else -> return false
        }
        stopPoll(handle)
        update(handle) {
            it.copy(access = access, inventory = it.inventory.downgraded(failure), pressure = it.pressure.downgraded(failure))
        }
        listener.accessLost(handle, api)
        return true
    }

    /** A definite rejection: the reserved mutation changed nothing. */
    fun settle(handle: MachineHandle) = update(handle) { machine ->
        val inventory = machine.inventory as? InventoryState.Superseded ?: return@update machine
        machine.copy(inventory = InventoryState.Fresh(inventory.snapshot))
    }

    fun settleMetadata(handle: MachineHandle, fence: Long) {
        if (metadataFences[handle] != fence) return
        metadataFences.remove(handle)
        update(handle) { machine ->
            val inventory = machine.inventory
            if (inventory is InventoryState.Superseded && inventory.requiredMutationFence == fence) {
                machine.copy(inventory = InventoryState.Fresh(inventory.snapshot))
            } else machine
        }
    }

    fun fail(handle: MachineHandle, failure: GatewayFailure) {
        polls[handle]?.contexts?.cancel()
        update(handle) { it.inventoryFailed(failure).copy(remoteContexts = emptyMap()) }
    }

    /** A closed lifetime leaves the snapshot before the read that confirms it. */
    fun forget(target: SessionTarget) = update(target.machineHandle) { machine ->
        val snapshot = machine.inventory.lastSnapshot() ?: return@update machine
        val trimmed = InventorySnapshot(snapshot.inventory.copy(
            sessions = snapshot.inventory.sessions.filterNot { it.sameLifetime(target.session) },
        ))
        machine.copy(inventory = when (val inventory = machine.inventory) {
            is InventoryState.Fresh -> InventoryState.Fresh(trimmed)
            is InventoryState.Superseded -> InventoryState.Superseded(trimmed, inventory.requiredMutationFence)
            is InventoryState.Stale -> InventoryState.Stale(trimmed, inventory.cause)
            InventoryState.Reading, is InventoryState.Unreachable -> inventory
        })
    }

    private fun startPoll(credential: MachineCredential) {
        val span = foreground ?: return
        if (credential.machine.origin == null) return
        val handle = credential.machine.handle
        stopPoll(handle)
        val poll = Poll(credential, CoroutineScope(span.coroutineContext + Job(span.coroutineContext.job)))
        polls[handle] = poll
        refresh(handle)
        // The body suspends only in delay, so ticks keep a fixed rate; a tick during a read is dropped.
        poll.scope.launch {
            while (true) {
                readPressure(handle, poll)
                delay(CADENCE_MILLIS)
                requestRead(handle, poll, awaited = false)
            }
        }
    }

    private fun stopPoll(handle: MachineHandle) {
        polls.remove(handle)?.scope?.cancel()
        metadataFences.remove(handle)
    }

    /** Starts a read, or coalesces: a tick during a read is dropped; an awaited request queues one trailing read. */
    private fun requestRead(handle: MachineHandle, poll: Poll, awaited: Boolean): Long? {
        if (!poll.reading) {
            poll.reading = true
            val sequence = ++poll.started
            launchRead(handle, poll, sequence)
            return sequence
        }
        if (!awaited) return null
        poll.trailing = true
        return poll.started + 1
    }

    private fun launchRead(handle: MachineHandle, poll: Poll, sequence: Long) = poll.scope.launch {
        listener.inventoryReadStarting()
        readInLane(handle, poll)
        if (poll.awaited?.let { sequence >= it } == true) poll.awaited = null
        poll.reading = false
        if (poll.trailing) {
            poll.trailing = false
            requestRead(handle, poll, awaited = false)
        }
        listener.changed()
    }

    /** One inventory read in [handle]'s lane, applied. Access loss inside it cancels the poll, so it returns only while the poll lives. */
    private suspend fun readInLane(handle: MachineHandle, poll: Poll): GatewayResult<SessionsResponse> {
        val lane = lanes.getOrPut(handle, ::Lane)
        val (fence, result) = lane.mutex.withLock { lane.completed to GatewayClient.listSessions(poll.credential) }
        applyRead(handle, fence, result)
        currentCoroutineContext().ensureActive()
        return result
    }

    /** [fence] is the lane's completed mutation fence when the read started. */
    private fun applyRead(handle: MachineHandle, fence: Long, result: GatewayResult<SessionsResponse>) {
        val machine = states[handle] ?: return
        // Access belongs to the credential, so it is routed whatever the read's order against mutations.
        when (result) {
            is GatewayResult.Failure -> if (accessFailure(handle, result.failure)) return
            is GatewayResult.Success -> if (result.value.machine.handle != handle) {
                accessFailure(handle, identityMismatch())
                return
            }
        }
        val inventory = machine.inventory
        // Started before a reserved mutation finished: it may not show that mutation.
        if (inventory is InventoryState.Superseded && fence < inventory.requiredMutationFence) return
        when (result) {
            is GatewayResult.Failure -> {
                polls[handle]?.contexts?.cancel()
                // A satisfied metadata fence waits for its own read; a failed read does not downgrade it.
                if (metadataFences[handle]?.let { fence >= it } != true) fail(handle, result.failure)
            }
            is GatewayResult.Success -> {
                val response = result.value
                update(handle) {
                    it.copy(
                        access = MachineAccess.Ready,
                        inventory = InventoryState.Fresh(InventorySnapshot(response)),
                        remoteContexts = carried(it.remoteContexts, it.inventory.lastSnapshot()?.inventory, response),
                    )
                }
                metadataFences[handle]?.let { if (fence >= it) metadataFences.remove(handle) }
                listener.inventoryRead(handle)
                resolveContexts(handle, response)
            }
        }
    }

    /** A resolved remote context stays valid while its session keeps the same tmux id and connection. */
    private fun carried(
        contexts: Map<String, ExecutionContext.Remote>,
        from: SessionsResponse?,
        to: SessionsResponse,
    ): Map<String, ExecutionContext.Remote> = contexts.filterKeys { token ->
        val old = from?.sessions?.firstOrNull { it.identityToken == token }
        val now = to.sessions.firstOrNull { it.identityToken == token }
        old != null && now != null && old.tmuxId == now.tmuxId && old.connection == now.connection
    }

    private fun supersede(handle: MachineHandle, fence: Long) {
        polls[handle]?.contexts?.cancel()
        update(handle) { machine ->
            val inventory = machine.inventory as? InventoryState.Fresh ?: return@update machine
            machine.copy(inventory = InventoryState.Superseded(inventory.snapshot, fence))
        }
    }

    private fun readPressure(handle: MachineHandle, poll: Poll) {
        if (poll.pressureReading) return
        poll.pressureReading = true
        poll.scope.launch {
            val result = GatewayClient.readPressure(poll.credential)
            poll.pressureReading = false
            when (result) {
                is GatewayResult.Success -> update(handle) { it.copy(pressure = PressureState.Fresh(result.value)) }
                is GatewayResult.Failure -> if (!accessFailure(handle, result.failure)) {
                    update(handle) { it.copy(pressure = it.pressure.downgraded(result.failure)) }
                }
            }
        }
    }

    /**
     * Follows each connected session through at most eight peer hops to the machine that runs it,
     * within one two-second budget per pass; partial results are kept. Cancelling the job is the fence.
     */
    private fun resolveContexts(handle: MachineHandle, inventory: SessionsResponse) {
        val poll = polls[handle] ?: return
        poll.contexts?.cancel()
        if (inventory.sessions.none { it.connection?.id != null }) return
        val peers = credentials.values.filter { it.machine.origin != null }
        poll.contexts = poll.scope.launch {
            val found = mutableMapOf<String, ExecutionContext.Remote>()
            val lookups = mutableMapOf<String, Pair<PairedMachine, TerminalContext>?>()
            // Exactly one peer must claim the connection id; none or several is unknown.
            suspend fun lookup(id: String): Pair<PairedMachine, TerminalContext>? {
                if (id in lookups) return lookups[id]
                val match = coroutineScope {
                    peers.map { peer ->
                        async { (GatewayClient.readTerminalContext(peer, id) as? GatewayResult.Success)?.let { peer.machine to it.value } }
                    }.awaitAll().filterNotNull().singleOrNull()
                }
                lookups[id] = match
                return match
            }
            withTimeoutOrNull(2_000) {
                for (session in inventory.sessions) {
                    var connection = session.connection
                    val seen = mutableSetOf<String>()
                    while (seen.size < 8) {
                        val id = connection?.id ?: break
                        if (!seen.add(id)) break
                        val (machine, context) = lookup(id) ?: break
                        if (context.connection == null) {
                            found[session.identityToken] = ExecutionContext.Remote(machine, context.cwd, context.agent)
                            break
                        }
                        connection = context.connection
                    }
                }
            }
            if (peers != credentials.values.filter { it.machine.origin != null }) return@launch
            update(handle) { machine ->
                val current = machine.inventory as? InventoryState.Fresh ?: return@update machine
                machine.copy(remoteContexts = carried(found, inventory, current.snapshot.inventory))
            }
        }
    }

    private fun update(handle: MachineHandle, transform: (MachineState) -> MachineState) {
        val machine = states[handle] ?: return
        states[handle] = transform(machine)
        listener.changed()
    }

    private fun identityMismatch() =
        GatewayFailure.Api(code = ApiErrorCode.MachineIdentityMismatch, message = apiErrorMessage(ApiErrorCode.MachineIdentityMismatch))
}
