package com.vansid.tapeandocartones.domain.repository

import com.vansid.tapeandocartones.domain.model.Deck
import com.vansid.tapeandocartones.domain.model.Playgroup

/**
 * Access to the authenticated user's play groups (`playgroups`). Used by `PlayerSetupScreen`'s
 * Group mode to choose a group, and by `PreGameScreen` to assign its members to seats and view a
 * teammate's decks for a proxy-join (see the backend's ADR-0013).
 */
interface PlaygroupRepository {

    suspend fun listPlaygroups(): Result<List<Playgroup>>

    suspend fun getPlaygroup(playgroupId: String): Result<Playgroup>

    suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<Deck>>
}
