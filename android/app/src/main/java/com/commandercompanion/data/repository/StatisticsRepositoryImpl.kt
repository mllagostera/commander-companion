package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.domain.repository.StatisticsRepository
import javax.inject.Inject

/** [StatisticsRepository] implementation. Consumed by `StatisticsScreen` via `LoadStatisticsUseCase`. */
class StatisticsRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : StatisticsRepository {

    override suspend fun userStats(): Result<UserStatsDto> = apiCall { api.getUserStats() }

    override suspend fun deckStats(deckId: String): Result<DeckStatsDto> = apiCall { api.getDeckStats(deckId) }

    override suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStatsDto> =
        apiCall { api.getPlaygroupStats(playgroupId) }
}
