package dev.finance.wallet

import android.app.Application
import androidx.room.Room
import dev.finance.wallet.capture.WalletDb
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class WalletApp : Application() {
    /** Outlives the listener service, so a write started just before it stops still finishes. */
    val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    val db: WalletDb by lazy {
        Room.databaseBuilder(this, WalletDb::class.java, "wallet.db").build()
    }
}
