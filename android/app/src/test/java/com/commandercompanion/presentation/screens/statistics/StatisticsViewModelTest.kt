package com.commandercompanion.presentation.screens.statistics

import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.data.repository.DeckRepositoryImpl
import com.commandercompanion.data.repository.PlaygroupRepositoryImpl
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.domain.usecase.LoadStatisticsUseCase
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeDeckDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.playgroupDto
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
        val playgroupRepository = PlaygroupRepositoryImpl(api)
        val loadStatisticsUseCase = LoadStatisticsUseCase(statisticsRepository, deckRepository, playgroupRepository)
        return StatisticsViewModel(loadStatisticsUseCase)
    }

    @Test
    fun `carga las estadisticas globales, por deck y por grupo`() = runTest {
        api.onGetUserStats = { UserStatsDto(userId = "user-1", gamesPlayed = 10, gamesWon = 4) }
        api.onListDecks = { listOf(deckDto("deck-a"), deckDto("deck-b")) }
        api.onGetDeckStats = { id -> DeckStatsDto(deckId = id, gamesPlayed = 5, gamesWon = 2) }
        api.onListPlaygroups = { listOf(playgroupDto(id = "group-1")) }
        api.onGetPlaygroupStats = { id -> PlaygroupStatsDto(playgroupId = id, gamesPlayed = 3) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertEquals(10, state.userStats?.gamesPlayed)
        assertEquals(4, state.userStats?.gamesWon)
        assertEquals(listOf("deck-a", "deck-b"), state.deckStats.map { it.deck.id })
        assertEquals(listOf(5, 5), state.deckStats.map { it.stats?.gamesPlayed })
        assertEquals("group-1", state.playgroupSummaries.single().playgroup.id)
        assertEquals(3, state.playgroupSummaries.single().gamesPlayed)
    }

    @Test
    fun `un grupo sin partidas jugadas cuenta como cero, no como error`() = runTest {
        api.onListPlaygroups = { listOf(playgroupDto(id = "group-1")) }
        api.onGetPlaygroupStats = { throw httpException(404) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertEquals(0, state.playgroupSummaries.single().gamesPlayed)
    }

    @Test
    fun `un deck sin estadisticas todavia no rompe la carga del resto`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-a")) }
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
}
