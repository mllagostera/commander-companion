package com.vansid.tapeandocartones.data.session

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.vansid.tapeandocartones.core.util.SingleFlight
import com.vansid.tapeandocartones.data.local.dao.DeckDao
import com.vansid.tapeandocartones.data.remote.api.AuthApi
import com.vansid.tapeandocartones.data.remote.dto.LogoutRequest
import com.vansid.tapeandocartones.data.remote.dto.RefreshRequest
import com.vansid.tapeandocartones.data.remote.dto.TokenResponse
import com.vansid.tapeandocartones.domain.repository.LastDeckRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.first

private val Context.sessionDataStore by preferencesDataStore(name = "session")

/**
 * Session persistence (DataStore) + refresh/logout orchestration.
 *
 * `authApi` here is always the instance WITHOUT the auth interceptor (see `NetworkModule`):
 * refresh and logout don't carry a Bearer (`security: []` in the spec), and using the
 * authenticated client would cause a recursion with [com.vansid.tapeandocartones.data.remote.interceptor.AuthAuthenticator].
 */
@Singleton
class SessionManager @Inject constructor(
    @ApplicationContext private val context: Context,
    private val authApi: AuthApi,
    private val googleAuthClient: GoogleAuthClient,
    private val deckDao: DeckDao,
    private val lastDeckRepository: LastDeckRepository
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

    // Owns refreshAccessToken()'s single-flight execution. Deliberately NOT the caller's own
    // scope: AuthAuthenticator invokes refreshAccessToken() via `runBlocking` from whichever
    // OkHttp dispatcher thread got the 401, and several of those can be in flight at once (e.g.
    // every in-parallel request from LoadStatisticsUseCase's `async`/`awaitAll` 401ing together
    // when the access token expires). A scope tied to any one of those `runBlocking` calls would
    // only dedupe within that single call; this one outlives all of them, for as long as this
    // singleton does (the app process).
    private val refreshScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val refreshSingleFlight = SingleFlight<String?>(refreshScope)

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

    /**
     * Ends the session on this device: the tokens, plus whatever was cached on the account's
     * behalf. The deck cache is the offline fallback of `DeckRepositoryImpl.listDecks()` and
     * isn't keyed by user, so the next account to sign in here would otherwise be shown the
     * previous one's decks whenever the network failed. Both are rebuilt from the backend or
     * from play. The local game history (Room's `games`) is deliberately kept: it exists
     * nowhere else, and clearing it would lose it for good.
     */
    private suspend fun clearSession() {
        context.sessionDataStore.edit { it.clear() }
        deckDao.clear()
        lastDeckRepository.forgetAll()
    }

    /**
     * Called synchronously (via `runBlocking`) from [AuthAuthenticator] on OkHttp's thread
     * when an authenticated request returns 401. Rotates the refresh token. Forces logout only
     * when the backend rejects the refresh token ([isRefreshRejected]); any other failure (no
     * network, a timeout, a 5xx) returns null and keeps the session, so the next request tries
     * again.
     *
     * Concurrent callers (several requests 401ing at once, e.g. a batch of parallel calls after
     * the access token expired) are collapsed into a single real refresh via [refreshSingleFlight]:
     * the backend rotates the refresh token on every use, so a second concurrent call presenting
     * the same not-yet-rotated token would look like a replay and get the WHOLE session family
     * revoked (see the class doc and [SingleFlight]'s doc for the full story) — logging the user
     * out on every device over what was really just a timing coincidence, not theft.
     */
    suspend fun refreshAccessToken(): String? = refreshSingleFlight.run {
        val refreshToken = currentRefreshToken() ?: return@run null
        try {
            val response = authApi.refresh(RefreshRequest(refreshToken))
            saveSession(response)
            response.accessToken
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            if (isRefreshRejected(e)) forceLogout()
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

    /** Logout forced by a rejected refresh (stolen/expired/revoked token). Doesn't hit the backend. */
    private suspend fun forceLogout() {
        clearSession()
        googleAuthClient.clearCredentialState(context)
        _forcedLogoutEvents.tryEmit(Unit)
    }
}
