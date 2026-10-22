package dev.niels.skidbladnir

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.launch

private const val NEEDS_INPUT_CHANNEL = "needs-input"
private const val NOTIFICATION_OPEN = "dev.niels.skidbladnir.NOTIFICATION_OPEN"
private const val NOTICE_SLOT = "skid.slot"
private const val NOTICE_MACHINE = "skid.machine"
private const val NOTICE_REFERENCE = "skid.reference"
private const val NOTICE_EPISODE = "skid.episode"
internal data class NotificationClick(val ref: String, val slot: String, val episode: Long)

/** The os notification shade, one notice per slot (tag = slot, id 0). Only the owner's sync loop calls it. */
internal class NativeNotifications(private val context: Context) {
    private val manager = context.getSystemService(NotificationManager::class.java)
    init {
        manager.createNotificationChannel(NotificationChannel(
            NEEDS_INPUT_CHANNEL, "needs input", NotificationManager.IMPORTANCE_HIGH,
        ).apply { enableVibration(true) })
    }

    fun allowed(): Boolean = manager.areNotificationsEnabled() &&
        manager.getNotificationChannel(NEEDS_INPUT_CHANNEL).importance != NotificationManager.IMPORTANCE_NONE

    /** A missing extra reads as empty, so an older build's notices reconcile away. */
    fun delivered(): List<NotificationNotice>? = try {
        manager.activeNotifications.filter { it.id == 0 && it.notification.channelId == NEEDS_INPUT_CHANNEL }.map { status ->
            val extras = status.notification.extras
            NotificationNotice(status.tag.orEmpty(), extras.getString(NOTICE_MACHINE).orEmpty(), extras.getString(NOTICE_REFERENCE).orEmpty(),
                extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty(),
                extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty(), extras.getLong(NOTICE_EPISODE))
        }
    } catch (_: SecurityException) { null }

    fun post(notice: NotificationNotice, silent: Boolean) {
        val click = Intent(context, MainActivity::class.java).apply {
            action = NOTIFICATION_OPEN
            data = Uri.Builder().scheme("skid-notice").authority("open").appendPath(notice.slot).appendPath(notice.episode.toString()).build()
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra(NOTICE_SLOT, notice.slot)
            putExtra(NOTICE_EPISODE, notice.episode)
            putExtra(NOTICE_REFERENCE, notice.ref)
        }
        val dismiss = Intent(context, NotificationDismissReceiver::class.java).apply {
            data = Uri.Builder().scheme("skid-notice").authority("dismiss").appendPath(notice.slot).appendPath(notice.episode.toString()).build()
            putExtra(NOTICE_SLOT, notice.slot)
            putExtra(NOTICE_EPISODE, notice.episode)
        }
        val pendingFlags = PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        val notification = NotificationCompat.Builder(context, NEEDS_INPUT_CHANNEL)
            .setSmallIcon(R.drawable.ic_launcher_monochrome)
            .setContentTitle(notice.title)
            .setContentText(notice.body)
            .setContentIntent(PendingIntent.getActivity(context, 0, click, pendingFlags))
            .setDeleteIntent(PendingIntent.getBroadcast(context, 0, dismiss, pendingFlags))
            .setAutoCancel(false)
            .setSilent(silent)
            .setOnlyAlertOnce(silent)
            .setExtras(android.os.Bundle().apply {
                putString(NOTICE_MACHINE, notice.machine)
                putString(NOTICE_REFERENCE, notice.ref)
                putLong(NOTICE_EPISODE, notice.episode)
            })
            .build()
        manager.notify(notice.slot, 0, notification)
    }

    fun cancel(slot: String) = manager.cancel(slot, 0)

    fun settingsIntent(): Intent = Intent(Settings.ACTION_CHANNEL_NOTIFICATION_SETTINGS).apply {
        putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
        putExtra(Settings.EXTRA_CHANNEL_ID, NEEDS_INPUT_CHANNEL)
    }
}

internal fun notificationClick(intent: Intent): NotificationClick? {
    if (intent.action != NOTIFICATION_OPEN) return null
    val ref = intent.getStringExtra(NOTICE_REFERENCE) ?: return null
    val slot = intent.getStringExtra(NOTICE_SLOT) ?: return null
    return NotificationClick(ref, slot, intent.getLongExtra(NOTICE_EPISODE, 0))
}

/** A swipe acknowledges the episode for the owner on every device. */
class NotificationDismissReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val slot = intent.getStringExtra(NOTICE_SLOT) ?: return
        val episode = intent.getLongExtra(NOTICE_EPISODE, 0)
        val pending = goAsync()
        val owner = context.notificationOwner()
        owner.scope.launch {
            try { owner.handled(slot, episode) } finally { pending.finish() }
        }
    }
}
