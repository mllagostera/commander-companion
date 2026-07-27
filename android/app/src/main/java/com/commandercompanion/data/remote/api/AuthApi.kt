package com.commandercompanion.data.remote.api

import com.commandercompanion.data.remote.dto.GoogleLoginRequest
import com.commandercompanion.data.remote.dto.LoginRequest
import com.commandercompanion.data.remote.dto.LogoutRequest
import com.commandercompanion.data.remote.dto.RefreshRequest
import com.commandercompanion.data.remote.dto.TokenResponse
import com.commandercompanion.data.remote.dto.UserDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST

/**
 * Endpoints de autenticación (`/auth`), ver `docs/api/openapi.yaml`.
 *
 * Vive en su propio archivo, separado de [CommanderApi], porque otro agente puede estar
 * extendiendo `CommanderApi.kt` con endpoints de decks/games/etc. en paralelo — mantenerlos
 * separados evita conflictos de merge.
 *
 * `login`/`google`/`refresh`/`logout` son públicos (`security: []` en el spec, no llevan Bearer).
 * `me` sí requiere el access token, pasado explícitamente como header en vez de depender de un
 * interceptor — así este API client no necesita conocer la sesión activa.
 */
interface AuthApi {

    @POST("api/v1/auth/login")
    suspend fun login(@Body request: LoginRequest): TokenResponse

    @POST("api/v1/auth/google")
    suspend fun loginWithGoogle(@Body request: GoogleLoginRequest): TokenResponse

    @POST("api/v1/auth/refresh")
    suspend fun refresh(@Body request: RefreshRequest): TokenResponse

    @POST("api/v1/auth/logout")
    suspend fun logout(@Body request: LogoutRequest)

    @GET("api/v1/auth/me")
    suspend fun me(@Header("Authorization") bearerToken: String): UserDto
}
