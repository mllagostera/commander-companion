package com.commandercompanion.domain.usecase

import com.commandercompanion.data.repository.DeckRepositoryImpl
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.domain.model.DeckStats
import com.commandercompanion.domain.model.Page
import com.commandercompanion.domain.model.UserStats
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeDeckDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.opponentStatsDto
import com.commandercompanion.testing.playgroupGameCountDto
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LoadStatisticsUseCaseTest {

    private val api = FakeCommanderApi()

    private suspend fun loadSnapshot() = LoadStatisticsUseCase(
        statisticsRepository = StatisticsRepositoryImpl(api),
        deckRepository = DeckRepositoryImpl(api, FakeDeckDao())
    ).invoke()

    @Test
    fun `carga las estadisticas globales, por deck y por grupo`() = runTest {
        api.onGetUserStats = { UserStats(userId = "user-1", gamesPlayed = 10, gamesWon = 4) }
        api.onListDecks = { Page(items = listOf(deckDto("deck-a"), deckDto("deck-b"))) }
        api.onGetDeckStats = { id -> DeckStats(deckId = id, gamesPlayed = 5, gamesWon = 2) }
        api.onListPlaygroupGameCounts = { listOf(playgroupGameCountDto(playgroupId = "group-1", gamesPlayed = 3)) }
        api.onGetOpponentStats = { listOf(opponentStatsDto(userId = "user-2", gamesTogether = 2)) }

        val snapshot = loadSnapshot()!!

        assertEquals(10, snapshot.userStats.gamesPlayed)
        assertEquals(4, snapshot.userStats.gamesWon)
        assertEquals(listOf("deck-a", "deck-b"), snapshot.deckStats.map { it.deck.id })
        assertEquals(listOf(5, 5), snapshot.deckStats.map { it.stats?.gamesPlayed })
        assertEquals("group-1", snapshot.playgroupGameCounts.single().playgroupId)
        assertEquals(3, snapshot.playgroupGameCounts.single().gamesPlayed)
        assertEquals("user-2", snapshot.opponentStats.single().userId)
        assertEquals(2, snapshot.opponentStats.single().gamesTogether)
    }

    @Test
    fun `un grupo sin partidas jugadas cuenta como cero, no como error`() = runTest {
        api.onListPlaygroupGameCounts = { throw httpException(500) }

        val snapshot = loadSnapshot()!!

        assertTrue(snapshot.playgroupGameCounts.isEmpty())
    }

    @Test
    fun `un fallo en las estadisticas de rivales no rompe la carga del resto`() = runTest {
        api.onGetOpponentStats = { throw httpException(500) }

        val snapshot = loadSnapshot()!!

        assertTrue(snapshot.opponentStats.isEmpty())
    }

    @Test
    fun `un deck sin estadisticas todavia no rompe la carga del resto`() = runTest {
        api.onListDecks = { Page(items = listOf(deckDto("deck-a"))) }
        api.onGetDeckStats = { throw httpException(404) }

        val snapshot = loadSnapshot()!!

        assertNull(snapshot.deckStats.single().stats)
    }

    @Test
    fun `si fallan las estadisticas globales no hay snapshot`() = runTest {
        api.onGetUserStats = { throw httpException(500) }

        assertNull(loadSnapshot())
    }
}
