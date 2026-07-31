package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.data.local.entity.PlayerResultEntity
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.CreateActionRequest
import com.commandercompanion.data.remote.dto.CreateGameRequest
import com.commandercompanion.data.remote.dto.GameActionDto
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.GamePlayerDto
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.data.remote.dto.JoinGameRequest
import com.commandercompanion.data.remote.dto.amountPayload
import kotlinx.coroutines.flow.Flow
import javax.inject.Inject
import javax.inject.Singleton

/** A local tracker seat, as configured by the user in `PlayerSetupScreen`. */
data class LocalSeat(
    val seatIndex: Int,
    val name: String,
    val colorKey: String,
    val life: Int,
    val mulligans: Int
)

/** Final result of a local seat, to persist when the game ends. */
data class LocalSeatResult(
    val seatIndex: Int,
    val finalLife: Int,
    val won: Boolean
)

/**
 * A seat to remotely seat in the bootstrap: which user ([userId]) and with which deck
 * ([deckId]). The backend decides whether it's a self-join or a proxy-join by comparing [userId]
 * against the authenticated user (see the backend's ADR-0013) — Android never needs to know
 * "which seat am I", just pass the assignment as it ended up in `PlayerSetupScreen`.
 */
data class SeatAssignment(val seatIndex: Int, val userId: String, val deckId: String)

/**
 * A backend game in which this device seated one or more seats with a real
 * `GamePlayer` — its own and, in Group mode, those of proxy-joined teammates. [seatPlayerIds]
 * maps the local seat index (0-based, the same as `PlayerConfig`/`PlayerSetupScreen`) to its
 * `GamePlayer`'s ID — the `actor_id`/`target_id` expected by `POST /games/{id}/actions` (which is
 * NOT the `user_id`).
 */
data class RemoteGameSession(
    val gameId: String,
    val seatPlayerIds: Map<Int, String>,
    val status: String
) {
    val isActive: Boolean get() = status == GameStatus.ACTIVE
}

/**
 * Single access point for games: decides what goes to the backend and what goes to Room.
 *
 * ## Why Room is still the tracker's source of truth
 *
 * The Android tracker is **single-device pass-and-play**: 2-6 local seats,
 * each optionally assigned to a real user (Group mode) or free (Casual/guest mode
 * — see `PlayerSetupScreen`). The game state itself (life, turn, commander damage) always
 * lives in Room; the remote mirror is best-effort and additive.
 *
 * Consequences, verified against `backend/internal/games/service.go`:
 *  - `POST /games/{id}/start` requires `minPlayersToStart = 2`. In Casual mode (no seats
 *    assigned) the remote game is never created. In Group mode, if 2+ seats were assigned
 *    in the same bootstrap the game starts `active` right away; with just 1 it stays
 *    `pending` waiting for someone else.
 *  - `POST /games/{id}/actions` only accepts actions if the game is `active`, and only from
 *    the owner of each `GamePlayer` or whoever proxy-joined it (see ADR-0013).
 */
@Singleton
class GameRepository @Inject constructor(
    private val api: CommanderApi,
    private val gameDao: GameDao
) {

    // ------------------------------------------------------------ local (Room)

    /** History of games played on this device. */
    fun observeHistory(): Flow<List<GameWithPlayers>> = gameDao.getGamesWithPlayers()

    suspend fun persistNewLocalGame(gameId: String, seats: List<LocalSeat>) {
        gameDao.insertGame(
            GameEntity(
                id = gameId,
                startTime = System.currentTimeMillis(),
                status = LOCAL_STATUS_IN_PROGRESS,
                playerCount = seats.size
            )
        )
        gameDao.insertPlayers(
            seats.map { seat ->
                PlayerResultEntity(
                    gameId = gameId,
                    seatIndex = seat.seatIndex,
                    name = seat.name,
                    colorKey = seat.colorKey,
                    finalLife = seat.life,
                    mulligans = seat.mulligans
                )
            }
        )
    }

    suspend fun persistLocalResult(gameId: String, results: List<LocalSeatResult>) {
        gameDao.finishGame(
            gameId = gameId,
            status = LOCAL_STATUS_FINISHED,
            endTime = System.currentTimeMillis()
        )
        results.forEach { result ->
            gameDao.updatePlayerResult(
                gameId = gameId,
                seatIndex = result.seatIndex,
                finalLife = result.finalLife,
                won = result.won
            )
        }
    }

    // ----------------------------------------------------------- remote (API)

    suspend fun listGames(): Result<List<GameDto>> = apiCall { api.listGames().items }

    suspend fun getGame(gameId: String): Result<GameDto> = apiCall { api.getGame(gameId) }

    suspend fun createGame(playgroupId: String? = null): Result<GameDto> =
        apiCall { api.createGame(CreateGameRequest(playgroupId)) }

    /** [userId] null or omitted = self-join. Different = proxy-join (see the backend's ADR-0013). */
    suspend fun joinGame(gameId: String, deckId: String, userId: String? = null): Result<GamePlayerDto> =
        apiCall { api.joinGame(gameId, JoinGameRequest(deckId, userId)) }

    suspend fun leaveGame(gameId: String): Result<Unit> = apiCall { api.leaveGame(gameId) }

    suspend fun startGame(gameId: String): Result<GameDto> = apiCall { api.startGame(gameId) }

    suspend fun finishGame(gameId: String): Result<GameDto> = apiCall { api.finishGame(gameId) }

    suspend fun timeline(gameId: String): Result<List<GameActionDto>> =
        apiCall { api.getTimeline(gameId) }

    suspend fun recordAction(gameId: String, request: CreateActionRequest): Result<GameActionDto> =
        apiCall { api.recordAction(gameId, request) }

    // ------------------------------------------------------------ orchestration

    /**
     * Full happy path for creating a game: `POST /games` (with `playgroupId` in Group mode)
     * → one `POST /games/{id}/join` per [assignments] (self-join or proxy-join, as decided
     * by the backend) → an attempted `POST /games/{id}/start`.
     *
     * A 409 on `start` **is not a failure**: it means "there aren't 2 players yet" and the
     * session is left in [GameStatus.PENDING] waiting for someone else to join. Any other
     * error — including an individual join failing — propagates and aborts the rest of the joins.
     *
     * Returns `null` (success, no session) if [assignments] is empty: Casual mode, or
     * Group mode with no seat assigned — the game isn't even created in the backend.
     */
    suspend fun bootstrapRemoteGame(
        playgroupId: String?, assignments: List<SeatAssignment>
    ): Result<RemoteGameSession?> {
        if (assignments.isEmpty()) return Result.success(null)

        val game = createGame(playgroupId).getOrElse { return Result.failure(it) }

        val seatPlayerIds = mutableMapOf<Int, String>()
        for (assignment in assignments) {
            val player = joinGame(game.id, assignment.deckId, assignment.userId)
                .getOrElse { return Result.failure(it) }
            seatPlayerIds[assignment.seatIndex] = player.id
        }

        val status = startGame(game.id).fold(
            onSuccess = { it.status },
            onFailure = { error ->
                if (error is ApiError.Http && error.code == HTTP_CONFLICT) {
                    GameStatus.PENDING
                } else {
                    return Result.failure(error)
                }
            }
        )

        return Result.success(
            RemoteGameSession(gameId = game.id, seatPlayerIds = seatPlayerIds, status = status)
        )
    }

    /**
     * Mirrors a life change of [playerId] on the backend. No `target_id`: the action affects the
     * actor itself.
     *
     * The backend applies the automatic elimination rule (`life_total <= 0`) when it receives it,
     * so there's no need to send an explicit `Elimination`.
     */
    suspend fun recordLifeChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameActionDto> =
        recordAction(
            gameId = session.gameId,
            request = CreateActionRequest(
                actorId = playerId,
                actionType = GameActionType.LIFE_CHANGE,
                payload = amountPayload(amount)
            )
        )

    /**
     * Mirrors commander damage from [attackerPlayerId] against [defenderPlayerId]. Only makes
     * sense to call when BOTH seats have a real `GamePlayer` (are in
     * [RemoteGameSession.seatPlayerIds]) — if the attacker is a seat without remote identity,
     * there's no one to attribute the damage to (see `GameViewModel.adjustCommanderDamage`).
     */
    suspend fun recordCommanderDamage(
        session: RemoteGameSession, attackerPlayerId: String, defenderPlayerId: String, amount: Int
    ): Result<GameActionDto> =
        recordAction(
            gameId = session.gameId,
            request = CreateActionRequest(
                actorId = attackerPlayerId,
                targetId = defenderPlayerId,
                actionType = GameActionType.COMMANDER_DAMAGE,
                payload = amountPayload(amount)
            )
        )

    /** Mirrors a poison counter change of [playerId] on the backend (no `target_id`). */
    suspend fun recordPoisonChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameActionDto> =
        recordAction(
            gameId = session.gameId,
            request = CreateActionRequest(
                actorId = playerId,
                actionType = GameActionType.POISON_COUNTER,
                payload = amountPayload(amount)
            )
        )

    private companion object {
        const val LOCAL_STATUS_IN_PROGRESS = "IN_PROGRESS"
        const val LOCAL_STATUS_FINISHED = "FINISHED"
        const val HTTP_CONFLICT = 409
    }
}
