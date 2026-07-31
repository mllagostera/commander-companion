package com.commandercompanion.data.remote.interceptor

import com.commandercompanion.data.session.SessionManager
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject

/**
 * Attaches the access token (if there's a session) as a Bearer to every request of the
 * authenticated HTTP client (see `NetworkModule` — used by `CommanderApi`, not by `AuthApi`:
 * login/google/refresh/logout are public per the spec).
 *
 * `runBlocking` is intentional: `Interceptor.intercept` is a synchronous OkHttp API (it runs on
 * OkHttp's dispatcher, not in a coroutine), and reading from DataStore is fast (local I/O).
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
