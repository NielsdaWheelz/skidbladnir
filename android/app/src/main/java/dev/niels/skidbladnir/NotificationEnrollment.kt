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

/** Sole registration entrypoint: durable before REGISTER, complete only after receipt and server CAS.
 * Endpoint capabilities never enter WorkManager's input or output.
 */
class NotificationEnrollment(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        when (applicationContext.notificationOwner().enroll()) {
            NotificationEnrollmentResult.Installed -> Result.success()
            NotificationEnrollmentResult.Conflict, NotificationEnrollmentResult.Unavailable -> Result.retry()
        }
    }

    internal companion object {
        fun schedule(context: Context, policy: ExistingWorkPolicy) {
            val request = OneTimeWorkRequestBuilder<NotificationEnrollment>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .setBackoffCriteria(BackoffPolicy.EXPONENTIAL, 10, TimeUnit.SECONDS)
                .build()
            WorkManager.getInstance(context).enqueueUniqueWork("notification-enrollment", policy, request)
        }
    }
}
