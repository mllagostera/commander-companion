package com.vansid.tapeandocartones.domain.repository

import com.vansid.tapeandocartones.domain.model.Game
import com.vansid.tapeandocartones.domain.model.GameAction
import com.vansid.tapeandocartones.domain.model.GamePlayer
import com.vansid.tapeandocartones.domain.model.GameSocketEvent
import com.vansid.tapeandocartones.domain.model.LocalSeat
import com.vansid.tapeandocartones.domain.model.LocalSeatResult
import com.vansid.tapeandocartones.domain.model.NewGameAction
import com.vansid.tapeandocartones.domain.model.PlayedGame
import com.vansid.tapeandocartones.domain.model.RemoteGameSession
import com.vansid.tapeandocartones.domain.model.SeatAssignment
import kotlinx.coroutines.flow.Flow

/**
 * Single access point for games: decides what goes to the backend and what goes to Room. See
 * `GameRepositoryImpl` for the implementation and the full rationale (Room as the tracker's
 * source of truth, best-effort remote mirroring, ADR-0013 proxy-join).
 */
interface GameRepository {

    // ------------------------------------------------------------ local (Room)

    /** History of games played on this device, newest first, seats ordered by index. */
    fun observeHistory(): Flow<List<PlayedGame>>

    suspend fun persistNewLocalGame(gameId: String, seats: List<LocalSeat>)

    suspend fun persistLocalResult(gameId: String, results: List<LocalSeatResult>)

    // ----------------------------------------------------------- remote (API)

    suspend fun listGames(): Result<List<Game>>

    /** Full history of a playgroup's games — used by `JoinGameScreen` to list open (`pending`) ones. */
    suspend fun listGamesForPlaygroup(playgroupId: String): Result<List<Game>>

    suspend fun getGame(gameId: String): Result<Game>

    suspend fun createGame(playgroupId: String? = null): Result<Game>

    /** [userId] null or omitted = self-join. Different = proxy-join (see the backend's ADR-0013). */
    suspend fun joinGame(gameId: String, deckId: String, userId: String? = null): Result<GamePlayer>

    suspend fun leaveGame(gameId: String): Result<Unit>

    suspend fun startGame(gameId: String): Result<Game>

    suspend fun finishGame(gameId: String): Result<Game>

    suspend fun timeline(gameId: String): Result<List<GameAction>>

    suspend fun recordAction(gameId: String, request: NewGameAction): Result<GameAction>

    /**
     * Live updates for [gameId] over WebSocket (see `GameSocketClient`/ADR-0005) — connects,
     * authenticates and reconnects with backoff on its own; the caller only needs to collect and
     * react to [GameSocketEvent]s (see `GameViewModel`).
     */
    fun observeGameEvents(gameId: String, accessToken: suspend () -> String?): Flow<GameSocketEvent>

    // ------------------------------------------------------------ orchestration

    /**
     * Full happy path for creating a game: `POST /games` (with `playgroupId` in Group mode)
     * → one `POST /games/{id}/join` per [assignments] (self-join or proxy-join, as decided
     * by the backend) → an attempted `POST /games/{id}/start`.
     *
     * A 409 on `start` **is not a failure**: it means "there aren't 2 players yet" and the
     * session is left `pending` waiting for someone else to join. Any other error — including an
     * individual join failing — propagates and aborts the rest of the joins.
     *
     * Returns `null` (success, no session) if [assignments] is empty: Casual mode, or
     * Group mode with no seat assigned — the game isn't even created in the backend.
     */
    suspend fun bootstrapRemoteGame(
        playgroupId: String?,
        assignments: List<SeatAssignment>
    ): Result<RemoteGameSession?>

    /** Mirrors a life change of [playerId] on the backend. No `target_id`: the action affects the actor itself. */
    suspend fun recordLifeChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameAction>

    /**
     * Mirrors commander damage from [attackerPlayerId] against [defenderPlayerId]. Only makes
     * sense to call when BOTH seats have a real `GamePlayer` (see `GameViewModel.adjustCommanderDamage`).
     */
    suspend fun recordCommanderDamage(
        session: RemoteGameSession,
        attackerPlayerId: String,
        defenderPlayerId: String,
        amount: Int
    ): Result<GameAction>

    /** Mirrors a poison counter change of [playerId] on the backend (no `target_id`). */
    suspend fun recordPoisonChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameAction>

    /**
     * Mirrors plain (non-commander) damage from [attackerPlayerId] against [defenderPlayerId].
     * Unlike a `LifeChange` on the defender, this is what the backend's statistics attribute to
     * the attacker as damage dealt. [amount] must be positive.
     */
    suspend fun recordCombatDamage(
        session: RemoteGameSession,
        attackerPlayerId: String,
        defenderPlayerId: String,
        amount: Int
    ): Result<GameAction>

    /** Records that [playerId]'s turn started -- the backend counts these as the game's turns. */
    suspend fun recordTurnStart(session: RemoteGameSession, playerId: String): Result<GameAction>

    /**
     * Reverts [actionIds] (most recent first) on the backend -- the undo of actions this device
     * recorded. All or nothing: the backend applies them in a single transaction.
     */
    suspend fun undoActions(session: RemoteGameSession, actionIds: List<String>): Result<List<GameAction>>

    /**
     * Records the end of [playerId]'s turn and how long it lasted ([durationMs], pauses excluded) --
     * what the backend's longest/average turn statistics are built from.
     */
    suspend fun recordTurnEnd(session: RemoteGameSession, playerId: String, durationMs: Long): Result<GameAction>

    /** Records that [actorPlayerId] eliminated [targetPlayerId] -- credited as an elimination in the statistics. */
    suspend fun recordElimination(
        session: RemoteGameSession,
        actorPlayerId: String,
        targetPlayerId: String
    ): Result<GameAction>
}
