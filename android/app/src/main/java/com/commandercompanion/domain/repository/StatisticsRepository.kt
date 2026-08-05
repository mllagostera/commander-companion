package com.commandercompanion.domain.repository

import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.FinishedGameDto
import com.commandercompanion.data.remote.dto.OpponentStatsDto
import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.data.remote.dto.PlaygroupGameCountDto
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

    /** Every playgroup the user belongs to, with its games_played count -- replaces one [playgroupStats] call per group. */
    suspend fun playgroupGameCounts(): Result<List<PlaygroupGameCountDto>>

    /** Head-to-head record against every opponent the user has shared a finished game with. */
    suspend fun opponentStats(): Result<List<OpponentStatsDto>>

    /**
     * One page of the finished-games history, most recent first. Unlike [GameRepository.listGames]
     * (which follows every page), the caller drives pagination itself (a "load more" tab, not a
     * bounded list) -- pass the previous page's cursor to get the next one.
     */
    suspend fun listFinishedGames(cursor: String? = null): Result<PagedResponse<FinishedGameDto>>
}
