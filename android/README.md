# Android app: Wallet notification forwarder

Kotlin + Jetpack Compose. Create the project in **Android Studio** in this folder
(*New Project → Empty Activity*, Kotlin DSL, package e.g. `dev.finance.wallet`, min SDK 29).
Android Studio generates the Gradle files with current versions, so they are not scaffolded by hand.

## Planned components (phase 2)

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
