package com.commandercompanion.domain.repository

import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto

/**
 * Aggregated statistics (endpoints under `/statistics`).
 *
 * Always remote: the backend recalculates them when each game finishes, and it doesn't make
 * sense to duplicate that aggregation on the client. Returns zeros (not an error) for a
 * user/deck that hasn't finished any game yet.
 */
interface StatisticsRepository {

    suspend fun userStats(): Result<UserStatsDto>

    suspend fun deckStats(deckId: String): Result<DeckStatsDto>

    suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStatsDto>
}
