package com.commandercompanion.data.remote.api

import com.commandercompanion.data.remote.dto.GoogleLoginRequest
import com.commandercompanion.data.remote.dto.LoginRequest
import com.commandercompanion.data.remote.dto.LogoutRequest
import com.commandercompanion.data.remote.dto.RefreshRequest
import com.commandercompanion.data.remote.dto.RegisterRequest
import com.commandercompanion.data.remote.dto.TokenResponse
import com.commandercompanion.data.remote.dto.UserDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST

/**
 * Authentication endpoints (`/auth`), see `docs/api/openapi.yaml`.
 *
 * Lives in its own file, separate from [CommanderApi], because another agent might be
 * extending `CommanderApi.kt` with decks/games/etc. endpoints in parallel — keeping them
 * separate avoids merge conflicts.
 *
 * `login`/`register`/`google`/`refresh`/`logout` are public (`security: []` in the spec, no
 * Bearer). `me` does require the access token, passed explicitly as a header instead of
 * relying on an interceptor — that way this API client doesn't need to know about the active session.
 */
interface AuthApi {

    @POST("api/v1/auth/login")
    suspend fun login(@Body request: LoginRequest): TokenResponse

    /**
     * Doesn't leave a session started (unlike [login]/[loginWithGoogle]): returns the created
     * user, not a [TokenResponse]. If the backend requires email verification
     * (`REQUIRE_EMAIL_VERIFICATION`), the subsequent login fails with 403 until it's confirmed — same
     * contract as the web client (see `web/app/composables/useAuth.ts`).
     */
    @POST("api/v1/auth/register")
    suspend fun register(@Body request: RegisterRequest): UserDto

    @POST("api/v1/auth/google")
    suspend fun loginWithGoogle(@Body request: GoogleLoginRequest): TokenResponse

    @POST("api/v1/auth/refresh")
    suspend fun refresh(@Body request: RefreshRequest): TokenResponse

    @POST("api/v1/auth/logout")
    suspend fun logout(@Body request: LogoutRequest)

    @GET("api/v1/auth/me")
    suspend fun me(@Header("Authorization") bearerToken: String): UserDto
}
