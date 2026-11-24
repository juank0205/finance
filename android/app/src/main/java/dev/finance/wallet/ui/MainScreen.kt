package dev.finance.wallet.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import dev.finance.wallet.capture.RawEvent
import dev.finance.wallet.capture.WALLET_PACKAGE
import dev.finance.wallet.ui.theme.FinanceWalletTheme
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

private val timeFormat = DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM).withZone(ZoneId.systemDefault())

@Composable
fun MainScreen(
    accessGranted: Boolean,
    listenerConnected: Boolean,
    batteryExempt: Boolean,
    events: List<RawEvent>,
    appsSeen: Map<String, Int>,
    onGrantAccess: () -> Unit,
    onBatteryExempt: () -> Unit,
    onClear: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var confirmClear by remember { mutableStateOf(false) }

    LazyColumn(
        modifier = modifier,
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            Card(Modifier.fillMaxWidth()) {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text("Setup", style = MaterialTheme.typography.titleMedium)
                    Check("Notification access", accessGranted, "Grant", onGrantAccess)
                    Check("Listener running", listenerConnected, null, null)
                    Check("Battery optimization off (optional)", batteryExempt, "Turn off", onBatteryExempt, optional = true)
                    Text(
                        "Only needed if payments go missing. On Samsung, also add this app to Settings → " +
                            "Battery → Background usage limits → Never auto sleeping apps.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        item {
            Card(Modifier.fillMaxWidth()) {
                Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    Text("Apps seen since the listener started", style = MaterialTheme.typography.titleMedium)
                    Text(
                        "Names only, nothing else is kept. Stored: $WALLET_PACKAGE.",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    if (appsSeen.isEmpty()) Text("None yet")
                    appsSeen.entries.sortedByDescending { it.value }.forEach { (pkg, count) ->
                        Text("$pkg  ×$count", style = MaterialTheme.typography.bodyMedium)
                    }
                }
            }
        }

        item {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Text("Captured (${events.size})", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.weight(1f))
                if (events.isNotEmpty()) TextButton(onClick = { confirmClear = true }) { Text("Clear") }
            }
        }

        if (events.isEmpty()) {
            item { Text("Nothing captured yet. Pay with Google Wallet to see the notification here.") }
        }
        items(events, key = { it.id }) { EventCard(it) }
    }

    if (confirmClear) {
        AlertDialog(
            onDismissRequest = { confirmClear = false },
            title = { Text("Delete captured notifications?") },
            text = { Text("They are only on this phone, so they cannot be recovered.") },
            confirmButton = {
                TextButton(onClick = { confirmClear = false; onClear() }) { Text("Delete") }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text("Cancel") } },
        )
    }
}

@Composable
private fun Check(label: String, ok: Boolean, action: String?, onAction: (() -> Unit)?, optional: Boolean = false) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        val colors = MaterialTheme.colorScheme
        when {
            ok -> Text("✓", color = colors.primary)
            optional -> Text("–", color = colors.onSurfaceVariant)
            else -> Text("✗", color = colors.error)
        }
        Spacer(Modifier.width(8.dp))
        Text(label, Modifier.weight(1f))
        if (!ok && action != null && onAction != null) OutlinedButton(onClick = onAction) { Text(action) }
    }
}

@Composable
private fun EventCard(event: RawEvent) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(timeFormat.format(Instant.ofEpochMilli(event.postedAt)), style = MaterialTheme.typography.labelMedium)
            if (event.title.isNotEmpty()) Text(event.title, style = MaterialTheme.typography.titleSmall)
            if (event.text.isNotEmpty()) Text(event.text)
            if (event.bigText.isNotEmpty() && event.bigText != event.text) Text(event.bigText)
            if (event.subText.isNotEmpty()) Text(event.subText, style = MaterialTheme.typography.bodySmall)
            Text(
                "${event.packageName} · channel ${event.channelId.ifEmpty { "-" }}",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Preview(showBackground = true)
@Composable
private fun MainScreenPreview() {
    FinanceWalletTheme {
        MainScreen(
            accessGranted = true,
            listenerConnected = true,
            batteryExempt = false,
            events = listOf(
                RawEvent.of(
                    WALLET_PACKAGE, "0|key", "payments", 1_764_000_000_000,
                    "Carrefour", "€12.50 with Visa ••0392", "", "", 1_764_000_000_000,
                ),
            ),
            appsSeen = mapOf(WALLET_PACKAGE to 1, "com.whatsapp" to 7),
            onGrantAccess = {},
            onBatteryExempt = {},
            onClear = {},
        )
    }
}
