package com.commandercompanion.domain.repository

import com.commandercompanion.domain.model.DeckStats
import com.commandercompanion.domain.model.FinishedGame
import com.commandercompanion.domain.model.OpponentStats
import com.commandercompanion.domain.model.Page
import com.commandercompanion.domain.model.PlaygroupGameCount
import com.commandercompanion.domain.model.PlaygroupStats
import com.commandercompanion.domain.model.UserStats

/**
 * Aggregated statistics (endpoints under `/statistics`).
 *
 * Always remote: the backend recalculates them when each game finishes, and it doesn't make
 * sense to duplicate that aggregation on the client. Returns zeros (not an error) for a
 * user/deck that hasn't finished any game yet.
 */
interface StatisticsRepository {

    suspend fun userStats(): Result<UserStats>

    suspend fun deckStats(deckId: String): Result<DeckStats>

    suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStats>

    /** Every playgroup the user belongs to, with its games_played count -- replaces one [playgroupStats] call per group. */
    suspend fun playgroupGameCounts(): Result<List<PlaygroupGameCount>>

    /** Head-to-head record against every opponent the user has shared a finished game with. */
    suspend fun opponentStats(): Result<List<OpponentStats>>

    /**
     * One page of the finished-games history, most recent first. Unlike [GameRepository.listGames]
     * (which follows every page), the caller drives pagination itself (a "load more" tab, not a
     * bounded list) -- pass the previous page's cursor to get the next one.
     */
    suspend fun listFinishedGames(cursor: String? = null): Result<Page<FinishedGame>>
}
