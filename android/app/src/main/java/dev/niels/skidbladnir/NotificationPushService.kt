package dev.niels.skidbladnir

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/** FCM carries only a content-free wake; the authenticated state read is the admission. */
class NotificationPushService : FirebaseMessagingService() {
    override fun onRegistered(installationId: String) { firebasePush().registered(installationId) }
    // unregister is used only for the owned cache reset; its callback cannot identify a request generation.
    override fun onMessageReceived(message: RemoteMessage) {
        if (message.notification != null || message.data["kind"] != "hint" || !firebasePush().accept(message.from)) return
        NotificationRefresh.schedule(this, message.priority == RemoteMessage.PRIORITY_HIGH)
    }
    override fun onDeletedMessages() {
        if (firebasePush().target() != null) NotificationRefresh.schedule(this, false)
    }
}
