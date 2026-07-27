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
 * Partida del backend en la que este dispositivo tiene un asiento.
 *
 * [localPlayerId] es el ID del `GamePlayer` del usuario autenticado — el `actor_id` que espera
 * `POST /games/{id}/actions` (que NO es el `user_id`).
 */
data class RemoteGameSession(
    val gameId: String,
    val localPlayerId: String,
    val status: String
) {
    val isActive: Boolean get() = status == GameStatus.ACTIVE
}

/**
 * Punto único de acceso a partidas: decide qué va al backend y qué a Room.
 *
 * ## Por qué Room sigue siendo la fuente de verdad del tracker
 *
 * El tracker de Android es **pass-and-play en un solo dispositivo**: 2-6 asientos locales con
 * nombres libres, sin cuenta propia. El modelo del backend es el opuesto: `POST /games/{id}/join`
 * sienta **siempre al usuario autenticado** (toma el `user_id` del JWT, el body solo lleva
 * `deck_id`), así que desde un dispositivo solo se puede crear **un** `GamePlayer`. Los demás
 * asientos locales no tienen identidad server-side.
 *
 * Consecuencias, todas verificadas contra `backend/internal/games/service.go`:
 *  - `POST /games/{id}/start` exige `minPlayersToStart = 2`, así que una partida creada y unida
 *    desde un único dispositivo se queda en `pending` hasta que alguien más se una (desde otro
 *    dispositivo o desde el cliente web).
 *  - `POST /games/{id}/actions` solo acepta acciones si la partida está `active`.
 *
 * Por eso el flujo remoto es **best-effort y aditivo**: la partida local nunca se bloquea ni se
 * degrada si el backend no está disponible o no hay quórum, y el espejo remoto se activa solo
 * cuando la partida llega de verdad a `active`.
 */
@Singleton
class GameRepository @Inject constructor(
    private val api: CommanderApi,
    private val gameDao: GameDao,
    private val deckRepository: DeckRepository
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

    suspend fun joinGame(gameId: String, deckId: String): Result<GamePlayerDto> =
        apiCall { api.joinGame(gameId, JoinGameRequest(deckId)) }

    suspend fun leaveGame(gameId: String): Result<Unit> = apiCall { api.leaveGame(gameId) }

    suspend fun startGame(gameId: String): Result<GameDto> = apiCall { api.startGame(gameId) }

    suspend fun finishGame(gameId: String): Result<GameDto> = apiCall { api.finishGame(gameId) }

    suspend fun timeline(gameId: String): Result<List<GameActionDto>> =
        apiCall { api.getTimeline(gameId) }

    suspend fun recordAction(gameId: String, request: CreateActionRequest): Result<GameActionDto> =
        apiCall { api.recordAction(gameId, request) }

    // ------------------------------------------------------------ orquestación

    /**
     * Camino feliz completo de alta de partida: `POST /games` → `POST /games/{id}/join` con el
     * primer deck del usuario → intento de `POST /games/{id}/start`.
     *
     * Un 409 en `start` **no es un fallo**: significa "todavía no hay 2 jugadores" y la sesión
     * queda en [GameStatus.PENDING] esperando que alguien más se una. Cualquier otro error sí
     * se propaga.
     *
     * Devuelve `null` (éxito, sin sesión) si el usuario no tiene ningún deck: sin deck no hay
     * forma de unirse, y la partida simplemente se juega local.
     */
    suspend fun bootstrapRemoteGame(): Result<RemoteGameSession?> {
        val deckId = deckRepository.firstDeckId().getOrElse { return Result.failure(it) }
            ?: return Result.success(null)

        val game = createGame().getOrElse { return Result.failure(it) }
        val localPlayer = joinGame(game.id, deckId).getOrElse { return Result.failure(it) }

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
            RemoteGameSession(gameId = game.id, localPlayerId = localPlayer.id, status = status)
        )
    }

    /**
     * Espeja un cambio de vida del asiento local en el backend.
     *
     * Sin `target_id`: la acción afecta al propio actor, que es el único `GamePlayer` que este
     * dispositivo controla.
     *
     * El backend aplica la regla de eliminación automática (`life_total <= 0`) al recibirla, así
     * que no hace falta mandar un `Elimination` explícito para el asiento local.
     */
    suspend fun recordLifeChange(session: RemoteGameSession, amount: Int): Result<GameActionDto> =
        recordAction(
            gameId = session.gameId,
            request = CreateActionRequest(
                actorId = session.localPlayerId,
                actionType = GameActionType.LIFE_CHANGE,
                payload = amountPayload(amount)
            )
        )

    private companion object {
        const val LOCAL_STATUS_IN_PROGRESS = "IN_PROGRESS"
        const val LOCAL_STATUS_FINISHED = "FINISHED"
        const val HTTP_CONFLICT = 409
    }
}
