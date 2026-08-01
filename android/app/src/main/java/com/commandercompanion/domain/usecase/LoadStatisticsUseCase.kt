package com.commandercompanion.domain.usecase

import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.model.PlaygroupSummary
import com.commandercompanion.domain.model.StatisticsSnapshot
import com.commandercompanion.domain.repository.DeckRepository
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.domain.repository.StatisticsRepository
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import javax.inject.Inject

/**
 * Fetches everything `StatisticsScreen` needs in one shot: global stats, then decks/playgroups
 * (to know *what* to show per-deck/per-group stats for), then both breakdowns resolved
 * concurrently — same scope as `web/app/pages/statistics.vue`.
 *
 * Returns null only if the global stats call itself fails; a per-deck/per-group failure just
 * leaves that entry without stats (`GetPlaygroupStats`'s 404 for a group with no finished games
 * is treated the same way, as zero games, not an error — see `internal/statistics/service.go`).
 */
class LoadStatisticsUseCase @Inject constructor(
    private val statisticsRepository: StatisticsRepository,
    private val deckRepository: DeckRepository,
    private val playgroupRepository: PlaygroupRepository
) {

    suspend operator fun invoke(): StatisticsSnapshot? {
        val userStats = statisticsRepository.userStats().getOrNull() ?: return null

        val decks = deckRepository.listDecks().getOrDefault(emptyList())
        val playgroups = playgroupRepository.listPlaygroups().getOrDefault(emptyList())

        return coroutineScope {
            val deckStats = decks.map { deck ->
                async { DeckWithStats(deck, statisticsRepository.deckStats(deck.id).getOrNull()) }
            }.awaitAll()
            val playgroupSummaries = playgroups.map { playgroup ->
                async {
                    val gamesPlayed = statisticsRepository.playgroupStats(playgroup.id)
                        .getOrNull()?.gamesPlayed ?: 0
                    PlaygroupSummary(playgroup, gamesPlayed)
                }
            }.awaitAll()
            StatisticsSnapshot(userStats, deckStats, playgroupSummaries)
        }
    }
}
