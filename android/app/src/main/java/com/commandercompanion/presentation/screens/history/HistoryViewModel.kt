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
 * Historial de partidas jugadas en este dispositivo.
 *
 * Va contra [GameRepository] en vez de contra `GameDao` directo: el `ViewModel` ya no sabe si el
 * dato sale de Room o del backend.
 *
 * TODO: sigue exponiendo `GameWithPlayers` (entidad de Room) porque `HistoryScreen` la consume
 *  tal cual. Mapearla a un modelo de dominio implicaría tocar la pantalla, fuera de alcance acá.
 *  El historial remoto (`GET /games`) tampoco se fusiona todavía con el local.
 */
@HiltViewModel
class HistoryViewModel @Inject constructor(
    gameRepository: GameRepository
) : ViewModel() {

    val games: StateFlow<List<GameWithPlayers>> = gameRepository.observeHistory()
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())
}
