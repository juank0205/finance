package dev.finance.wallet

import android.annotation.SuppressLint
import android.content.ActivityNotFoundException
import android.content.ComponentName
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.core.app.NotificationManagerCompat
import androidx.core.net.toUri
import dev.finance.wallet.capture.Diagnostics
import dev.finance.wallet.capture.WalletListenerService
import dev.finance.wallet.ui.MainScreen
import dev.finance.wallet.ui.theme.FinanceWalletTheme
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {
    // Both are settings the user changes outside the app, so they are read again on every resume.
    private var accessGranted by mutableStateOf(false)
    private var batteryExempt by mutableStateOf(false)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val app = application as WalletApp
        setContent {
            FinanceWalletTheme {
                val events by app.db.rawEvents().observeRecent().collectAsState(initial = emptyList())
                val connected by Diagnostics.connected.collectAsState()
                val appsSeen by Diagnostics.appsSeen.collectAsState()
                Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
                    MainScreen(
                        accessGranted = accessGranted,
                        listenerConnected = connected,
                        batteryExempt = batteryExempt,
                        events = events,
                        appsSeen = appsSeen,
                        onGrantAccess = ::openAccessSettings,
                        onBatteryExempt = ::requestBatteryExemption,
                        onClear = { app.scope.launch { app.db.rawEvents().deleteAll() } },
                        modifier = Modifier.padding(innerPadding),
                    )
                }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        accessGranted = packageName in NotificationManagerCompat.getEnabledListenerPackages(this)
        batteryExempt = getSystemService(PowerManager::class.java).isIgnoringBatteryOptimizations(packageName)
    }

    private fun openAccessSettings() {
        val component = ComponentName(this, WalletListenerService::class.java).flattenToString()
        val detail = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Intent(Settings.ACTION_NOTIFICATION_LISTENER_DETAIL_SETTINGS)
                .putExtra(Settings.EXTRA_NOTIFICATION_LISTENER_COMPONENT_NAME, component)
        } else {
            null
        }
        try {
            startActivity(detail ?: Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS))
        } catch (_: ActivityNotFoundException) {
            // Some phone makers drop the per-app screen; the list of listeners always exists.
            startActivity(Intent(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS))
        }
    }

    // The app is never published on Google Play, whose policy limits this direct request.
    @SuppressLint("BatteryLife")
    private fun requestBatteryExemption() {
        startActivity(Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, "package:$packageName".toUri()))
    }
}
