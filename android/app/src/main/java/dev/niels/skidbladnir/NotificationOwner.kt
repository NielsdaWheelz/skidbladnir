package dev.niels.skidbladnir

import android.app.Application
import android.app.KeyguardManager
import android.content.Context
import android.database.sqlite.SQLiteException
import android.os.PowerManager
import androidx.work.ExistingWorkPolicy
import java.io.IOException
import java.security.GeneralSecurityException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.decodeFromJsonElement
import org.unifiedpush.android.connector.UnifiedPush
import org.unifiedpush.android.connector.keys.DefaultKeyManager

class SkidNotificationApplication : Application() {
    internal val notifications by lazy { NotificationOwner(this) }
}
internal fun Context.notificationOwner(): NotificationOwner =
    (applicationContext as SkidNotificationApplication).notifications

internal enum class NotificationHealth { Ready, SetupRequired, PermissionBlocked, DistributorMissing, DeliveryUnavailable, ResetRequired }
internal data class NotificationView(
    val device: DeviceNotifications = DeviceNotifications(), val snapshot: ObserverSnapshot? = null,
    val health: NotificationHealth = NotificationHealth.SetupRequired,
    val resetIdentity: Any?,
) {
    fun presentsReady(key: NotificationKey, session: TmuxSession): Boolean {
        if (health == NotificationHealth.DeliveryUnavailable || health == NotificationHealth.ResetRequired) return false
        val machine = snapshot?.machines?.singleOrNull { it.machine == key.machine } ?: return false
        val row = machine.sessions?.singleOrNull { it.record.key == key } ?: return false
        val local = device.records.singleOrNull { it.key == key } ?: return false
        return row.readyQualified && session.connection == null &&
            row.record.foreground == session.agent?.let(::NotificationForeground) &&
            session.terminalStatus.source == TerminalStatusSource.Terminal &&
            session.terminalStatus.activity == TerminalActivity.Idle && session.terminalStatus.interaction == TerminalInteraction.None &&
            local.receivedReadyGeneration > local.acknowledgedReadyGeneration
    }
}
internal class NotificationVisitToken(
    val key: NotificationKey, val foreground: NotificationForeground?, val epoch: String?, val readyGeneration: Long,
    val resetIdentity: Any?,
)
private data class NotificationVisit(val token: NotificationVisitToken, val presented: Boolean)
private data class NotificationReadyBoundary(val token: NotificationVisitToken, val receivedGeneration: Long)
private data class NotificationFocus(val resumed: Boolean, val windowFocused: Boolean)
private data class NotificationRead(val snapshot: ObserverSnapshot, val alienHint: NotificationHint?)
private data class NotificationFetch(
    val fence: Long, val config: NotificationConfig, val credential: MachineCredential,
    val minimumRevision: Long, val alienHint: NotificationHint?,
)
private class NotificationEndpointOffer(
    val subscription: NotificationSubscription, val receipt: CompletableDeferred<NotificationEndpointOffer>?,
)
private data class NotificationPost(val row: NotificationRow, val silent: Boolean, val epoch: String, val rename: NativeNotice? = null) {
    val episode: Long get() = rename?.episode ?: row.record.attentionEpisode
    val readyGeneration: Long get() = rename?.readyGeneration ?:
        if (row.record.cause?.kind == NotificationCauseKind.Ready) row.record.readyGeneration else 0L
}

/** The process owns one state writer and one ordered native-effect lane; activities only observe. */
internal class NotificationOwner(private val context: Context) {
    internal val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val store = NotificationStore(context)
    private val client = NotificationClient()
    private val native = NativeNotifications(context)
    private val mutex = Mutex()
    private val initialized = CompletableDeferred<Unit>()
    private val effects = Channel<Unit>(Channel.CONFLATED)
    private val visitOutputs = Channel<NotificationVisitToken>(Channel.UNLIMITED)
    private val endpoints = Channel<NotificationEndpointOffer>(Channel.CONFLATED)
    private val endpointIntake = Any()
    private var registrationReceipt: CompletableDeferred<NotificationEndpointOffer>? = null
    @Volatile private var pendingEndpoint: NotificationEndpointOffer? = null
    private var consumedEndpoint: NotificationEndpointOffer? = null
    private var confirmedRegistration: NotificationRegistration? = null
    // Stable across ordinary visits and configuration; null while memory is being reset.
    private var resetIdentity: Any? = Any()
    private val current = MutableStateFlow(NotificationView(resetIdentity = resetIdentity))
    val view: StateFlow<NotificationView> = current
    private var device = DeviceNotifications()
    private var snapshot: ObserverSnapshot? = null
    private var unavailable = false
    private var storageAvailable = false
    private var epochMismatch = false
    private var resetting = false
    private var setupIncomplete = false
    private var fence = 0L
    private var fetchTask: Deferred<NotificationRead?>? = null
    private var hintFloor = 0L
    private var alienHint: NotificationHint? = null
    private val effectsCompleted = MutableStateFlow(0L)
    @Volatile private var focus = NotificationFocus(false, false)
    @Volatile private var visit: NotificationVisit? = null
    private var readyBoundary: NotificationReadyBoundary? = null

    init {
        scope.launch {
            initialized.await()
            for (offer in endpoints) mutex.withLock {
                val config = device.config ?: return@withLock
                if (offer !== pendingEndpoint || !storageAvailable || !offer.subscription.validFor(config) ||
                    offer.subscription.keys != registrationKeys()) return@withLock
                val desired = if (device.desiredRegistration?.subscription == offer.subscription) device.desiredRegistration
                else {
                    if (device.localRevision >= Long.MAX_VALUE - 1) { unavailable = true; publish(); return@withLock }
                    NotificationRegistration(device.localRevision + 1, offer.subscription)
                }
                // Identical callbacks also pass through the durable writer before waking their worker.
                if (!commit(device.copy(desiredRegistration = desired))) return@withLock
                if (confirmedRegistration != desired) confirmedRegistration = null
                consumedEndpoint = offer
                publish()
                offer.receipt?.complete(offer)
            }
        }
        scope.launch {
            initialized.await()
            for (token in visitOutputs) mutex.withLock {
                if (!resetting && token.resetIdentity != null && token.resetIdentity === resetIdentity && device.observerEpoch == token.epoch) {
                    acknowledge(token.key, token.foreground, token.readyGeneration)
                    if (visit?.token === token) {
                        val received = device.records.singleOrNull { it.key == token.key && it.foreground == token.foreground }
                            ?.receivedReadyGeneration ?: 0
                        readyBoundary = NotificationReadyBoundary(token, received)
                    }
                }
                publish()
                effects.trySend(Unit)
            }
        }
        scope.launch {
            mutex.withLock {
                try {
                    device = store.read()
                    storageAvailable = true
                } catch (_: IOException) {
                    unavailable = true
                }
                publish()
                initialized.complete(Unit)
            }
            if (device.config != null && UnifiedPush.getSavedDistributor(context) == "io.heckel.ntfy") {
                NotificationEnrollment.schedule(context, ExistingWorkPolicy.KEEP)
            }
            for (signal in effects) {
                val through = mutex.withLock { device.localRevision }
                present()
                effectsCompleted.value = maxOf(effectsCompleted.value, through)
            }
        }
    }

    fun refresh() {
        scope.launch { refreshAndPresent() }
    }

    private suspend fun refreshAndPresent() {
        fetchSnapshot()
        val through = mutex.withLock { device.localRevision }
        effects.trySend(Unit)
        effectsCompleted.first { it >= through }
    }

    private suspend fun fetchSnapshot(): NotificationRead? {
        initialized.await()
        val task = mutex.withLock {
            if (!storageAvailable || epochMismatch || resetting || device.config == null) return null
            fetchTask?.let { return@withLock it }
            scope.async<NotificationRead?> {
                val ownedTask = checkNotNull(currentCoroutineContext()[Job])
                try {
                    var read: NotificationRead? = null
                    while (true) {
                        val request = mutex.withLock {
                            if (!storageAvailable || epochMismatch || resetting) {
                                if (fetchTask === ownedTask) fetchTask = null
                                return@async null
                            }
                            val config = device.config ?: run {
                                if (fetchTask === ownedTask) fetchTask = null
                                return@async null
                            }
                            val credential = config.credential(MachineStore(context).read().credentials) ?: run {
                                snapshot = null
                                unavailable = true
                                publish()
                                if (fetchTask === ownedTask) fetchTask = null
                                return@async null
                            }
                            NotificationFetch(fence, config, credential, maxOf(device.admittedRevision, hintFloor), alienHint)
                        }
                        val result = client.state(request.credential)
                        val retry = mutex.withLock {
                            if (request.fence != fence || device.config != request.config ||
                                request.config.credential(MachineStore(context).read().credentials) != request.credential ||
                                request.alienHint !== alienHint) return@withLock true
                            if (!storageAvailable || epochMismatch || resetting) {
                                if (fetchTask === ownedTask) fetchTask = null
                                return@withLock false
                            }
                            val minimum = maxOf(device.admittedRevision, hintFloor)
                            when (result) {
                                is GatewayResult.Success -> {
                                    val value = result.value
                                    val foreign = request.alienHint
                                    if (value.epoch == device.observerEpoch && value.revision < minimum &&
                                        minimum > request.minimumRevision) return@withLock true
                                    val belowBoundFloor = value.epoch == device.observerEpoch && value.revision < minimum
                                    val belowUnboundHint = device.observerEpoch == null && foreign != null &&
                                        value.epoch == foreign.epoch && value.revision < foreign.revision
                                    if (belowBoundFloor || belowUnboundHint) {
                                        snapshot = null
                                        unavailable = true
                                        publish()
                                    } else if (admit(value)) {
                                        if (foreign != null && value.epoch == foreign.epoch) hintFloor = maxOf(hintFloor, foreign.revision)
                                        read = NotificationRead(value, request.alienHint)
                                    }
                                }
                                is GatewayResult.Failure -> {
                                    if (minimum > request.minimumRevision) return@withLock true
                                    snapshot = null
                                    unavailable = true
                                    publish()
                                }
                            }
                            // A later hint must start a new read, never join this terminal decision.
                            if (fetchTask === ownedTask) fetchTask = null
                            false
                        }
                        if (!retry) break
                    }
                    read
                } finally {
                    withContext(NonCancellable) { mutex.withLock { if (fetchTask === ownedTask) fetchTask = null } }
                }
            }.also { fetchTask = it }
        }
        return task.await()
    }

    fun settingsIntent(): android.content.Intent = native.settingsIntent()

    suspend fun receiveHint(bytes: ByteArray) {
        if (bytes.size !in 1..256) return
        val hint = try {
            productJson.decodeFromJsonElement<NotificationHint>(notificationJson(bytes.decodeToString(throwOnInvalidSequence = true)))
        } catch (_: kotlinx.serialization.SerializationException) { return }
          catch (_: IllegalArgumentException) { return }
        initialized.await()
        mutex.withLock {
            if (device.observerEpoch == hint.epoch) {
                if (hint.revision < maxOf(device.admittedRevision, hintFloor)) return
                hintFloor = maxOf(hintFloor, hint.revision)
                if (snapshot?.revision?.let { it < hintFloor } == true) {
                    snapshot = null
                    publish()
                }
            } else {
                val previous = alienHint
                if (previous?.epoch == hint.epoch && hint.revision < previous.revision) return
                if (previous != hint) {
                    alienHint = hint
                    snapshot = null
                    publish()
                }
            }
        }
        refreshAndPresent()
    }

    suspend fun configure(config: NotificationConfig) {
        initialized.await()
        mutex.withLock {
            if (!storageAvailable || config.credential(MachineStore(context).read().credentials) == null) return
            ++fence
            confirmedRegistration = null
            snapshot = null
            val desired = device.desiredRegistration?.takeIf { it.subscription.validFor(config) }
            setupIncomplete = true
            if (!commit(device.copy(config = config, desiredRegistration = desired))) return
            setupIncomplete = false
            unavailable = false
            publish()
        }
        refresh()
        if (UnifiedPush.getSavedDistributor(context) == "io.heckel.ntfy") {
            NotificationEnrollment.schedule(context, ExistingWorkPolicy.REPLACE)
        }
    }

    fun fleetChanged() {
        scope.launch {
            initialized.await()
            val configured = mutex.withLock { ++fence; confirmedRegistration = null; snapshot = null; publish(); device.config != null }
            if (configured && UnifiedPush.getSavedDistributor(context) == "io.heckel.ntfy") {
                NotificationEnrollment.schedule(context, ExistingWorkPolicy.REPLACE)
            }
            refresh()
        }
    }

    fun endpoint(subscription: NotificationSubscription) {
        synchronized(endpointIntake) {
            val offer = NotificationEndpointOffer(subscription, registrationReceipt)
            pendingEndpoint = offer
            endpoints.trySend(offer)
        }
    }

    /** Called only by the already-durable worker. Missing callbacks leave its retry obligation intact. */
    suspend fun enroll(): NotificationEnrollmentResult {
        initialized.await()
        val receipt = CompletableDeferred<NotificationEndpointOffer>()
        try {
            val (requestFence, config, credential) = mutex.withLock {
                currentCoroutineContext().ensureActive()
                if (!storageAvailable) return NotificationEnrollmentResult.Unavailable
                if (epochMismatch || resetting) return NotificationEnrollmentResult.Conflict
                val config = device.config ?: return NotificationEnrollmentResult.Unavailable
                val credential = config.credential(MachineStore(context).read().credentials)
                    ?: return NotificationEnrollmentResult.Unavailable
                if (UnifiedPush.getSavedDistributor(context) != "io.heckel.ntfy") return NotificationEnrollmentResult.Unavailable
                confirmedRegistration = null
                synchronized(endpointIntake) { registrationReceipt = receipt }
                val registered = try {
                    // Registration happens before observer access, so an observer outage cannot lose the endpoint.
                    UnifiedPush.register(context, instance = NOTIFICATION_INSTANCE, messageForDistributor = "skid needs input")
                    true
                } catch (_: GeneralSecurityException) { false }
                  catch (_: IOException) { false }
                  catch (_: SQLiteException) { false }
                if (!registered) { unavailable = true; publish(); return NotificationEnrollmentResult.Unavailable }
                publish()
                Triple(fence, config, credential)
            }
            if (withTimeoutOrNull(4_000) { receipt.await() } == null) return NotificationEnrollmentResult.Unavailable
            val remote = fetchSnapshot() ?: return mutex.withLock {
                currentCoroutineContext().ensureActive()
                if (requestFence != fence || device.config != config || epochMismatch ||
                    synchronized(endpointIntake) { registrationReceipt !== receipt } ||
                    config.credential(MachineStore(context).read().credentials) != credential) NotificationEnrollmentResult.Conflict
                else NotificationEnrollmentResult.Unavailable
            }
            val (desired, offer) = mutex.withLock {
                currentCoroutineContext().ensureActive()
                if (!storageAvailable || requestFence != fence || device.config != config || epochMismatch ||
                    device.observerEpoch != remote.snapshot.epoch || remote.alienHint !== alienHint ||
                    synchronized(endpointIntake) { registrationReceipt !== receipt } ||
                    config.credential(MachineStore(context).read().credentials) != credential) return NotificationEnrollmentResult.Conflict
                val offer = consumedEndpoint ?: return NotificationEnrollmentResult.Conflict
                val desired = device.desiredRegistration ?: return NotificationEnrollmentResult.Conflict
                if (offer !== pendingEndpoint || offer.receipt !== receipt || offer.subscription != desired.subscription ||
                    desired.subscription.keys != registrationKeys()) return NotificationEnrollmentResult.Conflict
                Pair(desired, offer)
            }
            mutex.withLock {
                currentCoroutineContext().ensureActive()
                if (!storageAvailable || requestFence != fence || device.config != config || epochMismatch ||
                    device.observerEpoch != remote.snapshot.epoch || remote.alienHint !== alienHint ||
                    synchronized(endpointIntake) { registrationReceipt !== receipt } ||
                    device.desiredRegistration != desired || consumedEndpoint !== offer || pendingEndpoint !== offer ||
                    config.credential(MachineStore(context).read().credentials) != credential || desired.subscription.keys != registrationKeys()) {
                    return NotificationEnrollmentResult.Conflict
                }
            }
            // A later entered callback invalidates completion even before its durable writer runs.
            val result = client.enroll(credential, desired.subscription, remote.snapshot.receiverTag)
            mutex.withLock {
                currentCoroutineContext().ensureActive()
                if (!storageAvailable || requestFence != fence || device.config != config || epochMismatch ||
                    device.observerEpoch != remote.snapshot.epoch || remote.alienHint !== alienHint ||
                    synchronized(endpointIntake) { registrationReceipt !== receipt } ||
                    device.desiredRegistration != desired || consumedEndpoint !== offer || pendingEndpoint !== offer ||
                    config.credential(MachineStore(context).read().credentials) != credential || desired.subscription.keys != registrationKeys()) {
                    return NotificationEnrollmentResult.Conflict
                }
                if (result == NotificationEnrollmentResult.Installed) confirmedRegistration = desired
                else unavailable = true
                publish()
            }
            if (result == NotificationEnrollmentResult.Installed || result == NotificationEnrollmentResult.Conflict) refresh()
            return result
        } finally {
            synchronized(endpointIntake) { if (registrationReceipt === receipt) registrationReceipt = null }
        }
    }

    private fun registrationKeys(): NotificationSubscriptionKeys? = try {
        DefaultKeyManager(context).getPublicKeySet(NOTIFICATION_INSTANCE)?.let { NotificationSubscriptionKeys(it.pubKey, it.auth) }
    } catch (_: GeneralSecurityException) { null }
      catch (_: IOException) { null }
      catch (_: SQLiteException) { null }
      catch (_: IllegalArgumentException) { null }

    fun activityFocus(resumed: Boolean, windowFocused: Boolean) {
        focus = NotificationFocus(resumed, windowFocused)
    }

    fun captureVisit(target: SessionTarget): NotificationVisitToken {
        val view = current.value
        val key = NotificationKey(target)
        val record = view.device.records.singleOrNull { it.key == key && it.foreground == target.session.agent?.let(::NotificationForeground) }
        return NotificationVisitToken(key, target.session.agent?.let(::NotificationForeground), view.device.observerEpoch,
            record?.receivedReadyGeneration ?: 0, view.resetIdentity)
    }

    @androidx.annotation.MainThread
    fun beginVisit(token: NotificationVisitToken) { visit = NotificationVisit(token, false) }

    @androidx.annotation.MainThread
    fun outputPresented(token: NotificationVisitToken) {
        val visiting = visit ?: return
        if (visiting.token != token || visiting.presented) return
        visit = NotificationVisit(token, true)
        visitOutputs.trySend(token)
    }

    @androidx.annotation.MainThread
    fun endVisit(token: NotificationVisitToken?) {
        if (visit?.token == token) visit = null
    }

    suspend fun dismiss(slot: String, episode: Long, epoch: String) {
        initialized.await()
        mutex.withLock {
            if (device.observerEpoch != epoch) return
            val record = device.records.singleOrNull { it.key.slot == slot && it.handledAttentionEpisode == episode } ?: return
            if (record.presentation != NotificationDelivery.Closed) replace(record.copy(presentation = NotificationDelivery.Closed))
            effects.trySend(Unit)
            publish()
        }
    }

    fun deliveryUnavailable() {
        scope.launch { initialized.await(); mutex.withLock { unavailable = true; publish() } }
    }

    suspend fun reset() {
        initialized.await()
        mutex.withLock {
            if (!storageAvailable) return
            // Keep exact cancellation intent and acknowledgement until owned native absence is known.
            if (!commit(device.copy(records = device.records.map { it.copy(presentation = NotificationDelivery.Closed) }))) return
            ++fence
            confirmedRegistration = null
            snapshot = null
            resetting = true
            resetIdentity = null
            visit = null
            readyBoundary = null
            unavailable = false
            publish()
            effects.trySend(Unit)
        }
    }

    private suspend fun admit(value: ObserverSnapshot): Boolean {
        if (resetting) return false
        val fleet = MachineStore(context).read().credentials
        if (value.machines.map { it.machine }.toSet() != fleet.map { it.machine.handle.encoded }.toSet()) {
            unavailable = true
            snapshot = null
            publish()
            return false
        }
        if (device.observerEpoch != null && device.observerEpoch != value.epoch) {
            confirmedRegistration = null
            epochMismatch = true
            unavailable = false
            snapshot = null
            publish()
            return false
        }
        if (value.revision < device.admittedRevision) return false
        val records = device.records.associateBy { it.key.slot }.toMutableMap()
        for (machine in value.machines) {
            if (machine.availability == NotificationAvailability.Gap) continue
            val rows = checkNotNull(machine.sessions)
            val slots = rows.map { it.record.key.slot }.toSet()
            records.entries.removeAll { it.value.key.machine == machine.machine && it.key !in slots }
            for (row in rows) {
                val previous = records[row.record.key.slot]?.takeIf { it.key == row.record.key && it.foreground == row.record.foreground }
                if (previous != null && (row.record.readyGeneration < previous.receivedReadyGeneration ||
                        row.record.attentionEpisode < previous.handledAttentionEpisode)) {
                    unavailable = true
                    snapshot = null
                    publish()
                    return false
                }
                records[row.record.key.slot] = DeviceNotificationRecord(
                    row.record.key, row.record.foreground, row.record.readyGeneration,
                    previous?.acknowledgedReadyGeneration ?: 0, previous?.handledAttentionEpisode ?: 0,
                    previous?.presentation ?: NotificationDelivery.Closed,
                )
            }
        }
        if (!commit(device.copy(observerEpoch = value.epoch, admittedRevision = value.revision, records = records.values.toList()))) return false
        snapshot = value
        unavailable = false
        // Ready under an unchanged action still counts as presented only in a proven live visit.
        for (row in value.machines.flatMap { it.sessions.orEmpty() }) {
            val prior = current.value.device.records.singleOrNull { it.key == row.record.key }
            if (row.readyQualified && row.record.readyGeneration > (prior?.receivedReadyGeneration ?: 0) && readyFocused(row)) {
                acknowledge(row.record.key, row.record.foreground, row.record.readyGeneration)
            }
        }
        publish()
        effects.trySend(Unit)
        return storageAvailable
    }

    private fun focused(row: NotificationRow): Boolean {
        val visiting = visit ?: return false
        val focused = focus
        return visiting.presented && visiting.token.resetIdentity != null && visiting.token.resetIdentity === resetIdentity &&
            visiting.token.key.slot == row.record.key.slot &&
            focused.resumed && focused.windowFocused && context.getSystemService(PowerManager::class.java).isInteractive &&
            !context.getSystemService(KeyguardManager::class.java).isKeyguardLocked
    }

    private fun readyFocused(row: NotificationRow): Boolean = focused(row) &&
        visit?.token?.key == row.record.key && visit?.token?.foreground == row.record.foreground &&
        visit?.token?.epoch == device.observerEpoch &&
        readyBoundary?.token === visit?.token && row.record.readyGeneration > checkNotNull(readyBoundary).receivedGeneration

    private suspend fun acknowledge(key: NotificationKey, foreground: NotificationForeground?, generation: Long) {
        val record = device.records.singleOrNull { it.key == key && it.foreground == foreground } ?: return
        if (generation > record.acknowledgedReadyGeneration && generation <= record.receivedReadyGeneration) {
            replace(record.copy(acknowledgedReadyGeneration = generation))
        }
    }

    private suspend fun present() {
        // Foreground presentation and positive clears are product facts, independent of OS inspection.
        mutex.withLock {
            if (!storageAvailable) return
            val value = snapshot
            if (value != null && !unavailable && !epochMismatch) for (machine in value.machines) {
                for (row in machine.sessions.orEmpty()) {
                    var record = device.records.single { it.key == row.record.key }
                    val cause = row.record.cause
                    val readyConsumed = cause?.kind == NotificationCauseKind.Ready &&
                        record.receivedReadyGeneration <= record.acknowledgedReadyGeneration
                    if (row.attentionQualified && (cause == null || readyConsumed)) {
                        if (record.presentation != NotificationDelivery.Closed) replace(record.copy(presentation = NotificationDelivery.Closed))
                    } else if (row.attentionQualified && cause != null &&
                        (cause.kind != NotificationCauseKind.Ready || row.readyQualified) &&
                        record.handledAttentionEpisode != row.record.attentionEpisode && focused(row)) {
                        if (row.readyQualified && readyFocused(row)) acknowledge(row.record.key, row.record.foreground, row.record.readyGeneration)
                        record = device.records.single { it.key == row.record.key }
                        replace(record.copy(handledAttentionEpisode = row.record.attentionEpisode, presentation = NotificationDelivery.Closed))
                    }
                }
            }
            publish()
        }
        val inspection = native.inspect()
        val visible = inspection.observed
        val known = (visible.orEmpty() + inspection.unresolved).distinct()
        if (mutex.withLock { resetting }) {
            for (notice in known) native.cancel(notice)
            // One confirmation per cancellation batch. Stale visibility waits for an existing wake.
            val confirmed = if (known.isEmpty()) inspection else native.inspect()
            val completed = mutex.withLock {
                if (!resetting || confirmed.observed == null || confirmed.observed.isNotEmpty() || confirmed.unresolved.isNotEmpty()) return@withLock false
                if (!commit(device.copy(observerEpoch = null, admittedRevision = 0, records = emptyList()))) return@withLock false
                resetting = false
                resetIdentity = Any()
                epochMismatch = false
                hintFloor = 0
                alienHint = null
                unavailable = false
                publish()
                true
            }
            if (completed) {
                val configured = mutex.withLock { device.config != null }
                if (configured && UnifiedPush.getSavedDistributor(context) == "io.heckel.ntfy") {
                    NotificationEnrollment.schedule(context, ExistingWorkPolicy.REPLACE)
                }
                refresh()
            }
            return
        }
        val posts = mutex.withLock {
            if (!storageAvailable) return
            if (resetting) return@withLock emptyList<NotificationPost>()
            // Native observation and captured output remain authoritative during producer gaps.
            for (notice in known) {
                if (notice.epoch != device.observerEpoch) continue
                val record = device.records.singleOrNull { it.key.slot == notice.slot &&
                    it.handledAttentionEpisode == notice.episode && it.key == notificationReference(notice.ref) } ?: continue
                if (notice.readyGeneration > 0 && notice.readyGeneration <= record.acknowledgedReadyGeneration &&
                    record.presentation != NotificationDelivery.Closed) {
                    replace(record.copy(presentation = NotificationDelivery.Closed))
                } else if (record.presentation == NotificationDelivery.Claimed && notice in visible.orEmpty() &&
                    inspection.unresolved.none { it.sameIdentity(notice) }) {
                    replace(record.copy(presentation = NotificationDelivery.Posted))
                }
            }
            if (visible == null) {
                publish()
                return@withLock emptyList<NotificationPost>()
            }
            val value = snapshot
            val eligible = mutableListOf<NotificationPost>()
            if (value != null && !unavailable && !epochMismatch) for (machine in value.machines) {
                for (row in machine.sessions.orEmpty()) {
                    var record = device.records.single { it.key == row.record.key }
                    val notice = visible.singleOrNull { it.slot == record.key.slot && it.episode == record.handledAttentionEpisode && it.epoch == device.observerEpoch }
                    val pending = inspection.unresolved.singleOrNull { it.slot == record.key.slot &&
                        it.episode == record.handledAttentionEpisode && it.epoch == device.observerEpoch }
                    val cause = row.record.cause
                    val readyConsumed = cause?.kind == NotificationCauseKind.Ready &&
                        record.receivedReadyGeneration <= record.acknowledgedReadyGeneration
                    if (row.attentionQualified && (cause == null || readyConsumed)) continue
                    if (!row.attentionQualified) {
                        if (record.presentation == NotificationDelivery.Posted) {
                            if (notice == null) {
                                if (pending == null) replace(record.copy(presentation = NotificationDelivery.Closed))
                            } else if (notice.title != notificationTitle(row.name) &&
                                pending != notice.copy(title = notificationTitle(row.name))) {
                                eligible += NotificationPost(row, true, value.epoch, notice)
                            }
                        }
                        continue
                    }
                    if (!row.attentionQualified || cause == null || cause.kind == NotificationCauseKind.Ready && !row.readyQualified) continue
                    if (record.handledAttentionEpisode != row.record.attentionEpisode) {
                        if (!native.allowed()) continue
                        record = record.copy(handledAttentionEpisode = row.record.attentionEpisode, presentation = NotificationDelivery.Claimed)
                        replace(record)
                        eligible += NotificationPost(row, false, value.epoch)
                    } else when (record.presentation) {
                        NotificationDelivery.Claimed -> if (pending == null && notice == null && native.allowed()) {
                            eligible += NotificationPost(row, true, value.epoch)
                        }
                        NotificationDelivery.Posted -> if (notice == null) {
                            if (pending == null) replace(record.copy(presentation = NotificationDelivery.Closed))
                        } else if (notice.title != notificationTitle(row.name) || notice.body != cause.text) {
                            if (pending != notice.copy(title = notificationTitle(row.name))) {
                                eligible += NotificationPost(row, true, value.epoch, notice)
                            }
                        }
                        NotificationDelivery.Closed -> Unit
                    }
                }
            }
            publish()
            eligible
        }
        // Cancellation is persisted first. Gaps retain notices unless local acknowledgement closes them.
        for (notice in known) {
            val keep = mutex.withLock {
                device.observerEpoch == notice.epoch && device.records.any { it.key.slot == notice.slot && it.handledAttentionEpisode == notice.episode &&
                    it.presentation != NotificationDelivery.Closed }
            }
            if (!keep) native.cancel(notice)
        }
        for (post in posts) {
            if (post.rename != null) {
                val latest = native.inspect()
                val observed = latest.observed ?: continue
                if (observed.none { it.sameIdentity(post.rename) }) {
                    mutex.withLock {
                        val record = device.records.singleOrNull { it.key == post.row.record.key &&
                            it.handledAttentionEpisode == post.rename.episode }
                        if (device.observerEpoch == post.epoch && record?.presentation == NotificationDelivery.Posted &&
                            latest.unresolved.none { it.sameIdentity(post.rename) }) {
                            replace(record.copy(presentation = NotificationDelivery.Closed))
                            publish()
                        }
                    }
                    continue
                }
            }
            val submitted = mutex.withLock {
                if (!currentPost(post)) null
                else try {
                    native.post(post.row, post.epoch, post.silent, post.readyGeneration, post.rename)
                } catch (_: SecurityException) {
                    publish()
                    null
                }
            }
            if (submitted == null) continue
            val keep = mutex.withLock {
                if (!desiredPost(post)) {
                    val record = device.records.singleOrNull { it.key == post.row.record.key &&
                        it.handledAttentionEpisode == post.episode }
                    if (device.observerEpoch == post.epoch && record != null && record.presentation != NotificationDelivery.Closed) {
                        replace(record.copy(presentation = NotificationDelivery.Closed))
                    }
                    publish()
                    false
                } else {
                    true
                }
            }
            if (!keep) {
                val cancellationCommitted = mutex.withLock {
                    device.observerEpoch != post.epoch || device.records.none { it.key == post.row.record.key &&
                        it.handledAttentionEpisode == post.episode && it.presentation != NotificationDelivery.Closed }
                }
                if (cancellationCommitted) native.cancel(submitted)
            }
        }
    }

    private fun currentPost(post: NotificationPost): Boolean {
        if (resetting) return false
        val record = device.records.singleOrNull { it.key == post.row.record.key } ?: return false
        val row = snapshot?.machines?.flatMap { it.sessions.orEmpty() }?.singleOrNull { it.record.key == post.row.record.key }
        if (post.rename != null) return !unavailable && !epochMismatch && row != null &&
            row.name == post.row.name && row.record.foreground == post.row.record.foreground &&
            record.presentation == NotificationDelivery.Posted && desiredPost(post)
        return !unavailable && !epochMismatch && row?.attentionQualified == true &&
            row.record == post.row.record && desiredPost(post) &&
            (row.record.cause?.kind != NotificationCauseKind.Ready || row.readyQualified &&
                record.receivedReadyGeneration > record.acknowledgedReadyGeneration)
    }

    /** Gaps preserve an already submitted notice; only positive obsolescence cancels its late effect. */
    private fun desiredPost(post: NotificationPost): Boolean {
        if (resetting) return false
        val record = device.records.singleOrNull { it.key == post.row.record.key && it.foreground == post.row.record.foreground } ?: return false
        if (post.readyGeneration > 0 && post.readyGeneration <= record.acknowledgedReadyGeneration) return false
        val row = snapshot?.machines?.flatMap { it.sessions.orEmpty() }?.singleOrNull { it.record.key == post.row.record.key }
        if (post.rename != null) return device.observerEpoch == post.epoch && record.presentation == NotificationDelivery.Posted &&
            record.handledAttentionEpisode == post.episode && (row == null || !row.attentionQualified ||
                row.record.cause != null && row.record.attentionEpisode == post.episode &&
                (row.record.cause.kind != NotificationCauseKind.Ready || row.readyQualified &&
                    record.receivedReadyGeneration > record.acknowledgedReadyGeneration))
        val stillCause = row == null || row.record.cause == post.row.record.cause && row.record.attentionEpisode == post.row.record.attentionEpisode
        return device.observerEpoch == post.epoch && stillCause && record.handledAttentionEpisode == post.row.record.attentionEpisode &&
            record.presentation != NotificationDelivery.Closed &&
            (post.row.record.cause?.kind != NotificationCauseKind.Ready || record.receivedReadyGeneration > record.acknowledgedReadyGeneration)
    }

    private suspend fun replace(record: DeviceNotificationRecord) {
        commit(device.copy(records = device.records.map { if (it.key.slot == record.key.slot) record else it }))
    }

    private suspend fun commit(value: DeviceNotifications): Boolean {
        if (!storageAvailable || device.localRevision >= Long.MAX_VALUE - 1) {
            storageAvailable = false
            unavailable = true
            publish()
            return false
        }
        try {
            // Once the atomic write starts, caller cancellation cannot leave memory behind disk.
            withContext(NonCancellable) { device = store.write(value.copy(localRevision = device.localRevision + 1)) }
            return true
        } catch (_: IOException) {
            storageAvailable = false
            unavailable = true
            publish()
            return false
        }
    }

    private fun publish() {
        val health = when {
            setupIncomplete -> NotificationHealth.SetupRequired
            !storageAvailable || unavailable -> NotificationHealth.DeliveryUnavailable
            epochMismatch || resetting -> NotificationHealth.ResetRequired
            device.config == null || device.desiredRegistration == null -> NotificationHealth.SetupRequired
            !native.allowed() -> NotificationHealth.PermissionBlocked
            !org.unifiedpush.android.connector.UnifiedPush.getDistributors(context).contains("io.heckel.ntfy") -> NotificationHealth.DistributorMissing
            org.unifiedpush.android.connector.UnifiedPush.getAckDistributor(context) != "io.heckel.ntfy" -> NotificationHealth.SetupRequired
            confirmedRegistration != device.desiredRegistration -> NotificationHealth.SetupRequired
            else -> NotificationHealth.Ready
        }
        current.value = NotificationView(device, snapshot, health, resetIdentity)
    }
}
