package com.commandercompanion.presentation.screens.setup

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.repository.PlaygroupRepository
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Datos para el modo Grupo de [PlayerSetupScreen]: los playgroups del usuario autenticado
 * (con sus miembros ya poblados, sin round-trip extra) y, on-demand, los decks de cualquier
 * miembro (propio o ajeno — ver `GET /playgroups/{id}/members/{userId}/decks`, autorizado por
 * membresía compartida). Best-effort: si falla (sin red, sin sesión) el modo Grupo queda sin
 * grupos para elegir — no bloquea el modo Casual.
 */
@HiltViewModel
class PlayerSetupViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository,
    private val sessionManager: SessionManager
) : ViewModel() {

    var playgroups by mutableStateOf<List<PlaygroupDto>>(emptyList())
        private set

    /** Username propio, solo para marcar "(vos)" en el picker de miembros. */
    var ownUsername by mutableStateOf<String?>(null)
        private set

    private val memberDecks = mutableStateMapOf<String, List<DeckDto>>()

    init {
        viewModelScope.launch {
            playgroupRepository.listPlaygroups().onSuccess { playgroups = it }
        }
        viewModelScope.launch {
            ownUsername = sessionManager.currentUsername()
        }
    }

    /** Decks ya cargados de un miembro (propio o ajeno). Vacío hasta que [loadMemberDecks] resuelva. */
    fun decksFor(userId: String): List<DeckDto> = memberDecks[userId] ?: emptyList()

    fun loadMemberDecks(playgroupId: String, userId: String) {
        if (memberDecks.containsKey(userId)) return
        viewModelScope.launch {
            playgroupRepository.getMemberDecks(playgroupId, userId).onSuccess { memberDecks[userId] = it }
        }
    }
}
