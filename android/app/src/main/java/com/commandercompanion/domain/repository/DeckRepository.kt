package com.commandercompanion.domain.repository

import com.commandercompanion.domain.model.Deck

/**
 * Access to the authenticated user's decks. See `DeckRepositoryImpl`: [listDecks] is
 * network-first with a Room cache as a fallback.
 */
interface DeckRepository {

    suspend fun listDecks(): Result<List<Deck>>

    suspend fun getDeck(deckId: String): Result<Deck>

    suspend fun createDeck(name: String, commander: String, moxfieldId: String? = null): Result<Deck>

    /** [urlOrPublicId] accepts either the full Moxfield URL or just the public ID. */
    suspend fun importFromMoxfield(urlOrPublicId: String): Result<Deck>

    suspend fun deleteDeck(deckId: String): Result<Unit>
}
