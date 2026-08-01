package com.commandercompanion.domain.repository

import com.commandercompanion.data.remote.dto.DeckDto

/**
 * Access to the authenticated user's decks. See `DeckRepositoryImpl`: [listDecks] is
 * network-first with a Room cache as a fallback.
 */
interface DeckRepository {

    suspend fun listDecks(): Result<List<DeckDto>>

    suspend fun getDeck(deckId: String): Result<DeckDto>

    suspend fun createDeck(name: String, commander: String, moxfieldId: String? = null): Result<DeckDto>

    /** [urlOrPublicId] accepts either the full Moxfield URL or just the public ID. */
    suspend fun importFromMoxfield(urlOrPublicId: String): Result<DeckDto>

    suspend fun deleteDeck(deckId: String): Result<Unit>
}
