package com.commandercompanion.presentation.navigation

import kotlinx.serialization.Serializable

@Serializable
object DashboardRoute

@Serializable
data class GameTrackerRoute(val gameId: String)
