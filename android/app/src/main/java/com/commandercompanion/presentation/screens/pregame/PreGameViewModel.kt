package com.commandercompanion.presentation.screens.pregame

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.session.SessionManager
import com.commandercompanion.domain.model.Deck
import com.commandercompanion.domain.model.Playgroup
import com.commandercompanion.domain.repository.PlaygroupRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

/**
 * Data for [PreGameScreen]'s Group-mode seat assignment: the playgroup's members (to assign
 * seats to) and, on-demand, a member's decks (own or someone else's — see
 * `GET /playgroups/{id}/members/{userId}/decks`, authorized by shared membership).
 *
 * Best-effort like [com.commandercompanion.presentation.screens.setup.PlayerSetupViewModel]:
 * a failed fetch just leaves the seat-assignment UI without members, it doesn't block Casual
 * mode (which never touches this ViewModel — `playgroupId` is null there).
 */
@HiltViewModel
class PreGameViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository,
    private val sessionManager: SessionManager
) : ViewModel() {

    var playgroup by mutableStateOf<Playgroup?>(null)
        private set

    /** Own username, only to mark "(tú)" in the member picker. */
    var ownUsername by mutableStateOf<String?>(null)
        private set

    private val memberDecks = mutableStateMapOf<String, List<Deck>>()

    fun loadPlaygroup(playgroupId: String) {
        if (playgroup != null) return
        viewModelScope.launch {
            playgroupRepository.getPlaygroup(playgroupId).onSuccess { playgroup = it }
        }
        viewModelScope.launch {
            ownUsername = sessionManager.currentUsername()
        }
    }

    /** Decks already loaded for a member (own or someone else's). Empty until [loadMemberDecks] resolves. */
    fun decksFor(userId: String): List<Deck> = memberDecks[userId] ?: emptyList()

    fun loadMemberDecks(playgroupId: String, userId: String) {
        if (memberDecks.containsKey(userId)) return
        viewModelScope.launch {
            playgroupRepository.getMemberDecks(playgroupId, userId).onSuccess { memberDecks[userId] = it }
        }
    }
}
