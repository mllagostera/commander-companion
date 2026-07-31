package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Access to the authenticated user's decks.
 *
 * Today it's 100% remote: decks live only in the backend and there's no Room entity for them.
 * It's still modeled as a repository so `ViewModel`s never touch [CommanderApi] directly, and
 * to have a single place to add offline caching later (Stage 5 of `TASKS.md`,
 * "Room as offline-first cache ... own decks").
 */
@Singleton
class DeckRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun listDecks(): Result<List<DeckDto>> = apiCall { api.listDecks().items }

    suspend fun getDeck(deckId: String): Result<DeckDto> = apiCall { api.getDeck(deckId) }

    suspend fun createDeck(
        name: String,
        commander: String,
        moxfieldId: String? = null
    ): Result<DeckDto> = apiCall {
        api.createDeck(CreateDeckRequest(name = name, commander = commander, moxfieldId = moxfieldId))
    }

    /** [urlOrPublicId] accepts either the full Moxfield URL or just the public ID. */
    suspend fun importFromMoxfield(urlOrPublicId: String): Result<DeckDto> = apiCall {
        api.importMoxfieldDeck(ImportMoxfieldRequest(urlOrPublicId))
    }

    suspend fun deleteDeck(deckId: String): Result<Unit> = apiCall { api.deleteDeck(deckId) }
}
