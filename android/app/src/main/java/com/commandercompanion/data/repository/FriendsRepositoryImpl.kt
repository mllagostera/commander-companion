package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.FriendDto
import com.commandercompanion.data.remote.dto.FriendRequestDto
import com.commandercompanion.data.remote.dto.IncomingFriendRequestDto
import com.commandercompanion.data.remote.dto.OutgoingFriendRequestDto
import com.commandercompanion.data.remote.dto.SendFriendRequestRequest
import com.commandercompanion.data.remote.dto.UserSearchResultDto
import com.commandercompanion.domain.repository.FriendsRepository
import javax.inject.Inject

/** Minimum query length `GET /users/search` accepts; below it the backend answers 400. */
private const val MIN_SEARCH_LENGTH = 2

/** [FriendsRepository] implementation. */
class FriendsRepositoryImpl @Inject constructor(
    private val api: CommanderApi
) : FriendsRepository {

    override suspend fun listFriends(): Result<List<FriendDto>> = apiCall { api.listFriends() }

    override suspend fun listIncomingRequests(): Result<List<IncomingFriendRequestDto>> =
        apiCall { api.listIncomingFriendRequests() }

    override suspend fun listOutgoingRequests(): Result<List<OutgoingFriendRequestDto>> =
        apiCall { api.listOutgoingFriendRequests() }

    override suspend fun sendRequest(userId: String): Result<FriendRequestDto> =
        apiCall { api.sendFriendRequest(SendFriendRequestRequest(addresseeId = userId)) }

    override suspend fun acceptRequest(requestId: String): Result<FriendDto> =
        apiCall { api.acceptFriendRequest(requestId) }

    override suspend fun rejectRequest(requestId: String): Result<Unit> =
        apiCall { api.rejectFriendRequest(requestId) }

    override suspend fun cancelRequest(requestId: String): Result<Unit> =
        apiCall { api.cancelFriendRequest(requestId) }

    override suspend fun removeFriend(userId: String): Result<Unit> =
        apiCall { api.removeFriend(userId) }

    override suspend fun searchUsers(query: String): Result<List<UserSearchResultDto>> {
        val trimmed = query.trim()
        // Typing "a" is a normal keystroke on the way to a real query, not an
        // error worth showing: answer empty instead of letting the backend 400.
        if (trimmed.length < MIN_SEARCH_LENGTH) return Result.success(emptyList())
        return apiCall { api.searchUsers(trimmed) }
    }
}
