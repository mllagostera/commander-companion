package com.commandercompanion.domain.model

import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.remote.dto.UserStatsDto

/** A deck alongside its stats — null [stats] means it hasn't finished a game yet, not an error. */
data class DeckWithStats(val deck: DeckDto, val stats: DeckStatsDto?)

/** A playgroup alongside how many finished games it has. */
data class PlaygroupSummary(val playgroup: PlaygroupDto, val gamesPlayed: Int)

/** Result of [com.commandercompanion.domain.usecase.LoadStatisticsUseCase]. */
data class StatisticsSnapshot(
    val userStats: UserStatsDto,
    val deckStats: List<DeckWithStats>,
    val playgroupSummaries: List<PlaygroupSummary>
)
