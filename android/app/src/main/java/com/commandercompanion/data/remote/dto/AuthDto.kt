package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for `/auth`, in their own file alongside [com.commandercompanion.data.remote.api.AuthApi]
 * and separate from any DTO another agent might add for `CommanderApi` (decks/games/etc.),
 * following the same schemas from `docs/api/openapi.yaml`.
 */

@Serializable
data class LoginRequest(
    val email: String,
    val password: String
)

@Serializable
data class RegisterRequest(
    val username: String,
    val email: String,
    val password: String
)

@Serializable
data class GoogleLoginRequest(
    @SerialName("id_token") val idToken: String
)

@Serializable
data class RefreshRequest(
    @SerialName("refresh_token") val refreshToken: String
)

@Serializable
data class LogoutRequest(
    @SerialName("refresh_token") val refreshToken: String
)

@Serializable
data class TokenResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
    @SerialName("token_type") val tokenType: String = "Bearer",
    @SerialName("expires_in") val expiresIn: Long,
    val user: UserDto? = null
)

@Serializable
data class UserDto(
    val id: String,
    val username: String,
    val email: String,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("moxfield_username") val moxfieldUsername: String? = null,
    /** false = account created via Google Sign-In, without its own password. */
    @SerialName("has_password") val hasPassword: Boolean = true
)

/**
 * Payload for `PATCH /users/{id}` (see `SettingsViewModel`). Both fields are optional and
 * omitted if not sent: the backend doesn't touch what isn't in the body (see
 * `backend/internal/users/dto.go: UpdateProfileRequest`).
 */
@Serializable
data class UpdateProfileRequest(
    val username: String? = null,
    @SerialName("moxfield_username") val moxfieldUsername: String? = null
)

/** Payload for `POST /users/{id}/password`. */
@Serializable
data class ChangePasswordRequest(
    @SerialName("current_password") val currentPassword: String,
    @SerialName("new_password") val newPassword: String
)
