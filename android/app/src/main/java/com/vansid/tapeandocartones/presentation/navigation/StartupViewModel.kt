package com.vansid.tapeandocartones.presentation.navigation

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.vansid.tapeandocartones.data.session.AccessTokenProvider
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/**
 * Picks the screen a cold start opens on: the dashboard when a session is stored, the login
 * otherwise. The app used to open on the login every time, so a session that survived a process
 * death -- an app update always causes one -- was ignored and the user had to sign in again.
 *
 * Only whether a token is stored matters here, not whether it is still valid: an expired access
 * token is refreshed by `AuthAuthenticator` on the first 401, and a refresh the backend rejects
 * forces the logout that sends the user back to the login (see [SessionViewModel]).
 */
@HiltViewModel
class StartupViewModel @Inject constructor(
    private val accessTokenProvider: AccessTokenProvider
) : ViewModel() {

    private val _startDestination = MutableStateFlow<Any?>(null)

    /** Null until the stored session has been read, which takes one DataStore read. */
    val startDestination: StateFlow<Any?> = _startDestination.asStateFlow()

    init {
        viewModelScope.launch {
            _startDestination.value = if (accessTokenProvider.currentAccessToken() != null) DashboardRoute else LoginRoute
        }
    }
}
