package com.commandercompanion.presentation.screens.statistics

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.data.repository.DeckRepository
import com.commandercompanion.data.repository.PlaygroupRepository
import com.commandercompanion.data.repository.StatisticsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class DeckWithStats(val deck: DeckDto, val stats: DeckStatsDto?)

data class PlaygroupSummary(val playgroup: PlaygroupDto, val gamesPlayed: Int)

data class StatisticsUiState(
    val isLoading: Boolean = true,
    val loadError: Boolean = false,
    val userStats: UserStatsDto? = null,
    val deckStats: List<DeckWithStats> = emptyList(),
    val playgroupSummaries: List<PlaygroupSummary> = emptyList()
)

/**
 * Global/per-deck/per-group statistics — same scope and endpoints as `web/app/pages/statistics.vue`.
 * Decks and playgroups are fetched to know *what* to show per-deck/per-group stats for; a group
 * with no finished games returns 404 from `GetPlaygroupStats`, which is treated as zero games,
 * not an error (same criterion the web client already applies, see `internal/statistics/service.go`).
 */
@HiltViewModel
class StatisticsViewModel @Inject constructor(
    private val statisticsRepository: StatisticsRepository,
    private val deckRepository: DeckRepository,
    private val playgroupRepository: PlaygroupRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(StatisticsUiState())
    val uiState: StateFlow<StatisticsUiState> = _uiState.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, loadError = false) }

            val userStats = statisticsRepository.userStats().getOrNull()
            if (userStats == null) {
                _uiState.update { it.copy(isLoading = false, loadError = true) }
                return@launch
            }

            val decks = deckRepository.listDecks().getOrDefault(emptyList())
            val playgroups = playgroupRepository.listPlaygroups().getOrDefault(emptyList())

            val deckStats = coroutineScope {
                decks.map { deck ->
                    async { DeckWithStats(deck, statisticsRepository.deckStats(deck.id).getOrNull()) }
                }.awaitAll()
            }
            val playgroupSummaries = coroutineScope {
                playgroups.map { playgroup ->
                    async {
                        val gamesPlayed = statisticsRepository.playgroupStats(playgroup.id)
                            .getOrNull()?.gamesPlayed ?: 0
                        PlaygroupSummary(playgroup, gamesPlayed)
                    }
                }.awaitAll()
            }

            _uiState.update {
                it.copy(
                    isLoading = false,
                    userStats = userStats,
                    deckStats = deckStats,
                    playgroupSummaries = playgroupSummaries
                )
            }
        }
    }
}
