package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.httpException
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class DeckRepositoryTest {

    private val api = FakeCommanderApi()
    private val repository = DeckRepository(api)

    @Test
    fun `listDecks devuelve los decks del backend`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-a"), deckDto("deck-b")) }

        val result = repository.listDecks()

        assertEquals(listOf("deck-a", "deck-b"), result.getOrThrow().map { it.id })
    }

    @Test
    fun `firstDeckId devuelve el primer deck`() = runTest {
        api.onListDecks = { listOf(deckDto("deck-a"), deckDto("deck-b")) }

        assertEquals("deck-a", repository.firstDeckId().getOrThrow())
    }

    @Test
    fun `firstDeckId es null cuando el usuario no tiene decks`() = runTest {
        api.onListDecks = { emptyList() }

        val result = repository.firstDeckId()

        assertTrue(result.isSuccess)
        assertNull(result.getOrThrow())
    }

    @Test
    fun `firstDeckId propaga el error de red en vez de fingir que no hay decks`() = runTest {
        api.onListDecks = { throw IOException("sin red") }

        val result = repository.firstDeckId()

        assertTrue(result.exceptionOrNull() is ApiError.Network)
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
