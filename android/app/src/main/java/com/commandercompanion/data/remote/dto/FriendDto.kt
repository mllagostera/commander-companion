package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Request bodies for `/friends`, mirroring `backend/internal/friends/dto.go`
 * and the `friends` section of `docs/api/openapi.yaml` (see ADR-0017).
 *
 * The response shapes (`Friend`, `FriendRequestResult`, the two request-list
 * entries, `UserSearchResult`) live in `domain/model/Friends.kt` — see
 * `domain/model/Deck.kt` for why.
 */

/** Body of `POST /friends/requests`. */
@Serializable
data class SendFriendRequestRequest(
    @SerialName("addressee_id") val addresseeId: String
)
