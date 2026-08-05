package com.commandercompanion.data.remote.api

import com.commandercompanion.data.remote.dto.ChangePasswordRequest
import com.commandercompanion.data.remote.dto.CreateActionRequest
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.CreateGameRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.FinishedGameDto
import com.commandercompanion.data.remote.dto.GameActionDto
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.GamePlayerDto
import com.commandercompanion.data.remote.dto.HealthDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import com.commandercompanion.data.remote.dto.JoinGameRequest
import com.commandercompanion.data.remote.dto.OpponentStatsDto
import com.commandercompanion.data.remote.dto.PagedResponse
import com.commandercompanion.data.remote.dto.PlaygroupDto
import com.commandercompanion.data.remote.dto.PlaygroupGameCountDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UpdateProfileRequest
import com.commandercompanion.data.remote.dto.UserDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Protected backend endpoints (`decks`, `games`, `game-actions`, `statistics`),
 * see `docs/api/openapi.yaml`.
 *
 * Always consumed through the **authenticated** HTTP client (see `NetworkModule`): the
 * `AuthInterceptor` attaches the Bearer and the `AuthAuthenticator` refreshes on a 401. The
 * public session endpoints live separately, in [AuthApi].
 *
 * `Response<T>`/`Call<T>` are not exposed: the repositories in `data/repository/` wrap each
 * call with `apiCall { }`, which translates `HttpException`/`IOException` into `ApiError`.
 */
interface CommanderApi {

    /** `/health` lives outside `/api/v1` and without auth (see `common.RegisterHealthRoute`). */
    @GET("health")
    suspend fun checkHealth(): HealthDto

    // ---------------------------------------------------------------- decks

    /**
     * Server-side cursor-based pagination (default 20 items/page). Pass the previous page's
     * `next_cursor` to get the next one; omit it for the first page. [DeckRepositoryImpl.listDecks]
     * follows it until exhausted — this raw call only fetches one page.
     */
    @GET("api/v1/decks")
    suspend fun listDecks(@Query("cursor") cursor: String? = null): PagedResponse<DeckDto>

    @POST("api/v1/decks")
    suspend fun createDeck(@Body request: CreateDeckRequest): DeckDto

    @GET("api/v1/decks/{id}")
    suspend fun getDeck(@Path("id") deckId: String): DeckDto

    @DELETE("api/v1/decks/{id}")
    suspend fun deleteDeck(@Path("id") deckId: String)

    @POST("api/v1/decks/import/moxfield")
    suspend fun importMoxfieldDeck(@Body request: ImportMoxfieldRequest): DeckDto

    // ---------------------------------------------------------------- games

    /**
     * Server-side cursor-based pagination (default 20 items/page), same shape as [listDecks].
     * [GameRepositoryImpl.listGames] follows [PagedResponse.nextCursor] until exhausted — this
     * raw call only fetches one page. Currently unused by any screen (`HistoryScreen` reads
     * local Room history instead, see `HistoryViewModel`), kept correct for whenever a
     * cross-playgroup history view needs it.
     */
    @GET("api/v1/games")
    suspend fun listGames(@Query("cursor") cursor: String? = null): PagedResponse<GameDto>

    /**
     * Full (unpaginated) history of a group's games — same `games` list endpoint, but scoped to a
     * `playgroup_id` (`ListGamesForPlaygroup` on the backend). Includes games in every status;
     * callers looking for open seats filter to [GameStatus.PENDING] themselves (see
     * `JoinGameViewModel`). Shares [GameDto]/[PagedResponse]'s shape with [listGames], the backend
     * responds the exact same `{items, next_cursor}` envelope for both.
     */
    @GET("api/v1/games")
    suspend fun listGamesForPlaygroup(@Query("playgroup_id") playgroupId: String): PagedResponse<GameDto>

    @POST("api/v1/games")
    suspend fun createGame(@Body request: CreateGameRequest): GameDto

    @GET("api/v1/games/{id}")
    suspend fun getGame(@Path("id") gameId: String): GameDto

    /** Seats the authenticated user in the game; returns their `GamePlayer` (not the `Game`). */
    @POST("api/v1/games/{id}/join")
    suspend fun joinGame(@Path("id") gameId: String, @Body request: JoinGameRequest): GamePlayerDto

    /** 204 with no body. Only valid while the game is still `pending`. */
    @POST("api/v1/games/{id}/leave")
    suspend fun leaveGame(@Path("id") gameId: String)

    /** 409 if the game isn't `pending` or there aren't yet 2 players. */
    @POST("api/v1/games/{id}/start")
    suspend fun startGame(@Path("id") gameId: String): GameDto

    /** 409 if the game isn't `active`. Triggers the server-side statistics recalculation. */
    @POST("api/v1/games/{id}/finish")
    suspend fun finishGame(@Path("id") gameId: String): GameDto

    // --------------------------------------------------------- game-actions

    /** 409 if the game isn't `active`; the backend applies the action to the player's state. */
    @POST("api/v1/games/{id}/actions")
    suspend fun recordAction(
        @Path("id") gameId: String,
        @Body request: CreateActionRequest
    ): GameActionDto

    @GET("api/v1/games/{id}/timeline")
    suspend fun getTimeline(@Path("id") gameId: String): List<GameActionDto>

    // ------------------------------------------------------------ playgroups

    /** Groups the authenticated user is a member of, with their members populated. */
    @GET("api/v1/playgroups")
    suspend fun listPlaygroups(): List<PlaygroupDto>

    @GET("api/v1/playgroups/{id}")
    suspend fun getPlaygroup(@Path("id") playgroupId: String): PlaygroupDto

    /** Decks of ANOTHER member of the same group — to choose their deck for a proxy-join. */
    @GET("api/v1/playgroups/{id}/members/{userId}/decks")
    suspend fun getMemberDecks(
        @Path("id") playgroupId: String,
        @Path("userId") userId: String
    ): List<DeckDto>

    // ----------------------------------------------------------- statistics

    @GET("api/v1/statistics/user")
    suspend fun getUserStats(): UserStatsDto

    @GET("api/v1/statistics/deck/{id}")
    suspend fun getDeckStats(@Path("id") deckId: String): DeckStatsDto

    @GET("api/v1/statistics/playgroup/{id}")
    suspend fun getPlaygroupStats(@Path("id") playgroupId: String): PlaygroupStatsDto

    /** Every playgroup the user belongs to, with its games_played count -- replaces one [getPlaygroupStats] call per group. */
    @GET("api/v1/statistics/playgroups")
    suspend fun listPlaygroupGameCounts(): List<PlaygroupGameCountDto>

    /** Head-to-head record against every opponent the user has shared a finished game with. */
    @GET("api/v1/statistics/opponents")
    suspend fun getOpponentStats(): List<OpponentStatsDto>

    /** Same cursor-pagination shape as [listGames], enriched per player (see [FinishedGameDto]). */
    @GET("api/v1/statistics/games")
    suspend fun listFinishedGames(@Query("cursor") cursor: String? = null): PagedResponse<FinishedGameDto>

    // ----------------------------------------------------------------- users

    /** 409 if the username is already in use by another account. */
    @PATCH("api/v1/users/{id}")
    suspend fun updateProfile(@Path("id") userId: String, @Body request: UpdateProfileRequest): UserDto

    /** 204 with no body. 401 if the current password doesn't match, or if the account has no password of its own. */
    @POST("api/v1/users/{id}/password")
    suspend fun changePassword(@Path("id") userId: String, @Body request: ChangePasswordRequest)
}
