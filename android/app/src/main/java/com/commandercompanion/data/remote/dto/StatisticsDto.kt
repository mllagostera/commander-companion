package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs de `/statistics`, siguiendo `UserStatsResponse`/`DeckStatsResponse`/`PlaygroupStatsResponse`
 * de `docs/api/openapi.yaml`.
 *
 * El backend devuelve ceros (no 404) cuando el usuario/deck todavía no terminó ninguna partida,
 * así que no hace falta modelar "sin datos" como un caso aparte.
 */

@Serializable
data class UserStatsDto(
    @SerialName("user_id") val userId: String,
    @SerialName("games_played") val gamesPlayed: Int = 0,
    @SerialName("games_won") val gamesWon: Int = 0,
    @SerialName("total_damage_dealt") val totalDamageDealt: Int = 0,
    @SerialName("total_commander_damage_dealt") val totalCommanderDamageDealt: Int = 0,
    @SerialName("total_eliminations") val totalEliminations: Int = 0
)

@Serializable
data class DeckStatsDto(
    @SerialName("deck_id") val deckId: String,
    @SerialName("games_played") val gamesPlayed: Int = 0,
    @SerialName("games_won") val gamesWon: Int = 0,
    @SerialName("highest_life_total_achieved") val highestLifeTotalAchieved: Int = 0,
    @SerialName("total_commander_damage_dealt") val totalCommanderDamageDealt: Int = 0
)

@Serializable
data class PlaygroupStatsDto(
    @SerialName("playgroup_id") val playgroupId: String,
    @SerialName("games_played") val gamesPlayed: Int = 0,
    val members: List<PlaygroupMemberStatsDto> = emptyList()
)

@Serializable
data class PlaygroupMemberStatsDto(
    @SerialName("user_id") val userId: String,
    @SerialName("games_played") val gamesPlayed: Int = 0,
    @SerialName("games_won") val gamesWon: Int = 0
)
