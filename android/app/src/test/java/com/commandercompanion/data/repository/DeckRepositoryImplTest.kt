package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.FakeDeckDao
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class DeckRepositoryImplTest {

    private val api = FakeCommanderApi()
    private val deckDao = FakeDeckDao()
    private val repository = DeckRepositoryImpl(api, deckDao)

    @Test
    fun `listDecks devuelve los decks del backend`() = runTest {
        api.onListDecks = { PagedResponse(items = listOf(deckDto("deck-a"), deckDto("deck-b"))) }

        val result = repository.listDecks()

        assertEquals(listOf("deck-a", "deck-b"), result.getOrThrow().map { it.id })
    }

    /**
     * Regression test: `GET /decks` paginates at 20 items server-side. Before this fix,
     * [DeckRepositoryImpl.listDecks] only read the first page and silently dropped the rest —
     * a user with more than 20 decks (e.g. after a bulk Moxfield import) would see an
     * incomplete list in the deck picker and per-deck statistics.
     */
    @Test
    fun `listDecks sigue next_cursor hasta la ultima pagina`() = runTest {
        api.onListDecks = { cursor ->
            when (cursor) {
                null -> PagedResponse(items = listOf(deckDto("deck-a"), deckDto("deck-b")), nextCursor = "page-2")
                "page-2" -> PagedResponse(items = listOf(deckDto("deck-c")), nextCursor = null)
                else -> error("cursor inesperado: $cursor")
            }
        }

        val result = repository.listDecks()

        assertEquals(listOf("deck-a", "deck-b", "deck-c"), result.getOrThrow().map { it.id })
    }

    @Test
    fun `listDecks propaga el error de red si no hay nada cacheado todavia`() = runTest {
        api.onListDecks = { throw IOException("sin red") }

        val result = repository.listDecks()

        assertTrue(result.exceptionOrNull() is ApiError.Network)
    }

    /** Offline-first: a previous successful fetch left the Room cache populated for next time. */
    @Test
    fun `listDecks devuelve lo cacheado si el backend no responde`() = runTest {
        api.onListDecks = { PagedResponse(items = listOf(deckDto("deck-a"), deckDto("deck-b"))) }
        repository.listDecks() // populates the cache

        api.onListDecks = { throw IOException("sin red") }
        val result = repository.listDecks()

        assertEquals(listOf("deck-a", "deck-b"), result.getOrThrow().map { it.id })
    }

    @Test
    fun `importFromMoxfield manda la url tal cual y devuelve el deck importado`() = runTest {
        var received: String? = null
        api.onImportMoxfield = { request ->
            received = request.url
            deckDto(id = "deck-importado", name = "Deck de Moxfield")
        }

        val result = repository.importFromMoxfield("https://moxfield.com/decks/abc123")

        assertEquals("https://moxfield.com/decks/abc123", received)
        assertEquals("deck-importado", result.getOrThrow().id)
    }

    @Test
    fun `importFromMoxfield mapea el 404 de deck inexistente`() = runTest {
        api.onImportMoxfield = { throw httpException(404) }

        val error = repository.importFromMoxfield("no-existe").exceptionOrNull()

        assertTrue(error is ApiError.Http)
        assertEquals(404, (error as ApiError.Http).code)
    }

    @Test
    fun `deleteDeck exitoso devuelve success`() = runTest {
        val result = repository.deleteDeck("deck-a")

        assertTrue(result.isSuccess)
        assertTrue(api.calls.contains("deleteDeck"))
    }
}
