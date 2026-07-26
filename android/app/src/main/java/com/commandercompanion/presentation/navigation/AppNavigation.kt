package com.commandercompanion.presentation.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import com.commandercompanion.presentation.screens.dashboard.DashboardScreen
import com.commandercompanion.presentation.screens.game.GameTrackerScreen

@Composable
fun AppNavigation(
    navController: NavHostController = rememberNavController()
) {
    NavHost(
        navController = navController,
        startDestination = DashboardRoute
    ) {
        composable<DashboardRoute> {
            DashboardScreen(
                onNavigateToGame = { gameId ->
                    navController.navigate(GameTrackerRoute(gameId))
                }
            )
        }
        composable<GameTrackerRoute> { backStackEntry ->
            val route = backStackEntry.toRoute<GameTrackerRoute>()
            GameTrackerScreen(gameId = route.gameId)
        }
    }
}
