package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Request bodies for `/games`, following the `CreateGameRequest`/`JoinGameRequest`
 * schemas from `docs/api/openapi.yaml`.
 *
 * The response shapes (`Game`, `GamePlayer`, `GameStatus`) live in
 * `domain/model/Game.kt` — see `domain/model/Deck.kt` for why.
 */

@Serializable
data class CreateGameRequest(
    @SerialName("playgroup_id") val playgroupId: String? = null
)

/**
 * Without [userId] (or if it matches the authenticated user): normal join, the player is the
 * caller itself. With a different [userId]: proxy-join (see the backend's ADR-0013) — the
 * caller joins another user on their behalf; only authorized if both share the game's
 * playgroup, and [deckId] must belong to [userId], not the caller.
 */
@Serializable
data class JoinGameRequest(
    @SerialName("deck_id") val deckId: String,
    @SerialName("user_id") val userId: String? = null
)
