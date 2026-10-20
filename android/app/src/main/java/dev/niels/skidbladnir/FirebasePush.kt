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

/** The default SDK app is immutable for this process; signed pairing state owns the desired options. */
internal class FirebasePush(private val context: Context) {
    private val store = PushStore(context)
    private val enrollment = Mutex()
    @Volatile private var options: AndroidPush? = null
    @Volatile private var request: FirebaseRegistration? = null

    private fun desired(): Pair<AndroidPush, PushTarget>? {
        val fleet = MachineStore(context).association() ?: return null
        val value = fleet.androidPush() ?: return null
        return value to PushTarget(value.id, fleet.fleetId, fleet.deviceId)
    }
    fun target(): PushTarget? = try {
        val desired = desired()
        desired?.second?.takeIf { options == desired.first }
    } catch (_: IOException) { null }
      catch (_: GeneralSecurityException) { null }
      catch (_: IllegalArgumentException) { null }
      catch (_: kotlinx.serialization.SerializationException) { null }
    @Synchronized fun reconcile(): FirebasePushStatus {
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
    fun status(): FirebasePushStatus = try { reconcile() }
        catch (_: IOException) { FirebasePushStatus.Unavailable }
        catch (_: GeneralSecurityException) { FirebasePushStatus.Unavailable }
        catch (_: IllegalArgumentException) { FirebasePushStatus.Unavailable }
        catch (_: kotlinx.serialization.SerializationException) { FirebasePushStatus.Unavailable }

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
            if (status() == FirebasePushStatus.Ready) NotificationEnrollment.schedule(context, androidx.work.ExistingWorkPolicy.KEEP)
            return
        }
        if (target() != captured.attempt.target) return
        FirebaseInstallations.getInstance().id.addOnCompleteListener { task ->
            if (!task.isSuccessful || task.result != fid) return@addOnCompleteListener
            try {
                MachineStore(context).withAssociation { fleet ->
                    val desired = fleet?.androidPush()
                    val target = desired?.let { PushTarget(it.id, checkNotNull(fleet).fleetId, fleet.deviceId) }
                    if (request === captured && options == desired && target == captured.attempt.target)
                        captured.result.complete(store.registered(captured.attempt, fid))
                }
            } catch (_: IOException) { captured.result.complete(null) }
              catch (_: GeneralSecurityException) { captured.result.complete(null) }
              catch (_: IllegalArgumentException) { captured.result.complete(null) }
              catch (_: kotlinx.serialization.SerializationException) { captured.result.complete(null) }
        }
    }

    fun accept(sender: String?, data: Map<String, String>): Boolean = MachineStore(context).withAssociation { fleet ->
        val desired = fleet?.androidPush() ?: return@withAssociation false
        if (options != desired || sender != desired.senderId) return@withAssociation false
        store.accept(PushTarget(desired.id, fleet.fleetId, fleet.deviceId), data)
    }
    fun unavailable() {
        try {
            MachineStore(context).withAssociation { fleet ->
                val desired = fleet?.androidPush()
                if (desired != null && options == desired) store.unavailable(PushTarget(desired.id, checkNotNull(fleet).fleetId, fleet.deviceId))
            }
        } catch (_: IOException) { /* push status already reports inaccessible push storage */ }
          catch (_: GeneralSecurityException) { /* inaccessible pairing grants no push authority */ }
          catch (_: IllegalArgumentException) { /* invalid optional push state cannot interrupt session management */ }
          catch (_: kotlinx.serialization.SerializationException) { /* invalid optional push state cannot interrupt session management */ }
    }

}

private suspend fun Task<Void>.completed(): Boolean = suspendCancellableCoroutine { continuation ->
    addOnCompleteListener { task -> if (continuation.isActive) continuation.resumeWith(Result.success(task.isSuccessful)) }
}
