package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs de `/games`, siguiendo los schemas `Game`/`GamePlayer`/`CreateGameRequest`/`JoinGameRequest`
 * de `docs/api/openapi.yaml`.
 */

/** Estados de la máquina `pending → active → finished` que aplica el backend server-side. */
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
 * El jugador que se une es SIEMPRE el usuario autenticado (el backend lo toma del JWT):
 * el body solo lleva el deck, que además debe pertenecerle.
 */
@Serializable
data class JoinGameRequest(
    @SerialName("deck_id") val deckId: String
)
