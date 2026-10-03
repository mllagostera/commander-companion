package com.vansid.tapeandocartones.data.repository

import com.vansid.tapeandocartones.core.util.apiCall
import com.vansid.tapeandocartones.data.remote.api.CommanderApi
import com.vansid.tapeandocartones.data.remote.dto.SendFriendRequestRequest
import com.vansid.tapeandocartones.domain.model.Friend
import com.vansid.tapeandocartones.domain.model.FriendRequestResult
import com.vansid.tapeandocartones.domain.model.IncomingFriendRequest
import com.vansid.tapeandocartones.domain.model.OutgoingFriendRequest
import com.vansid.tapeandocartones.domain.model.UserSearchResult
import com.vansid.tapeandocartones.domain.repository.FriendsRepository
import javax.inject.Inject

/** Minimum query length `GET /users/search` accepts; below it the backend answers 400. */
private const val MIN_SEARCH_LENGTH = 2

/** [FriendsRepository] implementation. */
class FriendsRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : FriendsRepository {

    override suspend fun listFriends(): Result<List<Friend>> = apiCall { api.listFriends() }

    override suspend fun listIncomingRequests(): Result<List<IncomingFriendRequest>> =
        apiCall { api.listIncomingFriendRequests() }

    override suspend fun listOutgoingRequests(): Result<List<OutgoingFriendRequest>> =
        apiCall { api.listOutgoingFriendRequests() }

    override suspend fun countIncomingRequests(): Result<Int> =
        apiCall { api.countIncomingFriendRequests().incoming }

    override suspend fun sendRequest(userId: String): Result<FriendRequestResult> =
        apiCall { api.sendFriendRequest(SendFriendRequestRequest(addresseeId = userId)) }

    override suspend fun acceptRequest(requestId: String): Result<Friend> =
        apiCall { api.acceptFriendRequest(requestId) }

    override suspend fun rejectRequest(requestId: String): Result<Unit> =
        apiCall { api.rejectFriendRequest(requestId) }

    override suspend fun cancelRequest(requestId: String): Result<Unit> =
        apiCall { api.cancelFriendRequest(requestId) }

    override suspend fun removeFriend(userId: String): Result<Unit> =
        apiCall { api.removeFriend(userId) }

    override suspend fun searchUsers(query: String): Result<List<UserSearchResult>> {
        val trimmed = query.trim()
        // Typing "a" is a normal keystroke on the way to a real query, not an
        // error worth showing: answer empty instead of letting the backend 400.
        if (trimmed.length < MIN_SEARCH_LENGTH) return Result.success(emptyList())
        return apiCall { api.searchUsers(trimmed) }
    }
}
