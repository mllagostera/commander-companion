package com.vansid.tapeandocartones.data.repository

import com.vansid.tapeandocartones.core.util.apiCall
import com.vansid.tapeandocartones.data.remote.api.CommanderApi
import com.vansid.tapeandocartones.domain.model.Deck
import com.vansid.tapeandocartones.domain.model.Playgroup
import com.vansid.tapeandocartones.domain.repository.PlaygroupRepository
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
