package dev.niels.skidbladnir

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.Constraints
import androidx.work.NetworkType
import androidx.work.WorkManager
import androidx.work.ExistingWorkPolicy
import androidx.work.OutOfQuotaPolicy
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.serialization.encodeToString

/** A bounded wake fetches current state; neither push bytes nor the worker can admit attention. */
class NotificationRefresh(context: Context, parameters: WorkerParameters) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result {
        if (applicationContext.firebasePush().target() == null) return Result.success()
        val completed = withTimeoutOrNull(10_000) {
            val hint = PushStore(applicationContext).read()?.hint
            if (hint != null) applicationContext.notificationOwner().receiveHint(productJson.encodeToString(hint).encodeToByteArray())
            else applicationContext.notificationOwner().refreshAndPresent()
        }
        return if (completed == true) Result.success() else if (runAttemptCount < 3) Result.retry() else Result.failure()
    }
    internal companion object {
        fun cancel(context: Context) { WorkManager.getInstance(context).cancelUniqueWork("notification-refresh") }
        fun schedule(context: Context, expedited: Boolean) {
            if (context.firebasePush().target() == null) return
            val request = OneTimeWorkRequestBuilder<NotificationRefresh>()
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .setBackoffCriteria(androidx.work.BackoffPolicy.EXPONENTIAL, 10, java.util.concurrent.TimeUnit.SECONDS)
            if (expedited) request.setExpedited(OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST)
            WorkManager.getInstance(context).enqueueUniqueWork("notification-refresh", ExistingWorkPolicy.REPLACE, request.build())
        }
    }
}
