package com.commandercompanion.data.remote.interceptor

import com.commandercompanion.data.session.SessionManager
import kotlinx.coroutines.runBlocking
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route
import javax.inject.Inject

/**
 * Ante un 401 del cliente HTTP autenticado, intenta refrescar el access token una vez y
 * reintenta la request original con el token nuevo. Si el refresh falla,
 * [SessionManager.refreshAccessToken] ya se encarga de forzar el logout — acá simplemente
 * devolvemos `null` para que OkHttp abandone (sin reintentar en loop).
 */
class AuthAuthenticator @Inject constructor(
    private val sessionManager: SessionManager
) : Authenticator {

    override fun authenticate(route: Route?, response: Response): Request? {
        // Ya reintentamos una vez para esta cadena de responses: no reintentar de nuevo.
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
