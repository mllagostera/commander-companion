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

/** DTO for one entry of `GET /statistics/opponents` -- the head-to-head record against one opponent. */
@Serializable
data class OpponentStatsDto(
    @SerialName("user_id") val userId: String,
    val username: String,
    @SerialName("games_together") val gamesTogether: Int = 0,
    @SerialName("times_you_eliminated_them") val timesYouEliminatedThem: Int = 0,
    @SerialName("times_eliminated_by_opponent") val timesEliminatedByOpponent: Int = 0
)

/** DTO for one entry of `GET /statistics/playgroups` -- replaces one `PlaygroupStatsDto` call per group. */
@Serializable
data class PlaygroupGameCountDto(
    @SerialName("playgroup_id") val playgroupId: String,
    @SerialName("playgroup_name") val playgroupName: String,
    @SerialName("games_played") val gamesPlayed: Int = 0
)

/** DTO for one item of `GET /statistics/games` (paginated via [PagedResponse]). */
@Serializable
data class FinishedGameDto(
    val id: String,
    @SerialName("playgroup_id") val playgroupId: String? = null,
    @SerialName("playgroup_name") val playgroupName: String? = null,
    @SerialName("started_at") val startedAt: String? = null,
    @SerialName("finished_at") val finishedAt: String? = null,
    val players: List<FinishedGamePlayerDto> = emptyList()
)

/** One seat within a [FinishedGameDto], already enriched with username/deck (no client-side lookup needed). */
@Serializable
data class FinishedGamePlayerDto(
    @SerialName("user_id") val userId: String,
    val username: String,
    @SerialName("deck_id") val deckId: String,
    @SerialName("deck_name") val deckName: String,
    @SerialName("deck_commander") val deckCommander: String,
    @SerialName("deck_image_url") val deckImageUrl: String? = null,
    val won: Boolean = false
)
