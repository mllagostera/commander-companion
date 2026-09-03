package com.commandercompanion.data.remote.interceptor

import com.commandercompanion.data.session.SessionManager
import javax.inject.Inject
import kotlinx.coroutines.runBlocking
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route

/**
 * On a 401 from the authenticated HTTP client, tries to refresh the access token once and
 * retries the original request with the new token. If the refresh fails,
 * [SessionManager.refreshAccessToken] already takes care of forcing the logout — here we simply
 * return `null` so OkHttp gives up (no retry loop).
 */
class AuthAuthenticator @Inject constructor(
    private val sessionManager: SessionManager
) : Authenticator {

    override fun authenticate(route: Route?, response: Response): Request? {
        // We already retried once for this response chain: don't retry again.
        if (responseCount(response) >= 2) return null

        val newAccessToken = runBlocking { sessionManager.refreshAccessToken() } ?: return null

        return response.request.newBuilder()
            .header("Authorization", "Bearer $newAccessToken")
            .build()
    }

    private fun responseCount(response: Response): Int {
        var count = 1
        var prior = response.priorResponse
        while (prior != null) {
            count++
            prior = prior.priorResponse
        }
        return count
    }
}
