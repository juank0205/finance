package dev.finance.wallet.capture

import android.app.Notification
import android.content.ComponentName
import android.service.notification.NotificationListenerService
import android.service.notification.StatusBarNotification
import dev.finance.wallet.WalletApp
import kotlinx.coroutines.launch

/**
 * Receives every notification once the user grants notification access, and stores the ones from
 * [capturedPackages]. Content from any other app is dropped without being stored.
 */
class WalletListenerService : NotificationListenerService() {

    override fun onListenerConnected() {
        Diagnostics.setConnected(true)
        // Catch up on Wallet notifications posted while the listener was not running.
        runCatching { activeNotifications }.getOrNull()?.forEach(::handle)
    }

    override fun onListenerDisconnected() {
        Diagnostics.setConnected(false)
        requestRebind(ComponentName(this, WalletListenerService::class.java))
    }

    override fun onNotificationPosted(sbn: StatusBarNotification) = handle(sbn)

    private fun handle(sbn: StatusBarNotification) {
        Diagnostics.seen(sbn.packageName)
        if (!shouldCapture(sbn.packageName, sbn.notification.flags)) return
        val event = sbn.toRawEvent(capturedAt = System.currentTimeMillis())
        val app = application as WalletApp
        app.scope.launch { app.db.rawEvents().insert(event) }
    }
}

private fun StatusBarNotification.toRawEvent(capturedAt: Long): RawEvent {
    val extras = notification.extras
    fun text(key: String) = extras.getCharSequence(key)?.toString().orEmpty()
    return RawEvent.of(
        packageName = packageName,
        notificationKey = key,
        channelId = notification.channelId.orEmpty(),
        postedAt = postTime,
        title = text(Notification.EXTRA_TITLE),
        text = text(Notification.EXTRA_TEXT),
        bigText = text(Notification.EXTRA_BIG_TEXT),
        subText = text(Notification.EXTRA_SUB_TEXT),
        capturedAt = capturedAt,
    )
}
