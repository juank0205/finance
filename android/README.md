# Android app: Wallet notification forwarder

Kotlin + Jetpack Compose, package `dev.finance.wallet`, min SDK 29. Open this folder in Android Studio.

## Status: step a, capture only

`WalletListenerService` stores Google Wallet notifications, as posted, in a local Room database
(`raw_events`), and the main screen lists them. Nothing leaves the phone yet: the app has no `INTERNET`
permission (the manifest strips it even if a library adds one), and backups and device transfer are off.

- **Setup screen:** grant notification access. Turning off battery optimization is optional: Android
  keeps a notification listener running on its own, so do it only if payments go missing. On Samsung,
  the setting that matters is Battery → Background usage limits → *Never auto sleeping apps*.
- **Apps seen:** package names (never content) of every app that posted since the listener started.
  Use it on the real phone to confirm Wallet's package is `com.google.android.apps.walletnfcrel`.
- **Duplicates:** the same notification seen twice is stored once (`fingerprint`). A re-post with a new
  time is a new row; the Pi's `dedup_key` decides whether it is the same payment.

### Testing on the emulator

Debug builds also capture notifications posted from adb (release builds keep Wallet only):

```bash
adb shell cmd notification allow_listener dev.finance.wallet/dev.finance.wallet.capture.WalletListenerService
adb shell "cmd notification post -t 'Carrefour' test1 'EUR 12.50 with Visa 0392'"
```

Real Wallet payments need the real phone (the emulator has no NFC). On Android 13+, a sideloaded app may
show notification access as a "restricted setting": App info → ⋮ → *Allow restricted settings*.

### Checks

```bash
./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleRelease
```

## Planned components (phase 2, step b onward)

| Component | Job |
|---|---|
| `WalletListenerService` (`NotificationListenerService`) | Keep only notifications from `com.google.android.apps.walletnfcrel` (verify on your phone); capture title, text, post time |
| Room DB + `WorkManager` | Queue events locally; retry upload with backoff when the Pi is unreachable |
| `IngestClient` (OkHttp) | POST to `https://<pi>.ts.net:8443/v1/events` with a **client certificate** (mTLS) and an HMAC-SHA256 signature header |
| Android Keystore | Holds the client cert's private key and the HMAC key; non-exportable |
| Compose UI | Permission onboarding, queue status, last successful sync, "send test event" |
| Heartbeat | Periodic ping so the Pi can alert when the phone goes silent |

## Rules

- Forward raw notification text only; parsing happens on the Pi so parser fixes never need an app update.
- No analytics or crash SDKs; the app sees financial data.
- Exclude the app from battery optimization or Android will kill the listener.
- Disable backups of app data (`android:allowBackup="false"`), so the keys and queue never leave the phone.
