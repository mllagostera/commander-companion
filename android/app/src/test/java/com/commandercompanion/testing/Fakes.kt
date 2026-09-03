package com.commandercompanion.testing

import com.commandercompanion.data.local.dao.DeckDao
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.DeckEntity
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.data.local.entity.PlayerResultEntity
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.ChangePasswordRequest
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.CreateGameRequest
import com.commandercompanion.data.remote.dto.HealthDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import com.commandercompanion.data.remote.dto.JoinGameRequest
import com.commandercompanion.data.remote.dto.SendFriendRequestRequest
import com.commandercompanion.data.remote.dto.UpdateProfileRequest
import com.commandercompanion.data.remote.dto.UserDto
import com.commandercompanion.data.remote.ws.GameSocketClient
import com.commandercompanion.domain.model.Deck
import com.commandercompanion.domain.model.DeckStats
import com.commandercompanion.domain.model.FinishedGame
import com.commandercompanion.domain.model.FinishedGamePlayer
import com.commandercompanion.domain.model.Friend
import com.commandercompanion.domain.model.FriendRequestResult
import com.commandercompanion.domain.model.Game
import com.commandercompanion.domain.model.GameAction
import com.commandercompanion.domain.model.GamePlayer
import com.commandercompanion.domain.model.GameSocketEvent
import com.commandercompanion.domain.model.GameStatus
import com.commandercompanion.domain.model.IncomingFriendRequest
import com.commandercompanion.domain.model.NewGameAction
import com.commandercompanion.domain.model.OpponentStats
import com.commandercompanion.domain.model.OutgoingFriendRequest
import com.commandercompanion.domain.model.Page
import com.commandercompanion.domain.model.Playgroup
import com.commandercompanion.domain.model.PlaygroupGameCount
import com.commandercompanion.domain.model.PlaygroupMember
import com.commandercompanion.domain.model.PlaygroupStats
import com.commandercompanion.domain.model.UserSearchResult
import com.commandercompanion.domain.model.UserStats
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.emptyFlow
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import retrofit2.HttpException
import retrofit2.Response

/** Builds a real [HttpException] with the given code, to test error mapping. */
fun httpException(code: Int, body: String = """{"message":"boom"}"""): HttpException =
    HttpException(Response.error<Any>(code, body.toResponseBody("application/json".toMediaType())))

// --------------------------------------------------------------------- fixture DTOs

fun deckDto(id: String = "deck-1", name: String = "Atraxa", imageUrl: String? = null) = Deck(
    id = id,
    userId = "user-1",
    name = name,
    commander = "Atraxa, Praetors' Voice",
    moxfieldId = null,
    imageUrl = imageUrl
)

fun gameDto(
    id: String = "game-1",
    status: String = GameStatus.PENDING,
    playgroupId: String? = null,
    players: List<GamePlayer> = emptyList()
) = Game(
    id = id,
    playgroupId = playgroupId,
    status = status,
    players = players
)

fun gamePlayerDto(id: String = "gp-1", gameId: String = "game-1", userId: String = "user-1", deckId: String = "deck-1") =
    GamePlayer(id = id, gameId = gameId, userId = userId, deckId = deckId)

fun playgroupMemberDto(playgroupId: String = "playgroup-1", userId: String = "user-1", username: String = "user-1") =
    PlaygroupMember(playgroupId = playgroupId, userId = userId, username = username)

fun playgroupDto(
    id: String = "playgroup-1",
    name: String = "Grupo de test",
    members: List<PlaygroupMember> = listOf(playgroupMemberDto(playgroupId = id))
) = Playgroup(id = id, name = name, members = members)

fun userDto(
    id: String = "user-1",
    username: String = "user-1",
    email: String = "user-1@example.com",
    moxfieldUsername: String? = null,
    hasPassword: Boolean = true
) = UserDto(
    id = id,
    username = username,
    email = email,
    moxfieldUsername = moxfieldUsername,
    hasPassword = hasPassword
)

private const val TEST_TIMESTAMP = "2026-08-16T10:00:00Z"

fun friendDto(id: String = "user-2", username: String = "ana") =
    Friend(id = id, username = username, friendsSince = TEST_TIMESTAMP)

fun friendRequestDto(
    id: String = "req-1",
    addresseeId: String = "user-2",
    addresseeUsername: String = "ana",
    status: String = FriendRequestResult.STATUS_PENDING
) = FriendRequestResult(
    id = id,
    addresseeId = addresseeId,
    addresseeUsername = addresseeUsername,
    status = status,
    createdAt = TEST_TIMESTAMP
)

fun incomingFriendRequestDto(
    id: String = "req-in-1",
    requesterId: String = "user-3",
    requesterUsername: String = "bruno"
) = IncomingFriendRequest(
    id = id,
    requesterId = requesterId,
    requesterUsername = requesterUsername,
    createdAt = TEST_TIMESTAMP
)

fun outgoingFriendRequestDto(
    id: String = "req-out-1",
    addresseeId: String = "user-4",
    addresseeUsername: String = "carla"
) = OutgoingFriendRequest(
    id = id,
    addresseeId = addresseeId,
    addresseeUsername = addresseeUsername,
    createdAt = TEST_TIMESTAMP
)

fun userSearchResultDto(id: String = "user-2", username: String = "ana") =
    UserSearchResult(id = id, username = username)

fun opponentStatsDto(
    userId: String = "user-2",
    username: String = "user-2",
    gamesTogether: Int = 1,
    timesYouEliminatedThem: Int = 0,
    timesEliminatedByOpponent: Int = 0
) = OpponentStats(
    userId = userId,
    username = username,
    gamesTogether = gamesTogether,
    timesYouEliminatedThem = timesYouEliminatedThem,
    timesEliminatedByOpponent = timesEliminatedByOpponent
)

fun playgroupGameCountDto(playgroupId: String = "playgroup-1", playgroupName: String = "Grupo de test", gamesPlayed: Int = 1) =
    PlaygroupGameCount(playgroupId = playgroupId, playgroupName = playgroupName, gamesPlayed = gamesPlayed)

fun finishedGamePlayerDto(
    userId: String = "user-1",
    username: String = "user-1",
    deckId: String = "deck-1",
    deckName: String = "Atraxa",
    won: Boolean = true
) = FinishedGamePlayer(
    userId = userId,
    username = username,
    deckId = deckId,
    deckName = deckName,
    deckCommander = "Atraxa, Praetors' Voice",
    won = won
)

fun finishedGameDto(
    id: String = "game-1",
    playgroupId: String? = null,
    players: List<FinishedGamePlayer> = listOf(finishedGamePlayerDto())
) = FinishedGame(
    id = id,
    playgroupId = playgroupId,
    finishedAt = "2026-07-27T10:00:00Z",
    players = players
)

fun gameActionDto(gameId: String, request: NewGameAction) = GameAction(
    id = "action-1",
    gameId = gameId,
    actorId = request.actorId,
    targetId = request.targetId,
    actionType = request.actionType,
    payload = request.payload,
    createdAt = "2026-07-27T10:00:00Z"
)

/**
 * Fake of [CommanderApi] with one handler per endpoint.
 *
 * Preferred over a mocking library: the interface is stable, so tests explicitly declare
 * what each endpoint responds with, without magic `verify` calls.
 */
class FakeCommanderApi : CommanderApi {

    /** All the actions received by `POST /games/{id}/actions`, in order. */
    val recordedActions = mutableListOf<Pair<String, NewGameAction>>()

    /** Names of the endpoints invoked, in order — to assert the bootstrap sequence. */
    val calls = mutableListOf<String>()

    var onListDecks: suspend (cursor: String?) -> Page<Deck> = { Page(items = listOf(deckDto())) }
    var onListGames: suspend (cursor: String?) -> Page<Game> = { Page(items = listOf(gameDto())) }
    var onListGamesForPlaygroup: suspend (String) -> List<Game> = { emptyList() }
    var onImportMoxfield: suspend (ImportMoxfieldRequest) -> Deck = { deckDto() }
    var onDeleteDeck: suspend (String) -> Unit = { }
    var onCreateGame: suspend (CreateGameRequest) -> Game = { gameDto() }
    var onJoinGame: suspend (String, JoinGameRequest) -> GamePlayer = { gameId, request ->
        gamePlayerDto(gameId = gameId, userId = request.userId ?: "user-1", deckId = request.deckId)
    }
    var onStartGame: suspend (String) -> Game = { id -> gameDto(id, GameStatus.ACTIVE) }
    var onFinishGame: suspend (String) -> Game = { id -> gameDto(id, GameStatus.FINISHED) }
    var onRecordAction: suspend (String, NewGameAction) -> GameAction = { id, request ->
        gameActionDto(id, request)
    }
    var onGetGame: suspend (String) -> Game = { id -> gameDto(id) }
    var onGetTimeline: suspend (String) -> List<GameAction> = { emptyList() }
    var onListPlaygroups: suspend () -> List<Playgroup> = { emptyList() }
    var onGetPlaygroup: suspend (String) -> Playgroup = { id -> playgroupDto(id = id) }
    var onGetMemberDecks: suspend (String, String) -> List<Deck> = { _, _ -> emptyList() }
    var onUpdateProfile: suspend (String, UpdateProfileRequest) -> UserDto = { id, request ->
        userDto(id = id, username = request.username ?: "user-1", moxfieldUsername = request.moxfieldUsername)
    }
    var onChangePassword: suspend (String, ChangePasswordRequest) -> Unit = { _, _ -> }
    var onGetUserStats: suspend () -> UserStats = { UserStats(userId = "user-1") }
    var onGetDeckStats: suspend (String) -> DeckStats = { id -> DeckStats(deckId = id) }
    var onGetPlaygroupStats: suspend (String) -> PlaygroupStats = { id -> PlaygroupStats(playgroupId = id) }
    var onListPlaygroupGameCounts: suspend () -> List<PlaygroupGameCount> = { emptyList() }
    var onGetOpponentStats: suspend () -> List<OpponentStats> = { emptyList() }
    var onListFinishedGames: suspend (cursor: String?) -> Page<FinishedGame> = { Page(items = emptyList()) }
    var onSearchUsers: suspend (String) -> List<UserSearchResult> = { emptyList() }
    var onListFriends: suspend () -> List<Friend> = { emptyList() }
    var onListIncomingFriendRequests: suspend () -> List<IncomingFriendRequest> = { emptyList() }
    var onListOutgoingFriendRequests: suspend () -> List<OutgoingFriendRequest> = { emptyList() }
    var onSendFriendRequest: suspend (SendFriendRequestRequest) -> FriendRequestResult = { request ->
        friendRequestDto(addresseeId = request.addresseeId)
    }
    var onAcceptFriendRequest: suspend (String) -> Friend = { friendDto() }
    var onRejectFriendRequest: suspend (String) -> Unit = { }
    var onCancelFriendRequest: suspend (String) -> Unit = { }
    var onRemoveFriend: suspend (String) -> Unit = { }

    override suspend fun checkHealth(): HealthDto = HealthDto(status = "ok", db = "ok")

    override suspend fun listDecks(cursor: String?): Page<Deck> {
        calls += "listDecks"
        return onListDecks(cursor)
    }

    override suspend fun createDeck(request: CreateDeckRequest): Deck =
        deckDto(name = request.name)

    override suspend fun getDeck(deckId: String): Deck = deckDto(id = deckId)

    override suspend fun deleteDeck(deckId: String) {
        calls += "deleteDeck"
        onDeleteDeck(deckId)
    }

    override suspend fun importMoxfieldDeck(request: ImportMoxfieldRequest): Deck {
        calls += "importMoxfieldDeck"
        return onImportMoxfield(request)
    }

    override suspend fun listGames(cursor: String?): Page<Game> {
        calls += "listGames"
        return onListGames(cursor)
    }

    override suspend fun listGamesForPlaygroup(playgroupId: String): Page<Game> {
        calls += "listGamesForPlaygroup"
        return Page(items = onListGamesForPlaygroup(playgroupId))
    }

    override suspend fun createGame(request: CreateGameRequest): Game {
        calls += "createGame"
        return onCreateGame(request)
    }

    override suspend fun getGame(gameId: String): Game {
        calls += "getGame"
        return onGetGame(gameId)
    }

    override suspend fun joinGame(gameId: String, request: JoinGameRequest): GamePlayer {
        calls += "joinGame"
        return onJoinGame(gameId, request)
    }

    override suspend fun leaveGame(gameId: String) {
        calls += "leaveGame"
    }

    override suspend fun startGame(gameId: String): Game {
        calls += "startGame"
        return onStartGame(gameId)
    }

    override suspend fun finishGame(gameId: String): Game {
        calls += "finishGame"
        return onFinishGame(gameId)
    }

    override suspend fun recordAction(
        gameId: String,
        request: NewGameAction
    ): GameAction {
        calls += "recordAction"
        recordedActions += gameId to request
        return onRecordAction(gameId, request)
    }

    override suspend fun getTimeline(gameId: String): List<GameAction> {
        calls += "getTimeline"
        return onGetTimeline(gameId)
    }

    override suspend fun listPlaygroups(): List<Playgroup> {
        calls += "listPlaygroups"
        return onListPlaygroups()
    }

    override suspend fun getPlaygroup(playgroupId: String): Playgroup {
        calls += "getPlaygroup"
        return onGetPlaygroup(playgroupId)
    }

    override suspend fun getMemberDecks(playgroupId: String, userId: String): List<Deck> {
        calls += "getMemberDecks"
        return onGetMemberDecks(playgroupId, userId)
    }

    override suspend fun getUserStats(): UserStats {
        calls += "getUserStats"
        return onGetUserStats()
    }

    override suspend fun getDeckStats(deckId: String): DeckStats {
        calls += "getDeckStats"
        return onGetDeckStats(deckId)
    }

    override suspend fun getPlaygroupStats(playgroupId: String): PlaygroupStats {
        calls += "getPlaygroupStats"
        return onGetPlaygroupStats(playgroupId)
    }

    override suspend fun listPlaygroupGameCounts(): List<PlaygroupGameCount> {
        calls += "listPlaygroupGameCounts"
        return onListPlaygroupGameCounts()
    }

    override suspend fun getOpponentStats(): List<OpponentStats> {
        calls += "getOpponentStats"
        return onGetOpponentStats()
    }

    override suspend fun listFinishedGames(cursor: String?): Page<FinishedGame> {
        calls += "listFinishedGames"
        return onListFinishedGames(cursor)
    }

    override suspend fun updateProfile(userId: String, request: UpdateProfileRequest): UserDto {
        calls += "updateProfile"
        return onUpdateProfile(userId, request)
    }

    override suspend fun changePassword(userId: String, request: ChangePasswordRequest) {
        calls += "changePassword"
        onChangePassword(userId, request)
    }

    override suspend fun searchUsers(query: String): List<UserSearchResult> {
        calls += "searchUsers"
        return onSearchUsers(query)
    }

    override suspend fun sendFriendRequest(request: SendFriendRequestRequest): FriendRequestResult {
        calls += "sendFriendRequest"
        return onSendFriendRequest(request)
    }

    override suspend fun listIncomingFriendRequests(direction: String): List<IncomingFriendRequest> {
        calls += "listIncomingFriendRequests"
        return onListIncomingFriendRequests()
    }

    override suspend fun listOutgoingFriendRequests(direction: String): List<OutgoingFriendRequest> {
        calls += "listOutgoingFriendRequests"
        return onListOutgoingFriendRequests()
    }

    override suspend fun acceptFriendRequest(requestId: String): Friend {
        calls += "acceptFriendRequest"
        return onAcceptFriendRequest(requestId)
    }

    override suspend fun rejectFriendRequest(requestId: String) {
        calls += "rejectFriendRequest"
        onRejectFriendRequest(requestId)
    }

    override suspend fun cancelFriendRequest(requestId: String) {
        calls += "cancelFriendRequest"
        onCancelFriendRequest(requestId)
    }

    override suspend fun listFriends(): List<Friend> {
        calls += "listFriends"
        return onListFriends()
    }

    override suspend fun removeFriend(userId: String) {
        calls += "removeFriend"
        onRemoveFriend(userId)
    }
}

/**
 * Fake of [GameSocketClient]: no real socket, no reconnection — just replays whatever [onConnect]
 * returns for the given `gameId`. Empty by default (never emits), which is enough for tests that
 * don't care about live-sync events.
 */
class FakeGameSocketClient : GameSocketClient {

    /** `gameId` of every [connect] call, in order. */
    val connectedGameIds = mutableListOf<String>()

    var onConnect: (gameId: String) -> Flow<GameSocketEvent> = { emptyFlow() }

    override fun connect(gameId: String, accessToken: suspend () -> String?): Flow<GameSocketEvent> {
        connectedGameIds += gameId
        return onConnect(gameId)
    }
}

/** In-memory fake of [GameDao]. */
class FakeGameDao : GameDao {

    val games = mutableListOf<GameEntity>()
    val players = mutableListOf<PlayerResultEntity>()
    val finished = mutableListOf<Triple<String, String, Long>>()
    val updatedResults = mutableListOf<PlayerResultUpdate>()

    data class PlayerResultUpdate(
        val gameId: String,
        val seatIndex: Int,
        val finalLife: Int,
        val won: Boolean
    )

    private val history = MutableStateFlow<List<GameWithPlayers>>(emptyList())

    override suspend fun insertGame(game: GameEntity) {
        games += game
    }

    override suspend fun insertPlayers(players: List<PlayerResultEntity>) {
        this.players += players
    }

    override suspend fun finishGame(gameId: String, status: String, endTime: Long) {
        finished += Triple(gameId, status, endTime)
    }

    override suspend fun updatePlayerResult(
        gameId: String,
        seatIndex: Int,
        finalLife: Int,
        won: Boolean
    ) {
        updatedResults += PlayerResultUpdate(gameId, seatIndex, finalLife, won)
    }

    override fun getGamesWithPlayers(): Flow<List<GameWithPlayers>> = history
}

/** In-memory fake of [DeckDao]. */
class FakeDeckDao : DeckDao {

    private val decks = mutableMapOf<String, DeckEntity>()

    override suspend fun getAll(): List<DeckEntity> = decks.values.toList()

    override suspend fun insert(deck: DeckEntity) {
        decks[deck.id] = deck
    }

    override suspend fun insertAll(decks: List<DeckEntity>) {
        decks.forEach { this.decks[it.id] = it }
    }

    override suspend fun clear() {
        decks.clear()
    }

    override suspend fun deleteById(deckId: String) {
        decks.remove(deckId)
    }
}
