package dev.niels.skidbladnir

import android.content.Context
import com.google.android.gms.tasks.Task
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.installations.FirebaseInstallations
import com.google.firebase.messaging.FirebaseMessaging
import java.io.IOException
import java.security.GeneralSecurityException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withTimeoutOrNull

internal enum class FirebasePushStatus { Disabled, Ready, RestartRequired, Unavailable }
private class FirebaseRegistration(val attempt: PushAttempt, val result: CompletableDeferred<PushRegistration?> = CompletableDeferred())

/** Unreadable pairing or push state grants no push authority and never interrupts session management. */
private inline fun <T> guarded(fallback: T, block: () -> T): T = try { block() }
    catch (_: IOException) { fallback }
    catch (_: GeneralSecurityException) { fallback }
    catch (_: IllegalArgumentException) { fallback } // includes SerializationException

/** The default SDK app is immutable for this process; the hub's fleet view owns the desired options. */
internal class FirebasePush(private val context: Context) {
    private val store = PushStore(context)
    private val enrollment = Mutex()
    @Volatile private var options: AndroidPush? = null
    @Volatile private var request: FirebaseRegistration? = null

    private fun desired(fleet: PhoneFleet? = MachineStore(context).association()): Pair<AndroidPush, PushTarget>? {
        val value = fleet?.androidPush() ?: return null
        return value to PushTarget(value.id, fleet.fleetId, fleet.deviceId)
    }
    fun target(): PushTarget? = guarded(null) { desired()?.takeIf { options == it.first }?.second }
    @Synchronized private fun reconcile(): FirebasePushStatus {
        val desired = desired()
        store.reconcile(desired?.second)
        if (desired == null) {
            request?.result?.complete(null)
            request = null
            NotificationEnrollment.cancel(context)
            NotificationRefresh.cancel(context)
            return FirebasePushStatus.Disabled
        }
        // FIS checks these synchronously on the messaging executor, outside its IOException handler.
        // Invalid optional options must stay a push error rather than terminate the application.
        if (!desired.first.apiKey.matches(Regex("A[\\w-]{38}")) || !desired.first.applicationId.contains(':')) return FirebasePushStatus.Unavailable
        if (options == null) {
            val value = desired.first
            FirebaseApp.initializeApp(context, FirebaseOptions.Builder().setApiKey(value.apiKey)
                .setApplicationId(value.applicationId).setProjectId(value.projectId).setGcmSenderId(value.senderId).build())
            FirebaseMessaging.getInstance().isAutoInitEnabled = false
            options = value
        }
        if (options != desired.first) {
            request?.result?.complete(null)
            request = null
            NotificationEnrollment.cancel(context)
            NotificationRefresh.cancel(context)
            return FirebasePushStatus.RestartRequired
        }
        return FirebasePushStatus.Ready
    }
    fun status(): FirebasePushStatus = guarded(FirebasePushStatus.Unavailable) { reconcile() }

    suspend fun register(): PushRegistration? = enrollment.withLock {
        if (status() != FirebasePushStatus.Ready) return@withLock null
        val target = target() ?: return@withLock null
        val messaging = FirebaseMessaging.getInstance()
        if (store.resetRequired()) {
            // SDK callbacks can report a cached route that Google rejects after changing projects.
            // Persisted intent survives death until the selected project's unregister completes.
            if (withTimeoutOrNull(10_000) { messaging.unregister().completed() } != true || target() != target) return@withLock null
            store.resetCompleted(target.configuration)
        }
        if (target() != target) return@withLock null
        val pending = FirebaseRegistration(store.attempt(target))
        request = pending
        try {
            withTimeoutOrNull(14_000) {
                if (!messaging.register().completed() || target() != target) return@withTimeoutOrNull null
                pending.result.await()
            }
        } finally { if (request === pending) request = null }
    }
    fun registered(fid: String) {
        val captured = request
        if (captured == null) {
            // Rotation is a wake-up only. Never bind its unscoped raw callback to a fleet.
            NotificationEnrollment.schedule(context, androidx.work.ExistingWorkPolicy.KEEP)
            return
        }
        if (target() != captured.attempt.target) return
        FirebaseInstallations.getInstance().id.addOnCompleteListener { task ->
            if (!task.isSuccessful || task.result != fid) return@addOnCompleteListener
            captured.result.complete(guarded(null) {
                MachineStore(context).withAssociation { fleet ->
                    val desired = desired(fleet)
                    if (request === captured && desired != null && options == desired.first && desired.second == captured.attempt.target) {
                        store.registered(captured.attempt, fid)
                    } else null
                }
            })
        }
    }

    /** A hint counts only from the sender of the options this process runs. */
    fun accept(sender: String?): Boolean = guarded(false) { desired()?.first?.let { options == it && sender == it.senderId } == true }
}

private suspend fun Task<Void>.completed(): Boolean = suspendCancellableCoroutine { continuation ->
    addOnCompleteListener { task -> if (continuation.isActive) continuation.resumeWith(Result.success(task.isSuccessful)) }
}
