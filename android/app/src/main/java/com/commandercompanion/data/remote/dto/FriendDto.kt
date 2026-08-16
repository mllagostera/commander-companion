package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Friends DTOs, mirroring `backend/internal/friends/dto.go` and the `friends`
 * section of `docs/api/openapi.yaml`.
 *
 * One row in `friend_requests` represents the whole lifecycle: an `accepted`
 * row *is* the friendship (there is no separate `friends` table), which is why
 * accepting a request answers with a [FriendDto] rather than an updated
 * request.
 *
 * Timestamps stay as the raw ISO-8601 strings the API sends, same as
 * [UserDto.createdAt] — nothing in the app formats them yet.
 */

/** Body of `POST /friends/requests`. */
@Serializable
data class SendFriendRequestRequest(
    @SerialName("addressee_id") val addresseeId: String
)

/**
 * Result of `POST /friends/requests`.
 *
 * [status] is the field that matters: the backend **auto-accepts** when the
 * target had already sent a request in the opposite direction, so this comes
 * back `"accepted"` instead of `"pending"` and the UI has to say "you're now
 * friends" rather than "request sent" (see `Service.SendFriendRequest`).
 */
@Serializable
data class FriendRequestDto(
    val id: String,
    @SerialName("addressee_id") val addresseeId: String,
    @SerialName("addressee_username") val addresseeUsername: String,
    val status: String,
    @SerialName("created_at") val createdAt: String
) {
    val wasAutoAccepted: Boolean get() = status == STATUS_ACCEPTED

    companion object {
        const val STATUS_PENDING = "pending"
        const val STATUS_ACCEPTED = "accepted"
    }
}

/** An entry of `GET /friends/requests?direction=incoming`. */
@Serializable
data class IncomingFriendRequestDto(
    val id: String,
    @SerialName("requester_id") val requesterId: String,
    @SerialName("requester_username") val requesterUsername: String,
    @SerialName("created_at") val createdAt: String
)

/** An entry of `GET /friends/requests?direction=outgoing`. */
@Serializable
data class OutgoingFriendRequestDto(
    val id: String,
    @SerialName("addressee_id") val addresseeId: String,
    @SerialName("addressee_username") val addresseeUsername: String,
    @SerialName("created_at") val createdAt: String
)

/**
 * An accepted friendship, already resolved to the OTHER user regardless of who
 * sent the original request — [id] is that user's id, not the request's.
 */
@Serializable
data class FriendDto(
    val id: String,
    val username: String,
    @SerialName("friends_since") val friendsSince: String
)

/**
 * A `GET /users/search` hit. Deliberately without email, unlike [UserDto]:
 * these are results about *other* users.
 */
@Serializable
data class UserSearchResultDto(
    val id: String,
    val username: String
)
