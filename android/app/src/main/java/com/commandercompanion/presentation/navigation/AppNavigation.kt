package com.commandercompanion.presentation.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.ExperimentalSafeArgsApi
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.commandercompanion.presentation.screens.dashboard.DashboardScreen
import com.commandercompanion.presentation.screens.game.GameTrackerScreen
import com.commandercompanion.presentation.screens.history.HistoryScreen
import com.commandercompanion.presentation.screens.setup.PlayerSetupScreen

@OptIn(ExperimentalSafeArgsApi::class)
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
                onNewGame = { navController.navigate(PlayerSetupRoute) },
                onViewHistory = { navController.navigate(HistoryRoute) }
            )
        }
        composable<PlayerSetupRoute> {
            PlayerSetupScreen(
                onStartGame = { gameId, playersEncoded ->
                    navController.navigate(GameTrackerRoute(gameId, playersEncoded)) {
                        popUpTo(DashboardRoute)
                    }
                }
            )
        }
        composable<GameTrackerRoute> {
            GameTrackerScreen(
                onFinish = {
                    navController.popBackStack(DashboardRoute, inclusive = false)
                }
            )
        }
        composable<HistoryRoute> {
            HistoryScreen(onBack = { navController.popBackStack() })
        }
    }
}
