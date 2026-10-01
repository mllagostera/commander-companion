package com.vansid.tapeandocartones.presentation.screens.game

import androidx.compose.ui.graphics.Color
import com.vansid.tapeandocartones.core.util.ApiFailure

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
    val commanderDamage: Map<Int, Int> = emptyMap(), // Key: Opponent ID, Value: Damage received
    /**
     * Running totals for the end-of-game summary: every point of damage (plain or commander) this
     * seat dealt to others / received, from the tracker's drags and the commander-damage grid.
     * Life gain doesn't offset [damageTaken]; a commander-grid "-" correction does.
     */
    val damageDealt: Int = 0,
    val damageTaken: Int = 0,
    /**
     * Art crop of the deck this seat is playing, when the seat belongs to a known player who
     * picked one. The tracker paints it behind the seat instead of the flat [color], which stays
     * as the seat's border and identity (the commander-damage grids read opponents by colour).
     * Null for guests, for Casual mode and for decks with no art.
     */
    val deckImageUrl: String? = null,
    /**
     * Time this seat has spent on its own turns, chess-clock style, not counting the turn in
     * progress (see [GameState.turnClockRunningSince] and [GameViewModel.turnTimeOf]).
     */
    val turnTimeMs: Long = 0,
    /** Turns this seat has finished (the one in progress isn't counted until it ends). */
    val turnsTaken: Int = 0,
    /** This seat's single longest finished turn, pauses excluded. */
    val longestTurnMs: Long = 0
)

/**
 * Average length of this seat's turns. Only exact once the game is over: during a turn,
 * [PlayerState.turnTimeMs] already includes part of the turn in progress, which [turnsTaken]
 * doesn't count yet.
 */
fun PlayerState.averageTurnMs(): Long = if (turnsTaken == 0) 0 else turnTimeMs / turnsTaken

/** Alive = positive life, no 21+ damage from a single commander, and fewer than 10 poison counters. */
fun PlayerState.isEliminated(): Boolean =
    life <= 0 || poison >= POISON_LETHAL || commanderDamage.values.any { it >= COMMANDER_DAMAGE_LETHAL }

/** Status of the game's mirror against the backend (see `GameRepository`). */
enum class RemoteSyncStatus {
    /** Still creating/joining the remote game and, in joined mode, fetching the rest of the table. */
    Connecting,

    /** Casual game (no playgroup, no assigned seats): local by design, so there's nothing to report. */
    Casual,

    /** Group game where no seat ended up assigned to a member with a deck: sync isn't attempted. */
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
    /** Whether the first turn has started, i.e. [GameViewModel.startTurnClock] has run. */
    val turnClockStarted: Boolean = false,
    /**
     * When the turn owner's clock started running, on [GameViewModel.nowMs]'s monotonic scale; null
     * while it is stopped -- before the first turn, while the game is paused, once it is over.
     */
    val turnClockRunningSince: Long? = null,
    /**
     * Time banked on the turn in progress before the clock's current running stretch -- a turn
     * interrupted by a pause is settled in several stretches, and this adds them up so the whole
     * turn's length is known when it ends (see [PlayerState.longestTurnMs]).
     */
    val currentTurnMs: Long = 0,
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
    val localSeatId: Int? = null,
    /** Whether there's a step [GameViewModel.undo] can revert (see [UndoLabel]). */
    val canUndo: Boolean = false,
    /** The step undone last, for the tracker's brief "undone: ..." notice. */
    val lastUndone: UndoNotice? = null
)

/**
 * What one undoable step was -- a single thing the player did on the tracker, whatever it
 * cascaded into (a lethal hit also eliminates and ends the game, and undoing it reverts all that).
 */
sealed interface UndoLabel {
    data class Damage(val sourceName: String, val targetName: String, val amount: Int, val commander: Boolean) : UndoLabel
    data class LifeChange(val name: String, val amount: Int) : UndoLabel
    data class Poison(val name: String, val amount: Int) : UndoLabel
    /** The turn handed back to [name], who had passed it. */
    data class PassTurn(val name: String) : UndoLabel
    data object FinishGame : UndoLabel
}

/** One "undone" notice; [id] tells two notices for identical steps apart, so each one shows. */
data class UndoNotice(val id: Long, val label: UndoLabel)
