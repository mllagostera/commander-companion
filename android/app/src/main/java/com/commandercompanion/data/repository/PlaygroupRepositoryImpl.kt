package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.domain.model.Deck
import com.commandercompanion.domain.model.Playgroup
import com.commandercompanion.domain.repository.PlaygroupRepository
import javax.inject.Inject

/** [PlaygroupRepository] implementation. */
class PlaygroupRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : PlaygroupRepository {

    override suspend fun listPlaygroups(): Result<List<Playgroup>> = apiCall { api.listPlaygroups() }

    override suspend fun getPlaygroup(playgroupId: String): Result<Playgroup> =
        apiCall { api.getPlaygroup(playgroupId) }

    override suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<Deck>> =
        apiCall { api.getMemberDecks(playgroupId, userId) }
}
