package com.commandercompanion.presentation.screens.game

import androidx.lifecycle.SavedStateHandle
import com.commandercompanion.core.util.ApiFailure
import com.commandercompanion.data.repository.GameRepositoryImpl
import com.commandercompanion.data.repository.PlaygroupRepositoryImpl
import com.commandercompanion.data.session.AccessTokenProvider
import com.commandercompanion.domain.model.GameActionType
import com.commandercompanion.domain.model.GameSocketEvent
import com.commandercompanion.domain.model.GameStatus
import com.commandercompanion.domain.model.NewGameAction
import com.commandercompanion.domain.model.amountPayload
import com.commandercompanion.domain.usecase.ReplayCommanderDamageUseCase
import com.commandercompanion.domain.usecase.ResolveGameOutcomeUseCase
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeGameDao
import com.commandercompanion.testing.FakeGameSocketClient
import com.commandercompanion.testing.gameActionDto
import com.commandercompanion.testing.gameDto
import com.commandercompanion.testing.gamePlayerDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.playgroupDto
import com.commandercompanion.testing.playgroupMemberDto
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * Covers the mapping of network errors to UI state and the mirroring of actions against the
 * backend. The local tracker must keep working no matter what happens with the network.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class GameViewModelTest {

    private val dispatcher = StandardTestDispatcher()
    private val api = FakeCommanderApi()
    private val dao = FakeGameDao()
    private val socket = FakeGameSocketClient()
    private val accessTokenProvider = AccessTokenProvider { "access-token" }

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
        // By default, each join returns a different GamePlayer per user (so actor/target can be
        // distinguished when both seats end up assigned).
        api.onJoinGame = { gameId, request ->
            gamePlayerDto(id = "gp-${request.userId}", gameId = gameId, userId = request.userId!!, deckId = request.deckId)
        }
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    /** By default, Ana (seat 1) is assigned to a real user; Beto (seat 2) is a guest. */
    private fun viewModel(
        ana: PlayerConfig = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
        beto: PlayerConfig = PlayerConfig(name = "Beto", colorKey = "red")
    ): GameViewModel {
        val players = encodePlayerConfigs(listOf(ana, beto))
        val repository = GameRepositoryImpl(api, dao, socket)
        return GameViewModel(
            savedStateHandle = SavedStateHandle(
                mapOf(
                    "gameId" to "game-local",
                    "playersEncoded" to players,
                    "startingPlayerSeat" to 0
                )
            ),
            gameRepository = repository,
            playgroupRepository = PlaygroupRepositoryImpl(api),
            accessTokenProvider = accessTokenProvider,
            resolveGameOutcomeUseCase = ResolveGameOutcomeUseCase(),
            replayCommanderDamageUseCase = ReplayCommanderDamageUseCase()
        )
    }

    /** Joined mode: this device is NOT the one that created [gameId], it just joined it (`JoinGameScreen`). */
    private fun joinedViewModel(gameId: String = "game-1", localPlayerId: String = "gp-user-1"): GameViewModel {
        val repository = GameRepositoryImpl(api, dao, socket)
        return GameViewModel(
            savedStateHandle = SavedStateHandle(
                mapOf("gameId" to gameId, "localPlayerId" to localPlayerId)
            ),
            gameRepository = repository,
            playgroupRepository = PlaygroupRepositoryImpl(api),
            accessTokenProvider = accessTokenProvider,
            resolveGameOutcomeUseCase = ResolveGameOutcomeUseCase(),
            replayCommanderDamageUseCase = ReplayCommanderDamageUseCase()
        )
    }

    /**
     * A four-seat pass-and-play table of guests (no assigned user, so nothing is mirrored
     * remotely): the smallest table where the seating ring and the seat list disagree.
     */
    private fun fourSeatViewModel(): GameViewModel {
        val configs = listOf("Ana" to "blue", "Beto" to "red", "Carla" to "green", "Dani" to "white")
            .map { (name, colorKey) -> PlayerConfig(name = name, colorKey = colorKey) }
        val repository = GameRepositoryImpl(api, dao, socket)
        return GameViewModel(
            savedStateHandle = SavedStateHandle(
                mapOf(
                    "gameId" to "game-local",
                    "playersEncoded" to encodePlayerConfigs(configs),
                    "startingPlayerSeat" to 0
                )
            ),
            gameRepository = repository,
            playgroupRepository = PlaygroupRepositoryImpl(api),
            accessTokenProvider = accessTokenProvider,
            resolveGameOutcomeUseCase = ResolveGameOutcomeUseCase(),
            replayCommanderDamageUseCase = ReplayCommanderDamageUseCase()
        )
    }

    @Test
    fun `partida activa en el backend deja el estado en Synced`() = runTest(dispatcher) {
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Synced, vm.state.value.remoteSync.status)
    }

    /** Connecting any earlier would be pointless: `pending`-state transitions aren't broadcast (ADR-0005). */
    @Test
    fun `la partida activa conecta el socket de sincronizacion en vivo`() = runTest(dispatcher) {
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        viewModel()
        advanceUntilIdle()

        assertEquals(listOf("game-1"), socket.connectedGameIds)
    }

    @Test
    fun `sin quorum para iniciar queda esperando jugadores, no en error`() = runTest(dispatcher) {
        api.onStartGame = { throw httpException(409) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.WaitingForPlayers, vm.state.value.remoteSync.status)
        assertTrue(socket.connectedGameIds.isEmpty())
    }

    @Test
    fun `sin nadie asignado la sincronizacion queda deshabilitada`() =
        runTest(dispatcher) {
            val vm = viewModel(
                ana = PlayerConfig(name = "Ana", colorKey = "blue"),
                beto = PlayerConfig(name = "Beto", colorKey = "red")
            )
            advanceUntilIdle()

            // The wording the banner shows for this status lives in strings.xml
            // (`tracker_sync_local_only`); the status alone is what this asserts.
            assertEquals(RemoteSyncStatus.Disabled, vm.state.value.remoteSync.status)
            assertNull(vm.state.value.remoteSync.failure)
        }

    @Test
    fun `sin red el estado es Failed con el fallo de conexion`() = runTest(dispatcher) {
        api.onCreateGame = { throw IOException("sin red") }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Failed, vm.state.value.remoteSync.status)
        assertEquals(ApiFailure.Network, vm.state.value.remoteSync.failure)
    }

    @Test
    fun `sesion expirada se traduce al fallo de sesion caducada`() = runTest(dispatcher) {
        api.onCreateGame = { throw httpException(401) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Failed, vm.state.value.remoteSync.status)
        assertEquals(ApiFailure.SessionExpired, vm.state.value.remoteSync.failure)
    }

    @Test
    fun `el tracker local sigue funcionando aunque falle la red`() = runTest(dispatcher) {
        api.onCreateGame = { throw IOException("sin red") }

        val vm = viewModel()
        advanceUntilIdle()
        vm.adjustLife(playerId = 1, amount = -5)
        advanceUntilIdle()

        assertEquals(35, vm.state.value.players.first { it.id == 1 }.life)
        assertTrue(api.recordedActions.isEmpty())
    }

    @Test
    fun `el cambio de vida de un asiento asignado se espeja como LifeChange`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 1, amount = -4)
        advanceUntilIdle()

        val (_, request) = api.recordedActions.single()
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        assertEquals("gp-user-1", request.actorId)
    }

    /**
     * "Guest" seats (without assignedUserId) don't have their own `GamePlayer` in the backend:
     * sending their changes would corrupt another user's state and statistics.
     */
    @Test
    fun `el cambio de vida de un asiento invitado no se espeja`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 2, amount = -4)
        advanceUntilIdle()

        assertTrue(api.recordedActions.isEmpty())
        assertEquals(36, vm.state.value.players.first { it.id == 2 }.life)
    }

    @Test
    fun `dano de comandante entre dos asientos asignados se espeja con actor y target reales`() =
        runTest(dispatcher) {
            val vm = viewModel(
                ana = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
                beto = PlayerConfig(name = "Beto", colorKey = "red", assignedUserId = "user-2", deckId = "deck-2")
            )
            advanceUntilIdle()

            vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 5)
            advanceUntilIdle()

            val (_, request) = api.recordedActions.single()
            assertEquals(GameActionType.COMMANDER_DAMAGE, request.actionType)
            assertEquals("gp-user-2", request.actorId)
            assertEquals("gp-user-1", request.targetId)
        }

    /**
     * Resolves what used to be a known limitation: if the attacker is a guest seat
     * (without its own `GamePlayer`), there's no one to attribute the damage to in the backend.
     */
    @Test
    fun `dano de comandante no se espeja si el atacante es un asiento invitado`() = runTest(dispatcher) {
        val vm = viewModel() // Ana assigned, Beto is a guest
        advanceUntilIdle()

        vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 5)
        advanceUntilIdle()

        assertTrue(api.recordedActions.isEmpty())
    }

    @Test
    fun `finalizar la partida activa tambien la finaliza en el backend`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.finishGame(winnerId = 1)
        advanceUntilIdle()

        assertTrue(api.calls.contains("finishGame"))
        assertTrue(vm.state.value.isFinished)
        assertEquals("FINISHED", dao.finished.single().second)
    }

    /**
     * If the finishing blow and the end of the game went out as separate loose coroutines, the
     * `finish` could race ahead of the `LifeChange` and the backend would reject the action with 409.
     */
    @Test
    fun `el golpe letal se registra antes de finalizar la partida en el backend`() =
        runTest(dispatcher) {
            val vm = viewModel()
            advanceUntilIdle()

            // Brings the assigned seat down to 0 life: triggers the automatic end of the game.
            vm.adjustLife(playerId = 1, amount = -40)
            advanceUntilIdle()

            val remoteCalls = api.calls.filter { it == "recordAction" || it == "finishGame" }
            assertEquals(listOf("recordAction", "finishGame"), remoteCalls)
        }

    /** Commander rule: 21+ commander damage from the same attacker eliminates, even if life is still positive. */
    @Test
    fun `21 damage from the same commander ends the game even if life stays positive`() =
        runTest(dispatcher) {
            val vm = viewModel()
            advanceUntilIdle()

            vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 21)
            advanceUntilIdle()

            assertTrue(vm.state.value.isFinished)
            assertEquals(2, vm.state.value.winnerId)
            assertTrue(vm.state.value.players.first { it.id == 1 }.life > 0)
        }

    @Test
    fun `20 commander damage doesn't eliminate yet`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 20)
        advanceUntilIdle()

        assertTrue(!vm.state.value.isFinished)
    }

    @Test
    fun `el turno inicial coincide con el jugador inicial sorteado en la pregame`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(1, vm.state.value.currentTurnPlayerId)
    }

    @Test
    fun `pasar turno avanza al siguiente asiento y envuelve al llegar al ultimo`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.nextTurn()
        assertEquals(2, vm.state.value.currentTurnPlayerId)
        assertEquals(2, vm.state.value.currentTurn)

        vm.nextTurn()
        assertEquals(1, vm.state.value.currentTurnPlayerId)
        assertEquals(3, vm.state.value.currentTurn)
    }

    @Test
    fun `reiniciar vidas restaura vida veneno y dano de comandante manteniendo asientos y turno inicial`() =
        runTest(dispatcher) {
            val vm = viewModel(
                ana = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
                beto = PlayerConfig(name = "Beto", colorKey = "red", assignedUserId = "user-2", deckId = "deck-2")
            )
            advanceUntilIdle()

            vm.adjustLife(playerId = 1, amount = -10)
            vm.adjustPoison(playerId = 1, amount = 3)
            vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 5)
            vm.nextTurn()
            advanceUntilIdle()

            vm.resetLives()
            advanceUntilIdle()

            val ana = vm.state.value.players.first { it.id == 1 }
            assertEquals(STARTING_LIFE, ana.life)
            assertEquals(0, ana.poison)
            assertTrue(ana.commanderDamage.isEmpty())
            assertEquals(1, vm.state.value.currentTurn)
            assertEquals(vm.state.value.startingPlayerId, vm.state.value.currentTurnPlayerId)
            assertEquals(listOf("Ana", "Beto"), vm.state.value.players.map { it.name })
        }

    @Test
    fun `reiniciar vidas espeja los deltas de vuelta al backend para un asiento asignado`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 1, amount = -10)
        advanceUntilIdle()
        api.recordedActions.clear()

        vm.resetLives()
        advanceUntilIdle()

        val (_, request) = api.recordedActions.single()
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        assertEquals("gp-user-1", request.actorId)
    }

    @Test
    fun `no se llama a finish remoto si la partida nunca llego a activa`() = runTest(dispatcher) {
        api.onStartGame = { throw httpException(409) }

        val vm = viewModel()
        advanceUntilIdle()
        vm.finishGame(winnerId = 1)
        advanceUntilIdle()

        assertTrue(!api.calls.contains("finishGame"))
        // The local result is still saved regardless.
        assertEquals("FINISHED", dao.finished.single().second)
    }

    // ------------------------------------------------------- joined mode (JoinGameScreen)

    private fun givenTwoSeatGame(status: String = GameStatus.ACTIVE) {
        api.onGetGame = { id ->
            gameDto(
                id = id,
                status = status,
                playgroupId = "pg-1",
                players = listOf(
                    gamePlayerDto(id = "gp-user-1", gameId = id, userId = "user-1", deckId = "deck-1"),
                    gamePlayerDto(id = "gp-user-2", gameId = id, userId = "user-2", deckId = "deck-2")
                )
            )
        }
        api.onGetPlaygroup = { id ->
            playgroupDto(
                id = id,
                members = listOf(
                    playgroupMemberDto(playgroupId = id, userId = "user-1", username = "Ana"),
                    playgroupMemberDto(playgroupId = id, userId = "user-2", username = "Beto")
                )
            )
        }
    }

    @Test
    fun `unirse a una partida existente carga el resto de la mesa`() = runTest(dispatcher) {
        givenTwoSeatGame()

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()

        assertEquals(2, vm.state.value.players.size)
        assertEquals("Ana", vm.state.value.players.first { it.id == 1 }.name)
        assertEquals("Beto", vm.state.value.players.first { it.id == 2 }.name)
        assertEquals(1, vm.state.value.localSeatId)
        assertEquals(RemoteSyncStatus.Synced, vm.state.value.remoteSync.status)
        assertEquals(listOf("game-1"), socket.connectedGameIds)
    }

    @Test
    fun `una partida unida que sigue pendiente no conecta el socket`() = runTest(dispatcher) {
        givenTwoSeatGame(status = GameStatus.PENDING)

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.WaitingForPlayers, vm.state.value.remoteSync.status)
        assertTrue(socket.connectedGameIds.isEmpty())
    }

    @Test
    fun `una accion de vida de otro asiento se refleja en el estado local`() = runTest(dispatcher) {
        givenTwoSeatGame()
        val events = MutableSharedFlow<GameSocketEvent>(extraBufferCapacity = 1)
        socket.onConnect = { events }

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()
        val betoInitialLife = vm.state.value.players.first { it.id == 2 }.life

        events.tryEmit(
            GameSocketEvent.ActionReceived(
                gameActionDto(
                    "game-1",
                    NewGameAction(actorId = "gp-user-2", actionType = GameActionType.LIFE_CHANGE, payload = amountPayload(-3))
                )
            )
        )
        advanceUntilIdle()

        assertEquals(betoInitialLife - 3, vm.state.value.players.first { it.id == 2 }.life)
    }

    /** Otherwise the device would double-count a change it already applied synchronously before mirroring it. */
    @Test
    fun `una accion recibida del propio asiento no se vuelve a aplicar`() = runTest(dispatcher) {
        givenTwoSeatGame()
        val events = MutableSharedFlow<GameSocketEvent>(extraBufferCapacity = 1)
        socket.onConnect = { events }

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()
        val ownInitialLife = vm.state.value.players.first { it.id == 1 }.life

        events.tryEmit(
            GameSocketEvent.ActionReceived(
                gameActionDto(
                    "game-1",
                    NewGameAction(actorId = "gp-user-1", actionType = GameActionType.LIFE_CHANGE, payload = amountPayload(-10))
                )
            )
        )
        advanceUntilIdle()

        assertEquals(ownInitialLife, vm.state.value.players.first { it.id == 1 }.life)
    }

    @Test
    fun `el dano de comandante ya registrado se reconstruye desde el timeline al unirse`() = runTest(dispatcher) {
        givenTwoSeatGame()
        api.onGetTimeline = {
            listOf(
                gameActionDto(
                    "game-1",
                    NewGameAction(actorId = "gp-user-2", targetId = "gp-user-1", actionType = GameActionType.COMMANDER_DAMAGE, payload = amountPayload(7))
                )
            )
        }

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()

        assertEquals(7, vm.state.value.players.first { it.id == 1 }.commanderDamage[2])
    }

    @Test
    fun `si no se puede cargar la partida unida el estado queda en Failed`() = runTest(dispatcher) {
        api.onGetGame = { throw httpException(404) }

        val vm = joinedViewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Failed, vm.state.value.remoteSync.status)
        assertTrue(vm.state.value.players.isEmpty())
        assertNull(vm.state.value.localSeatId)
    }

    @Test
    fun `the turn goes around the table clockwise, not down the seat list`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()

        // Seats 1 and 2 sit on the top row, 3 and 4 below them: seat 3 is under seat 1, so the
        // ring is 1 -> 2 -> 4 -> 3 and not the 1 -> 2 -> 3 -> 4 the list order used to give.
        val visited = (1..4).map {
            vm.nextTurn()
            vm.state.value.currentTurnPlayerId
        }

        assertEquals(listOf(2, 4, 3, 1), visited)
        assertEquals(5, vm.state.value.currentTurn)
    }

    @Test
    fun `an eliminated seat is skipped instead of being handed the turn`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()
        vm.adjustLife(playerId = 2, amount = -STARTING_LIFE)

        vm.nextTurn()

        // Seat 2 is next around the ring but dead, so the turn moves on to seat 4.
        assertEquals(4, vm.state.value.currentTurnPlayerId)
        assertEquals(2, vm.state.value.currentTurn)
    }

    @Test
    fun `a finished game does not keep passing the turn`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()
        vm.finishGame(winnerId = 1)

        vm.nextTurn()

        assertEquals(1, vm.state.value.currentTurnPlayerId)
        assertEquals(1, vm.state.value.currentTurn)
    }
}
