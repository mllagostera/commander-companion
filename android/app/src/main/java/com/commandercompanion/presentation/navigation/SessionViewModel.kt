package com.commandercompanion.presentation.navigation

import androidx.lifecycle.ViewModel
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharedFlow
import javax.inject.Inject

/**
 * Bridge between [SessionManager] (which lives outside Compose, including on the OkHttp thread
 * of [com.commandercompanion.data.remote.interceptor.AuthAuthenticator]) and the NavHost: when a
 * token refresh fails, the session clears itself but someone has to force navigation back to
 * `LoginRoute` — that's the only thing this ViewModel does.
 */
@HiltViewModel
class SessionViewModel @Inject constructor(
    sessionManager: SessionManager
) : ViewModel() {
    val forcedLogoutEvents: SharedFlow<Unit> = sessionManager.forcedLogoutEvents
}
