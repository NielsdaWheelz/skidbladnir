package dev.niels.skidbladnir

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

internal sealed interface SkidbladnirUiState {
    data object Booting : SkidbladnirUiState

    data class Pairing(
        val draft: String,
        val pending: Boolean,
        val error: String?,
    ) : SkidbladnirUiState

    data class Dashboard(
        val inventory: SessionsResponse?,
        val pressure: PressureResponse?,
        val inventoryStale: Boolean,
        val inventoryAgeAdvanceSeconds: Long,
        val refreshing: Boolean,
        val error: String?,
        val forge: ForgeState?,
        val forgeRecovery: ForgeRecovery?,
        val kill: KillState?,
    ) : SkidbladnirUiState

    data class Terminal(
        val session: AgentSession,
        val attempt: Int,
        val connection: TerminalUiStatus,
        val kill: KillState?,
    ) : SkidbladnirUiState
}

internal data class ForgeState(
    val draft: ForgeDraft,
    val pending: Boolean,
    val error: String?,
)

internal sealed interface ForgeRecovery {
    val draft: ForgeDraft

    data class RefreshRequired(override val draft: ForgeDraft) : ForgeRecovery
    data class ReviewReady(override val draft: ForgeDraft) : ForgeRecovery
}

internal data class KillState(
    val target: AgentSession,
    val pending: Boolean,
    val error: String?,
)

internal sealed interface TerminalUiStatus {
    data object Preparing : TerminalUiStatus
    data object Connecting : TerminalUiStatus

    data class Connected(
        val attachedClients: Int,
        val geometry: TerminalGeometry,
    ) : TerminalUiStatus

    data class ReconnectRequired(val message: String) : TerminalUiStatus
}

internal data class ForgeCarry(
    val forge: ForgeState?,
    val recovery: ForgeRecovery?,
)

internal fun forgeCarry(state: SkidbladnirUiState): ForgeCarry {
    val dashboard = state as? SkidbladnirUiState.Dashboard ?: return ForgeCarry(null, null)
    val forge = dashboard.forge
    return if (forge?.pending == true) {
        ForgeCarry(null, ForgeRecovery.RefreshRequired(forge.draft))
    } else {
        ForgeCarry(forge, dashboard.forgeRecovery)
    }
}

internal fun pressurePollRequiresPairing(result: GatewayResult<PressureResponse>): Boolean =
    result is GatewayResult.Failure &&
        result.failure == GatewayFailure.Api(ApiErrorCode.Unauthenticated)

internal class SkidbladnirController(context: Context) {
    var state: SkidbladnirUiState by mutableStateOf(SkidbladnirUiState.Booting)
        private set

    private val main = Handler(Looper.getMainLooper())
    private val worker = Executors.newSingleThreadScheduledExecutor { task ->
        Thread(task, "skidbladnir-client").apply { isDaemon = true }
    }
    private val bearerStore = BearerStore(context.applicationContext)
    private val client = GatewayClient()
    @Volatile
    private var bearer: GatewayBearer? = null
    private var inventory: SessionsResponse? = null
    private var inventoryReceivedAtElapsedMillis: Long? = null
    private var pressure: PressureResponse? = null
    @Volatile
    private var foreground = false
    @Volatile
    private var foregroundGeneration = 0L
    private var poller: ScheduledFuture<*>? = null
    private var terminalConnection: TerminalConnection? = null
    private var terminalPage: TerminalPage? = null
    private var terminalOwner: Any? = null
    private var nextTerminalAttempt = 1

    fun start() {
        if (foreground) return
        foreground = true
        val generation = ++foregroundGeneration
        if (bearer != null) {
            showDashboard(refreshing = true)
            startPolling(requireNotNull(bearer), generation, immediately = true)
            return
        }
        state = SkidbladnirUiState.Booting
        executeClient {
            val storedText = try {
                bearerStore.read()
            } catch (_: Exception) {
                val reset = try {
                    bearerStore.reset()
                    true
                } catch (_: Exception) {
                    false
                }
                main.post {
                    state = SkidbladnirUiState.Pairing(
                        draft = "",
                        pending = false,
                        error = if (reset) {
                            "Stored pairing could not be read and was cleared. Enter the bearer again."
                        } else {
                            "Stored pairing could not be read or cleared."
                        },
                    )
                }
                return@executeClient
            }
            val stored = storedText?.let { GatewayBearer.parse(it) }
            if (storedText != null && stored == null) {
                val reset = try {
                    bearerStore.reset()
                    true
                } catch (_: Exception) {
                    false
                }
                main.post {
                    if (!isForegroundGeneration(generation)) return@post
                    state = SkidbladnirUiState.Pairing(
                        draft = "",
                        pending = false,
                        error = if (reset) {
                            "Stored pairing was invalid and was cleared. Enter the bearer again."
                        } else {
                            "Stored pairing was invalid and could not be cleared."
                        },
                    )
                }
                return@executeClient
            }
            main.post {
                if (!isForegroundGeneration(generation)) return@post
                if (stored == null) {
                    state = SkidbladnirUiState.Pairing("", pending = false, error = null)
                } else {
                    bearer = stored
                    showDashboard(refreshing = true)
                    startPolling(stored, generation, immediately = true)
                }
            }
        }
    }

    fun stopForBackground() {
        if (!foreground) return
        foreground = false
        foregroundGeneration += 1
        poller?.cancel(false)
        poller = null
        leaveTerminal()
        state = when (val current = state) {
            is SkidbladnirUiState.Terminal -> dashboardState(refreshing = false)
            is SkidbladnirUiState.Dashboard -> current.forge
                ?.takeIf(ForgeState::pending)
                ?.let { forge ->
                    current.copy(
                        inventoryStale = current.inventory != null,
                        refreshing = false,
                        forge = null,
                        forgeRecovery = ForgeRecovery.RefreshRequired(forge.draft),
                    )
                }
                ?: current
            else -> current
        }
    }

    fun close() {
        stopForBackground()
        leaveTerminal()
        worker.shutdownNow()
        client.http.dispatcher.executorService.shutdown()
        client.http.connectionPool.evictAll()
    }

    fun updatePairingDraft(value: String) {
        val current = state as? SkidbladnirUiState.Pairing ?: return
        state = current.copy(draft = value, error = null)
    }

    fun pair() {
        val current = state as? SkidbladnirUiState.Pairing ?: return
        if (current.pending) return
        val candidate = GatewayBearer.parse(current.draft)
        if (candidate == null) {
            state = current.copy(
                error = "Enter the 43-character bearer exactly as minted on the devbox.",
            )
            return
        }
        val generation = foregroundGeneration
        state = current.copy(pending = true, error = null)
        executeClient {
            when (val result = client.listSessions(candidate)) {
                is GatewayResult.Success -> {
                    val inventoryReceivedAt = SystemClock.elapsedRealtime()
                    if (!isForegroundGeneration(generation)) return@executeClient
                    try {
                        bearerStore.write(candidate.encoded)
                    } catch (_: Exception) {
                        val reset = try {
                            bearerStore.reset()
                            true
                        } catch (_: Exception) {
                            false
                        }
                        main.post {
                            if (!isForegroundGeneration(generation)) return@post
                            val pairing = state as? SkidbladnirUiState.Pairing ?: return@post
                            state = pairing.copy(
                                pending = false,
                                error = if (reset) {
                                    "Pairing worked, but secure storage failed and was reset. Try again."
                                } else {
                                    "Pairing worked, but secure storage failed and could not be reset."
                                },
                            )
                        }
                        return@executeClient
                    }
                    main.post {
                        if (!isForegroundGeneration(generation)) return@post
                        bearer = candidate
                        inventory = result.value
                        inventoryReceivedAtElapsedMillis = inventoryReceivedAt
                        showDashboard(refreshing = false, inventoryStale = false)
                        startPolling(candidate, generation, immediately = true)
                    }
                }
                is GatewayResult.Failure -> main.post {
                    if (!isForegroundGeneration(generation)) return@post
                    val pairing = state as? SkidbladnirUiState.Pairing ?: return@post
                    state = pairing.copy(
                        pending = false,
                        error = when (result.failure) {
                            is GatewayFailure.Api -> gatewayFailureMessage(result.failure)
                            GatewayFailure.Transport -> "Could not reach Skíðblaðnir over your Tailnet."
                        },
                    )
                }
            }
        }
    }

    fun refresh() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val activeBearer = bearer ?: return
        val generation = foregroundGeneration
        if (current.refreshing) return
        state = current.copy(pressure = null, refreshing = true, error = null)
        executeClient { poll(activeBearer, generation) }
    }

    fun openForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val profiles = current.inventory?.profiles.orEmpty()
        if (profiles.isEmpty()) return
        if (current.forgeRecovery is ForgeRecovery.RefreshRequired) return
        val draft = current.forgeRecovery?.draft
            ?: ForgeDraft(cwd = "", profile = profiles.first().key, optionalName = "", objective = "")
        state = current.copy(
            forgeRecovery = null,
            forge = ForgeState(
                draft = draft,
                pending = false,
                error = null,
            ),
        )
    }

    fun dismissForge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.forge?.pending == true) return
        state = current.copy(forge = null)
    }

    fun discardForgeRecovery() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        if (current.forgeRecovery is ForgeRecovery.RefreshRequired) return
        state = current.copy(forgeRecovery = null)
    }

    fun updateForgeDraft(transform: (ForgeDraft) -> ForgeDraft) {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        if (forge.pending) return
        state = current.copy(forge = forge.copy(draft = transform(forge.draft), error = null))
    }

    fun forge() {
        val current = state as? SkidbladnirUiState.Dashboard ?: return
        val forge = current.forge ?: return
        val activeBearer = bearer ?: return
        val generation = foregroundGeneration
        if (forge.pending) return
        state = current.copy(forge = forge.copy(pending = true, error = null))
        executeClient {
            when (val result = client.createSession(activeBearer, forge.draft)) {
                is GatewayResult.Success -> main.post {
                    if (!isActive(generation, activeBearer)) return@post
                    openTerminal(result.value)
                }
                is GatewayResult.Failure -> {
                    if (result.failure == GatewayFailure.Api(ApiErrorCode.Unauthenticated)) {
                        if (!isActive(generation, activeBearer)) return@executeClient
                        val storageError = clearStoredPairingError()
                        main.post { acceptAuthenticationLoss(generation, activeBearer, storageError) }
                        return@executeClient
                    }
                    main.post {
                        if (!isActive(generation, activeBearer)) return@post
                        val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                        val activeForge = dashboard.forge ?: return@post
                        if (createFailureIsDefinitive(result.failure)) {
                            state = dashboard.copy(
                                forge = activeForge.copy(
                                    pending = false,
                                    error = gatewayFailureMessage(result.failure),
                                ),
                            )
                        } else {
                            state = dashboard.copy(
                                inventoryStale = dashboard.inventory != null,
                                inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
                                pressure = null,
                                refreshing = true,
                                error = null,
                                forge = null,
                                forgeRecovery = ForgeRecovery.RefreshRequired(activeForge.draft),
                            )
                            executeClient { poll(activeBearer, generation) }
                        }
                    }
                }
            }
        }
    }

    fun openTerminal(session: AgentSession) {
        leaveTerminal()
        state = SkidbladnirUiState.Terminal(
            session = session,
            attempt = nextTerminalAttempt++,
            connection = TerminalUiStatus.Preparing,
            kill = null,
        )
    }

    fun terminalPageReady(attempt: Int, page: TerminalPage) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        val activeBearer = bearer ?: return
        if (current.attempt != attempt || current.connection != TerminalUiStatus.Preparing) return
        terminalPage = page
        state = current.copy(connection = TerminalUiStatus.Connecting)
        val owner = Any()
        terminalOwner = owner
        val connection = TerminalConnection(
            client = client,
            bearer = activeBearer,
            session = current.session,
            page = page,
            observer = object : TerminalConnectionObserver {
                override fun onPresence(attachedClients: Int, geometry: TerminalGeometry) {
                    main.post {
                        val terminal = state as? SkidbladnirUiState.Terminal ?: return@post
                        if (terminal.attempt != attempt || terminalOwner !== owner) return@post
                        if (terminal.connection !is TerminalUiStatus.Connecting &&
                            terminal.connection !is TerminalUiStatus.Connected
                        ) {
                            return@post
                        }
                        state = terminal.copy(
                            connection = TerminalUiStatus.Connected(attachedClients, geometry),
                        )
                    }
                }

                override fun onFailure(code: ApiErrorCode) {
                    main.post {
                        val terminal = state as? SkidbladnirUiState.Terminal ?: return@post
                        if (terminal.attempt != attempt || terminalOwner !== owner) return@post
                        leaveTerminal()
                        state = terminal.copy(
                            connection = TerminalUiStatus.ReconnectRequired(apiErrorMessage(code)),
                        )
                    }
                }
            },
        )
        terminalConnection = connection
        connection.start()
    }

    fun resizeTerminal(attempt: Int, columns: Int, rows: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt == attempt) terminalConnection?.resize(columns, rows)
    }

    fun terminalPageFailed(attempt: Int) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt) return
        terminalConnection?.terminalUnavailable()
        leaveTerminal()
        state = current.copy(
            connection = TerminalUiStatus.ReconnectRequired("Reconnect required."),
        )
    }

    fun sendTerminal(attempt: Int, bytes: ByteArray) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection !is TerminalUiStatus.Connected) return
        terminalConnection?.send(bytes)
        terminalPage?.focus()
    }

    fun sendTerminalAccessory(attempt: Int, accessory: TerminalAccessory) {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        if (current.attempt != attempt || current.connection !is TerminalUiStatus.Connected) return
        terminalPage?.sendAccessory(accessory)
    }

    fun reattachTerminal() {
        val current = state as? SkidbladnirUiState.Terminal ?: return
        leaveTerminal()
        state = current.copy(
            attempt = nextTerminalAttempt++,
            connection = TerminalUiStatus.Preparing,
            kill = null,
        )
    }

    fun detachToAgents() {
        leaveTerminal()
        showDashboard(refreshing = true)
        val activeBearer = bearer ?: return
        val generation = foregroundGeneration
        executeClient { poll(activeBearer, generation) }
    }

    fun requestKill(session: AgentSession) {
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> state = current.copy(kill = KillState(session, false, null))
            is SkidbladnirUiState.Terminal -> state = current.copy(kill = KillState(session, false, null))
            else -> Unit
        }
    }

    fun dismissKill() {
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> if (current.kill?.pending != true) state = current.copy(kill = null)
            is SkidbladnirUiState.Terminal -> if (current.kill?.pending != true) state = current.copy(kill = null)
            else -> Unit
        }
    }

    fun confirmKill() {
        val kill = when (val current = state) {
            is SkidbladnirUiState.Dashboard -> current.kill
            is SkidbladnirUiState.Terminal -> current.kill
            else -> null
        } ?: return
        val activeBearer = bearer ?: return
        val generation = foregroundGeneration
        if (kill.pending) return
        when (val current = state) {
            is SkidbladnirUiState.Dashboard -> state = current.copy(kill = kill.copy(pending = true, error = null))
            is SkidbladnirUiState.Terminal -> {
                leaveTerminal()
                state = current.copy(kill = kill.copy(pending = true, error = null))
            }
            else -> return
        }
        executeClient {
            when (val result = client.killSession(activeBearer, kill.target)) {
                is GatewayResult.Success -> main.post {
                    if (!isActive(generation, activeBearer)) return@post
                    leaveTerminal()
                    inventory = inventory?.copy(
                        sessions = inventory?.sessions.orEmpty().filterNot {
                            it.id == kill.target.id && it.identityToken == kill.target.identityToken
                        },
                    )
                    showDashboard(refreshing = true)
                    executeClient { poll(activeBearer, generation) }
                }
                is GatewayResult.Failure -> {
                    if (result.failure == GatewayFailure.Api(ApiErrorCode.Unauthenticated)) {
                        if (!isActive(generation, activeBearer)) return@executeClient
                        val storageError = clearStoredPairingError()
                        main.post { acceptAuthenticationLoss(generation, activeBearer, storageError) }
                        return@executeClient
                    }
                    main.post {
                        if (!isActive(generation, activeBearer)) return@post
                        val message = gatewayFailureMessage(result.failure)
                        if (killFailureIsDefinitive(result.failure)) {
                            leaveTerminal()
                            state = SkidbladnirUiState.Dashboard(
                                inventory = inventory,
                                pressure = null,
                                inventoryStale = inventory != null,
                                inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
                                refreshing = true,
                                error = "$message Agents are refreshing.",
                                forge = null,
                                forgeRecovery = null,
                                kill = null,
                            )
                            executeClient { poll(activeBearer, generation) }
                        } else {
                            leaveTerminal()
                            state = SkidbladnirUiState.Dashboard(
                                inventory = inventory,
                                pressure = null,
                                inventoryStale = inventory != null,
                                inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
                                refreshing = true,
                                error = "Kill outcome unknown. Agents are refreshing.",
                                forge = null,
                                forgeRecovery = null,
                                kill = null,
                            )
                            executeClient { poll(activeBearer, generation) }
                        }
                    }
                }
            }
        }
    }

    private fun startPolling(activeBearer: GatewayBearer, generation: Long, immediately: Boolean) {
        poller?.cancel(false)
        poller = worker.scheduleWithFixedDelay(
            { surfaceWorkerDefect { poll(activeBearer, generation) } },
            if (immediately) 0 else 5,
            5,
            TimeUnit.SECONDS,
        )
    }

    private fun executeClient(action: () -> Unit) {
        worker.execute { surfaceWorkerDefect(action) }
    }

    private fun surfaceWorkerDefect(action: () -> Unit) {
        try {
            action()
        } catch (defect: RuntimeException) {
            // justify-defect: executor futures otherwise swallow same-system
            // invariant failures instead of surfacing them.
            main.post { throw defect }
        }
    }

    private fun poll(activeBearer: GatewayBearer, generation: Long) {
        if (!isActive(generation, activeBearer)) return
        val sessionsResult = client.listSessions(activeBearer)
        if (sessionsResult is GatewayResult.Failure) {
            if (sessionsResult.failure == GatewayFailure.Api(ApiErrorCode.Unauthenticated)) {
                if (!isActive(generation, activeBearer)) return
                val storageError = clearStoredPairingError()
                main.post { acceptAuthenticationLoss(generation, activeBearer, storageError) }
                return
            }
            main.post {
                if (!isActive(generation, activeBearer)) return@post
                val dashboard = state as? SkidbladnirUiState.Dashboard ?: return@post
                pressure = null
                state = dashboard.copy(
                    pressure = null,
                    inventoryStale = dashboard.inventory != null,
                    inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
                    refreshing = false,
                    error = gatewayFailureMessage(sessionsResult.failure),
                )
            }
            return
        }
        sessionsResult as GatewayResult.Success
        val inventoryReceivedAt = SystemClock.elapsedRealtime()
        if (!isActive(generation, activeBearer)) return
        val pressureResult = client.readPressure(activeBearer)
        if (pressurePollRequiresPairing(pressureResult)) {
            if (!isActive(generation, activeBearer)) return
            val storageError = clearStoredPairingError()
            main.post { acceptAuthenticationLoss(generation, activeBearer, storageError) }
            return
        }
        main.post {
            if (!isActive(generation, activeBearer)) return@post
            inventory = sessionsResult.value
            inventoryReceivedAtElapsedMillis = inventoryReceivedAt
            val pressureError = when (pressureResult) {
                is GatewayResult.Success -> {
                    pressure = pressureResult.value
                    null
                }
                is GatewayResult.Failure -> {
                    pressure = null
                    "Sessions are current. Devbox pressure could not be read."
                }
            }
            val dashboard = state as? SkidbladnirUiState.Dashboard
            if (dashboard != null) {
                state = dashboard.copy(
                    inventory = inventory,
                    pressure = pressure,
                    inventoryStale = false,
                    inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
                    refreshing = false,
                    error = pressureError,
                    forgeRecovery = when (val recovery = dashboard.forgeRecovery) {
                        is ForgeRecovery.RefreshRequired -> ForgeRecovery.ReviewReady(recovery.draft)
                        else -> recovery
                    },
                )
            }
        }
    }

    private fun showDashboard(
        refreshing: Boolean,
        inventoryStale: Boolean = inventory != null,
    ) {
        state = dashboardState(refreshing, inventoryStale)
    }

    private fun dashboardState(
        refreshing: Boolean,
        inventoryStale: Boolean = inventory != null,
    ): SkidbladnirUiState.Dashboard {
        val carry = forgeCarry(state)
        return SkidbladnirUiState.Dashboard(
            inventory = inventory,
            pressure = if (refreshing) null else pressure,
            inventoryStale = inventoryStale,
            inventoryAgeAdvanceSeconds = inventoryAgeAdvanceSeconds(),
            refreshing = refreshing,
            error = null,
            forge = carry.forge,
            forgeRecovery = carry.recovery,
            kill = null,
        )
    }

    private fun leaveTerminal() {
        terminalOwner = null
        val connection = terminalConnection
        terminalConnection = null
        terminalPage = null
        connection?.detach()
    }

    private fun clearStoredPairingError(): String? = try {
        bearerStore.clear()
        null
    } catch (_: Exception) {
        " Stored pairing could not be cleared."
    }

    private fun acceptAuthenticationLoss(
        generation: Long,
        activeBearer: GatewayBearer,
        storageError: String?,
    ) {
        if (!isActive(generation, activeBearer)) return
        poller?.cancel(false)
        poller = null
        leaveTerminal()
        bearer = null
        inventory = null
        inventoryReceivedAtElapsedMillis = null
        pressure = null
        state = SkidbladnirUiState.Pairing(
            draft = "",
            pending = false,
            error = "Authentication required.${storageError.orEmpty()}",
        )
    }

    private fun inventoryAgeAdvanceSeconds(): Long {
        val receivedAt = inventoryReceivedAtElapsedMillis ?: return 0
        return ((SystemClock.elapsedRealtime() - receivedAt).coerceAtLeast(0) / 1_000)
    }

    private fun isForegroundGeneration(generation: Long): Boolean =
        foreground && foregroundGeneration == generation

    private fun isActive(generation: Long, activeBearer: GatewayBearer): Boolean =
        isForegroundGeneration(generation) && bearer == activeBearer
}
