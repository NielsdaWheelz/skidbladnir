package dev.niels.skidbladnir

import android.app.Application
import android.content.Context
import androidx.work.ExistingWorkPolicy
import java.io.IOException
import java.security.GeneralSecurityException
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

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

internal enum class NotificationHealth { Ready, Disabled, SetupRequired, Unconfigured, PermissionBlocked, DeliveryUnavailable, RestartRequired }
internal fun notificationHealthMessage(health: NotificationHealth): String = when (health) {
    NotificationHealth.Disabled -> "background notifications are disabled on the hub."
    NotificationHealth.RestartRequired -> "stop and reopen skid to activate changed notification settings."
    NotificationHealth.Unconfigured -> "configure android push credentials on the hub."
    NotificationHealth.Ready -> "allow notifications for needs input."
    NotificationHealth.SetupRequired -> "notification setup required."
    NotificationHealth.PermissionBlocked -> "notifications are blocked in system settings."
    NotificationHealth.DeliveryUnavailable -> "notification delivery unavailable."
}
internal data class NotificationPresentation(val ready: Boolean = false, val unavailable: Boolean = false)

/** The last hub state this process read (null: the read failed) and the health line. */
internal data class NotificationView(val state: HubNotifications?, val health: NotificationHealth) {
    val rows: Map<NotificationKey, NotificationRow> = state?.machines.orEmpty().flatMap { it.sessions }
        .mapNotNull { row -> notificationReference(row.ref)?.let { it to row } }.toMap()

    /** The hub's pending ready, qualified by current direct evidence for the same agent lifetime. */
    fun presentsReady(key: NotificationKey, session: TmuxSession): Boolean {
        val row = rows[key] ?: return false
        return row.readyGeneration != 0L && session.connection == null &&
            row.foreground == session.agent?.let(::NotificationForeground) &&
            session.terminalStatus.source == TerminalStatusSource.Terminal &&
            session.terminalStatus.activity == TerminalActivity.Idle && session.terminalStatus.interaction == TerminalInteraction.None
    }
}

/**
 * The process's one notification owner: a single sync loop reconciles the hub's state against the shade.
 * The hub holds the owner's acknowledgements; this process keeps only the episodes it alerted or saw dismissed.
 */
internal class NotificationOwner(private val context: Context) {
    internal val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val client = NotificationClient()
    private val native = NativeNotifications(context)
    private val pushStore = PushStore(context)
    private val fleetClient = FleetClient(MachineStore(context))
    // slot -> episode this process alerted or the owner dismissed or clicked; written by the sync loop and handled().
    private val alerted = ConcurrentHashMap<String, Long>()
    private val requests = Channel<Unit>(Channel.CONFLATED)
    private val requested = AtomicLong()
    private val completed = MutableStateFlow(0L)
    @Volatile private var credential: MachineCredential? = readCredential()
    @Volatile private var lastOk = false
    // Empty until the first read, so the dashboard does not flash "notifications unavailable" at start.
    private val current = MutableStateFlow(NotificationView(HubNotifications(emptyList()), NotificationHealth.SetupRequired))
    val view: StateFlow<NotificationView> = current

    init {
        // A request made during a sync gets one more sync: single flight with a trailing run.
        scope.launch {
            for (request in requests) {
                val through = requested.get()
                lastOk = sync()
                completed.value = through
            }
        }
        scope.launch { NotificationEnrollment.schedule(context, ExistingWorkPolicy.KEEP) }
    }

    fun refresh() {
        requested.incrementAndGet()
        requests.trySend(Unit)
    }

    suspend fun refreshAndPresent(): Boolean {
        val ticket = requested.incrementAndGet()
        requests.trySend(Unit)
        completed.first { it >= ticket }
        return lastOk
    }

    private fun sync(): Boolean {
        // The shade is read before the hub: a notice swiped during the read is still a same-episode copy, never an
        // absent one that would post again.
        val delivered = native.delivered()
        val captured = credential
        val state = captured?.let(client::state)
        // A fleet change during the read already queued the next sync.
        if (captured != credential) return false
        if (state != null && delivered != null) {
            for (effect in reconcileNotifications(state, delivered, alerted, native.allowed())) {
                val slot = effect.notice.slot
                try {
                    if (effect.cancel) {
                        native.cancel(slot)
                        alerted.remove(slot)
                    } else {
                        native.post(effect.notice, silent = !effect.audible)
                        if (effect.audible) alerted[slot] = effect.notice.episode
                    }
                } catch (_: SecurityException) { /* permission withdrawn mid-sync: the next sync retries */ }
            }
        }
        val push = context.firebasePush().status()
        val health = when {
            state == null || delivered == null -> NotificationHealth.DeliveryUnavailable
            push == FirebasePushStatus.Disabled -> NotificationHealth.Disabled
            push == FirebasePushStatus.RestartRequired -> NotificationHealth.RestartRequired
            push == FirebasePushStatus.Unavailable -> NotificationHealth.DeliveryUnavailable
            !native.allowed() || com.google.android.gms.common.GoogleApiAvailability.getInstance()
                .isGooglePlayServicesAvailable(context) != com.google.android.gms.common.ConnectionResult.SUCCESS -> NotificationHealth.PermissionBlocked
            state.receiver?.status == "unconfigured" -> NotificationHealth.Unconfigured
            registration() == null -> NotificationHealth.SetupRequired
            state.receiver?.status != "active" -> NotificationHealth.DeliveryUnavailable
            else -> NotificationHealth.Ready
        }
        current.value = NotificationView(state, health)
        return state != null && delivered != null
    }

    /** The controller calls this on every fleet mutation; the sync reads only this cached credential. */
    fun fleetChanged() {
        credential = readCredential()
        NotificationEnrollment.schedule(context, ExistingWorkPolicy.REPLACE)
        refresh()
    }

    /** Dismiss or click: the episode is handled for the owner on every device, and this process never posts it again. */
    suspend fun handled(slot: String, episode: Long) {
        alerted[slot] = episode
        val credential = credential ?: return
        if (client.handled(credential, slot, episode, 0)) refreshAndPresent()
    }

    /** The capture at entry: this pane's row from a fresh read, concurrent with dialing. */
    fun beginVisit(key: NotificationKey): Deferred<NotificationRow?> = scope.async {
        refreshAndPresent()
        current.value.rows[key]
    }

    /** The visit's first output acknowledges what the capture saw: its action episode and pending ready. */
    fun presented(visit: Deferred<NotificationRow?>) {
        scope.launch {
            val row = visit.await() ?: return@launch
            if (row.episode == 0L && row.readyGeneration == 0L) return@launch
            val credential = credential ?: return@launch
            if (client.handled(credential, row.slot, row.episode, row.readyGeneration)) refresh()
        }
    }

    suspend fun enroll(): NotificationEnrollmentResult {
        val push = context.firebasePush()
        if (push.status() != FirebasePushStatus.Ready || !native.allowed()) return NotificationEnrollmentResult.Installed
        // The hub's fleet view carries the push options this registration must match.
        fleetClient.refresh(4_000)
        val credential = readCredential() ?: return NotificationEnrollmentResult.Unavailable
        val registration = push.register() ?: return NotificationEnrollmentResult.Unavailable
        val target = push.target() ?: return NotificationEnrollmentResult.Conflict
        val result = client.register(credential, target.configuration, registration.generation, registration.fid)
        if (result == NotificationEnrollmentResult.Conflict) pushStore.renew(registration)
        if (result == NotificationEnrollmentResult.Installed) refresh()
        return result
    }

    fun settingsIntent(): android.content.Intent = native.settingsIntent()

    private fun readCredential(): MachineCredential? = try { MachineStore(context).association()?.hubCredential() }
        catch (_: IOException) { null }
        catch (_: GeneralSecurityException) { null }
        catch (_: IllegalArgumentException) { null }

    private fun registration(): PushRegistration? = try { pushStore.read() }
        catch (_: IOException) { null }
        catch (_: IllegalArgumentException) { null }
}
