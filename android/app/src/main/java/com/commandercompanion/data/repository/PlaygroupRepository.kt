package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Access to the authenticated user's play groups (`playgroups`). Used by `PlayerSetupScreen`'s
 * Group mode: choosing a group, assigning its members to seats, and viewing a
 * teammate's decks for a proxy-join (see the backend's ADR-0013).
 */
@Singleton
class PlaygroupRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun listPlaygroups(): Result<List<PlaygroupDto>> = apiCall { api.listPlaygroups() }

    suspend fun getPlaygroup(playgroupId: String): Result<PlaygroupDto> =
        apiCall { api.getPlaygroup(playgroupId) }

    suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<DeckDto>> =
        apiCall { api.getMemberDecks(playgroupId, userId) }
}
