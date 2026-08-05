package com.commandercompanion.domain.model

import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.OpponentStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupGameCountDto
import com.commandercompanion.data.remote.dto.UserStatsDto

/** A deck alongside its stats — null [stats] means it hasn't finished a game yet, not an error. */
data class DeckWithStats(val deck: DeckDto, val stats: DeckStatsDto?)

/** Result of [com.commandercompanion.domain.usecase.LoadStatisticsUseCase]. */
data class StatisticsSnapshot(
    val userStats: UserStatsDto,
    val deckStats: List<DeckWithStats>,
    val playgroupGameCounts: List<PlaygroupGameCountDto>,
    val opponentStats: List<OpponentStatsDto>
)
