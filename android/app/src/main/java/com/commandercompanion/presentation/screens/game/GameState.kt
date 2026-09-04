package com.commandercompanion.presentation.screens.game

import androidx.compose.ui.graphics.Color
import com.commandercompanion.core.util.ApiFailure

/** Commander elimination rules, shared by [GameViewModel] and the tracker UI. */
const val COMMANDER_DAMAGE_LETHAL = 21
const val POISON_LETHAL = 10
const val STARTING_LIFE = 40

data class PlayerState(
    val id: Int,
    val name: String,
    val life: Int = STARTING_LIFE,
    val color: Color,
    val mulligans: Int = 0,
    val poison: Int = 0,
    val commanderDamage: Map<Int, Int> = emptyMap() // Key: Opponent ID, Value: Damage received
)

/** Alive = positive life, no 21+ damage from a single commander, and fewer than 10 poison counters. */
fun PlayerState.isEliminated(): Boolean =
    life <= 0 || poison >= POISON_LETHAL || commanderDamage.values.any { it >= COMMANDER_DAMAGE_LETHAL }

/** Status of the game's mirror against the backend (see `GameRepository`). */
enum class RemoteSyncStatus {
    /** Still creating/joining the remote game and, in joined mode, fetching the rest of the table. */
    Connecting,

    /** Sync isn't attempted (e.g. the user has no decks). 100% local game. */
    Disabled,

    /** Game created and joined, but `pending`: waiting for a second player to join. */
    WaitingForPlayers,

    /** Game `active` on the backend: local seat changes are recorded there. */
    Synced,

    /** Sync failed. The game keeps working locally. */
    Failed
}

data class RemoteSyncState(
    val status: RemoteSyncStatus = RemoteSyncStatus.Connecting,
    /**
     * Only set when [status] is [RemoteSyncStatus.Failed]: what went wrong, for the screen to
     * turn into a string resource. Every other status already says all there is to say, so its
     * wording is picked from [status] alone. It used to be a `String` built here, which could
     * not be translated into the three locales the app ships.
     */
    val failure: ApiFailure? = null,
    val gameId: String? = null
)

data class GameState(
    val players: List<PlayerState> = emptyList(),
    val currentTurn: Int = 1,
    val startingPlayerId: Int? = null,
    /** Seat whose turn it currently is, so the tracker UI can ring-highlight its quadrant. Mirrors [startingPlayerId] until [nextTurn][GameViewModel.nextTurn] advances it. */
    val currentTurnPlayerId: Int? = startingPlayerId,
    val isFinished: Boolean = false,
    val winnerId: Int? = null,
    val remoteSync: RemoteSyncState = RemoteSyncState(),
    /**
     * Which seat THIS device controls, when it joined someone else's already-created remote game
     * (see `JoinGameScreen`/`GameViewModel`'s joined mode). Null in the usual pass-and-play mode,
     * where every seat on this single device is editable, same as always. Non-null seats are
     * read-only in the UI — their life/poison/commander damage only change via the WebSocket
     * events broadcast by whichever device actually controls them.
     */
    val localSeatId: Int? = null
)
