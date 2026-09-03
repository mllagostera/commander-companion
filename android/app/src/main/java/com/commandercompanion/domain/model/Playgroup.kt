package com.commandercompanion.domain.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * A play group and its members, following the `Playgroup`/`PlaygroupMember`
 * schemas in `docs/api/openapi.yaml`. See [Deck] for why these live in
 * `domain/` while keeping their serialization annotations.
 */

@Serializable
data class Playgroup(
    val id: String,
    val name: String,
    val members: List<PlaygroupMember> = emptyList()
)

@Serializable
data class PlaygroupMember(
    @SerialName("playgroup_id") val playgroupId: String,
    @SerialName("user_id") val userId: String,
    val username: String
)
