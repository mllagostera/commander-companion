package com.commandercompanion.domain.repository

import com.commandercompanion.domain.model.Game
import com.commandercompanion.domain.model.GameAction
import com.commandercompanion.domain.model.GamePlayer
import com.commandercompanion.domain.model.GameSocketEvent
import com.commandercompanion.domain.model.LocalSeat
import com.commandercompanion.domain.model.LocalSeatResult
import com.commandercompanion.domain.model.NewGameAction
import com.commandercompanion.domain.model.PlayedGame
import com.commandercompanion.domain.model.RemoteGameSession
import com.commandercompanion.domain.model.SeatAssignment
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
}
