package com.commandercompanion.presentation.screens.history

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.domain.model.PlayedGame
import com.commandercompanion.domain.repository.GameRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn

/**
 * History of games played on this device.
 *
 * Goes through [GameRepository] instead of `GameDao` directly: the `ViewModel` no longer knows
 * whether the data comes from Room or the backend.
 *
 * Exposes [PlayedGame], a domain model: the repository maps Room's `GameWithPlayers` at the
 * data boundary, so neither this `ViewModel` nor the screen knows Room exists.
 *
 * TODO: the remote history (`GET /games`) isn't merged with the local one yet.
 */
@HiltViewModel
class HistoryViewModel @Inject constructor(
    gameRepository: GameRepository
) : ViewModel() {

    val games: StateFlow<List<PlayedGame>> = gameRepository.observeHistory()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())
}
