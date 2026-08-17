package com.commandercompanion.data.repository

import com.commandercompanion.core.util.ApiError
import com.commandercompanion.data.remote.dto.FriendRequestDto
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.friendDto
import com.commandercompanion.testing.friendRequestDto
import com.commandercompanion.testing.incomingFriendRequestDto
import com.commandercompanion.testing.outgoingFriendRequestDto
import com.commandercompanion.testing.userSearchResultDto
import com.commandercompanion.testing.httpException
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

class FriendsRepositoryImplTest {

    private val api = FakeCommanderApi()
    private val repository = FriendsRepositoryImpl(api)

    @Test
    fun `listFriends devuelve las amistades aceptadas`() = runTest {
        api.onListFriends = { listOf(friendDto(id = "user-9", username = "ana")) }

        val result = repository.listFriends().getOrThrow()

        assertEquals("ana", result.single().username)
        assertEquals("user-9", result.single().id)
    }

    @Test
    fun `listFriends propaga el error de red`() = runTest {
        api.onListFriends = { throw IOException("sin red") }

        assertTrue(repository.listFriends().exceptionOrNull() is ApiError.Network)
    }

    @Test
    fun `las dos direcciones de solicitudes usan endpoints distintos`() = runTest {
        api.onListIncomingFriendRequests = { listOf(incomingFriendRequestDto(requesterUsername = "bruno")) }
        api.onListOutgoingFriendRequests = { listOf(outgoingFriendRequestDto(addresseeUsername = "carla")) }

        assertEquals("bruno", repository.listIncomingRequests().getOrThrow().single().requesterUsername)
        assertEquals("carla", repository.listOutgoingRequests().getOrThrow().single().addresseeUsername)
    }

    @Test
    fun `sendRequest envia el id del destinatario en el cuerpo`() = runTest {
        var received: String? = null
        api.onSendFriendRequest = { request ->
            received = request.addresseeId
            friendRequestDto(addresseeId = request.addresseeId)
        }

        repository.sendRequest("user-77").getOrThrow()

        assertEquals("user-77", received)
    }

    /**
     * The backend auto-accepts when the other user had already sent a request
     * the other way: same 2xx, but `status` comes back "accepted". The UI has
     * to tell the two apart, so the flag is asserted here rather than left to
     * each caller comparing strings.
     */
    @Test
    fun `sendRequest marca la auto-aceptacion cuando ya habia solicitud inversa`() = runTest {
        api.onSendFriendRequest = { friendRequestDto(status = FriendRequestDto.STATUS_ACCEPTED) }

        assertTrue(repository.sendRequest("user-2").getOrThrow().wasAutoAccepted)

        api.onSendFriendRequest = { friendRequestDto(status = FriendRequestDto.STATUS_PENDING) }

        assertFalse(repository.sendRequest("user-2").getOrThrow().wasAutoAccepted)
    }

    @Test
    fun `sendRequest propaga el 409 de amistad ya existente`() = runTest {
        api.onSendFriendRequest = { throw httpException(409) }

        val error = repository.sendRequest("user-2").exceptionOrNull()

        assertTrue(error is ApiError.Http)
        assertEquals(409, (error as ApiError.Http).code)
    }

    @Test
    fun `aceptar una solicitud devuelve la amistad resultante`() = runTest {
        api.onAcceptFriendRequest = { friendDto(id = "user-5", username = "diego") }

        val friend = repository.acceptRequest("req-1").getOrThrow()

        assertEquals("diego", friend.username)
    }

    @Test
    fun `rechazar y cancelar llegan a sus endpoints`() = runTest {
        repository.rejectRequest("req-1").getOrThrow()
        repository.cancelRequest("req-2").getOrThrow()

        assertTrue(api.calls.contains("rejectFriendRequest"))
        assertTrue(api.calls.contains("cancelFriendRequest"))
    }

    @Test
    fun `removeFriend usa el id del otro usuario`() = runTest {
        var received: String? = null
        api.onRemoveFriend = { received = it }

        repository.removeFriend("user-2").getOrThrow()

        assertEquals("user-2", received)
    }

    @Test
    fun `searchUsers recorta la consulta y devuelve resultados`() = runTest {
        var received: String? = null
        api.onSearchUsers = { query ->
            received = query
            listOf(userSearchResultDto(username = "ana"))
        }

        val result = repository.searchUsers("  ana  ").getOrThrow()

        assertEquals("ana", received)
        assertEquals("ana", result.single().username)
    }

    /**
     * One character is a keystroke on the way to a real query, not a mistake:
     * the backend would answer 400, so it never gets asked.
     */
    @Test
    fun `searchUsers no llama al backend con menos de dos caracteres`() = runTest {
        val result = repository.searchUsers("a").getOrThrow()

        assertTrue(result.isEmpty())
        assertFalse(api.calls.contains("searchUsers"))
    }

    @Test
    fun `searchUsers con solo espacios tampoco llama al backend`() = runTest {
        assertTrue(repository.searchUsers("   ").getOrThrow().isEmpty())
        assertFalse(api.calls.contains("searchUsers"))
    }
}
