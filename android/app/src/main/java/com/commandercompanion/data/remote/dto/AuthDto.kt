package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs de `/auth`, en su propio archivo junto a [com.commandercompanion.data.remote.api.AuthApi]
 * y separados de cualquier DTO que otro agente agregue para `CommanderApi` (decks/games/etc.),
 * siguiendo los mismos schemas de `docs/api/openapi.yaml`.
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
    /** false = cuenta creada vía Google Sign-In, sin password propio. */
    @SerialName("has_password") val hasPassword: Boolean = true
)

/**
 * Payload de `PATCH /users/{id}` (ver `SettingsViewModel`). Ambos campos son opcionales y
 * omitidos si no se mandan: el backend no toca lo que no venga en el body (ver
 * `backend/internal/users/dto.go: UpdateProfileRequest`).
 */
@Serializable
data class UpdateProfileRequest(
    val username: String? = null,
    @SerialName("moxfield_username") val moxfieldUsername: String? = null
)

/** Payload de `POST /users/{id}/password`. */
@Serializable
data class ChangePasswordRequest(
    @SerialName("current_password") val currentPassword: String,
    @SerialName("new_password") val newPassword: String
)
