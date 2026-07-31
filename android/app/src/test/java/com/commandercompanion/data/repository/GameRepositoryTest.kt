package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeGameDao
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

class GameRepositoryTest {

    private val api = FakeCommanderApi()
    private val dao = FakeGameDao()
    private val repository = GameRepository(api, dao)

    private fun oneAssignment(userId: String = "user-1", deckId: String = "deck-a") =
        listOf(SeatAssignment(seatIndex = 0, userId = userId, deckId = deckId))

    // ------------------------------------------------------------ bootstrap

    @Test
    fun `bootstrap crea, se une e inicia la partida en ese orden`() = runTest {
        api.onCreateGame = { gameDto("game-42", GameStatus.PENDING) }
        api.onJoinGame = { gameId, request ->
            gamePlayerDto(id = "gp-99", gameId = gameId, userId = request.userId!!, deckId = request.deckId)
        }
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        val session = repository.bootstrapRemoteGame(playgroupId = null, assignments = oneAssignment()).getOrThrow()!!

        assertEquals(listOf("createGame", "joinGame", "startGame"), api.calls)
        assertEquals("game-42", session.gameId)
        assertEquals("gp-99", session.seatPlayerIds[0])
        assertTrue(session.isActive)
    }

    @Test
    fun `bootstrap se une con el deckId y userId de cada asignacion`() = runTest {
        var joinedWith: Pair<String, String?>? = null
        api.onJoinGame = { gameId, request ->
            joinedWith = request.deckId to request.userId
            gamePlayerDto(gameId = gameId, userId = request.userId!!, deckId = request.deckId)
        }

        repository.bootstrapRemoteGame(playgroupId = null, assignments = oneAssignment("user-9", "deck-elegido"))
            .getOrThrow()

        assertEquals("deck-elegido" to "user-9", joinedWith)
    }

    @Test
    fun `bootstrap sienta a varios asientos, uno por cada asignacion`() = runTest {
        val assignments = listOf(
            SeatAssignment(seatIndex = 0, userId = "user-1", deckId = "deck-1"),
            SeatAssignment(seatIndex = 2, userId = "user-2", deckId = "deck-2")
        )
        api.onJoinGame = { gameId, request ->
            gamePlayerDto(id = "gp-${request.userId}", gameId = gameId, userId = request.userId!!, deckId = request.deckId)
        }
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        val session = repository.bootstrapRemoteGame(playgroupId = "playgroup-1", assignments = assignments)
            .getOrThrow()!!

        assertEquals(mapOf(0 to "gp-user-1", 2 to "gp-user-2"), session.seatPlayerIds)
        assertEquals(listOf("createGame", "joinGame", "joinGame", "startGame"), api.calls)
    }

    /**
     * The backend requires `minPlayersToStart = 2`. With only one seat assigned in the bootstrap
     * (Group mode with a single member, or the old best-effort Casual mode), this 409 is the
     * NORMAL case, not a failure: the game stays `pending` waiting for someone to join.
     */
    @Test
    fun `bootstrap trata el 409 de start como pending, no como error`() = runTest {
        api.onStartGame = { throw httpException(409) }

        val session = repository.bootstrapRemoteGame(playgroupId = null, assignments = oneAssignment()).getOrThrow()!!

        assertEquals(GameStatus.PENDING, session.status)
        assertTrue(!session.isActive)
    }

    @Test
    fun `bootstrap si falla start con algo que no es 409 propaga el error`() = runTest {
        api.onStartGame = { throw httpException(500) }

        val error = repository.bootstrapRemoteGame(playgroupId = null, assignments = oneAssignment()).exceptionOrNull()

        assertTrue(error is ApiError.Http)
        assertEquals(500, (error as ApiError.Http).code)
    }

    @Test
    fun `bootstrap sin asignaciones devuelve success sin sesion y no toca games`() = runTest {
        val result = repository.bootstrapRemoteGame(playgroupId = null, assignments = emptyList())

        assertTrue(result.isSuccess)
        assertNull(result.getOrThrow())
        assertEquals(emptyList<String>(), api.calls)
    }

    @Test
    fun `bootstrap propaga un 404 al unirse y no intenta el resto de los asientos`() = runTest {
        api.onJoinGame = { _, _ -> throw httpException(404) }
        val assignments = listOf(
            SeatAssignment(seatIndex = 0, userId = "user-1", deckId = "deck-ajeno"),
            SeatAssignment(seatIndex = 1, userId = "user-2", deckId = "deck-2")
        )

        val error = repository.bootstrapRemoteGame(playgroupId = null, assignments = assignments).exceptionOrNull()

        assertEquals(404, (error as ApiError.Http).code)
        assertEquals(listOf("createGame", "joinGame"), api.calls)
    }

    // --------------------------------------------------------------- actions

    @Test
    fun `recordLifeChange manda LifeChange con el actor indicado y el amount`() = runTest {
        val session = RemoteGameSession("game-1", mapOf(0 to "gp-7"), GameStatus.ACTIVE)

        repository.recordLifeChange(session, "gp-7", -3).getOrThrow()

        val (gameId, request) = api.recordedActions.single()
        assertEquals("game-1", gameId)
        assertEquals("gp-7", request.actorId)
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        // No target: the action affects the actor itself.
        assertNull(request.targetId)
        assertEquals(-3, request.payload!!["amount"]!!.jsonPrimitive.int)
    }

    @Test
    fun `recordLifeChange mapea el 409 de partida no activa`() = runTest {
        api.onRecordAction = { _, _ -> throw httpException(409) }
        val session = RemoteGameSession("game-1", mapOf(0 to "gp-7"), GameStatus.ACTIVE)

        val error = repository.recordLifeChange(session, "gp-7", 5).exceptionOrNull()

        assertEquals(409, (error as ApiError.Http).code)
    }

    @Test
    fun `recordCommanderDamage manda CommanderDamage con actor y target distintos`() = runTest {
        val session = RemoteGameSession("game-1", mapOf(0 to "gp-atacante", 1 to "gp-defensor"), GameStatus.ACTIVE)

        repository.recordCommanderDamage(session, "gp-atacante", "gp-defensor", 7).getOrThrow()

        val (gameId, request) = api.recordedActions.single()
        assertEquals("game-1", gameId)
        assertEquals("gp-atacante", request.actorId)
        assertEquals("gp-defensor", request.targetId)
        assertEquals(GameActionType.COMMANDER_DAMAGE, request.actionType)
        assertEquals(7, request.payload!!["amount"]!!.jsonPrimitive.int)
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
