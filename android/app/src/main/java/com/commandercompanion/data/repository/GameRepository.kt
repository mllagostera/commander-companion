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

/** Un asiento local del tracker, tal como lo configuró el usuario en `PlayerSetupScreen`. */
data class LocalSeat(
    val seatIndex: Int,
    val name: String,
    val colorKey: String,
    val life: Int,
    val mulligans: Int
)

/** Resultado final de un asiento local, para persistir al terminar la partida. */
data class LocalSeatResult(
    val seatIndex: Int,
    val finalLife: Int,
    val won: Boolean
)

/**
 * Un asiento a sentar remotamente en el bootstrap: a qué usuario ([userId]) y con qué deck
 * ([deckId]). El backend decide si es un self-join o un proxy-join comparando [userId] contra
 * el usuario autenticado (ver ADR-0013 del backend) — Android nunca necesita saber "cuál asiento
 * soy yo", solo pasar la asignación tal como quedó en `PlayerSetupScreen`.
 */
data class SeatAssignment(val seatIndex: Int, val userId: String, val deckId: String)

/**
 * Partida del backend en la que este dispositivo sentó uno o más asientos con `GamePlayer`
 * real — el propio y, en modo Grupo, los de compañeros proxy-joineados. [seatPlayerIds] mapea
 * el índice de asiento local (0-based, el mismo que `PlayerConfig`/`PlayerSetupScreen`) al ID
 * de su `GamePlayer` — el `actor_id`/`target_id` que espera `POST /games/{id}/actions` (que NO
 * es el `user_id`).
 */
data class RemoteGameSession(
    val gameId: String,
    val seatPlayerIds: Map<Int, String>,
    val status: String
) {
    val isActive: Boolean get() = status == GameStatus.ACTIVE
}

/**
 * Punto único de acceso a partidas: decide qué va al backend y qué a Room.
 *
 * ## Por qué Room sigue siendo la fuente de verdad del tracker
 *
 * El tracker de Android es **pass-and-play en un solo dispositivo**: 2-6 asientos locales,
 * cada uno opcionalmente asignado a un usuario real (modo Grupo) o libre (modo Casual/invitado
 * — ver `PlayerSetupScreen`). El estado del juego en sí (vida, turno, daño de comandante) vive
 * siempre en Room; el espejo remoto es best-effort y aditivo.
 *
 * Consecuencias, verificadas contra `backend/internal/games/service.go`:
 *  - `POST /games/{id}/start` exige `minPlayersToStart = 2`. En modo Casual (sin asientos
 *    asignados) no se llega a crear la partida remota. En modo Grupo, si se asignaron 2+
 *    asientos en el mismo bootstrap la partida arranca `active` en el momento; con 1 solo queda
 *    `pending` esperando a alguien más.
 *  - `POST /games/{id}/actions` solo acepta acciones si la partida está `active`, y solo del
 *    dueño de cada `GamePlayer` o de quien lo proxy-joineó (ver ADR-0013).
 */
@Singleton
class GameRepository @Inject constructor(
    private val api: CommanderApi,
    private val gameDao: GameDao
) {

    // ------------------------------------------------------------ local (Room)

    /** Historial de partidas jugadas en este dispositivo. */
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

    /** [userId] null u omitido = self-join. Distinto = proxy-join (ver ADR-0013 del backend). */
    suspend fun joinGame(gameId: String, deckId: String, userId: String? = null): Result<GamePlayerDto> =
        apiCall { api.joinGame(gameId, JoinGameRequest(deckId, userId)) }

    suspend fun leaveGame(gameId: String): Result<Unit> = apiCall { api.leaveGame(gameId) }

    suspend fun startGame(gameId: String): Result<GameDto> = apiCall { api.startGame(gameId) }

    suspend fun finishGame(gameId: String): Result<GameDto> = apiCall { api.finishGame(gameId) }

    suspend fun timeline(gameId: String): Result<List<GameActionDto>> =
        apiCall { api.getTimeline(gameId) }

    suspend fun recordAction(gameId: String, request: CreateActionRequest): Result<GameActionDto> =
        apiCall { api.recordAction(gameId, request) }

    // ------------------------------------------------------------ orquestación

    /**
     * Camino feliz completo de alta de partida: `POST /games` (con `playgroupId` en modo Grupo)
     * → un `POST /games/{id}/join` por cada [assignments] (self-join o proxy-join, según decida
     * el backend) → intento de `POST /games/{id}/start`.
     *
     * Un 409 en `start` **no es un fallo**: significa "todavía no hay 2 jugadores" y la sesión
     * queda en [GameStatus.PENDING] esperando que alguien más se una. Cualquier otro error —
     * incluido que un join individual falle— se propaga y aborta el resto de los joins.
     *
     * Devuelve `null` (éxito, sin sesión) si [assignments] viene vacía: modo Casual, o modo
     * Grupo sin ningún asiento asignado — ni siquiera se crea la partida en el backend.
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
     * Espeja un cambio de vida de [playerId] en el backend. Sin `target_id`: la acción afecta al
     * propio actor.
     *
     * El backend aplica la regla de eliminación automática (`life_total <= 0`) al recibirla, así
     * que no hace falta mandar un `Elimination` explícito.
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
     * Espeja daño de comandante de [attackerPlayerId] contra [defenderPlayerId]. Solo tiene
     * sentido llamarla cuando AMBOS asientos tienen `GamePlayer` real (están en
     * [RemoteGameSession.seatPlayerIds]) — si el atacante es un asiento sin identidad remota,
     * no hay a quién atribuirle el daño (ver `GameViewModel.adjustCommanderDamage`).
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

    /** Espeja un cambio de contadores de veneno de [playerId] en el backend (sin `target_id`). */
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
