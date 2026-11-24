package dev.finance.wallet.capture

/**
 * Debug builds also store notifications posted from adb, so capture can be tested on the emulator:
 *
 *     adb shell cmd notification post -t "Title" tag "Paid €12.50 at Carrefour"
 */
internal val extraSources: Set<String> = setOf("com.android.shell")
