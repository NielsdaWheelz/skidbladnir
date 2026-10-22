package dev.niels.skidbladnir

import android.content.Context
import androidx.work.BackoffPolicy
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

/** Durable registration reconciliation; an authenticated PUT of the current registration activates delivery.
 * Registration capabilities never enter WorkManager's input or output.
 */
class NotificationEnrollment(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        if (applicationContext.firebasePush().status() != FirebasePushStatus.Ready) return@withContext Result.success()
        when (applicationContext.notificationOwner().enroll()) {
            NotificationEnrollmentResult.Installed -> Result.success()
            NotificationEnrollmentResult.Conflict, NotificationEnrollmentResult.Unavailable -> Result.retry()
        }
    }

    internal companion object {
        fun cancel(context: Context) { WorkManager.getInstance(context).cancelUniqueWork("notification-enrollment") }
        fun schedule(context: Context, policy: ExistingWorkPolicy) {
            if (context.firebasePush().status() != FirebasePushStatus.Ready) return
            val request = OneTimeWorkRequestBuilder<NotificationEnrollment>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 10, TimeUnit.SECONDS)
                .build()
            WorkManager.getInstance(context).enqueueUniqueWork("notification-enrollment", policy, request)
        }
    }
}
