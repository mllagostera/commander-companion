package com.vansid.tapeandocartones.data.repository

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.vansid.tapeandocartones.domain.repository.LastDeckRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

private val Context.lastDeckDataStore by preferencesDataStore(name = "last_decks")

/**
 * [LastDeckRepository] on a device-local DataStore: never synced, by design.
 *
 * Writes run on this singleton's own scope rather than the caller's: they happen right as the
 * pregame screen leaves the back stack, which would cancel its ViewModel's scope mid-write.
 */
@Singleton
class LastDeckRepositoryImpl @Inject constructor(
    @ApplicationContext private val context: Context
) : LastDeckRepository {

    private val writeScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    override suspend fun lastDeckId(playgroupId: String, userId: String): String? =
        context.lastDeckDataStore.data.first()[key(playgroupId, userId)]

    override fun remember(playgroupId: String, deckIdsByUserId: Map<String, String>) {
        if (deckIdsByUserId.isEmpty()) return
        writeScope.launch {
            context.lastDeckDataStore.edit { prefs ->
                deckIdsByUserId.forEach { (userId, deckId) -> prefs[key(playgroupId, userId)] = deckId }
            }
        }
    }

    private fun key(playgroupId: String, userId: String) = stringPreferencesKey("$playgroupId/$userId")
}
