package com.vansid.tapeandocartones.domain.repository

/**
 * Remembers, on this device only, the deck each member last sat down with in each playgroup, so
 * the pregame seat picker can preselect it — repeating a deck is the common case.
 */
interface LastDeckRepository {

    suspend fun lastDeckId(playgroupId: String, userId: String): String?

    /** [deckIdsByUserId] maps each seated member to the deck they picked. Fire-and-forget. */
    fun remember(playgroupId: String, deckIdsByUserId: Map<String, String>)
}
