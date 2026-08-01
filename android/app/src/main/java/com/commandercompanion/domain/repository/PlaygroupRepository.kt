package com.commandercompanion.domain.repository

import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto

/**
 * Access to the authenticated user's play groups (`playgroups`). Used by `PlayerSetupScreen`'s
 * Group mode: choosing a group, assigning its members to seats, and viewing a
 * teammate's decks for a proxy-join (see the backend's ADR-0013).
 */
interface PlaygroupRepository {

    suspend fun listPlaygroups(): Result<List<PlaygroupDto>>

    suspend fun getPlaygroup(playgroupId: String): Result<PlaygroupDto>

    suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<DeckDto>>
}
