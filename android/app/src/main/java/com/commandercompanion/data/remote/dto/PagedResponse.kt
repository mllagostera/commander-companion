package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Cursor-based pagination wrapper for `GET /decks` and `GET /games`
 * (`DeckListResponse`/`GameListResponse` in `docs/api/openapi.yaml`).
 *
 * [nextCursor] is `null` on the last page. [com.commandercompanion.data.repository.DeckRepositoryImpl.listDecks]
 * and [com.commandercompanion.data.repository.GameRepositoryImpl.listGames] both follow it until
 * exhausted. `listGamesForPlaygroup` never needs to: the backend returns that group's full,
 * unpaginated history in one response (`next_cursor` always null) when `playgroup_id` is given.
 */
@Serializable
data class PagedResponse<T>(
    val items: List<T>,
    @SerialName("next_cursor") val nextCursor: String? = null
)
