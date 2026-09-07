package com.commandercompanion.data.remote.dto

import kotlinx.serialization.Serializable

/**
 * Body of `GET /health` (`backend/internal/common/health.go`), of which this maps the half
 * the app cares about: `status` and `db`. The response also carries `commit` and `started_at`
 * (which build is answering, see ADR-0020) — deliberately not mapped, nothing on Android reads
 * them, and `NetworkModule`'s `ignoreUnknownKeys` drops them.
 */
@Serializable
data class HealthDto(
    val status: String,
    val db: String
)
