package dev.niels.skidbladnir

import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import org.unifiedpush.android.connector.FailedReason
import org.unifiedpush.android.connector.PushService
import org.unifiedpush.android.connector.data.PushEndpoint
import org.unifiedpush.android.connector.data.PushMessage

/** The official receiver owns token checks, key storage and binding lifetime. */
class NotificationPushService : PushService() {
    override fun onNewEndpoint(endpoint: PushEndpoint, instance: String) {
        if (instance != NOTIFICATION_INSTANCE || endpoint.temporary) return
        val keys = endpoint.pubKeySet ?: return
        val subscription = try {
            NotificationSubscription(endpoint.url, NotificationSubscriptionKeys(keys.pubKey, keys.auth))
        } catch (_: IllegalArgumentException) { return }
        // Enter the sole owner synchronously; the existing durable registration job owns replay.
        notificationOwner().endpoint(subscription)
    }

    override fun onMessage(message: PushMessage, instance: String) {
        if (instance != NOTIFICATION_INSTANCE || !message.decrypted || message.content.size !in 1..256) return
        val owner = notificationOwner()
        // ntfy lends process importance for five seconds through the library's separate
        // RaiseToForegroundService. The SDK may reuse this adapter after its own unbind.
        owner.scope.launch { withTimeoutOrNull(4_000) { owner.receiveHint(message.content) } }
    }

    override fun onRegistrationFailed(reason: FailedReason, instance: String) {
        if (instance == NOTIFICATION_INSTANCE) notificationOwner().deliveryUnavailable()
    }

    override fun onUnregistered(instance: String) {
        if (instance == NOTIFICATION_INSTANCE) notificationOwner().deliveryUnavailable()
    }

    override fun onTempUnavailable(instance: String) {
        if (instance == NOTIFICATION_INSTANCE) notificationOwner().deliveryUnavailable()
    }

}

internal const val NOTIFICATION_INSTANCE = "needs-input"
