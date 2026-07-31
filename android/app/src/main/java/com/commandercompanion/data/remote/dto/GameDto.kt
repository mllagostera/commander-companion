package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for `/games`, following the `Game`/`GamePlayer`/`CreateGameRequest`/`JoinGameRequest`
 * schemas from `docs/api/openapi.yaml`.
 */

/** States of the `pending → active → finished` state machine applied by the backend server-side. */
object GameStatus {
    const val PENDING = "pending"
    const val ACTIVE = "active"
    const val FINISHED = "finished"
}

@Serializable
data class GameDto(
    val id: String,
    @SerialName("playgroup_id") val playgroupId: String? = null,
    val status: String,
    @SerialName("started_at") val startedAt: String? = null,
    @SerialName("finished_at") val finishedAt: String? = null,
    val players: List<GamePlayerDto> = emptyList()
)

@Serializable
data class GamePlayerDto(
    val id: String,
    @SerialName("game_id") val gameId: String,
    @SerialName("user_id") val userId: String,
    @SerialName("deck_id") val deckId: String,
    @SerialName("life_total") val lifeTotal: Int = 40,
    @SerialName("poison_counters") val poisonCounters: Int = 0,
    @SerialName("energy_counters") val energyCounters: Int = 0,
    @SerialName("experience_counters") val experienceCounters: Int = 0,
    @SerialName("is_eliminated") val isEliminated: Boolean = false
)

@Serializable
data class CreateGameRequest(
    @SerialName("playgroup_id") val playgroupId: String? = null
)

/**
 * Without [userId] (or if it matches the authenticated user): normal join, the player is the
 * caller itself. With a different [userId]: proxy-join (see the backend's ADR-0013) — the
 * caller joins another user on their behalf; only authorized if both share the game's
 * playgroup, and [deckId] must belong to [userId], not the caller.
 */
@Serializable
data class JoinGameRequest(
    @SerialName("deck_id") val deckId: String,
    @SerialName("user_id") val userId: String? = null
)
