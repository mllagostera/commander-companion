package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Aggregated statistics (endpoints under `/statistics`).
 *
 * Always remote: the backend recalculates them when each game finishes, and it doesn't make
 * sense to duplicate that aggregation on the client. Returns zeros (not an error) for a
 * user/deck that hasn't finished any game yet.
 *
 * TODO: there's no screen consuming this yet — the statistics UI is Stage 7 of `TASKS.md`,
 * out of scope for this pass (which was about unblocking it by wiring up the API).
 */
@Singleton
class StatisticsRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun userStats(): Result<UserStatsDto> = apiCall { api.getUserStats() }

    suspend fun deckStats(deckId: String): Result<DeckStatsDto> = apiCall { api.getDeckStats(deckId) }

    suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStatsDto> =
        apiCall { api.getPlaygroupStats(playgroupId) }
}
