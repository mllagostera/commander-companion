package com.commandercompanion.presentation.screens.statistics

import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.finishedGameDto
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
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class FinishedGamesViewModelTest {

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

    private fun viewModel() = FinishedGamesViewModel(StatisticsRepositoryImpl(api))

    @Test
    fun `carga la primera pagina al iniciar`() = runTest {
        api.onListFinishedGames = { PagedResponse(items = listOf(finishedGameDto("game-1")), nextCursor = "cursor-1") }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertFalse(state.loadError)
        assertEquals(listOf("game-1"), state.games.map { it.id })
        assertTrue(state.hasMore)
    }

    @Test
    fun `loadMore anade la siguiente pagina sin descartar la anterior`() = runTest {
        api.onListFinishedGames = { cursor ->
            if (cursor == null) {
                PagedResponse(items = listOf(finishedGameDto("game-1")), nextCursor = "cursor-1")
            } else {
                PagedResponse(items = listOf(finishedGameDto("game-2")), nextCursor = null)
            }
        }

        val viewModel = viewModel()
        advanceUntilIdle()
        viewModel.loadMore()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertEquals(listOf("game-1", "game-2"), state.games.map { it.id })
        assertFalse(state.hasMore)
    }

    @Test
    fun `loadMore no hace nada si no hay siguiente pagina`() = runTest {
        api.onListFinishedGames = { PagedResponse(items = listOf(finishedGameDto("game-1")), nextCursor = null) }

        val viewModel = viewModel()
        advanceUntilIdle()
        viewModel.loadMore()
        advanceUntilIdle()

        assertEquals(1, api.calls.count { it == "listFinishedGames" })
    }

    @Test
    fun `un fallo al cargar marca error`() = runTest {
        api.onListFinishedGames = { throw httpException(500) }

        val viewModel = viewModel()
        advanceUntilIdle()
        val state = viewModel.uiState.value

        assertFalse(state.isLoading)
        assertTrue(state.loadError)
        assertNull(state.nextCursor)
    }
}
