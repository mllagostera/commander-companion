package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.domain.model.DeckStats
import com.commandercompanion.domain.model.FinishedGame
import com.commandercompanion.domain.model.OpponentStats
import com.commandercompanion.domain.model.Page
import com.commandercompanion.domain.model.PlaygroupGameCount
import com.commandercompanion.domain.model.PlaygroupStats
import com.commandercompanion.domain.model.UserStats
import com.commandercompanion.domain.repository.StatisticsRepository
import javax.inject.Inject

/** [StatisticsRepository] implementation. Consumed by `StatisticsScreen` via `LoadStatisticsUseCase`. */
class StatisticsRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : StatisticsRepository {

    override suspend fun userStats(): Result<UserStats> = apiCall { api.getUserStats() }

    override suspend fun deckStats(deckId: String): Result<DeckStats> = apiCall { api.getDeckStats(deckId) }

    override suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStats> =
        apiCall { api.getPlaygroupStats(playgroupId) }

    override suspend fun playgroupGameCounts(): Result<List<PlaygroupGameCount>> =
        apiCall { api.listPlaygroupGameCounts() }

    override suspend fun opponentStats(): Result<List<OpponentStats>> = apiCall { api.getOpponentStats() }

    override suspend fun listFinishedGames(cursor: String?): Result<Page<FinishedGame>> =
        apiCall { api.listFinishedGames(cursor) }
}
