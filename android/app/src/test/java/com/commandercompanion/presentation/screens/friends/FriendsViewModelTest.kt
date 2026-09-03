package com.commandercompanion.presentation.screens.friends

import com.commandercompanion.data.repository.FriendsRepositoryImpl
import com.commandercompanion.domain.model.FriendRequestResult
import com.commandercompanion.testing.FakeCommanderApi
import com.commandercompanion.testing.friendDto
import com.commandercompanion.testing.friendRequestDto
import com.commandercompanion.testing.httpException
import com.commandercompanion.testing.incomingFriendRequestDto
import com.commandercompanion.testing.outgoingFriendRequestDto
import com.commandercompanion.testing.userSearchResultDto
import java.io.IOException
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceTimeBy
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
class FriendsViewModelTest {

    private val dispatcher = StandardTestDispatcher()
    private val api = FakeCommanderApi()

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    private fun viewModel() = FriendsViewModel(FriendsRepositoryImpl(api))

    private companion object {
        const val SCANNED_ID = "0d2dff0e-6208-4783-8236-e83974d900c6"
    }

    @Test
    fun `carga amigos y solicitudes de ambas direcciones`() = runTest(dispatcher) {
        api.onListFriends = { listOf(friendDto(username = "ana")) }
        api.onListIncomingFriendRequests = { listOf(incomingFriendRequestDto(requesterUsername = "bruno")) }
        api.onListOutgoingFriendRequests = { listOf(outgoingFriendRequestDto(addresseeUsername = "carla")) }

        val vm = viewModel()
        advanceUntilIdle()

        val state = vm.uiState.value
        assertFalse(state.isLoading)
        assertNull(state.loadError)
        assertEquals("ana", state.friends.single().username)
        assertEquals("bruno", state.incoming.single().requesterUsername)
        assertEquals("carla", state.outgoing.single().addresseeUsername)
        assertFalse(state.isEmpty)
    }

    @Test
    fun `un fallo de red deja loadError y no se queda cargando`() = runTest(dispatcher) {
        api.onListFriends = { throw IOException("sin red") }

        val vm = viewModel()
        advanceUntilIdle()

        assertFalse(vm.uiState.value.isLoading)
        assertEquals(FriendsError.NETWORK, vm.uiState.value.loadError)
    }

    @Test
    fun `una cuenta sin nada se marca como vacia`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()

        assertTrue(vm.uiState.value.isEmpty)
    }

    /**
     * Every keystroke restarts the debounce, so typing a name must produce one
     * search, not one per character.
     */
    @Test
    fun `la busqueda se debouncea a una sola llamada`() = runTest(dispatcher) {
        api.onSearchUsers = { listOf(userSearchResultDto(username = "ana")) }
        val vm = viewModel()
        advanceUntilIdle()
        api.calls.clear()

        vm.onQueryChange("a")
        vm.onQueryChange("an")
        vm.onQueryChange("ana")
        advanceUntilIdle()

        assertEquals(1, api.calls.count { it == "searchUsers" })
        assertEquals("ana", vm.uiState.value.results.single().username)
    }

    @Test
    fun `no se busca con menos de dos caracteres`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        api.calls.clear()

        vm.onQueryChange("a")
        advanceUntilIdle()

        assertEquals(0, api.calls.count { it == "searchUsers" })
        assertTrue(vm.uiState.value.results.isEmpty())
    }

    @Test
    fun `borrar la consulta cancela la busqueda pendiente`() = runTest(dispatcher) {
        api.onSearchUsers = { listOf(userSearchResultDto()) }
        val vm = viewModel()
        advanceUntilIdle()
        api.calls.clear()

        vm.onQueryChange("ana")
        advanceTimeBy(100L) // dentro del debounce, aún no ha salido
        vm.clearSearch()
        advanceUntilIdle()

        assertEquals(0, api.calls.count { it == "searchUsers" })
        assertEquals("", vm.uiState.value.query)
    }

    @Test
    fun `enviar una solicitud normal reporta REQUEST_SENT`() = runTest(dispatcher) {
        api.onSendFriendRequest = { friendRequestDto(status = FriendRequestResult.STATUS_PENDING) }
        val vm = viewModel()
        advanceUntilIdle()

        vm.sendRequest("user-2")
        advanceUntilIdle()

        assertEquals(SendOutcome.REQUEST_SENT, vm.uiState.value.lastOutcome)
    }

    /**
     * The backend resolves a crossed request by accepting it outright. The UI
     * has to say "you're now friends", not "request sent".
     */
    @Test
    fun `si habia solicitud inversa reporta FRIENDS_NOW`() = runTest(dispatcher) {
        api.onSendFriendRequest = { friendRequestDto(status = FriendRequestResult.STATUS_ACCEPTED) }
        val vm = viewModel()
        advanceUntilIdle()

        vm.sendRequest("user-2")
        advanceUntilIdle()

        assertEquals(SendOutcome.FRIENDS_NOW, vm.uiState.value.lastOutcome)
    }

    @Test
    fun `un 409 al enviar se traduce a ALREADY_RELATED`() = runTest(dispatcher) {
        api.onSendFriendRequest = { throw httpException(409) }
        val vm = viewModel()
        advanceUntilIdle()

        vm.sendRequest("user-2")
        advanceUntilIdle()

        assertEquals(FriendsError.ALREADY_RELATED, vm.uiState.value.actionError)
        assertNull(vm.uiState.value.lastOutcome)
    }

    /**
     * The same 409 means different things by call: "already friends / already
     * pending" when sending, "no longer pending" when responding. One blanket
     * mapping would word one of the two wrong.
     */
    @Test
    fun `un 409 al aceptar se traduce a REQUEST_GONE, no a ALREADY_RELATED`() = runTest(dispatcher) {
        api.onAcceptFriendRequest = { throw httpException(409) }
        val vm = viewModel()
        advanceUntilIdle()

        vm.acceptRequest("req-1")
        advanceUntilIdle()

        assertEquals(FriendsError.REQUEST_GONE, vm.uiState.value.actionError)
    }

    @Test
    fun `aceptar recarga las listas`() = runTest(dispatcher) {
        api.onListIncomingFriendRequests = { listOf(incomingFriendRequestDto(id = "req-1")) }
        val vm = viewModel()
        advanceUntilIdle()

        // Accepting moves the row out of `incoming` and into `friends`, so the
        // lists are refetched rather than patched.
        api.onListIncomingFriendRequests = { emptyList() }
        api.onListFriends = { listOf(friendDto(username = "bruno")) }

        vm.acceptRequest("req-1")
        advanceUntilIdle()

        assertTrue(vm.uiState.value.incoming.isEmpty())
        assertEquals("bruno", vm.uiState.value.friends.single().username)
    }

    @Test
    fun `solo la fila en curso se marca ocupada y se libera al terminar`() = runTest(dispatcher) {
        // The fake parks on this gate, so the action is observably in flight
        // rather than already finished by the time the assertions run.
        val gate = CompletableDeferred<Unit>()
        api.onAcceptFriendRequest = {
            gate.await()
            friendDto()
        }
        val vm = viewModel()
        advanceUntilIdle()

        vm.acceptRequest("req-1")
        advanceUntilIdle()
        assertTrue(vm.uiState.value.busyIds.contains("req-1"))
        assertFalse(vm.uiState.value.busyIds.contains("req-2"))

        gate.complete(Unit)
        advanceUntilIdle()
        assertTrue(vm.uiState.value.busyIds.isEmpty())
    }

    // ------------------------------------------------------------- escaneo

    @Test
    fun `escanear un enlace valido envia la solicitud`() = runTest(dispatcher) {
        var sentTo: String? = null
        api.onSendFriendRequest = { request ->
            sentTo = request.addresseeId
            friendRequestDto(addresseeId = request.addresseeId)
        }
        val vm = viewModel()
        advanceUntilIdle()

        vm.onScanned("https://commander.example/friends/add/$SCANNED_ID")
        advanceUntilIdle()

        assertEquals(SCANNED_ID, sentTo)
        assertEquals(SendOutcome.REQUEST_SENT, vm.uiState.value.lastOutcome)
    }

    @Test
    fun `escanear un uuid pelado tambien funciona`() = runTest(dispatcher) {
        var sentTo: String? = null
        api.onSendFriendRequest = { request ->
            sentTo = request.addresseeId
            friendRequestDto(addresseeId = request.addresseeId)
        }
        val vm = viewModel()
        advanceUntilIdle()

        vm.onScanned(SCANNED_ID)
        advanceUntilIdle()

        assertEquals(SCANNED_ID, sentTo)
    }

    /**
     * A camera pointed at the world reads Wi-Fi codes, product barcodes and
     * unrelated URLs. None of them should cost a request.
     */
    @Test
    fun `un codigo ajeno no llega al backend`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        api.calls.clear()

        vm.onScanned("WIFI:S:MiRed;T:WPA;P:secreto;;")
        advanceUntilIdle()

        assertEquals(FriendsError.INVALID_CODE, vm.uiState.value.actionError)
        assertEquals(0, api.calls.count { it == "sendFriendRequest" })
    }

    /** Cancelling the scanner, or Play Services failing, both arrive as null. */
    @Test
    fun `cancelar el escaner no llega al backend`() = runTest(dispatcher) {
        val vm = viewModel()
        advanceUntilIdle()
        api.calls.clear()

        vm.onScanned(null)
        advanceUntilIdle()

        assertEquals(FriendsError.INVALID_CODE, vm.uiState.value.actionError)
        assertEquals(0, api.calls.count { it == "sendFriendRequest" })
    }

    /**
     * A search result for someone already involved shouldn't offer "add": the
     * request would just 409.
     */
    @Test
    fun `knownUserIds cubre amigos y solicitudes en ambas direcciones`() = runTest(dispatcher) {
        api.onListFriends = { listOf(friendDto(id = "user-friend")) }
        api.onListIncomingFriendRequests = { listOf(incomingFriendRequestDto(requesterId = "user-in")) }
        api.onListOutgoingFriendRequests = { listOf(outgoingFriendRequestDto(addresseeId = "user-out")) }

        val vm = viewModel()
        advanceUntilIdle()

        val known = vm.uiState.value.knownUserIds
        assertTrue(known.containsAll(setOf("user-friend", "user-in", "user-out")))
        assertFalse(known.contains("user-stranger"))
    }

    @Test
    fun `consumeOutcome limpia el resultado y el error`() = runTest(dispatcher) {
        api.onSendFriendRequest = { throw httpException(409) }
        val vm = viewModel()
        advanceUntilIdle()

        vm.sendRequest("user-2")
        advanceUntilIdle()
        vm.consumeOutcome()

        assertNull(vm.uiState.value.actionError)
        assertNull(vm.uiState.value.lastOutcome)
    }
}
