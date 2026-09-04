package com.commandercompanion.presentation.screens.friends

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.parseScannedFriendCode
import com.commandercompanion.domain.model.Friend
import com.commandercompanion.domain.model.IncomingFriendRequest
import com.commandercompanion.domain.model.OutgoingFriendRequest
import com.commandercompanion.domain.model.UserSearchResult
import com.commandercompanion.domain.repository.FriendsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Matches the web client's search debounce (see playgroups/[id].vue). */
private const val SEARCH_DEBOUNCE_MS = 300L

/** Minimum characters before searching, mirroring what the backend accepts. */
private const val MIN_QUERY_LENGTH = 2

/**
 * What went wrong, for the screen to turn into a string resource.
 *
 * The ViewModel deliberately does NOT build the message itself: the app ships
 * `values/`, `values-en/` and `values-ca/`, and a hardcoded literal here would
 * be untranslatable. It also keeps the unit tests asserting an enum instead of
 * prose. This was the first screen to do it; the rest of the app followed later
 * (`LoginError`, `RegisterError`, `SettingsError`, `JoinGameError`, `ApiFailure`).
 */
enum class FriendsError { NETWORK, SELF, USER_NOT_FOUND, ALREADY_RELATED, REQUEST_GONE, INVALID_CODE, UNKNOWN }

/**
 * Result of sending a request. [FRIENDS_NOW] is the auto-accept case: the
 * other user had already sent a request the other way, so this one resolved
 * the friendship instead of queueing behind it.
 */
enum class SendOutcome { REQUEST_SENT, FRIENDS_NOW }

data class FriendsUiState(
    val isLoading: Boolean = true,
    val loadError: FriendsError? = null,
    val friends: List<Friend> = emptyList(),
    val incoming: List<IncomingFriendRequest> = emptyList(),
    val outgoing: List<OutgoingFriendRequest> = emptyList(),

    val query: String = "",
    val isSearching: Boolean = false,
    val results: List<UserSearchResult> = emptyList(),

    /** Ids with an action in flight, so only that row shows as busy. */
    val busyIds: Set<String> = emptySet(),
    val actionError: FriendsError? = null,
    val lastOutcome: SendOutcome? = null
) {
    val isEmpty: Boolean
        get() = friends.isEmpty() && incoming.isEmpty() && outgoing.isEmpty()

    /**
     * Ids this user is already involved with, so a search result can offer
     * "add" only where it would work. Outgoing requests are keyed by
     * addressee and incoming by requester — both are the *other* user.
     */
    val knownUserIds: Set<String>
        get() = friends.map { it.id }.toSet() +
            outgoing.map { it.addresseeId }.toSet() +
            incoming.map { it.requesterId }.toSet()
}

/**
 * Friends list, pending requests in both directions, and adding someone by
 * username — the Android counterpart of `web/app/pages/friends/index.vue`.
 *
 * Depends only on [FriendsRepository], never on `SessionManager`: that class
 * takes a `Context` and can't be faked in a pure-JVM test, which is exactly
 * why `SettingsViewModel`, `DashboardViewModel` and three others have no unit
 * tests (see DECISIONS-LOG, "Known, deliberate Android test gaps"). Nothing
 * here needs the session, so nothing here inherits that.
 */
@HiltViewModel
class FriendsViewModel @Inject constructor(
    private val repository: FriendsRepository
) : ViewModel() {

    private val _uiState = MutableStateFlow(FriendsUiState())
    val uiState: StateFlow<FriendsUiState> = _uiState.asStateFlow()

    private var searchJob: Job? = null

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _uiState.update { it.copy(isLoading = true, loadError = null) }

            // In parallel: three independent GETs, and on a phone network
            // three round-trips in sequence is a visible wait for no reason.
            val (friends, incoming, outgoing) = coroutineScope {
                val f = async { repository.listFriends() }
                val i = async { repository.listIncomingRequests() }
                val o = async { repository.listOutgoingRequests() }
                Triple(f.await(), i.await(), o.await())
            }

            val failure = friends.exceptionOrNull() ?: incoming.exceptionOrNull() ?: outgoing.exceptionOrNull()
            if (failure != null) {
                _uiState.update { it.copy(isLoading = false, loadError = failure.toFriendsError()) }
                return@launch
            }

            _uiState.update {
                it.copy(
                    isLoading = false,
                    loadError = null,
                    friends = friends.getOrDefault(emptyList()),
                    incoming = incoming.getOrDefault(emptyList()),
                    outgoing = outgoing.getOrDefault(emptyList())
                )
            }
        }
    }

    fun onQueryChange(query: String) {
        _uiState.update { it.copy(query = query, actionError = null) }

        // Cancelling the previous job *is* the debounce: every keystroke
        // restarts the delay, so only the last one lives long enough to fetch.
        searchJob?.cancel()
        if (query.trim().length < MIN_QUERY_LENGTH) {
            _uiState.update { it.copy(results = emptyList(), isSearching = false) }
            return
        }

        searchJob = viewModelScope.launch {
            delay(SEARCH_DEBOUNCE_MS)
            _uiState.update { it.copy(isSearching = true) }
            repository.searchUsers(query).fold(
                onSuccess = { results -> _uiState.update { it.copy(isSearching = false, results = results) } },
                onFailure = { error ->
                    _uiState.update {
                        it.copy(isSearching = false, results = emptyList(), actionError = error.toFriendsError())
                    }
                }
            )
        }
    }

    fun clearSearch() {
        searchJob?.cancel()
        _uiState.update { it.copy(query = "", results = emptyList(), isSearching = false) }
    }

    /**
     * Handles whatever the camera read. Parsing lives in [parseScannedFriendCode]
     * (tested on the JVM) rather than in the screen, so the "that is not one of
     * our codes" path is covered without a camera: a scanner pointed at the
     * world reads Wi-Fi codes, product barcodes and unrelated URLs, and none of
     * them should reach the backend.
     */
    fun onScanned(raw: String?) {
        val userId = parseScannedFriendCode(raw)
        if (userId == null) {
            _uiState.update { it.copy(actionError = FriendsError.INVALID_CODE, lastOutcome = null) }
            return
        }
        sendRequest(userId)
    }

    /** [userId] comes from a search result or from a scanned QR — same call either way. */
    fun sendRequest(userId: String) {
        runRowAction(userId) {
            repository.sendRequest(userId).map { request ->
                _uiState.update {
                    it.copy(
                        lastOutcome = if (request.wasAutoAccepted) SendOutcome.FRIENDS_NOW else SendOutcome.REQUEST_SENT
                    )
                }
            }
        }
    }

    fun acceptRequest(requestId: String) =
        runRowAction(requestId, Conflict.REQUEST_GONE) { repository.acceptRequest(requestId).map { } }

    fun rejectRequest(requestId: String) =
        runRowAction(requestId, Conflict.REQUEST_GONE) { repository.rejectRequest(requestId) }

    fun cancelRequest(requestId: String) =
        runRowAction(requestId, Conflict.REQUEST_GONE) { repository.cancelRequest(requestId) }

    fun removeFriend(userId: String) = runRowAction(userId) { repository.removeFriend(userId) }

    fun consumeOutcome() = _uiState.update { it.copy(lastOutcome = null, actionError = null) }

    /**
     * Runs one row's action, marking only that row busy, and reloads on
     * success — each of these changes more than one list (accepting moves a
     * row out of `incoming` and into `friends`), so refetching is both simpler
     * and less wrong than patching the lists in place.
     */
    private fun runRowAction(
        id: String,
        conflict: Conflict = Conflict.ALREADY_RELATED,
        action: suspend () -> Result<Unit>
    ) {
        viewModelScope.launch {
            _uiState.update { it.copy(busyIds = it.busyIds + id, actionError = null) }
            val result = action()
            _uiState.update { it.copy(busyIds = it.busyIds - id) }
            result.fold(
                onSuccess = {
                    clearSearch()
                    load()
                },
                onFailure = { error -> _uiState.update { it.copy(actionError = error.toFriendsError(conflict)) } }
            )
        }
    }
}

/**
 * The status codes carry real meaning here, so they are not collapsed into
 * "the server responded with an error": sending a second request, or one to
 * someone already a friend, is the ordinary way this fails.
 */
/** What a 409 means for a given call — see [Throwable.toFriendsError]. */
internal enum class Conflict { ALREADY_RELATED, REQUEST_GONE }

/**
 * The status codes carry real meaning here, so they are not collapsed into
 * "the server responded with an error".
 *
 * 409 is the one that needs [conflict]: sending to someone you're already
 * related to and responding to a request that is no longer pending are the
 * same status with opposite wording (see `ErrAlreadyFriends` /
 * `ErrRequestAlreadyPending` vs `ErrRequestNotPending` in
 * `internal/friends/service.go`).
 */
internal fun Throwable.toFriendsError(
    conflict: Conflict = Conflict.ALREADY_RELATED
): FriendsError = when (this) {
    is ApiError.Network -> FriendsError.NETWORK
    is ApiError.Http -> when (code) {
        400 -> FriendsError.SELF
        404 -> FriendsError.USER_NOT_FOUND
        409 -> if (conflict == Conflict.REQUEST_GONE) FriendsError.REQUEST_GONE else FriendsError.ALREADY_RELATED
        else -> FriendsError.UNKNOWN
    }
    else -> FriendsError.UNKNOWN
}
