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
import com.commandercompanion.domain.model.turnDurationPayload
import com.commandercompanion.domain.usecase.ReplayCommanderDamageUseCase
import com.commandercompanion.domain.usecase.ResolveGameOutcomeUseCase
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeGameDao
import com.commandercompanion.testing.FakeGameSocketClient
import com.commandercompanion.testing.deckDto
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
     * A guest attacker has no `GamePlayer` to credit the commander damage to, but the assigned
     * defender's life still has to go down on the backend: it's mirrored as a plain life loss.
     */
    @Test
    fun `commander damage from a guest seat only mirrors the defender's life loss`() = runTest(dispatcher) {
        val vm = viewModel() // Ana assigned, Beto is a guest
        advanceUntilIdle()

        vm.adjustCommanderDamage(targetPlayerId = 1, attackerId = 2, amount = 5)
        advanceUntilIdle()

        val (_, request) = api.recordedActions.single()
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        assertEquals("gp-user-1", request.actorId)
        assertEquals(amountPayload(-5), request.payload)
    }

    /** A `LifeChange` on the target would leave the backend with nobody to credit the damage to. */
    @Test
    fun `plain damage between two assigned seats is mirrored as CombatDamage from the attacker`() =
        runTest(dispatcher) {
            val vm = viewModel(
                ana = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
                beto = PlayerConfig(name = "Beto", colorKey = "red", assignedUserId = "user-2", deckId = "deck-2")
            )
            advanceUntilIdle()

            vm.dealDamage(sourceId = 2, targetId = 1, amount = 7, commander = false)
            advanceUntilIdle()

            val (_, request) = api.recordedActions.single()
            assertEquals(GameActionType.COMBAT_DAMAGE, request.actionType)
            assertEquals("gp-user-2", request.actorId)
            assertEquals("gp-user-1", request.targetId)
            assertEquals(amountPayload(7), request.payload)
        }

    @Test
    fun `plain damage from a guest seat still mirrors the target's life loss`() = runTest(dispatcher) {
        val vm = viewModel() // Ana assigned, Beto is a guest
        advanceUntilIdle()

        vm.dealDamage(sourceId = 2, targetId = 1, amount = 7, commander = false)
        advanceUntilIdle()

        val (_, request) = api.recordedActions.single()
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
        assertEquals("gp-user-1", request.actorId)
        assertEquals(amountPayload(-7), request.payload)
    }

    @Test
    fun `a lethal hit credits the elimination to the attacker before finishing the game`() =
        runTest(dispatcher) {
            val vm = viewModel(
                ana = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
                beto = PlayerConfig(name = "Beto", colorKey = "red", assignedUserId = "user-2", deckId = "deck-2")
            )
            advanceUntilIdle()

            vm.dealDamage(sourceId = 1, targetId = 2, amount = STARTING_LIFE, commander = false)
            vm.confirmFinish()
            advanceUntilIdle()

            val types = api.recordedActions.map { (_, request) -> request.actionType }
            assertEquals(listOf(GameActionType.COMBAT_DAMAGE, GameActionType.ELIMINATION), types)
            val elimination = api.recordedActions.last().second
            assertEquals("gp-user-1", elimination.actorId)
            assertEquals("gp-user-2", elimination.targetId)
            val remoteCalls = api.calls.filter { it == "recordAction" || it == "finishGame" }
            assertEquals(listOf("recordAction", "recordAction", "finishGame"), remoteCalls)
        }

    @Test
    fun `each finished turn counts towards its seat's longest and average, pauses excluded`() =
        runTest(dispatcher) {
            val vm = fourSeatViewModel()
            advanceUntilIdle()
            var now = 0L
            vm.nowMs = { now }

            vm.startTurnClock() // Ana
            now += 30_000
            vm.pauseTurnClock()
            now += 600_000 // a long pause doesn't make the turn longer
            vm.resumeTurnClock()
            now += 20_000
            vm.nextTurn() // Ana's turn: 50s
            now += 70_000
            vm.nextTurn() // next seat: 70s
            advanceUntilIdle()

            val ana = vm.state.value.players.first { it.id == 1 }
            assertEquals(1, ana.turnsTaken)
            assertEquals(50_000L, ana.longestTurnMs)
            val second = vm.state.value.players.first { it.turnsTaken == 1 && it.id != 1 }
            assertEquals(70_000L, second.longestTurnMs)
            assertEquals(0L, vm.state.value.currentTurnMs)
        }

    @Test
    fun `finishing the game closes the turn in progress`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()
        var now = 0L
        vm.nowMs = { now }

        vm.startTurnClock()
        now += 40_000
        vm.nextTurn()
        now += 20_000
        vm.nextTurn()
        now += 60_000
        vm.nextTurn()
        now += 10_000
        vm.nextTurn() // back to Ana
        now += 80_000
        vm.finishGame(winnerId = 1)

        val ana = vm.state.value.players.first { it.id == 1 }
        assertEquals(2, ana.turnsTaken)
        assertEquals(80_000L, ana.longestTurnMs)
        assertEquals(60_000L, ana.averageTurnMs()) // (40s + 80s) / 2
    }

    @Test
    fun `an assigned seat's turn end is mirrored with its duration before finishing`() = runTest(dispatcher) {
        val vm = viewModel() // Ana (seat 1, starts) assigned, Beto is a guest
        advanceUntilIdle()
        var now = 0L
        vm.nowMs = { now }

        vm.startTurnClock()
        now += 45_000
        vm.nextTurn() // Ana's 45s turn ends; Beto (guest) sends nothing
        now += 15_000
        vm.nextTurn()
        now += 5_000
        vm.finishGame(winnerId = 1)
        vm.confirmFinish()
        advanceUntilIdle()

        val turnEnds = api.recordedActions.map { it.second }.filter { it.actionType == GameActionType.TURN_END }
        assertEquals(listOf(turnDurationPayload(45_000), turnDurationPayload(5_000)), turnEnds.map { it.payload })
        assertTrue(turnEnds.all { it.actorId == "gp-user-1" })
        assertEquals("finishGame", api.calls.last { it == "recordAction" || it == "finishGame" })
    }

    @Test
    fun `every turn start of an assigned seat is recorded, guests' are not`() = runTest(dispatcher) {
        val vm = viewModel() // Ana (seat 1, starts) assigned, Beto is a guest
        advanceUntilIdle()

        vm.startTurnClock()
        vm.nextTurn() // Beto
        vm.nextTurn() // Ana again
        advanceUntilIdle()

        val turnStarts = api.recordedActions.map { it.second }.filter { it.actionType == GameActionType.TURN_START }
        assertEquals(listOf("gp-user-1", "gp-user-1"), turnStarts.map { it.actorId })
    }

    @Test
    fun `finalizar la partida activa tambien la finaliza en el backend`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.finishGame(winnerId = 1)
        advanceUntilIdle()
        // Only on this device until the player leaves the summary.
        assertTrue(vm.state.value.isFinished)
        assertTrue(!api.calls.contains("finishGame"))

        vm.confirmFinish()
        advanceUntilIdle()

        assertTrue(api.calls.contains("finishGame"))
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
            vm.confirmFinish()
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
    fun `dealDamage without commander only costs the target life`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 2, targetId = 1, amount = 5, commander = false)
        advanceUntilIdle()

        val target = vm.state.value.players.first { it.id == 1 }
        assertEquals(STARTING_LIFE - 5, target.life)
        assertTrue(target.commanderDamage.isEmpty())
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 2 }.life)
    }

    @Test
    fun `dealDamage as commander damage costs life and is recorded against the source`() =
        runTest(dispatcher) {
            val vm = viewModel()
            advanceUntilIdle()

            vm.dealDamage(sourceId = 2, targetId = 1, amount = 6, commander = true)
            advanceUntilIdle()

            val target = vm.state.value.players.first { it.id == 1 }
            assertEquals(STARTING_LIFE - 6, target.life)
            assertEquals(mapOf(2 to 6), target.commanderDamage)
        }

    @Test
    fun `dealDamage ignores a seat damaging itself and non-positive amounts`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        val before = vm.state.value.players

        vm.dealDamage(sourceId = 1, targetId = 1, amount = 5, commander = true)
        vm.dealDamage(sourceId = 2, targetId = 1, amount = 0, commander = false)
        vm.dealDamage(sourceId = 2, targetId = 1, amount = -3, commander = false)
        advanceUntilIdle()

        assertEquals(before, vm.state.value.players)
    }

    @Test
    fun `gainLife adds life to that seat only and ignores non-positive amounts`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.gainLife(playerId = 1, amount = 3)
        vm.gainLife(playerId = 1, amount = 0)
        vm.gainLife(playerId = 1, amount = -2)
        advanceUntilIdle()

        assertEquals(STARTING_LIFE + 3, vm.state.value.players.first { it.id == 1 }.life)
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 2 }.life)
    }

    @Test
    fun `summary damage totals count plain and commander damage for both seats`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 1, targetId = 2, amount = 5, commander = false)
        vm.dealDamage(sourceId = 1, targetId = 3, amount = 4, commander = true)
        vm.dealDamage(sourceId = 2, targetId = 1, amount = 3, commander = false)
        vm.gainLife(playerId = 2, amount = 10)
        advanceUntilIdle()

        val players = vm.state.value.players.associateBy { it.id }
        assertEquals(9, players.getValue(1).damageDealt)
        assertEquals(3, players.getValue(1).damageTaken)
        assertEquals(3, players.getValue(2).damageDealt)
        assertEquals(5, players.getValue(2).damageTaken)
        assertEquals(0, players.getValue(3).damageDealt)
        assertEquals(4, players.getValue(3).damageTaken)
    }

    @Test
    fun `a commander grid correction takes damage back off the totals`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()

        vm.adjustCommanderDamage(targetPlayerId = 2, attackerId = 1, amount = 1)
        vm.adjustCommanderDamage(targetPlayerId = 2, attackerId = 1, amount = 1)
        vm.adjustCommanderDamage(targetPlayerId = 2, attackerId = 1, amount = 1)
        vm.undoCommanderDamage(targetPlayerId = 2, attackerId = 1)
        advanceUntilIdle()

        val players = vm.state.value.players.associateBy { it.id }
        assertEquals(2, players.getValue(1).damageDealt)
        assertEquals(2, players.getValue(2).damageTaken)
        assertEquals(mapOf(1 to 2), players.getValue(2).commanderDamage)
        assertEquals(STARTING_LIFE - 2, players.getValue(2).life)
    }

    @Test
    fun `the lethal hit is already counted in the finished game's totals`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 2, targetId = 1, amount = STARTING_LIFE, commander = false)
        advanceUntilIdle()

        assertTrue(vm.state.value.isFinished)
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 2 }.damageDealt)
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 1 }.damageTaken)
    }

    @Test
    fun `the turn clock only counts each seat's own turns`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        var now = 1_000L
        vm.nowMs = { now }

        // Nothing runs until the first turn starts.
        now += 5_000
        assertEquals(0L, vm.turnTimeOf(1))

        vm.startTurnClock()
        val first = vm.state.value.currentTurnPlayerId!!
        val second = if (first == 1) 2 else 1
        now += 30_000
        assertEquals(30_000L, vm.turnTimeOf(first))
        assertEquals(0L, vm.turnTimeOf(second))

        vm.nextTurn()
        now += 12_000
        assertEquals(30_000L, vm.turnTimeOf(first))
        assertEquals(12_000L, vm.turnTimeOf(second))

        vm.nextTurn()
        now += 4_000
        assertEquals(34_000L, vm.turnTimeOf(first))
        assertEquals(12_000L, vm.turnTimeOf(second))
    }

    @Test
    fun `turn clock reads as minutes and seconds, adding hours past the hour`() {
        assertEquals("0:00", formatTurnClock(0))
        assertEquals("0:59", formatTurnClock(59_999))
        assertEquals("12:05", formatTurnClock(725_000))
        assertEquals("1:00:07", formatTurnClock(3_607_000))
    }

    @Test
    fun `a paused game stops the turn clock`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        var now = 0L
        vm.nowMs = { now }
        vm.startTurnClock()
        val first = vm.state.value.currentTurnPlayerId!!

        now += 10_000
        vm.pauseTurnClock()
        now += 60_000
        assertEquals(10_000L, vm.turnTimeOf(first))

        vm.resumeTurnClock()
        now += 5_000
        assertEquals(15_000L, vm.turnTimeOf(first))
    }

    @Test
    fun `resetting lives also resets the turn clocks`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        var now = 0L
        vm.nowMs = { now }
        vm.startTurnClock()
        now += 20_000
        vm.nextTurn()
        now += 20_000

        vm.resetLives()

        assertEquals(0L, vm.turnTimeOf(1))
        assertEquals(0L, vm.turnTimeOf(2))
        now += 3_000
        assertEquals(3_000L, vm.turnTimeOf(vm.state.value.currentTurnPlayerId!!))
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
    fun `resetting lives undoes every action this device recorded on the backend`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 1, amount = -10) // action-1
        vm.adjustPoison(playerId = 1, amount = 2) // action-2
        advanceUntilIdle()

        vm.resetLives()
        advanceUntilIdle()

        assertEquals(listOf(listOf("action-2", "action-1")), api.undoneActionIds)
        assertTrue(api.recordedActions.none { (_, request) -> request.payload == amountPayload(10) })
        assertTrue(!vm.state.value.canUndo)
    }

    @Test
    fun `no se llama a finish remoto si la partida nunca llego a activa`() = runTest(dispatcher) {
        api.onStartGame = { throw httpException(409) }

        val vm = viewModel()
        advanceUntilIdle()
        vm.finishGame(winnerId = 1)
        vm.confirmFinish()
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

    @Test
    fun `a seat with a chosen deck carries its art into the tracker`() = runTest(dispatcher) {
        val vm = viewModel(
            ana = PlayerConfig(
                name = "Ana",
                colorKey = "blue",
                assignedUserId = "user-1",
                deckId = "deck-1",
                deckImageUrl = "https://art/atraxa.jpg"
            )
        )
        advanceUntilIdle()

        assertEquals("https://art/atraxa.jpg", vm.state.value.players.first { it.id == 1 }.deckImageUrl)
        // Beto is a guest: no deck, no art, the seat keeps its flat colour.
        assertNull(vm.state.value.players.first { it.id == 2 }.deckImageUrl)
    }

    @Test
    fun `joined mode resolves each seat's deck art through the playgroup`() = runTest(dispatcher) {
        givenTwoSeatGame()
        api.onGetMemberDecks = { _, userId ->
            when (userId) {
                "user-1" -> listOf(deckDto(id = "deck-1", imageUrl = "https://art/atraxa.jpg"))
                else -> listOf(deckDto(id = "deck-2", name = "Krenko", imageUrl = null))
            }
        }

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()

        assertEquals("https://art/atraxa.jpg", vm.state.value.players.first { it.id == 1 }.deckImageUrl)
        // A deck with no art (not imported from Moxfield) leaves its seat as it was.
        assertNull(vm.state.value.players.first { it.id == 2 }.deckImageUrl)
    }

    @Test
    fun `a failing deck lookup leaves the joined table on screen without art`() = runTest(dispatcher) {
        givenTwoSeatGame()
        api.onGetMemberDecks = { _, _ -> throw httpException(500) }

        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()

        assertEquals(2, vm.state.value.players.size)
        assertTrue(vm.state.value.players.all { it.deckImageUrl == null })
    }

    // ------------------------------------------------------------------ undo

    private fun twoAssignedSeats() = viewModel(
        ana = PlayerConfig(name = "Ana", colorKey = "blue", assignedUserId = "user-1", deckId = "deck-1"),
        beto = PlayerConfig(name = "Beto", colorKey = "red", assignedUserId = "user-2", deckId = "deck-2")
    )

    @Test
    fun `undo reverts a damage drag locally and on the backend`() = runTest(dispatcher) {
        val vm = twoAssignedSeats()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 2, targetId = 1, amount = 7, commander = false) // action-1
        advanceUntilIdle()
        assertTrue(vm.state.value.canUndo)

        vm.undo()
        advanceUntilIdle()

        val ana = vm.state.value.players.first { it.id == 1 }
        assertEquals(STARTING_LIFE, ana.life)
        assertEquals(0, ana.damageTaken)
        assertEquals(0, vm.state.value.players.first { it.id == 2 }.damageDealt)
        assertEquals(listOf(listOf("action-1")), api.undoneActionIds)
        assertTrue(!vm.state.value.canUndo)
        assertEquals(UndoLabel.Damage("Beto", "Ana", 7, commander = false), vm.state.value.lastUndone?.label)
    }

    @Test
    fun `undoing the lethal hit from the summary reopens the game without finishing it remotely`() =
        runTest(dispatcher) {
            val vm = twoAssignedSeats()
            advanceUntilIdle()
            var now = 0L
            vm.nowMs = { now }
            vm.startTurnClock() // action-1: Ana's TurnStart
            now += 30_000

            vm.dealDamage(sourceId = 1, targetId = 2, amount = STARTING_LIFE, commander = false) // action-2, action-3
            advanceUntilIdle()
            assertTrue(vm.state.value.isFinished)
            now += 60_000 // time on the summary screen doesn't count

            vm.undo()
            advanceUntilIdle()

            val state = vm.state.value
            assertTrue(!state.isFinished)
            assertNull(state.winnerId)
            assertEquals(STARTING_LIFE, state.players.first { it.id == 2 }.life)
            assertEquals(0, state.players.first { it.id == 1 }.turnsTaken) // the last turn goes on
            assertEquals(30_000L, vm.turnTimeOf(1))
            // Elimination first, then the hit -- most recent first.
            assertEquals(listOf(listOf("action-3", "action-2")), api.undoneActionIds)
            assertTrue(!api.calls.contains("finishGame"))
            assertTrue(dao.finished.isEmpty())
        }

    @Test
    fun `confirming the finish makes it final`() = runTest(dispatcher) {
        val vm = twoAssignedSeats()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 1, targetId = 2, amount = STARTING_LIFE, commander = false)
        vm.confirmFinish()
        advanceUntilIdle()

        assertTrue(!vm.state.value.canUndo)
        vm.undo()
        advanceUntilIdle()
        assertTrue(vm.state.value.isFinished)
        assertTrue(api.undoneActionIds.isEmpty())
        assertTrue(api.calls.contains("finishGame"))
    }

    @Test
    fun `undoing a pass hands the turn back with the time spent since`() = runTest(dispatcher) {
        val vm = twoAssignedSeats()
        advanceUntilIdle()
        var now = 0L
        vm.nowMs = { now }

        vm.startTurnClock() // Ana; action-1 TurnStart
        now += 20_000
        vm.nextTurn() // action-2 Ana's TurnEnd, action-3 Beto's TurnStart
        now += 5_000 // Beto "plays" 5s before anyone notices the mistake
        vm.undo()
        advanceUntilIdle()

        val state = vm.state.value
        assertEquals(1, state.currentTurn)
        assertEquals(1, state.currentTurnPlayerId)
        assertEquals(0, state.players.first { it.id == 1 }.turnsTaken)
        assertEquals(25_000L, vm.turnTimeOf(1))
        assertEquals(0L, vm.turnTimeOf(2))
        assertEquals(listOf(listOf("action-3", "action-2")), api.undoneActionIds)
        assertEquals(UndoLabel.PassTurn("Ana"), state.lastUndone?.label)
    }

    @Test
    fun `undo goes back one step at a time, most recent first`() = runTest(dispatcher) {
        val vm = fourSeatViewModel()
        advanceUntilIdle()

        vm.dealDamage(sourceId = 1, targetId = 2, amount = 3, commander = false)
        vm.adjustPoison(playerId = 3, amount = 1)
        vm.gainLife(playerId = 4, amount = 2)

        vm.undo()
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 4 }.life)
        assertEquals(1, vm.state.value.players.first { it.id == 3 }.poison)
        vm.undo()
        assertEquals(0, vm.state.value.players.first { it.id == 3 }.poison)
        assertEquals(STARTING_LIFE - 3, vm.state.value.players.first { it.id == 2 }.life)
        vm.undo()
        assertEquals(STARTING_LIFE, vm.state.value.players.first { it.id == 2 }.life)
        assertTrue(!vm.state.value.canUndo)
    }

    @Test
    fun `an undo broadcast from another seat reverts its change here`() = runTest(dispatcher) {
        givenTwoSeatGame()
        val events = MutableSharedFlow<GameSocketEvent>(extraBufferCapacity = 2)
        socket.onConnect = { events }
        val vm = joinedViewModel(localPlayerId = "gp-user-1")
        advanceUntilIdle()
        val lifeChange = gameActionDto(
            "game-1",
            NewGameAction(actorId = "gp-user-2", actionType = GameActionType.LIFE_CHANGE, payload = amountPayload(-3))
        )

        events.tryEmit(GameSocketEvent.ActionReceived(lifeChange))
        advanceUntilIdle()
        events.tryEmit(GameSocketEvent.ActionUndone(lifeChange))
        advanceUntilIdle()

        val beto = vm.state.value.players.first { it.id == 2 }
        assertEquals(STARTING_LIFE, beto.life)
        assertEquals(0, beto.damageTaken)
    }
}
