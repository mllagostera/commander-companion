package com.commandercompanion.presentation.screens.statistics

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.OpponentStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupGameCountDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.usecase.LoadStatisticsUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject
import kotlin.math.roundToInt

/** How [StatisticsUiState.deckStats] should be ordered in the "By deck" tab. */
enum class DeckSortOrder { RECENT, WIN_RATE, GAMES_PLAYED }

data class StatisticsUiState(
    val isLoading: Boolean = true,
    val loadError: Boolean = false,
    val userStats: UserStatsDto? = null,
    val deckStats: List<DeckWithStats> = emptyList(),
    val deckSortOrder: DeckSortOrder = DeckSortOrder.RECENT,
    val playgroupGameCounts: List<PlaygroupGameCountDto> = emptyList(),
    val opponentStats: List<OpponentStatsDto> = emptyList()
) {
    /** [deckStats] ordered by [deckSortOrder] -- the fetch order (backend's `created_at DESC`) is preserved for RECENT. */
    val sortedDeckStats: List<DeckWithStats>
        get() = when (deckSortOrder) {
            DeckSortOrder.RECENT -> deckStats
            DeckSortOrder.WIN_RATE -> deckStats.sortedWith(
                compareByDescending<DeckWithStats> { winRatePercent(it.stats?.gamesPlayed ?: 0, it.stats?.gamesWon ?: 0) }
                    .thenByDescending { it.stats?.gamesPlayed ?: 0 }
            )
            DeckSortOrder.GAMES_PLAYED -> deckStats.sortedByDescending { it.stats?.gamesPlayed ?: 0 }
        }

    /** The playgroup the user has played the most finished games in, if any. */
    val mostPlayedPlaygroup: PlaygroupGameCountDto?
        get() = playgroupGameCounts.filter { it.gamesPlayed > 0 }.maxByOrNull { it.gamesPlayed }

    /** The opponent shared the most finished games with, if any. */
    val mostPlayedOpponent: OpponentStatsDto?
        get() = opponentStats.maxByOrNull { it.gamesTogether }

    /** The opponent who has eliminated this user the most, if any (0 eliminations doesn't count as an archenemy). */
    val archenemy: OpponentStatsDto?
        get() = opponentStats.filter { it.timesEliminatedByOpponent > 0 }.maxByOrNull { it.timesEliminatedByOpponent }
}

/**
 * Global/per-deck/per-group statistics — same scope and endpoints as `web/app/pages/statistics.vue`.
 * The actual fetching/aggregation lives in [LoadStatisticsUseCase]; this `ViewModel` only turns its
 * result into [StatisticsUiState].
 */
@HiltViewModel
class StatisticsViewModel @Inject constructor(
    private val loadStatisticsUseCase: LoadStatisticsUseCase
) : ViewModel() {

    private val _uiState = MutableStateFlow(StatisticsUiState())
    val uiState: StateFlow<StatisticsUiState> = _uiState.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, loadError = false) }

            val snapshot = loadStatisticsUseCase()
            if (snapshot == null) {
                _uiState.update { it.copy(isLoading = false, loadError = true) }
                return@launch
            }

            _uiState.update {
                it.copy(
                    isLoading = false,
                    userStats = snapshot.userStats,
                    deckStats = snapshot.deckStats,
                    playgroupGameCounts = snapshot.playgroupGameCounts,
                    opponentStats = snapshot.opponentStats
                )
            }
        }
    }

    fun setDeckSortOrder(order: DeckSortOrder) {
        _uiState.update { it.copy(deckSortOrder = order) }
    }
}

/** Shared by [StatisticsUiState.sortedDeckStats] and `StatisticsScreen`'s deck/global cards. */
fun winRatePercent(played: Int, won: Int): Int =
    if (played == 0) 0 else ((won.toDouble() / played) * 100).roundToInt()
