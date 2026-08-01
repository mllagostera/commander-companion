package com.commandercompanion.presentation.screens.setup

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * Data for [PlayerSetupScreen]'s Group mode: the authenticated user's playgroups
 * (with their members already populated, no extra round-trip) and, on-demand, the decks of any
 * member (own or someone else's — see `GET /playgroups/{id}/members/{userId}/decks`, authorized
 * by shared membership). Best-effort: if it fails (no network, no session) Group mode is left
 * with no groups to choose from — it doesn't block Casual mode.
 */
@HiltViewModel
class PlayerSetupViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository,
    private val sessionManager: SessionManager
) : ViewModel() {

    var playgroups by mutableStateOf<List<PlaygroupDto>>(emptyList())
        private set

    /** Own username, only to mark "(you)" in the member picker. */
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

    /** Decks already loaded for a member (own or someone else's). Empty until [loadMemberDecks] resolves. */
    fun decksFor(userId: String): List<DeckDto> = memberDecks[userId] ?: emptyList()

    fun loadMemberDecks(playgroupId: String, userId: String) {
        if (memberDecks.containsKey(userId)) return
        viewModelScope.launch {
            playgroupRepository.getMemberDecks(playgroupId, userId).onSuccess { memberDecks[userId] = it }
        }
    }
}
