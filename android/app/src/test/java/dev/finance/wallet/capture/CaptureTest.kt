package dev.finance.wallet.capture

import android.app.Notification
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class CaptureTest {
    private val allowed = setOf(WALLET_PACKAGE)

    @Test
    fun storesWalletNotifications() {
        assertTrue(shouldCapture(WALLET_PACKAGE, 0, allowed))
    }

    @Test
    fun dropsOtherApps() {
        assertFalse(shouldCapture("com.whatsapp", 0, allowed))
        assertFalse(shouldCapture("com.android.shell", 0, allowed))
    }

    @Test
    fun dropsGroupSummaries() {
        assertFalse(shouldCapture(WALLET_PACKAGE, Notification.FLAG_GROUP_SUMMARY, allowed))
    }

    @Test
    fun fingerprintIsStable() {
        assertEquals(fingerprint("a", "b"), fingerprint("a", "b"))
        assertEquals(64, fingerprint("a").length)
    }

    @Test
    fun fingerprintSeparatesParts() {
        assertNotEquals(fingerprint("ab", "c"), fingerprint("a", "bc"))
        assertNotEquals(fingerprint("", "x"), fingerprint("x", ""))
    }

    @Test
    fun captureTimeDoesNotChangeFingerprint() {
        fun event(capturedAt: Long) = RawEvent.of(WALLET_PACKAGE, "k", "c", 1, "t", "x", "", "", capturedAt)
        assertEquals(event(1).fingerprint, event(2).fingerprint)
    }
}
