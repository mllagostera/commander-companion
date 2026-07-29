package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs de `/playgroups`, siguiendo los schemas `Playgroup`/`PlaygroupMember` de
 * `docs/api/openapi.yaml`.
 */

@Serializable
data class PlaygroupDto(
    val id: String,
    val name: String,
    val members: List<PlaygroupMemberDto> = emptyList()
)

@Serializable
data class PlaygroupMemberDto(
    @SerialName("playgroup_id") val playgroupId: String,
    @SerialName("user_id") val userId: String,
    val username: String
)
