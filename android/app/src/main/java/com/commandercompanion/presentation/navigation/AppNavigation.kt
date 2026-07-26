package com.commandercompanion.presentation.navigation

import androidx.compose.runtime.Composable
import androidx.navigation.ExperimentalSafeArgsApi
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import com.commandercompanion.presentation.screens.dashboard.DashboardScreen
import com.commandercompanion.presentation.screens.game.GameTrackerScreen
import com.commandercompanion.presentation.screens.history.HistoryScreen
import com.commandercompanion.presentation.screens.login.LoginScreen
import com.commandercompanion.presentation.screens.pregame.PreGameScreen
import com.commandercompanion.presentation.screens.setup.PlayerSetupScreen

@OptIn(ExperimentalSafeArgsApi::class)
@Composable
fun AppNavigation(
    navController: NavHostController = rememberNavController()
) {
    NavHost(
        navController = navController,
        startDestination = LoginRoute
    ) {
        composable<LoginRoute> {
            LoginScreen(
                onLoginWithPassword = { _, _ ->
                    navController.navigate(DashboardRoute) { popUpTo(LoginRoute) { inclusive = true } }
                },
                onLoginWithGoogle = {
                    navController.navigate(DashboardRoute) { popUpTo(LoginRoute) { inclusive = true } }
                }
            )
        }
        composable<DashboardRoute> {
            DashboardScreen(
                onNewGame = { navController.navigate(PlayerSetupRoute) },
                onViewHistory = { navController.navigate(HistoryRoute) }
            )
        }
        composable<PlayerSetupRoute> {
            PlayerSetupScreen(
                onStartGame = { gameId, playersEncoded ->
                    navController.navigate(PreGameRoute(gameId, playersEncoded))
                }
            )
        }
        composable<PreGameRoute> { backStackEntry ->
            val route = backStackEntry.toRoute<PreGameRoute>()
            PreGameScreen(
                playersEncoded = route.playersEncoded,
                onContinue = { playersEncoded, startingPlayerSeat ->
                    navController.navigate(GameTrackerRoute(route.gameId, playersEncoded, startingPlayerSeat)) {
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
