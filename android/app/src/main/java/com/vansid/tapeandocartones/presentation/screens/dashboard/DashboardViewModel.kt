package com.vansid.tapeandocartones.presentation.screens.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.session.SessionManager
import com.vansid.tapeandocartones.domain.repository.FriendsRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class DashboardViewModel @Inject constructor(
    private val sessionManager: SessionManager,
    private val friendsRepository: FriendsRepository
) : ViewModel() {

    private val _pendingFriendRequests = MutableStateFlow(0)

    /** Pending incoming friend requests, shown as a badge on the "Friends" entry. */
    val pendingFriendRequests: StateFlow<Int> = _pendingFriendRequests.asStateFlow()

    /**
     * Re-checks the badge. The screen calls it on every resume — coming back from the friends
     * screen or from the background — since there's no push channel for new requests. A failed
     * check keeps the last known value: the badge isn't worth an error message.
     */
    fun refreshPendingFriendRequests() {
        viewModelScope.launch {
            friendsRepository.countIncomingRequests()
                .onSuccess { _pendingFriendRequests.value = it }
        }
    }

    /**
     * Revokes the refresh token against the backend (best-effort), clears the local session
     * (DataStore) and the Google credential state (`clearCredentialState`), and only then navigates.
     */
    fun logout(onComplete: () -> Unit) {
        viewModelScope.launch {
            sessionManager.logout()
            onComplete()
        }
    }
}
