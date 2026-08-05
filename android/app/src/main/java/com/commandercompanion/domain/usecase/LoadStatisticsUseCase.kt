package com.commandercompanion.domain.usecase

import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.model.StatisticsSnapshot
import com.commandercompanion.domain.repository.DeckRepository
import com.commandercompanion.domain.repository.StatisticsRepository
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import javax.inject.Inject

/**
 * Fetches everything `StatisticsScreen` needs in one shot (except the finished-games tab, which
 * paginates on its own -- see `FinishedGamesViewModel`): global stats, then decks (to know *what*
 * to show per-deck stats for) resolved concurrently with the playgroup/opponent breakdowns — same
 * scope as `web/app/pages/statistics.vue`.
 *
 * Returns null only if the global stats call itself fails; a per-deck failure just leaves that
 * entry without stats. `playgroupGameCounts`/`opponentStats` failing independently just leaves
 * those lists empty (`?: emptyList()`) rather than failing the whole screen — same
 * "don't let one breakdown's failure break the rest" reasoning as the per-deck case.
 */
class LoadStatisticsUseCase @Inject constructor(
    private val statisticsRepository: StatisticsRepository,
    private val deckRepository: DeckRepository
) {

    suspend operator fun invoke(): StatisticsSnapshot? {
        val userStats = statisticsRepository.userStats().getOrNull() ?: return null

        val decks = deckRepository.listDecks().getOrDefault(emptyList())

        return coroutineScope {
            val deckStats = async {
                decks.map { deck ->
                    async { DeckWithStats(deck, statisticsRepository.deckStats(deck.id).getOrNull()) }
                }.awaitAll()
            }
            val playgroupGameCounts = async { statisticsRepository.playgroupGameCounts().getOrDefault(emptyList()) }
            val opponentStats = async { statisticsRepository.opponentStats().getOrDefault(emptyList()) }

            StatisticsSnapshot(
                userStats = userStats,
                deckStats = deckStats.await(),
                playgroupGameCounts = playgroupGameCounts.await(),
                opponentStats = opponentStats.await()
            )
        }
    }
}
