package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for `/statistics`, following `UserStatsResponse`/`DeckStatsResponse`/`PlaygroupStatsResponse`
 * in `docs/api/openapi.yaml`.
 *
 * The backend returns zeros (not 404) when the user/deck hasn't finished any game yet,
 * so there's no need to model "no data" as a separate case.
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
