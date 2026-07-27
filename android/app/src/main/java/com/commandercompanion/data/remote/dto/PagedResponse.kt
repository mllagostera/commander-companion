package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Envoltorio de paginación cursor-based de `GET /decks` y `GET /games`
 * (`DeckListResponse`/`GameListResponse` en `docs/api/openapi.yaml`).
 *
 * [nextCursor] es `null` en la última página. Ningún consumidor de este repo pagina todavía —
 * se lee siempre [items] entero, como si la lista completa cupiera en una página — así que pedir
 * la página siguiente queda para cuando exista una UI que la necesite.
 */
@Serializable
data class PagedResponse<T>(
    val items: List<T>,
    @SerialName("next_cursor") val nextCursor: String? = null
)
