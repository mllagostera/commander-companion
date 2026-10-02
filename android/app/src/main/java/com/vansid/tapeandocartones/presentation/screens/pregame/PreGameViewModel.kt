package com.vansid.tapeandocartones.presentation.screens.pregame

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.session.SessionManager
import com.vansid.tapeandocartones.domain.model.Deck
import com.vansid.tapeandocartones.domain.model.Playgroup
import com.vansid.tapeandocartones.domain.repository.LastDeckRepository
import com.vansid.tapeandocartones.domain.repository.PlaygroupRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

/**
 * Data for [PreGameScreen]'s Group-mode seat assignment: the playgroup's members (to assign
 * seats to) and, on-demand, a member's decks (own or someone else's — see
 * `GET /playgroups/{id}/members/{userId}/decks`, authorized by shared membership).
 *
 * Best-effort like [com.vansid.tapeandocartones.presentation.screens.setup.PlayerSetupViewModel]:
 * a failed fetch just leaves the seat-assignment UI without members, it doesn't block Casual
 * mode (which never touches this ViewModel — `playgroupId` is null there).
 */
@HiltViewModel
class PreGameViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository,
    private val sessionManager: SessionManager,
    private val lastDeckRepository: LastDeckRepository
) : ViewModel() {

    var playgroup by mutableStateOf<Playgroup?>(null)
        private set

    /** Own username, only to mark "(tú)" in the member picker. */
    var ownUsername by mutableStateOf<String?>(null)
        private set

    private val memberDecks = mutableStateMapOf<String, List<Deck>>()
    private val lastDeckIds = mutableStateMapOf<String, String>()

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

    /** False while a member's decks are still being fetched; a failed fetch counts as loaded (no decks). */
    fun decksLoaded(userId: String): Boolean = memberDecks.containsKey(userId)

    /** The deck this member last played in this playgroup on this device, if it's still one of theirs. */
    fun lastDeckFor(userId: String): Deck? = lastDeckIds[userId]?.let { id -> decksFor(userId).firstOrNull { it.id == id } }

    fun loadMemberDecks(playgroupId: String, userId: String) {
        if (memberDecks.containsKey(userId)) return
        viewModelScope.launch {
            // Read before publishing the decks, so the seat sees the suggestion the moment its decks appear.
            lastDeckRepository.lastDeckId(playgroupId, userId)?.let { lastDeckIds[userId] = it }
            memberDecks[userId] = playgroupRepository.getMemberDecks(playgroupId, userId).getOrDefault(emptyList())
        }
    }

    fun rememberDecks(playgroupId: String, deckIdsByUserId: Map<String, String>) {
        lastDeckRepository.remember(playgroupId, deckIdsByUserId)
    }
}
