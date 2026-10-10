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
private const val NOTICE_EPISODE = "skid.episode"
private const val NOTICE_REFERENCE = "skid.reference"
private const val NOTICE_SLOT = "skid.slot"
private const val NOTICE_EPOCH = "skid.epoch"
private const val NOTICE_READY_GENERATION = "skid.ready-generation"
internal data class NativeNotice(
    val slot: String, val episode: Long, val epoch: String, val ref: String,
    val title: String, val body: String, val readyGeneration: Long,
) {
    fun sameIdentity(other: NativeNotice): Boolean = slot == other.slot && episode == other.episode &&
        epoch == other.epoch && ref == other.ref && readyGeneration == other.readyGeneration
}
internal data class NativeInspection(val observed: List<NativeNotice>?, val unresolved: List<NativeNotice>)
internal data class NotificationClick(val ref: String, val slot: String, val episode: Long, val epoch: String)

/** The application owner's ordered effect lane owns all submission state. */
internal class NativeNotifications(private val context: Context) {
    private val manager = context.getSystemService(NotificationManager::class.java)
    private val unresolved = mutableMapOf<String, NativeNotice>()
    init {
        manager.createNotificationChannel(NotificationChannel(
            NEEDS_INPUT_CHANNEL, "needs input", NotificationManager.IMPORTANCE_HIGH,
        ).apply { enableVibration(true) })
    }

    fun allowed(): Boolean = manager.areNotificationsEnabled() &&
        manager.getNotificationChannel(NEEDS_INPUT_CHANNEL).importance != NotificationManager.IMPORTANCE_NONE

    /** notify-return precedes OS visibility; only the exact observed copy settles a submission. */
    fun inspect(): NativeInspection {
        val observed = notices()
        if (observed != null) unresolved.entries.removeAll { it.value in observed }
        return NativeInspection(observed, unresolved.values.toList())
    }

    private fun notices(): List<NativeNotice>? = try {
        manager.activeNotifications.filter { it.id == 0 && it.notification.channelId == NEEDS_INPUT_CHANNEL }
            .map { status ->
                val extras = status.notification.extras
                NativeNotice(status.tag, extras.getLong(NOTICE_EPISODE), extras.getString(NOTICE_EPOCH).orEmpty(), extras.getString(NOTICE_REFERENCE).orEmpty(),
                    extras.getCharSequence(Notification.EXTRA_TITLE)?.toString().orEmpty(),
                    extras.getCharSequence(Notification.EXTRA_TEXT)?.toString().orEmpty(), extras.getLong(NOTICE_READY_GENERATION))
            }
    } catch (_: SecurityException) {
        null
    }

    fun post(row: NotificationRow, epoch: String, silent: Boolean, readyGeneration: Long, rename: NativeNotice? = null): NativeNotice {
        val slot = row.record.key.slot
        val episode = rename?.episode ?: row.record.attentionEpisode
        val ref = rename?.ref ?: row.ref
        val expected = NativeNotice(slot, episode, epoch, ref, notificationTitle(row.name),
            rename?.body ?: checkNotNull(row.record.cause).text, readyGeneration)
        val click = Intent(context, MainActivity::class.java).apply {
            action = "dev.niels.skidbladnir.NOTIFICATION_OPEN"
            data = Uri.Builder().scheme("skid-notice").authority("open").appendPath(slot).appendPath(epoch).appendPath(episode.toString()).build()
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra(NOTICE_SLOT, slot)
            putExtra(NOTICE_EPISODE, episode)
            putExtra(NOTICE_REFERENCE, ref)
            putExtra(NOTICE_EPOCH, epoch)
        }
        val dismiss = Intent(context, NotificationDismissReceiver::class.java).apply {
            data = Uri.Builder().scheme("skid-notice").authority("dismiss").appendPath(slot).appendPath(epoch).appendPath(episode.toString()).build()
            putExtra(NOTICE_SLOT, slot)
            putExtra(NOTICE_EPISODE, episode)
            putExtra(NOTICE_EPOCH, epoch)
        }
        val pendingFlags = PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        val notification = NotificationCompat.Builder(context, NEEDS_INPUT_CHANNEL)
            .setSmallIcon(R.drawable.ic_launcher_monochrome)
            .setContentTitle(expected.title)
            .setContentText(expected.body)
            .setContentIntent(PendingIntent.getActivity(context, 0, click, pendingFlags))
            .setDeleteIntent(PendingIntent.getBroadcast(context, 0, dismiss, pendingFlags))
            .setAutoCancel(false)
            .setSilent(silent)
            .setOnlyAlertOnce(silent)
            .setExtras(android.os.Bundle().apply {
                putString(NOTICE_SLOT, slot)
                putLong(NOTICE_EPISODE, episode)
                putString(NOTICE_REFERENCE, ref)
                putString(NOTICE_EPOCH, epoch)
                putLong(NOTICE_READY_GENERATION, readyGeneration)
            })
            .build()
        manager.notify(slot, 0, notification)
        unresolved[slot] = expected
        return expected
    }

    fun cancel(notice: NativeNotice) {
        val pending = unresolved[notice.slot]
        // cancel addresses the whole slot; an older visible copy cannot authorize cancelling its successor.
        if (pending != null && !pending.sameIdentity(notice)) return
        if (pending != null || notices()?.any { it.sameIdentity(notice) } == true) {
            manager.cancel(notice.slot, 0)
            unresolved.remove(notice.slot)
        }
    }

    fun settingsIntent(): Intent = Intent(Settings.ACTION_CHANNEL_NOTIFICATION_SETTINGS).apply {
        putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
        putExtra(Settings.EXTRA_CHANNEL_ID, NEEDS_INPUT_CHANNEL)
    }
}

internal fun notificationTitle(name: String): String = buildString {
    name.codePoints().forEach { codePoint ->
        if (Character.isISOControl(codePoint) || Character.getType(codePoint) == Character.FORMAT.toInt() ||
            Character.getType(codePoint) == Character.LINE_SEPARATOR.toInt() || Character.getType(codePoint) == Character.PARAGRAPH_SEPARATOR.toInt()) {
            append(' ')
        } else appendCodePoint(codePoint)
    }
}

internal fun notificationClick(intent: Intent): NotificationClick? {
    if (intent.action != "dev.niels.skidbladnir.NOTIFICATION_OPEN") return null
    val ref = intent.getStringExtra(NOTICE_REFERENCE) ?: return null
    val slot = intent.getStringExtra(NOTICE_SLOT) ?: return null
    val episode = intent.getLongExtra(NOTICE_EPISODE, 0)
    val epoch = intent.getStringExtra(NOTICE_EPOCH) ?: return null
    val key = try { notificationReference(ref) } catch (_: IllegalArgumentException) { return null }
        catch (_: kotlinx.serialization.SerializationException) { return null }
    return if (key.slot == slot && episode > 0 && epoch.matches(Regex("[0-9a-f]{32}"))) NotificationClick(ref, slot, episode, epoch) else null
}

class NotificationDismissReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val slot = intent.getStringExtra(NOTICE_SLOT) ?: return
        val episode = intent.getLongExtra(NOTICE_EPISODE, 0)
        val epoch = intent.getStringExtra(NOTICE_EPOCH) ?: return
        if (episode <= 0) return
        val pending = goAsync()
        val owner = context.notificationOwner()
        owner.scope.launch {
            try { owner.dismiss(slot, episode, epoch) } finally { pending.finish() }
        }
    }
}
