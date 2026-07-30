package com.commandercompanion.presentation.navigation

import kotlinx.serialization.Serializable

@Serializable
object LoginRoute

@Serializable
object RegisterRoute

@Serializable
object DashboardRoute

@Serializable
object PlayerSetupRoute

@Serializable
object HistoryRoute

@Serializable
data class PreGameRoute(val gameId: String, val playersEncoded: String, val playgroupId: String? = null)

@Serializable
data class GameTrackerRoute(
    val gameId: String,
    val playersEncoded: String,
    val startingPlayerSeat: Int,
    val playgroupId: String? = null
)
