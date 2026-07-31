package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Cursor-based pagination wrapper for `GET /decks` and `GET /games`
 * (`DeckListResponse`/`GameListResponse` in `docs/api/openapi.yaml`).
 *
 * [nextCursor] is `null` on the last page. No consumer in this repo paginates yet —
 * [items] is always read in full, as if the whole list fit on one page — so fetching
 * the next page is left for when a UI that needs it exists.
 */
@Serializable
data class PagedResponse<T>(
    val items: List<T>,
    @SerialName("next_cursor") val nextCursor: String? = null
)
