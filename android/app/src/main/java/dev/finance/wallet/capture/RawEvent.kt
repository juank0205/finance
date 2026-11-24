package dev.finance.wallet.capture

import androidx.room.ColumnInfo
import androidx.room.Dao
import androidx.room.Database
import androidx.room.Entity
import androidx.room.Index
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.PrimaryKey
import androidx.room.Query
import androidx.room.RoomDatabase
import kotlinx.coroutines.flow.Flow
import java.security.MessageDigest

/**
 * A notification exactly as posted. Parsing happens on the Pi, so a parser fix never needs an app
 * update and the original text can always be parsed again.
 */
@Entity(tableName = "raw_events", indices = [Index(value = ["fingerprint"], unique = true)])
data class RawEvent(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    /** Hash of everything below except [capturedAt]; the same notification seen twice is stored once. */
    val fingerprint: String,
    @ColumnInfo(name = "package_name") val packageName: String,
    @ColumnInfo(name = "notification_key") val notificationKey: String,
    @ColumnInfo(name = "channel_id") val channelId: String,
    /** When the app posted it, epoch milliseconds. */
    @ColumnInfo(name = "posted_at") val postedAt: Long,
    val title: String,
    val text: String,
    @ColumnInfo(name = "big_text") val bigText: String,
    @ColumnInfo(name = "sub_text") val subText: String,
    /** When this app stored it, epoch milliseconds. */
    @ColumnInfo(name = "captured_at") val capturedAt: Long,
) {
    companion object {
        fun of(
            packageName: String,
            notificationKey: String,
            channelId: String,
            postedAt: Long,
            title: String,
            text: String,
            bigText: String,
            subText: String,
            capturedAt: Long,
        ) = RawEvent(
            fingerprint = fingerprint(
                packageName, notificationKey, channelId, postedAt.toString(), title, text, bigText, subText,
            ),
            packageName = packageName,
            notificationKey = notificationKey,
            channelId = channelId,
            postedAt = postedAt,
            title = title,
            text = text,
            bigText = bigText,
            subText = subText,
            capturedAt = capturedAt,
        )
    }
}

/** SHA-256 of the parts, each prefixed by its length so different splits never hash the same. */
internal fun fingerprint(vararg parts: String): String {
    val digest = MessageDigest.getInstance("SHA-256")
    for (part in parts) {
        val bytes = part.toByteArray(Charsets.UTF_8)
        digest.update("${bytes.size}:".toByteArray(Charsets.UTF_8))
        digest.update(bytes)
    }
    return digest.digest().joinToString("") { "%02x".format(it) }
}

@Dao
interface RawEventDao {
    /** Returns -1 when the notification was already stored. */
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insert(event: RawEvent): Long

    @Query("SELECT * FROM raw_events ORDER BY posted_at DESC LIMIT 200")
    fun observeRecent(): Flow<List<RawEvent>>

    @Query("DELETE FROM raw_events")
    suspend fun deleteAll()
}

@Database(entities = [RawEvent::class], version = 1)
abstract class WalletDb : RoomDatabase() {
    abstract fun rawEvents(): RawEventDao
}
