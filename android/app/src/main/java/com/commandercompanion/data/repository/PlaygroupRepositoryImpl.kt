package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.domain.repository.PlaygroupRepository
import javax.inject.Inject

/** [PlaygroupRepository] implementation. */
class PlaygroupRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : PlaygroupRepository {

    override suspend fun listPlaygroups(): Result<List<PlaygroupDto>> = apiCall { api.listPlaygroups() }

    override suspend fun getPlaygroup(playgroupId: String): Result<PlaygroupDto> =
        apiCall { api.getPlaygroup(playgroupId) }

    override suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<DeckDto>> =
        apiCall { api.getMemberDecks(playgroupId, userId) }
}
