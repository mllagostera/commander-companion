package com.commandercompanion.data.remote.dto

import kotlinx.serialization.Serializable

/** Body of `GET /health` (`backend/internal/common/health.go`): `{"status": "ok", "db": "ok"}`. */
@Serializable
data class HealthDto(
    val status: String,
    val db: String
)
