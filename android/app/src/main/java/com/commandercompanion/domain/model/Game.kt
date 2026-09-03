package com.commandercompanion.domain.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * A backend game and its seats, following the `Game`/`GamePlayer` schemas in
 * `docs/api/openapi.yaml`.
 *
 * See [Deck] for why these live in `domain/` while keeping their serialization
 * annotations. The request bodies (`CreateGameRequest`, `JoinGameRequest`) stay
 * in `data/remote/dto/`.
 */

/** States of the `pending → active → finished` state machine applied by the backend server-side. */
object GameStatus {
    const val PENDING = "pending"
    const val ACTIVE = "active"
    const val FINISHED = "finished"
}

@Serializable
data class Game(
    val id: String,
    @SerialName("playgroup_id") val playgroupId: String? = null,
    val status: String,
    @SerialName("started_at") val startedAt: String? = null,
    @SerialName("finished_at") val finishedAt: String? = null,
    val players: List<GamePlayer> = emptyList()
)

@Serializable
data class GamePlayer(
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
