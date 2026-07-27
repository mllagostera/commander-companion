package com.commandercompanion.presentation.screens.game

import androidx.lifecycle.SavedStateHandle
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.GameStatus
import com.commandercompanion.data.repository.DeckRepository
import com.commandercompanion.data.repository.GameRepository
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeGameDao
import com.commandercompanion.testing.gameDto
import com.commandercompanion.testing.httpException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.IOException

/**
 * Cubre el mapeo de errores de red al estado de UI y el espejo de acciones contra el backend.
 * El tracker local debe seguir funcionando pase lo que pase con la red.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class GameViewModelTest {

    private val dispatcher = StandardTestDispatcher()
    private val api = FakeCommanderApi()
    private val dao = FakeGameDao()

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    private fun viewModel(): GameViewModel {
        val players = encodePlayerConfigs(
            listOf(
                PlayerConfig(name = "Ana", colorKey = "blue"),
                PlayerConfig(name = "Beto", colorKey = "red")
            )
        )
        val repository = GameRepository(api, dao, DeckRepository(api))
        return GameViewModel(
            savedStateHandle = SavedStateHandle(
                mapOf(
                    "gameId" to "game-local",
                    "playersEncoded" to players,
                    "startingPlayerSeat" to 0
                )
            ),
            gameRepository = repository
        )
    }

    @Test
    fun `partida activa en el backend deja el estado en Synced`() = runTest(dispatcher) {
        api.onStartGame = { id -> gameDto(id, GameStatus.ACTIVE) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Synced, vm.state.value.remoteSync.status)
    }

    @Test
    fun `sin quorum para iniciar queda esperando jugadores, no en error`() = runTest(dispatcher) {
        api.onStartGame = { throw httpException(409) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.WaitingForPlayers, vm.state.value.remoteSync.status)
    }

    @Test
    fun `sin decks la sincronizacion queda deshabilitada y se explica por que`() =
        runTest(dispatcher) {
            api.onListDecks = { emptyList() }

            val vm = viewModel()
            advanceUntilIdle()

            assertEquals(RemoteSyncStatus.Disabled, vm.state.value.remoteSync.status)
            assertTrue(vm.state.value.remoteSync.message!!.contains("solo en este dispositivo"))
        }

    @Test
    fun `sin red el estado es Failed con el mensaje de conexion`() = runTest(dispatcher) {
        api.onListDecks = { throw IOException("sin red") }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Failed, vm.state.value.remoteSync.status)
        assertEquals("No se pudo conectar con el servidor", vm.state.value.remoteSync.message)
    }

    @Test
    fun `sesion expirada se traduce a un mensaje de re-login`() = runTest(dispatcher) {
        api.onListDecks = { throw httpException(401) }

        val vm = viewModel()
        advanceUntilIdle()

        assertEquals(RemoteSyncStatus.Failed, vm.state.value.remoteSync.status)
        assertEquals("Tu sesión expiró, iniciá sesión de nuevo", vm.state.value.remoteSync.message)
    }

    @Test
    fun `el tracker local sigue funcionando aunque falle la red`() = runTest(dispatcher) {
        api.onListDecks = { throw IOException("sin red") }

        val vm = viewModel()
        advanceUntilIdle()
        vm.adjustLife(playerId = 1, amount = -5)
        advanceUntilIdle()

        assertEquals(35, vm.state.value.players.first { it.id == 1 }.life)
        assertTrue(api.recordedActions.isEmpty())
    }

    @Test
    fun `el cambio de vida del asiento local se espeja como LifeChange`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 1, amount = -4)
        advanceUntilIdle()

        val (_, request) = api.recordedActions.single()
        assertEquals(GameActionType.LIFE_CHANGE, request.actionType)
    }

    /**
     * Los demás asientos son jugadores locales sin `GamePlayer` propio en el backend: mandar sus
     * cambios con el actor local corrompería el estado y las estadísticas del servidor.
     */
    @Test
    fun `el cambio de vida de otro asiento no se espeja`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        vm.adjustLife(playerId = 2, amount = -4)
        advanceUntilIdle()

        assertTrue(api.recordedActions.isEmpty())
        assertEquals(36, vm.state.value.players.first { it.id == 2 }.life)
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
     * Si el golpe de gracia y el fin de partida salieran como corrutinas sueltas, el `finish`
     * podía adelantarse al `LifeChange` y el backend rechazaba la acción con 409.
     */
    @Test
    fun `el golpe letal se registra antes de finalizar la partida en el backend`() =
        runTest(dispatcher) {
            val vm = viewModel()
            advanceUntilIdle()

            // Deja al asiento local en 0 de vida: dispara el fin automático de partida.
            vm.adjustLife(playerId = 1, amount = -40)
            advanceUntilIdle()

            val remoteCalls = api.calls.filter { it == "recordAction" || it == "finishGame" }
            assertEquals(listOf("recordAction", "finishGame"), remoteCalls)
        }

    @Test
    fun `no se llama a finish remoto si la partida nunca llego a activa`() = runTest(dispatcher) {
        api.onStartGame = { throw httpException(409) }

        val vm = viewModel()
        advanceUntilIdle()
        vm.finishGame(winnerId = 1)
        advanceUntilIdle()

        assertTrue(!api.calls.contains("finishGame"))
        // El resultado local sí se guarda igual.
        assertEquals("FINISHED", dao.finished.single().second)
    }
}
