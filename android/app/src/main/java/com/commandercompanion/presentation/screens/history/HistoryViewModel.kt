package com.commandercompanion.presentation.screens.history

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.data.repository.GameRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import javax.inject.Inject

/**
 * History of games played on this device.
 *
 * Goes through [GameRepository] instead of `GameDao` directly: the `ViewModel` no longer knows
 * whether the data comes from Room or the backend.
 *
 * TODO: still exposes `GameWithPlayers` (a Room entity) because `HistoryScreen` consumes it
 *  as-is. Mapping it to a domain model would mean touching the screen, out of scope here.
 *  The remote history (`GET /games`) isn't merged with the local one yet either.
 */
@HiltViewModel
class HistoryViewModel @Inject constructor(
    gameRepository: GameRepository
) : ViewModel() {

    val games: StateFlow<List<GameWithPlayers>> = gameRepository.observeHistory()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())
}
