package com.commandercompanion.data.session

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.dto.LogoutRequest
import com.commandercompanion.data.remote.dto.RefreshRequest
import com.commandercompanion.data.remote.dto.TokenResponse
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.sessionDataStore by preferencesDataStore(name = "session")

/**
 * Session persistence (DataStore) + refresh/logout orchestration.
 *
 * `authApi` here is always the instance WITHOUT the auth interceptor (see `NetworkModule`):
 * refresh and logout don't carry a Bearer (`security: []` in the spec), and using the
 * authenticated client would cause a recursion with [com.commandercompanion.data.remote.interceptor.AuthAuthenticator].
 */
@Singleton
class SessionManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val authApi: AuthApi,
    private val googleAuthClient: GoogleAuthClient
) {

    private object Keys {
        val ACCESS_TOKEN = stringPreferencesKey("access_token")
        val REFRESH_TOKEN = stringPreferencesKey("refresh_token")
        val EXPIRES_AT = longPreferencesKey("expires_at")
        val USERNAME = stringPreferencesKey("username")
    }

    /** Emits when the automatic refresh fails and logout is forced (see [AuthAuthenticator]). */
    private val _forcedLogoutEvents = MutableSharedFlow<Unit>(extraBufferCapacity = 1)
    val forcedLogoutEvents: SharedFlow<Unit> = _forcedLogoutEvents.asSharedFlow()

    val isLoggedIn: Flow<Boolean> = context.sessionDataStore.data.map { it[Keys.ACCESS_TOKEN] != null }

    suspend fun saveSession(token: TokenResponse) {
        context.sessionDataStore.edit { prefs ->
            prefs[Keys.ACCESS_TOKEN] = token.accessToken
            prefs[Keys.REFRESH_TOKEN] = token.refreshToken
            prefs[Keys.EXPIRES_AT] = System.currentTimeMillis() + token.expiresIn * 1000
            if (token.user != null) {
                prefs[Keys.USERNAME] = token.user.username
            }
        }
    }

    suspend fun currentAccessToken(): String? =
        context.sessionDataStore.data.first()[Keys.ACCESS_TOKEN]

    /** Username of the authenticated user, to prefill "who am I" in the game setup. */
    suspend fun currentUsername(): String? =
        context.sessionDataStore.data.first()[Keys.USERNAME]

    private suspend fun currentRefreshToken(): String? =
        context.sessionDataStore.data.first()[Keys.REFRESH_TOKEN]

    private suspend fun clearSession() {
        context.sessionDataStore.edit { it.clear() }
    }

    /**
     * Called synchronously (via `runBlocking`) from [AuthAuthenticator] on OkHttp's thread
     * when an authenticated request returns 401. Rotates the refresh token; forces logout on failure.
     */
    suspend fun refreshAccessToken(): String? {
        val refreshToken = currentRefreshToken() ?: return null
        return try {
            val response = authApi.refresh(RefreshRequest(refreshToken))
            saveSession(response)
            response.accessToken
        } catch (e: Exception) {
            forceLogout()
            null
        }
    }

    /** Logout initiated by the user from the UI (the "Sign out" button). */
    suspend fun logout() {
        val refreshToken = currentRefreshToken()
        if (refreshToken != null) {
            try {
                authApi.logout(LogoutRequest(refreshToken))
            } catch (e: Exception) {
                // Best-effort: if the backend is unavailable we still clear the local session.
            }
        }
        clearSession()
        googleAuthClient.clearCredentialState(context)
    }

    /** Logout forced by a failed refresh (stolen/expired/revoked token). Doesn't hit the backend. */
    private suspend fun forceLogout() {
        clearSession()
        googleAuthClient.clearCredentialState(context)
        _forcedLogoutEvents.tryEmit(Unit)
    }
}
