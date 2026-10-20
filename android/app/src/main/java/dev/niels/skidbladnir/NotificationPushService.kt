package dev.niels.skidbladnir

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import androidx.work.ExistingWorkPolicy
import java.io.IOException
import java.security.GeneralSecurityException

/** FCM carries only enrollment proofs or fetch hints; admission still requires an authenticated read. */
class NotificationPushService : FirebaseMessagingService() {
    override fun onRegistered(installationId: String) { firebasePush().registered(installationId) }
    // unregister is used only for the owned cache reset; its callback cannot identify a request generation.
    override fun onMessageReceived(message: RemoteMessage) {
        if (message.notification != null || message.data.size !in 1..9 || message.data.any { it.key.length > 32 || it.value.length > 256 }) return
        try {
            if (!firebasePush().accept(message.from, message.data)) return
            when (message.data["kind"]) {
                "enrollment" -> NotificationEnrollment.schedule(this, ExistingWorkPolicy.REPLACE)
                "hint" -> NotificationRefresh.schedule(this, message.priority == RemoteMessage.PRIORITY_HIGH)
            }
        } catch (_: IOException) { notificationOwner().deliveryUnavailable() }
          catch (_: GeneralSecurityException) { /* inaccessible pairing state grants no push authority */ }
          catch (_: IllegalArgumentException) { /* malformed external hints have no product authority */ }
    }
    override fun onDeletedMessages() {
        if (firebasePush().target() != null) NotificationRefresh.schedule(this, false)
    }
}
