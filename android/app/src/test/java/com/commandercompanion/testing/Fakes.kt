package com.commandercompanion.testing

import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.data.local.entity.PlayerResultEntity
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.CreateActionRequest
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.CreateGameRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.GameActionDto
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.GamePlayerDto
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import com.commandercompanion.data.remote.dto.JoinGameRequest
import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import retrofit2.HttpException
import retrofit2.Response

/** Construye una [HttpException] real con el código dado, para testear el mapeo de errores. */
fun httpException(code: Int, body: String = """{"message":"boom"}"""): HttpException =
    HttpException(Response.error<Any>(code, body.toResponseBody("application/json".toMediaType())))

// --------------------------------------------------------------------- DTOs de fixture

fun deckDto(id: String = "deck-1", name: String = "Atraxa") = DeckDto(
    id = id,
    userId = "user-1",
    name = name,
    commander = "Atraxa, Praetors' Voice",
    moxfieldId = null
)

fun gameDto(id: String = "game-1", status: String = GameStatus.PENDING) = GameDto(
    id = id,
    status = status
)

fun gamePlayerDto(id: String = "gp-1", gameId: String = "game-1", deckId: String = "deck-1") =
    GamePlayerDto(id = id, gameId = gameId, userId = "user-1", deckId = deckId)

fun gameActionDto(gameId: String, request: CreateActionRequest) = GameActionDto(
    id = "action-1",
    gameId = gameId,
    actorId = request.actorId,
    targetId = request.targetId,
    actionType = request.actionType,
    payload = request.payload,
    createdAt = "2026-07-27T10:00:00Z"
)

/**
 * Fake de [CommanderApi] con un handler por endpoint.
 *
 * Se prefiere sobre una librería de mocking: la interfaz es estable y así los tests declaran
 * explícitamente qué responde cada endpoint, sin `verify` mágicos.
 */
class FakeCommanderApi : CommanderApi {

    /** Todas las acciones que recibió `POST /games/{id}/actions`, en orden. */
    val recordedActions = mutableListOf<Pair<String, CreateActionRequest>>()

    /** Nombres de endpoint invocados, en orden — para aseverar la secuencia del bootstrap. */
    val calls = mutableListOf<String>()

    var onListDecks: suspend () -> List<DeckDto> = { listOf(deckDto()) }
    var onImportMoxfield: suspend (ImportMoxfieldRequest) -> DeckDto = { deckDto() }
    var onDeleteDeck: suspend (String) -> Unit = { }
    var onCreateGame: suspend (CreateGameRequest) -> GameDto = { gameDto() }
    var onJoinGame: suspend (String, JoinGameRequest) -> GamePlayerDto = { gameId, request ->
        gamePlayerDto(gameId = gameId, deckId = request.deckId)
    }
    var onStartGame: suspend (String) -> GameDto = { id -> gameDto(id, GameStatus.ACTIVE) }
    var onFinishGame: suspend (String) -> GameDto = { id -> gameDto(id, GameStatus.FINISHED) }
    var onRecordAction: suspend (String, CreateActionRequest) -> GameActionDto = { id, request ->
        gameActionDto(id, request)
    }

    override suspend fun checkHealth(): String = "ok"

    override suspend fun listDecks(): PagedResponse<DeckDto> {
        calls += "listDecks"
        return PagedResponse(items = onListDecks())
    }

    override suspend fun createDeck(request: CreateDeckRequest): DeckDto =
        deckDto(name = request.name)

    override suspend fun getDeck(deckId: String): DeckDto = deckDto(id = deckId)

    override suspend fun deleteDeck(deckId: String) {
        calls += "deleteDeck"
        onDeleteDeck(deckId)
    }

    override suspend fun importMoxfieldDeck(request: ImportMoxfieldRequest): DeckDto {
        calls += "importMoxfieldDeck"
        return onImportMoxfield(request)
    }

    override suspend fun listGames(): PagedResponse<GameDto> = PagedResponse(items = listOf(gameDto()))

    override suspend fun createGame(request: CreateGameRequest): GameDto {
        calls += "createGame"
        return onCreateGame(request)
    }

    override suspend fun getGame(gameId: String): GameDto = gameDto(gameId)

    override suspend fun joinGame(gameId: String, request: JoinGameRequest): GamePlayerDto {
        calls += "joinGame"
        return onJoinGame(gameId, request)
    }

    override suspend fun leaveGame(gameId: String) {
        calls += "leaveGame"
    }

    override suspend fun startGame(gameId: String): GameDto {
        calls += "startGame"
        return onStartGame(gameId)
    }

    override suspend fun finishGame(gameId: String): GameDto {
        calls += "finishGame"
        return onFinishGame(gameId)
    }

    override suspend fun recordAction(
        gameId: String,
        request: CreateActionRequest
    ): GameActionDto {
        calls += "recordAction"
        recordedActions += gameId to request
        return onRecordAction(gameId, request)
    }

    override suspend fun getTimeline(gameId: String): List<GameActionDto> = emptyList()

    override suspend fun getUserStats(): UserStatsDto = UserStatsDto(userId = "user-1")

    override suspend fun getDeckStats(deckId: String): DeckStatsDto = DeckStatsDto(deckId = deckId)

    override suspend fun getPlaygroupStats(playgroupId: String): PlaygroupStatsDto =
        PlaygroupStatsDto(playgroupId = playgroupId)
}

/** Fake en memoria de [GameDao]. */
class FakeGameDao : GameDao {

    val games = mutableListOf<GameEntity>()
    val players = mutableListOf<PlayerResultEntity>()
    val finished = mutableListOf<Triple<String, String, Long>>()
    val updatedResults = mutableListOf<PlayerResultUpdate>()

    data class PlayerResultUpdate(
        val gameId: String,
        val seatIndex: Int,
        val finalLife: Int,
        val won: Boolean
    )

    private val history = MutableStateFlow<List<GameWithPlayers>>(emptyList())

    override suspend fun insertGame(game: GameEntity) {
        games += game
    }

    override suspend fun insertPlayers(players: List<PlayerResultEntity>) {
        this.players += players
    }

    override suspend fun finishGame(gameId: String, status: String, endTime: Long) {
        finished += Triple(gameId, status, endTime)
    }

    override suspend fun updatePlayerResult(
        gameId: String,
        seatIndex: Int,
        finalLife: Int,
        won: Boolean
    ) {
        updatedResults += PlayerResultUpdate(gameId, seatIndex, finalLife, won)
    }

    override fun getGamesWithPlayers(): Flow<List<GameWithPlayers>> = history
}
