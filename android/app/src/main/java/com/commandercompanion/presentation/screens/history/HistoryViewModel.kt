package com.commandercompanion.presentation.screens.history

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameWithPlayers
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import javax.inject.Inject

@HiltViewModel
class HistoryViewModel @Inject constructor(
    gameDao: GameDao
) : ViewModel() {

    val games: StateFlow<List<GameWithPlayers>> = gameDao.getGamesWithPlayers()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())
}
