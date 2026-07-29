package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.deckDto
import com.commandercompanion.testing.playgroupDto
import com.commandercompanion.testing.playgroupMemberDto
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class PlaygroupRepositoryTest {

    private val api = FakeCommanderApi()
    private val repository = PlaygroupRepository(api)

    @Test
    fun `listPlaygroups devuelve los grupos del usuario con sus miembros`() = runTest {
        api.onListPlaygroups = {
            listOf(playgroupDto(id = "pg-1", name = "Mesa de los viernes", members = listOf(playgroupMemberDto())))
        }

        val result = repository.listPlaygroups().getOrThrow()

        assertEquals("Mesa de los viernes", result.single().name)
        assertEquals(1, result.single().members.size)
    }

    @Test
    fun `listPlaygroups propaga el error de red`() = runTest {
        api.onListPlaygroups = { throw IOException("sin red") }

        val result = repository.listPlaygroups()

        assertTrue(result.exceptionOrNull() is ApiError.Network)
    }

    @Test
    fun `getPlaygroup devuelve el detalle del grupo`() = runTest {
        api.onGetPlaygroup = { id -> playgroupDto(id = id, name = "Grupo real") }

        val result = repository.getPlaygroup("pg-1").getOrThrow()

        assertEquals("pg-1", result.id)
        assertEquals("Grupo real", result.name)
    }

    @Test
    fun `getMemberDecks devuelve los decks del miembro indicado`() = runTest {
        api.onGetMemberDecks = { _, userId -> listOf(deckDto(id = "deck-de-$userId")) }

        val result = repository.getMemberDecks("pg-1", "user-9").getOrThrow()

        assertEquals("deck-de-user-9", result.single().id)
    }
}
