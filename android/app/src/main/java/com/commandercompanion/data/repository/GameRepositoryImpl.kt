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
import com.commandercompanion.data.remote.ws.GameSocketClient
import com.commandercompanion.data.remote.ws.GameSocketEvent
import com.commandercompanion.domain.model.LocalSeat
import com.commandercompanion.domain.model.LocalSeatResult
import com.commandercompanion.domain.model.RemoteGameSession
import com.commandercompanion.domain.model.SeatAssignment
import com.commandercompanion.domain.repository.GameRepository
import kotlinx.coroutines.flow.Flow
import javax.inject.Inject

/**
 * [GameRepository] implementation: decides what goes to the backend and what goes to Room.
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
class GameRepositoryImpl @Inject constructor(
    private val api: CommanderApi,
    private val gameDao: GameDao,
    private val gameSocketClient: GameSocketClient
) : GameRepository {

    // ------------------------------------------------------------ local (Room)

    override fun observeHistory(): Flow<List<GameWithPlayers>> = gameDao.getGamesWithPlayers()

    override suspend fun persistNewLocalGame(gameId: String, seats: List<LocalSeat>) {
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

    override suspend fun persistLocalResult(gameId: String, results: List<LocalSeatResult>) {
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

    override suspend fun listGames(): Result<List<GameDto>> {
        val all = mutableListOf<GameDto>()
        var cursor: String? = null
        do {
            val page = apiCall { api.listGames(cursor) }.getOrElse { return Result.failure(it) }
            all += page.items
            cursor = page.nextCursor
        } while (cursor != null)
        return Result.success(all)
    }

    override suspend fun listGamesForPlaygroup(playgroupId: String): Result<List<GameDto>> =
        apiCall { api.listGamesForPlaygroup(playgroupId).items }

    override suspend fun getGame(gameId: String): Result<GameDto> = apiCall { api.getGame(gameId) }

    override suspend fun createGame(playgroupId: String?): Result<GameDto> =
        apiCall { api.createGame(CreateGameRequest(playgroupId)) }

    override suspend fun joinGame(gameId: String, deckId: String, userId: String?): Result<GamePlayerDto> =
        apiCall { api.joinGame(gameId, JoinGameRequest(deckId, userId)) }

    override suspend fun leaveGame(gameId: String): Result<Unit> = apiCall { api.leaveGame(gameId) }

    override suspend fun startGame(gameId: String): Result<GameDto> = apiCall { api.startGame(gameId) }

    override suspend fun finishGame(gameId: String): Result<GameDto> = apiCall { api.finishGame(gameId) }

    override suspend fun timeline(gameId: String): Result<List<GameActionDto>> =
        apiCall { api.getTimeline(gameId) }

    override suspend fun recordAction(gameId: String, request: CreateActionRequest): Result<GameActionDto> =
        apiCall { api.recordAction(gameId, request) }

    override fun observeGameEvents(gameId: String, accessToken: suspend () -> String?): Flow<GameSocketEvent> =
        gameSocketClient.connect(gameId, accessToken)

    // ------------------------------------------------------------ orchestration

    override suspend fun bootstrapRemoteGame(
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

    override suspend fun recordLifeChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameActionDto> =
        recordAction(
            gameId = session.gameId,
            request = CreateActionRequest(
                actorId = playerId,
                actionType = GameActionType.LIFE_CHANGE,
                payload = amountPayload(amount)
            )
        )

    override suspend fun recordCommanderDamage(
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

    override suspend fun recordPoisonChange(session: RemoteGameSession, playerId: String, amount: Int): Result<GameActionDto> =
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
