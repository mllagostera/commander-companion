package com.commandercompanion.presentation.screens.joingame

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.toUserMessage
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.domain.repository.DeckRepository
import com.commandercompanion.domain.repository.GameRepository
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.launch
import javax.inject.Inject

/** Same seat cap `PlayerSetupScreen`/`GameTrackerScreen`'s quadrant grid supports (2-6). */
private const val MAX_SEATS = 6

data class JoinGameUiState(
    val playgroups: List<PlaygroupDto> = emptyList(),
    val selectedPlaygroup: PlaygroupDto? = null,
    val isLoadingGames: Boolean = false,
    val joinableGames: List<GameDto> = emptyList(),
    val selectedGameId: String? = null,
    val ownDecks: List<DeckDto> = emptyList(),
    val selectedDeckId: String? = null,
    val isJoining: Boolean = false,
    val error: String? = null
)

/**
 * "Join a game" — the counterpart of `PlayerSetupScreen`'s Group mode: instead of hosting a new
 * pass-and-play session on this device, this seats the authenticated user (self-join, `POST
 * /games/{id}/join` without `user_id`) into a `pending` game someone else already created for a
 * shared playgroup. Closes the "second device joins an existing remote game" half of the
 * "Complete end-to-end flow" gap tracked in Stage 5 of `docs/roadmap/TASKS.md` — `GameViewModel`
 * covers the other half (rendering the rest of the table and reconciling their live actions).
 *
 * Only `pending` games can be joined at all (`JoinGame` on the backend rejects anything else with
 * 409) — [selectPlaygroup] filters to those, additionally excluding games the user is already
 * seated in and ones already at the tracker's 6-seat cap.
 */
@HiltViewModel
class JoinGameViewModel @Inject constructor(
    private val playgroupRepository: PlaygroupRepository,
    private val gameRepository: GameRepository,
    private val deckRepository: DeckRepository,
    private val sessionManager: SessionManager
) : ViewModel() {

    var uiState by mutableStateOf(JoinGameUiState())
        private set

    init {
        viewModelScope.launch {
            playgroupRepository.listPlaygroups().onSuccess { groups ->
                uiState = uiState.copy(playgroups = groups)
            }
        }
        viewModelScope.launch {
            deckRepository.listDecks().onSuccess { decks ->
                uiState = uiState.copy(ownDecks = decks)
            }
        }
    }

    fun selectPlaygroup(playgroup: PlaygroupDto) {
        uiState = uiState.copy(
            selectedPlaygroup = playgroup,
            joinableGames = emptyList(),
            selectedGameId = null,
            isLoadingGames = true,
            error = null
        )
        viewModelScope.launch {
            val ownUsername = sessionManager.currentUsername()
            val ownUserId = playgroup.members.firstOrNull { it.username == ownUsername }?.userId
            gameRepository.listGamesForPlaygroup(playgroup.id).fold(
                onSuccess = { games ->
                    val joinable = games.filter { game ->
                        game.status == GameStatus.PENDING &&
                            game.players.size < MAX_SEATS &&
                            game.players.none { it.userId == ownUserId }
                    }
                    uiState = uiState.copy(isLoadingGames = false, joinableGames = joinable)
                },
                onFailure = { error ->
                    uiState = uiState.copy(
                        isLoadingGames = false,
                        error = (error as? ApiError)?.toUserMessage() ?: "No se pudieron cargar las partidas"
                    )
                }
            )
        }
    }

    fun selectGame(gameId: String) {
        uiState = uiState.copy(selectedGameId = gameId)
    }

    fun selectDeck(deckId: String) {
        uiState = uiState.copy(selectedDeckId = deckId)
    }

    /**
     * Joins [JoinGameUiState.selectedGameId] with [JoinGameUiState.selectedDeckId] and, best-effort,
     * attempts to start the game right after: if this is the second seat to join, the game is still
     * `pending` until someone calls `start` — the host's own bootstrap already tried once and may have
     * given up with a 409 if fewer than 2 seats were assigned at the time. A 409 here (still not
     * enough players, or someone else already started it) is expected, not a failure — [onJoined]
     * still fires; `GameViewModel` resolves the authoritative status itself via `GET /games/{id}`.
     */
    fun join(onJoined: (gameId: String, localPlayerId: String) -> Unit) {
        val gameId = uiState.selectedGameId ?: return
        val deckId = uiState.selectedDeckId ?: return
        uiState = uiState.copy(isJoining = true, error = null)
        viewModelScope.launch {
            gameRepository.joinGame(gameId, deckId).fold(
                onSuccess = { player ->
                    gameRepository.startGame(gameId)
                    uiState = uiState.copy(isJoining = false)
                    onJoined(gameId, player.id)
                },
                onFailure = { error ->
                    uiState = uiState.copy(
                        isJoining = false,
                        error = (error as? ApiError)?.toUserMessage() ?: "No se pudo unir a la partida"
                    )
                }
            )
        }
    }
}
