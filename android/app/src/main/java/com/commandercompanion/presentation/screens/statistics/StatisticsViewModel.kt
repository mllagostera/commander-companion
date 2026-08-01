package com.commandercompanion.presentation.screens.statistics

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.model.PlaygroupSummary
import com.commandercompanion.domain.usecase.LoadStatisticsUseCase
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class StatisticsUiState(
    val isLoading: Boolean = true,
    val loadError: Boolean = false,
    val userStats: UserStatsDto? = null,
    val deckStats: List<DeckWithStats> = emptyList(),
    val playgroupSummaries: List<PlaygroupSummary> = emptyList()
)

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
                    playgroupSummaries = snapshot.playgroupSummaries
                )
            }
        }
    }
}
