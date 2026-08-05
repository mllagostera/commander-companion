package com.commandercompanion.presentation.screens.statistics

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.FinishedGameDto
import com.commandercompanion.domain.repository.StatisticsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import javax.inject.Inject

data class FinishedGamesUiState(
    val isLoading: Boolean = true,
    val isLoadingMore: Boolean = false,
    val loadError: Boolean = false,
    val games: List<FinishedGameDto> = emptyList(),
    val nextCursor: String? = null
) {
    val hasMore: Boolean get() = nextCursor != null
}

/**
 * Drives the "Finished games" tab of `StatisticsScreen`. Separate from [StatisticsViewModel]
 * (and its [com.commandercompanion.domain.usecase.LoadStatisticsUseCase]) because this list
 * paginates on its own -- a "load more" tab, not a bounded snapshot like the deck/opponent
 * breakdowns.
 */
@HiltViewModel
class FinishedGamesViewModel @Inject constructor(
    private val statisticsRepository: StatisticsRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(FinishedGamesUiState())
    val uiState: StateFlow<FinishedGamesUiState> = _uiState.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _uiState.update { FinishedGamesUiState(isLoading = true) }
            val page = statisticsRepository.listFinishedGames()
            if (page.isFailure) {
                _uiState.update { it.copy(isLoading = false, loadError = true) }
                return@launch
            }
            page.getOrNull()?.let { res ->
                _uiState.update {
                    it.copy(isLoading = false, games = res.items, nextCursor = res.nextCursor)
                }
            }
        }
    }

    fun loadMore() {
        val cursor = _uiState.value.nextCursor ?: return
        if (_uiState.value.isLoadingMore) return

        viewModelScope.launch {
            _uiState.update { it.copy(isLoadingMore = true) }
            val page = statisticsRepository.listFinishedGames(cursor)
            _uiState.update { state ->
                page.getOrNull()?.let { res ->
                    state.copy(isLoadingMore = false, games = state.games + res.items, nextCursor = res.nextCursor)
                } ?: state.copy(isLoadingMore = false)
            }
        }
    }
}
