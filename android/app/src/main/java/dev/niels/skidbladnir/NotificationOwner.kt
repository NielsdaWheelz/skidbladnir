package dev.niels.skidbladnir

import android.app.Application
import android.app.KeyguardManager
import android.content.Context
import android.os.PowerManager
import androidx.work.ExistingWorkPolicy
import java.io.IOException
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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.decodeFromJsonElement

class SkidNotificationApplication : Application() {
    internal lateinit var firebasePush: FirebasePush
    internal val notifications by lazy { NotificationOwner(this) }
    override fun onCreate() {
        super.onCreate()
        firebasePush = FirebasePush(this)
        firebasePush.status()
    }
}
internal fun Context.firebasePush(): FirebasePush = (applicationContext as SkidNotificationApplication).firebasePush
internal fun Context.notificationOwner(): NotificationOwner =
    (applicationContext as SkidNotificationApplication).notifications

internal enum class NotificationHealth { Ready, Disabled, SetupRequired, Unconfigured, PermissionBlocked, DeliveryUnavailable, ResetRequired, RestartRequired }
internal fun notificationHealthMessage(health: NotificationHealth): String = when (health) {
    NotificationHealth.Disabled -> "background notifications are disabled on the hub."
    NotificationHealth.RestartRequired -> "stop and reopen skid to activate changed notification settings."
    NotificationHealth.Unconfigured -> "configure android push credentials on the hub."
    NotificationHealth.Ready -> "allow notifications for needs input."
    NotificationHealth.SetupRequired, NotificationHealth.ResetRequired -> "notification setup required."
    NotificationHealth.PermissionBlocked -> "notifications are blocked in system settings."
    NotificationHealth.DeliveryUnavailable -> "notification delivery unavailable."
}
internal data class NotificationView(
    val device: DeviceNotifications = DeviceNotifications(), val snapshot: ObserverSnapshot? = null,
    val health: NotificationHealth = NotificationHealth.SetupRequired,
    val resetIdentity: Any?, val observerUnavailable: Boolean = false,
) {
    fun presentsReady(key: NotificationKey, session: TmuxSession): Boolean {
        if (observerUnavailable || health == NotificationHealth.ResetRequired) return false
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
internal class NotificationVisitSeed(
    val resetIdentity: Any?, val epoch: String?, val candidate: NotificationVisitToken?,
)
private data class NotificationVisit(val token: NotificationVisitToken, val presented: Boolean)
private data class NotificationReadyBoundary(val token: NotificationVisitToken, val receivedGeneration: Long)
private data class NotificationFocus(val resumed: Boolean, val windowFocused: Boolean)
private data class NotificationRead(val snapshot: ObserverSnapshot, val alienHint: NotificationHint?)
private data class NotificationFetch(
    val fence: Long, val credential: MachineCredential,
    val minimumRevision: Long, val alienHint: NotificationHint?,
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
    private val client = NotificationClient(context)
    private val fleetClient = FleetClient(MachineStore(context))
    private val native = NativeNotifications(context)
    private val mutex = Mutex()
    private val initialized = CompletableDeferred<Unit>()
    private val effects = Channel<Unit>(Channel.CONFLATED)
    private val visitOutputs = Channel<NotificationVisitToken>(Channel.UNLIMITED)
    private val pushStore = PushStore(context)
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
            if (context.firebasePush().status() == FirebasePushStatus.Ready) NotificationEnrollment.schedule(context, ExistingWorkPolicy.KEEP)
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

    suspend fun refreshAndPresent(): Boolean {
        val admitted = fetchSnapshot() != null
        val through = mutex.withLock { device.localRevision }
        effects.trySend(Unit)
        effectsCompleted.first { it >= through }
        return admitted
    }

    private suspend fun fetchSnapshot(): NotificationRead? {
        initialized.await()
        val task = mutex.withLock {
            if (!storageAvailable || epochMismatch || resetting) return null
            fetchTask?.let { return@withLock it }
            scope.async<NotificationRead?> {
                val ownedTask = checkNotNull(currentCoroutineContext()[Job])
                try {
                    var read: NotificationRead? = null
                    while (true) {
                        fleetClient.refresh(4_000)
                        context.firebasePush().status()
                        val request = mutex.withLock {
                            if (!storageAvailable || epochMismatch || resetting) {
                                if (fetchTask === ownedTask) fetchTask = null
                                return@async null
                            }
                            val credential = MachineStore(context).association()?.hubCredential() ?: run {
                                snapshot = null
                                unavailable = true
                                publish()
                                if (fetchTask === ownedTask) fetchTask = null
                                return@async null
                            }
                            NotificationFetch(fence, credential, maxOf(device.admittedRevision, hintFloor), alienHint)
                        }
                        val registration = pushRegistration()
                        val receiver = registration?.let { client.receiver(request.credential) }
                        val result = client.state(request.credential)
                        val retry = mutex.withLock {
                            if (request.fence != fence ||
                                MachineStore(context).association()?.hubCredential() != request.credential ||
                                request.alienHint !== alienHint) return@withLock true
                            if (!storageAvailable || epochMismatch || resetting) {
                                if (fetchTask === ownedTask) fetchTask = null
                                return@withLock false
                            }
                            val fleet = MachineStore(context).association()
                            if (registration != null && receiver != null && context.firebasePush().target()?.configuration == registration.configuration &&
                                fleet?.deviceId == registration.deviceId &&
                                fleet.fleetId == registration.fleetId && receiver.desiredGeneration == registration.generation &&
                                receiver.activeGeneration >= registration.activeGeneration) pushStore.status(registration, receiver)
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

    suspend fun receiveHint(bytes: ByteArray): Boolean {
        if (bytes.size !in 1..256) return true
        val hint = try {
            productJson.decodeFromJsonElement<NotificationHint>(notificationJson(bytes.decodeToString(throwOnInvalidSequence = true)))
        } catch (_: kotlinx.serialization.SerializationException) { return true }
          catch (_: IllegalArgumentException) { return true }
        initialized.await()
        mutex.withLock {
            if (device.observerEpoch == hint.epoch) {
                if (hint.revision < maxOf(device.admittedRevision, hintFloor)) return true
                hintFloor = maxOf(hintFloor, hint.revision)
                if (snapshot?.revision?.let { it < hintFloor } == true) {
                    snapshot = null
                    publish()
                }
            } else {
                val previous = alienHint
                if (previous?.epoch == hint.epoch && hint.revision < previous.revision) return true
                if (previous != hint) {
                    alienHint = hint
                    snapshot = null
                    publish()
                }
            }
        }
        return refreshAndPresent()
    }

    fun fleetChanged() {
        val push = context.firebasePush().status()
        scope.launch {
            initialized.await()
            mutex.withLock { ++fence; snapshot = null; publish() }
            if (push == FirebasePushStatus.Ready) NotificationEnrollment.schedule(context, ExistingWorkPolicy.REPLACE)
            refresh()
        }
    }

    suspend fun enroll(): NotificationEnrollmentResult {
        initialized.await()
        if (context.firebasePush().status() != FirebasePushStatus.Ready) return NotificationEnrollmentResult.Installed
        fleetClient.refresh(4_000)
        val credential = MachineStore(context).association()?.hubCredential() ?: return NotificationEnrollmentResult.Unavailable
        if (!mutex.withLock { storageAvailable && !epochMismatch && !resetting }) return NotificationEnrollmentResult.Conflict
        if (context.firebasePush().status() != FirebasePushStatus.Ready || !native.allowed()) return NotificationEnrollmentResult.Installed
        val desired = context.firebasePush().register() ?: run {
            deliveryUnavailable()
            return NotificationEnrollmentResult.Unavailable
        }
        val fleet = MachineStore(context).association() ?: return NotificationEnrollmentResult.Conflict
        if (context.firebasePush().target() != PushTarget(desired.configuration, desired.fleetId, desired.deviceId) ||
            desired.fleetId != fleet.fleetId || desired.deviceId != fleet.deviceId) return NotificationEnrollmentResult.Conflict
        val result = client.enroll(credential, desired)
        mutex.withLock {
            if (MachineStore(context).association()?.hubCredential() != credential ||
                context.firebasePush().target() != PushTarget(desired.configuration, desired.fleetId, desired.deviceId)) return NotificationEnrollmentResult.Conflict
            if (result == null) { pushStore.registrationUnavailable(desired); publish(); return NotificationEnrollmentResult.Unavailable }
            if (!pushStore.status(desired, result, confirmed = desired.challenge != null)) return NotificationEnrollmentResult.Conflict
            publish()
        }
        refresh()
        return if (result?.status == "active") NotificationEnrollmentResult.Installed else NotificationEnrollmentResult.Unavailable
    }

    fun activityFocus(resumed: Boolean, windowFocused: Boolean) {
        focus = NotificationFocus(resumed, windowFocused)
    }

    fun captureVisit(target: SessionTarget): NotificationVisitSeed {
        val view = current.value
        val slot = NotificationKey(target).slot
        val record = view.device.records.singleOrNull { it.key.slot == slot }
        val admitted = view.snapshot?.takeIf { it.epoch == view.device.observerEpoch }
            ?.machines?.flatMap { it.sessions.orEmpty() }?.singleOrNull {
                it.record.key == record?.key && it.record.foreground == record?.foreground && it.readyQualified
            }
        val candidate = record?.takeIf {
            admitted != null && it.foreground != null && it.receivedReadyGeneration > it.acknowledgedReadyGeneration
        }?.let {
            NotificationVisitToken(it.key, it.foreground, view.device.observerEpoch, it.receivedReadyGeneration, view.resetIdentity)
        }
        return NotificationVisitSeed(view.resetIdentity, view.device.observerEpoch, candidate)
    }

    fun qualifyVisit(
        seed: NotificationVisitSeed, target: SessionTarget, paneId: String, foreground: ForegroundObservation?,
    ): NotificationVisitToken {
        val key = NotificationKey(target.machineHandle.encoded, target.session.tmuxId, target.session.identityToken, paneId)
        val observed = foreground?.let { NotificationForeground(it.provider, it.pid, it.startIdentity) }
        val candidate = seed.candidate?.takeIf { observed != null && it.key == key && it.foreground == observed }
        return NotificationVisitToken(key, observed, seed.epoch, candidate?.readyGeneration ?: 0, seed.resetIdentity)
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
        context.firebasePush().unavailable()
        scope.launch { initialized.await(); mutex.withLock { publish() } }
    }

    suspend fun reset() {
        initialized.await()
        mutex.withLock {
            if (!storageAvailable) return
            // Keep exact cancellation intent and acknowledgement until owned native absence is known.
            if (!commit(device.copy(records = device.records.map { it.copy(presentation = NotificationDelivery.Closed) }))) return
            ++fence
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
        val fleet = MachineStore(context).association()?.view?.machines ?: return false
        if (value.machines.map { it.machine }.toSet() != fleet.map { it.machine }.toSet()) {
            unavailable = true
            snapshot = null
            publish()
            return false
        }
        if (device.observerEpoch != null && device.observerEpoch != value.epoch) {
            epochMismatch = true
            unavailable = false
            snapshot = null
            publish()
            return false
        }
        if (value.revision < device.admittedRevision) return false
        val presentMachines = fleet.map { it.machine }.toSet()
        val records = device.records.map { if (it.key.machine !in presentMachines) it.copy(presentation = NotificationDelivery.Closed) else it }
            .associateBy { it.key.slot }.toMutableMap()
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
                val configured = MachineStore(context).read().credentials.isNotEmpty()
                if (configured) {
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

    private fun pushRegistration(): PushRegistration? = try { pushStore.read() }
        catch (_: IOException) { null }
        catch (_: IllegalArgumentException) { null }
        catch (_: kotlinx.serialization.SerializationException) { null }

    private fun publish() {
        val push = context.firebasePush().status()
        val registration = pushRegistration()
        val health = when {
            !storageAvailable || unavailable -> NotificationHealth.DeliveryUnavailable
            epochMismatch || resetting -> NotificationHealth.ResetRequired
            push == FirebasePushStatus.Disabled -> NotificationHealth.Disabled
            push == FirebasePushStatus.RestartRequired -> NotificationHealth.RestartRequired
            push == FirebasePushStatus.Unavailable -> NotificationHealth.DeliveryUnavailable
            !native.allowed() || com.google.android.gms.common.GoogleApiAvailability.getInstance()
                .isGooglePlayServicesAvailable(context) != com.google.android.gms.common.ConnectionResult.SUCCESS -> NotificationHealth.PermissionBlocked
            registration?.status == "unconfigured" -> NotificationHealth.Unconfigured
            registration == null -> NotificationHealth.SetupRequired
            registration.transportUnavailable || registration.status == "unavailable" -> NotificationHealth.DeliveryUnavailable
            registration.status != "active" -> NotificationHealth.SetupRequired
            else -> NotificationHealth.Ready
        }
        current.value = NotificationView(device, snapshot, health, resetIdentity, !storageAvailable || unavailable)
    }
}
