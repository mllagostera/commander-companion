package com.commandercompanion.data.remote.interceptor

import com.commandercompanion.data.session.SessionManager
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject

/**
 * Adjunta el access token (si hay sesión) como Bearer a cada request del cliente HTTP
 * autenticado (ver `NetworkModule` — usado por `CommanderApi`, no por `AuthApi`: login/google/
 * refresh/logout son públicos según el spec).
 *
 * `runBlocking` es intencional: `Interceptor.intercept` es una API síncrona de OkHttp (corre en
 * el dispatcher de OkHttp, no en una corrutina), y la lectura de DataStore es rápida (I/O local).
 */
class AuthInterceptor @Inject constructor(
    private val sessionManager: SessionManager
) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val token = runBlocking { sessionManager.currentAccessToken() }
        val request = if (token != null) {
            chain.request().newBuilder()
                .header("Authorization", "Bearer $token")
                .build()
        } else {
            chain.request()
        }
        return chain.proceed(request)
    }
}
