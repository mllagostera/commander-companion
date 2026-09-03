package com.commandercompanion.domain.repository

import com.commandercompanion.domain.model.Friend
import com.commandercompanion.domain.model.FriendRequestResult
import com.commandercompanion.domain.model.IncomingFriendRequest
import com.commandercompanion.domain.model.OutgoingFriendRequest
import com.commandercompanion.domain.model.UserSearchResult

/**
 * Friends and their request lifecycle (`/friends`, see ADR-0017).
 *
 * There is no local cache here, unlike [GameRepository]: friendship is not
 * needed at the table mid-game, so a Room mirror would be state to invalidate
 * for no benefit. Every call goes to the network and returns `Result`.
 */
interface FriendsRepository {

    /** Accepted friendships, already resolved to the other user. */
    suspend fun listFriends(): Result<List<Friend>>

    suspend fun listIncomingRequests(): Result<List<IncomingFriendRequest>>

    suspend fun listOutgoingRequests(): Result<List<OutgoingFriendRequest>>

    /**
     * Sends a request to [userId], whether it came from a username search or
     * from a scanned QR — both entry points end here.
     *
     * Check [FriendRequestResult.wasAutoAccepted] on success: if the other user
     * had already sent a request the other way, this *is* the friendship now.
     */
    suspend fun sendRequest(userId: String): Result<FriendRequestResult>

    /** Returns the resulting friendship, not the updated request. */
    suspend fun acceptRequest(requestId: String): Result<Friend>

    suspend fun rejectRequest(requestId: String): Result<Unit>

    /** Withdraws a request this user sent. */
    suspend fun cancelRequest(requestId: String): Result<Unit>

    /** Takes the other user's id, not a request id. */
    suspend fun removeFriend(userId: String): Result<Unit>

    /**
     * Username search for the "add by name" path. The backend requires at
     * least 2 characters and caps results at 10; shorter queries short-circuit
     * to an empty list here rather than spending a request that would 400.
     */
    suspend fun searchUsers(query: String): Result<List<UserSearchResult>>
}
