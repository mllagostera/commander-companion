package com.commandercompanion.domain.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Cursor-based pagination wrapper (`DeckListResponse`/`GameListResponse` in
 * `docs/api/openapi.yaml`).
 *
 * [nextCursor] is `null` on the last page. `DeckRepositoryImpl.listDecks` and
 * `GameRepositoryImpl.listGames` both follow it until exhausted.
 * `listGamesForPlaygroup` never needs to: the backend returns that group's full,
 * unpaginated history in one response (`next_cursor` always null) when
 * `playgroup_id` is given.
 */
@Serializable
data class Page<T>(
    val items: List<T>,
    @SerialName("next_cursor") val nextCursor: String? = null
)
