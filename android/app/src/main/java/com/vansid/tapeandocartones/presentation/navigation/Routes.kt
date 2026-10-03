package com.vansid.tapeandocartones.presentation.navigation

import kotlinx.serialization.Serializable

@Serializable
object LoginRoute

@Serializable
object RegisterRoute

/** [email] prefills the field with whatever was typed on the login screen. */
@Serializable
data class ForgotPasswordRoute(val email: String = "")

@Serializable
object DashboardRoute

@Serializable
object PlayerSetupRoute

@Serializable
object HistoryRoute

@Serializable
object SettingsRoute

@Serializable
object StatisticsRoute

@Serializable
object FriendsRoute

@Serializable
object JoinGameRoute

@Serializable
data class PreGameRoute(val gameId: String, val playersEncoded: String, val playgroupId: String? = null)

@Serializable
data class GameTrackerRoute(
    val gameId: String,
    val playersEncoded: String,
    val startingPlayerSeat: Int,
    val playgroupId: String? = null
)

/**
 * Enters [com.vansid.tapeandocartones.presentation.screens.game.GameTrackerScreen] for a game this
 * device joined on ANOTHER device's remote game (see `JoinGameScreen`), instead of hosting a new
 * pass-and-play session. [localPlayerId] is the `GamePlayer.id` this device just got back from
 * `POST /games/{id}/join` — `GameViewModel` uses its presence (instead of `playersEncoded`, absent
 * here) to tell the two modes apart and fetches the rest of the table from the backend.
 */
@Serializable
data class JoinedGameTrackerRoute(val gameId: String, val localPlayerId: String)
