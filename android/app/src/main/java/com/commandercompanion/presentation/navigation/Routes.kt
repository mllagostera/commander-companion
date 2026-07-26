package com.commandercompanion.presentation.navigation

import kotlinx.serialization.Serializable

@Serializable
object DashboardRoute

@Serializable
object PlayerSetupRoute

@Serializable
object HistoryRoute

@Serializable
data class GameTrackerRoute(val gameId: String, val playersEncoded: String)
