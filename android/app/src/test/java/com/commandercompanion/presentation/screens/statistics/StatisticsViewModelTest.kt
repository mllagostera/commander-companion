package com.commandercompanion.presentation.screens.statistics

import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.data.repository.DeckRepositoryImpl
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.domain.model.DeckWithStats
import com.commandercompanion.domain.usecase.LoadStatisticsUseCase
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeDeckDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.opponentStatsDto
import com.commandercompanion.testing.playgroupGameCountDto
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class StatisticsViewModelTest {

    private val dispatcher = StandardTestDispatcher()
    private val api = FakeCommanderApi()

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    private fun viewModel(): StatisticsViewModel {
        val statisticsRepository = StatisticsRepositoryImpl(api)
        val deckRepository = DeckRepositoryImpl(api, FakeDeckDao())
        val loadStatisticsUseCase = LoadStatisticsUseCase(statisticsRepository, deckRepository)
        return StatisticsViewModel(loadStatisticsUseCase)
    }

    @Test
    fun `carga las estadisticas globales, por deck y por grupo`() = runTest {
        api.onGetUserStats = { UserStatsDto(userId = "user-1", gamesPlayed = 10, gamesWon = 4) }
        api.onListDecks = { PagedResponse(items = listOf(deckDto("deck-a"), deckDto("deck-b"))) }
        api.onGetDeckStats = { id -> DeckStatsDto(deckId = id, gamesPlayed = 5, gamesWon = 2) }
        api.onListPlaygroupGameCounts = { listOf(playgroupGameCountDto(playgroupId = "group-1", gamesPlayed = 3)) }
        api.onGetOpponentStats = { listOf(opponentStatsDto(userId = "user-2", gamesTogether = 2)) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertEquals(10, state.userStats?.gamesPlayed)
        assertEquals(4, state.userStats?.gamesWon)
        assertEquals(listOf("deck-a", "deck-b"), state.deckStats.map { it.deck.id })
        assertEquals(listOf(5, 5), state.deckStats.map { it.stats?.gamesPlayed })
        assertEquals("group-1", state.playgroupGameCounts.single().playgroupId)
        assertEquals(3, state.playgroupGameCounts.single().gamesPlayed)
        assertEquals("user-2", state.opponentStats.single().userId)
    }

    @Test
    fun `un grupo sin partidas jugadas cuenta como cero, no como error`() = runTest {
        api.onListPlaygroupGameCounts = { throw httpException(500) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertTrue(state.playgroupGameCounts.isEmpty())
    }

    @Test
    fun `un deck sin estadisticas todavia no rompe la carga del resto`() = runTest {
        api.onListDecks = { PagedResponse(items = listOf(deckDto("deck-a"))) }
        api.onGetDeckStats = { throw httpException(404) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertNull(state.deckStats.single().stats)
    }

    @Test
    fun `si fallan las estadisticas globales, se marca error sin romper la pantalla`() = runTest {
        api.onGetUserStats = { throw httpException(500) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertTrue(state.loadError)
        assertNull(state.userStats)
    }

    @Test
    fun `mostPlayedOpponent elige el rival con mas partidas juntos`() {
        val state = StatisticsUiState(
            opponentStats = listOf(
                opponentStatsDto(userId = "user-2", gamesTogether = 2),
                opponentStatsDto(userId = "user-3", gamesTogether = 5)
            )
        )

        assertEquals("user-3", state.mostPlayedOpponent?.userId)
    }

    @Test
    fun `archenemy ignora rivales que nunca te han eliminado`() {
        val state = StatisticsUiState(
            opponentStats = listOf(
                opponentStatsDto(userId = "user-2", timesEliminatedByOpponent = 0),
                opponentStatsDto(userId = "user-3", timesEliminatedByOpponent = 3)
            )
        )

        assertEquals("user-3", state.archenemy?.userId)
    }

    @Test
    fun `archenemy es null si ningun rival te ha eliminado nunca`() {
        val state = StatisticsUiState(
            opponentStats = listOf(opponentStatsDto(userId = "user-2", timesEliminatedByOpponent = 0))
        )

        assertNull(state.archenemy)
    }

    @Test
    fun `sortedDeckStats ordena por porcentaje de victorias`() {
        val lowWinRate = DeckWithStats(deckDto("deck-a"), DeckStatsDto(deckId = "deck-a", gamesPlayed = 10, gamesWon = 2))
        val highWinRate = DeckWithStats(deckDto("deck-b"), DeckStatsDto(deckId = "deck-b", gamesPlayed = 4, gamesWon = 3))
        val state = StatisticsUiState(
            deckStats = listOf(lowWinRate, highWinRate),
            deckSortOrder = DeckSortOrder.WIN_RATE
        )

        assertEquals(listOf("deck-b", "deck-a"), state.sortedDeckStats.map { it.deck.id })
    }

    @Test
    fun `mostPlayedPlaygroup ignora grupos sin partidas jugadas`() {
        val state = StatisticsUiState(
            playgroupGameCounts = listOf(
                playgroupGameCountDto(playgroupId = "group-1", gamesPlayed = 0),
                playgroupGameCountDto(playgroupId = "group-2", gamesPlayed = 4)
            )
        )

        assertEquals("group-2", state.mostPlayedPlaygroup?.playgroupId)
    }
}
