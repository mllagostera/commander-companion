package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeGameDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.gameDto
import com.commandercompanion.testing.gamePlayerDto
import com.commandercompanion.testing.httpException
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class GameRepositoryTest {

    private val api = FakeCommanderApi()
    private val dao = FakeGameDao()
    private val repository = GameRepository(api, dao, DeckRepository(api))

    // ------------------------------------------------------------ bootstrap

    @Test
    fun `bootstrap crea, se une e inicia la partida en ese orden`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-a")) }
        api.onCreateGame = { gameDto("game-42", GameStatus.PENDING) }
        api.onJoinGame = { gameId, request ->
            gamePlayerDto(id = "gp-99", gameId = gameId, deckId = request.deckId)
        }
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        val session = repository.bootstrapRemoteGame().getOrThrow()!!

        assertEquals(listOf("listDecks", "createGame", "joinGame", "startGame"), api.calls)
        assertEquals("game-42", session.gameId)
        assertEquals("gp-99", session.localPlayerId)
        assertTrue(session.isActive)
    }

    @Test
    fun `bootstrap se une con el primer deck del usuario`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-elegido"), deckDto("deck-otro")) }
        var joinedWith: String? = null
        api.onJoinGame = { gameId, request ->
            joinedWith = request.deckId
            gamePlayerDto(gameId = gameId, deckId = request.deckId)
        }

        repository.bootstrapRemoteGame().getOrThrow()

        assertEquals("deck-elegido", joinedWith)
    }

    /**
     * El backend exige `minPlayersToStart = 2`. Desde un solo dispositivo solo puede sentarse el
     * usuario autenticado, así que este 409 es el caso NORMAL, no un fallo: la partida queda en
     * `pending` esperando que alguien se una desde otro cliente.
     */
    @Test
    fun `bootstrap trata el 409 de start como pending, no como error`() = runTest {
        api.onStartGame = { throw httpException(409) }

        val session = repository.bootstrapRemoteGame().getOrThrow()!!

        assertEquals(GameStatus.PENDING, session.status)
        assertTrue(!session.isActive)
    }

    @Test
    fun `bootstrap si falla start con algo que no es 409 propaga el error`() = runTest {
        api.onStartGame = { throw httpException(500) }

        val error = repository.bootstrapRemoteGame().exceptionOrNull()

        assertTrue(error is ApiError.Http)
        assertEquals(500, (error as ApiError.Http).code)
    }

    @Test
    fun `bootstrap sin decks devuelve success sin sesion y no toca games`() = runTest {
        api.onListDecks = { emptyList() }

        val result = repository.bootstrapRemoteGame()

        assertTrue(result.isSuccess)
        assertNull(result.getOrThrow())
        assertEquals(listOf("listDecks"), api.calls)
    }

    @Test
    fun `bootstrap propaga un error de red al listar decks`() = runTest {
        api.onListDecks = { throw IOException("sin red") }

        val error = repository.bootstrapRemoteGame().exceptionOrNull()

        assertTrue(error is ApiError.Network)
        assertEquals(listOf("listDecks"), api.calls)
    }

    @Test
    fun `bootstrap propaga un 404 al unirse porque el deck no es del usuario`() = runTest {
        api.onJoinGame = { _, _ -> throw httpException(404) }

        val error = repository.bootstrapRemoteGame().exceptionOrNull()

        assertEquals(404, (error as ApiError.Http).code)
    }

    // --------------------------------------------------------------- acciones

    @Test
    fun `recordLifeChange manda LifeChange con el actor local y el amount`() = runTest {
        val session = RemoteGameSession("game-1", "gp-7", GameStatus.ACTIVE)

        repository.recordLifeChange(session, -3).getOrThrow()

        val (gameId, request) = api.recordedActions.single()
        assertEquals("game-1", gameId)
        assertEquals("gp-7", request.actorId)
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        // Sin target: la acción afecta al propio actor, el único GamePlayer de este dispositivo.
        assertNull(request.targetId)
        assertEquals(-3, request.payload!!["amount"]!!.jsonPrimitive.int)
    }

    @Test
    fun `recordLifeChange mapea el 409 de partida no activa`() = runTest {
        api.onRecordAction = { _, _ -> throw httpException(409) }
        val session = RemoteGameSession("game-1", "gp-7", GameStatus.ACTIVE)

        val error = repository.recordLifeChange(session, 5).exceptionOrNull()

        assertEquals(409, (error as ApiError.Http).code)
    }

    // ----------------------------------------------------------- local (Room)

    @Test
    fun `persistNewLocalGame guarda la partida y todos los asientos`() = runTest {
        val seats = listOf(
            LocalSeat(seatIndex = 0, name = "Ana", colorKey = "blue", life = 40, mulligans = 1),
            LocalSeat(seatIndex = 1, name = "Beto", colorKey = "red", life = 40, mulligans = 0)
        )

        repository.persistNewLocalGame("game-local", seats)

        val game = dao.games.single()
        assertEquals("game-local", game.id)
        assertEquals("IN_PROGRESS", game.status)
        assertEquals(2, game.playerCount)
        assertEquals(listOf("Ana", "Beto"), dao.players.map { it.name })
        assertEquals(listOf(1, 0), dao.players.map { it.mulligans })
    }

    @Test
    fun `persistLocalResult marca la partida finalizada y actualiza cada asiento`() = runTest {
        repository.persistLocalResult(
            "game-local",
            listOf(
                LocalSeatResult(seatIndex = 0, finalLife = 12, won = true),
                LocalSeatResult(seatIndex = 1, finalLife = 0, won = false)
            )
        )

        val (gameId, status, _) = dao.finished.single()
        assertEquals("game-local", gameId)
        assertEquals("FINISHED", status)
        assertEquals(2, dao.updatedResults.size)
        assertEquals(true, dao.updatedResults[0].won)
        assertEquals(12, dao.updatedResults[0].finalLife)
        assertEquals(false, dao.updatedResults[1].won)
    }
}
