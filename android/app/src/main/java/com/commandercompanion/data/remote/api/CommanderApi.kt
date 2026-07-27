package com.commandercompanion.data.remote.api

import com.commandercompanion.data.remote.dto.CreateActionRequest
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.CreateGameRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.GameActionDto
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.GamePlayerDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import com.commandercompanion.data.remote.dto.JoinGameRequest
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path

/**
 * Endpoints protegidos del backend (`decks`, `games`, `game-actions`, `statistics`),
 * ver `docs/api/openapi.yaml`.
 *
 * Se consume siempre a través del cliente HTTP **autenticado** (ver `NetworkModule`): el
 * `AuthInterceptor` adjunta el Bearer y el `AuthAuthenticator` refresca ante un 401. Los endpoints
 * públicos de sesión viven aparte, en [AuthApi].
 *
 * No se exponen `Response<T>` ni `Call<T>`: los repositorios de `data/repository/` envuelven cada
 * llamada con `apiCall { }`, que traduce `HttpException`/`IOException` a `ApiError`.
 */
interface CommanderApi {

    @GET("api/v1/health")
    suspend fun checkHealth(): String

    // ---------------------------------------------------------------- decks

    @GET("api/v1/decks")
    suspend fun listDecks(): List<DeckDto>

    @POST("api/v1/decks")
    suspend fun createDeck(@Body request: CreateDeckRequest): DeckDto

    @GET("api/v1/decks/{id}")
    suspend fun getDeck(@Path("id") deckId: String): DeckDto

    @DELETE("api/v1/decks/{id}")
    suspend fun deleteDeck(@Path("id") deckId: String)

    @POST("api/v1/decks/import/moxfield")
    suspend fun importMoxfieldDeck(@Body request: ImportMoxfieldRequest): DeckDto

    // ---------------------------------------------------------------- games

    @GET("api/v1/games")
    suspend fun listGames(): List<GameDto>

    @POST("api/v1/games")
    suspend fun createGame(@Body request: CreateGameRequest): GameDto

    @GET("api/v1/games/{id}")
    suspend fun getGame(@Path("id") gameId: String): GameDto

    /** Sienta al usuario autenticado en la partida; devuelve su `GamePlayer` (no el `Game`). */
    @POST("api/v1/games/{id}/join")
    suspend fun joinGame(@Path("id") gameId: String, @Body request: JoinGameRequest): GamePlayerDto

    /** 204 sin body. Solo válido mientras la partida siga en `pending`. */
    @POST("api/v1/games/{id}/leave")
    suspend fun leaveGame(@Path("id") gameId: String)

    /** 409 si la partida no está en `pending` o todavía no hay 2 jugadores. */
    @POST("api/v1/games/{id}/start")
    suspend fun startGame(@Path("id") gameId: String): GameDto

    /** 409 si la partida no está `active`. Dispara el recálculo de estadísticas server-side. */
    @POST("api/v1/games/{id}/finish")
    suspend fun finishGame(@Path("id") gameId: String): GameDto

    // --------------------------------------------------------- game-actions

    /** 409 si la partida no está `active`; el backend aplica la acción al estado del jugador. */
    @POST("api/v1/games/{id}/actions")
    suspend fun recordAction(
        @Path("id") gameId: String,
        @Body request: CreateActionRequest
    ): GameActionDto

    @GET("api/v1/games/{id}/timeline")
    suspend fun getTimeline(@Path("id") gameId: String): List<GameActionDto>

    // ----------------------------------------------------------- statistics

    @GET("api/v1/statistics/user")
    suspend fun getUserStats(): UserStatsDto

    @GET("api/v1/statistics/deck/{id}")
    suspend fun getDeckStats(@Path("id") deckId: String): DeckStatsDto

    @GET("api/v1/statistics/playgroup/{id}")
    suspend fun getPlaygroupStats(@Path("id") playgroupId: String): PlaygroupStatsDto
}
