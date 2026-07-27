package com.commandercompanion.presentation.navigation

import androidx.lifecycle.ViewModel
import com.commandercompanion.data.session.SessionManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.SharedFlow
import javax.inject.Inject

/**
 * Puente entre [SessionManager] (que vive fuera de Compose, incluye el hilo de OkHttp del
 * [com.commandercompanion.data.remote.interceptor.AuthAuthenticator]) y el NavHost: cuando un
 * refresh de token falla, la sesión se limpia sola pero alguien tiene que forzar la navegación
 * de vuelta a `LoginRoute` — eso es lo único que hace este ViewModel.
 */
@HiltViewModel
class SessionViewModel @Inject constructor(
    sessionManager: SessionManager
) : ViewModel() {
    val forcedLogoutEvents: SharedFlow<Unit> = sessionManager.forcedLogoutEvents
}
