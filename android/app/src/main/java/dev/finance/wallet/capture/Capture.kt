package dev.finance.wallet.capture

import android.app.Notification
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

/** Google Wallet. Verify on the real phone: the "Apps seen" list on the main screen shows the sender. */
const val WALLET_PACKAGE = "com.google.android.apps.walletnfcrel"

/** Packages whose notifications are stored. Debug builds add the adb shell, see `DebugSources.kt`. */
val capturedPackages: Set<String> = setOf(WALLET_PACKAGE) + extraSources

/**
 * Whether a notification is stored. Group summaries are skipped: they repeat the notifications they
 * group, which are captured on their own.
 */
fun shouldCapture(packageName: String, flags: Int, allowed: Set<String> = capturedPackages): Boolean =
    packageName in allowed && flags and Notification.FLAG_GROUP_SUMMARY == 0

/**
 * Listener state for the main screen, kept in memory only. Apps seen holds package names and counts,
 * never content, so a wrong Wallet package name can be spotted without storing other apps' notifications.
 */
object Diagnostics {
    private val _connected = MutableStateFlow(false)
    val connected: StateFlow<Boolean> = _connected.asStateFlow()

    private val _appsSeen = MutableStateFlow<Map<String, Int>>(emptyMap())
    val appsSeen: StateFlow<Map<String, Int>> = _appsSeen.asStateFlow()

    fun setConnected(value: Boolean) {
        _connected.value = value
    }

    fun seen(packageName: String) = _appsSeen.update { it + (packageName to (it[packageName] ?: 0) + 1) }
}
