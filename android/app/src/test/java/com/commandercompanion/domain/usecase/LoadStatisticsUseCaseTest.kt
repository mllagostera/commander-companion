package com.commandercompanion.domain.usecase

import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import com.commandercompanion.data.repository.DeckRepositoryImpl
import com.commandercompanion.data.repository.PlaygroupRepositoryImpl
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeDeckDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.playgroupDto
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class LoadStatisticsUseCaseTest {

    private val api = FakeCommanderApi()

    private fun useCase(): LoadStatisticsUseCase = LoadStatisticsUseCase(
        statisticsRepository = StatisticsRepositoryImpl(api),
        deckRepository = DeckRepositoryImpl(api, FakeDeckDao()),
        playgroupRepository = PlaygroupRepositoryImpl(api)
    )

    @Test
    fun `carga las estadisticas globales, por deck y por grupo`() = runTest {
        api.onGetUserStats = { UserStatsDto(userId = "user-1", gamesPlayed = 10, gamesWon = 4) }
        api.onListDecks = { listOf(deckDto("deck-a"), deckDto("deck-b")) }
        api.onGetDeckStats = { id -> DeckStatsDto(deckId = id, gamesPlayed = 5, gamesWon = 2) }
        api.onListPlaygroups = { listOf(playgroupDto(id = "group-1")) }
        api.onGetPlaygroupStats = { id -> PlaygroupStatsDto(playgroupId = id, gamesPlayed = 3) }

        val snapshot = useCase()!!

        assertEquals(10, snapshot.userStats.gamesPlayed)
        assertEquals(4, snapshot.userStats.gamesWon)
        assertEquals(listOf("deck-a", "deck-b"), snapshot.deckStats.map { it.deck.id })
        assertEquals(listOf(5, 5), snapshot.deckStats.map { it.stats?.gamesPlayed })
        assertEquals("group-1", snapshot.playgroupSummaries.single().playgroup.id)
        assertEquals(3, snapshot.playgroupSummaries.single().gamesPlayed)
    }

    @Test
    fun `un grupo sin partidas jugadas cuenta como cero, no como error`() = runTest {
        api.onListPlaygroups = { listOf(playgroupDto(id = "group-1")) }
        api.onGetPlaygroupStats = { throw httpException(404) }

        val snapshot = useCase()!!

        assertEquals(0, snapshot.playgroupSummaries.single().gamesPlayed)
    }

    @Test
    fun `un deck sin estadisticas todavia no rompe la carga del resto`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-a")) }
        api.onGetDeckStats = { throw httpException(404) }

        val snapshot = useCase()!!

        assertNull(snapshot.deckStats.single().stats)
    }

    @Test
    fun `si fallan las estadisticas globales no hay snapshot`() = runTest {
        api.onGetUserStats = { throw httpException(500) }

        assertNull(useCase())
    }
}
