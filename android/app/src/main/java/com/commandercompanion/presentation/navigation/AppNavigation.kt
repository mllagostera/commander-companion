package com.commandercompanion.presentation.navigation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.hilt.navigation.compose.hiltViewModel
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
import com.commandercompanion.presentation.screens.register.RegisterScreen
import com.commandercompanion.presentation.screens.settings.SettingsScreen
import com.commandercompanion.presentation.screens.setup.PlayerSetupScreen
import com.commandercompanion.presentation.screens.statistics.StatisticsScreen

@OptIn(ExperimentalSafeArgsApi::class)
@Composable
fun AppNavigation(
    navController: NavHostController = rememberNavController(),
    sessionViewModel: SessionViewModel = hiltViewModel()
) {
    // Failed token refresh (stolen/expired/revoked token) on any screen -> Login.
    LaunchedEffect(Unit) {
        sessionViewModel.forcedLogoutEvents.collect {
            navController.navigate(LoginRoute) { popUpTo(0) { inclusive = true } }
        }
    }

    NavHost(
        navController = navController,
        startDestination = LoginRoute
    ) {
        composable<LoginRoute> {
            LoginScreen(
                onLoginSuccess = {
                    navController.navigate(DashboardRoute) { popUpTo(LoginRoute) { inclusive = true } }
                },
                onNavigateToRegister = { navController.navigate(RegisterRoute) }
            )
        }
        composable<RegisterRoute> {
            RegisterScreen(
                onLoginSuccess = {
                    navController.navigate(DashboardRoute) { popUpTo(LoginRoute) { inclusive = true } }
                },
                onNavigateToLogin = {
                    navController.popBackStack(LoginRoute, inclusive = false)
                }
            )
        }
        composable<DashboardRoute> {
            DashboardScreen(
                onNewGame = { navController.navigate(PlayerSetupRoute) },
                onViewHistory = { navController.navigate(HistoryRoute) },
                onViewStatistics = { navController.navigate(StatisticsRoute) },
                onOpenSettings = { navController.navigate(SettingsRoute) },
                onLogout = {
                    navController.navigate(LoginRoute) { popUpTo(0) { inclusive = true } }
                }
            )
        }
        composable<PlayerSetupRoute> {
            PlayerSetupScreen(
                onStartGame = { gameId, playersEncoded, playgroupId ->
                    navController.navigate(PreGameRoute(gameId, playersEncoded, playgroupId))
                }
            )
        }
        composable<PreGameRoute> { backStackEntry ->
            val route = backStackEntry.toRoute<PreGameRoute>()
            PreGameScreen(
                playersEncoded = route.playersEncoded,
                onContinue = { playersEncoded, startingPlayerSeat ->
                    navController.navigate(
                        GameTrackerRoute(route.gameId, playersEncoded, startingPlayerSeat, route.playgroupId)
                    ) {
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
        composable<SettingsRoute> {
            SettingsScreen(
                onBack = { navController.popBackStack() },
                onLoggedOut = {
                    navController.navigate(LoginRoute) { popUpTo(0) { inclusive = true } }
                }
            )
        }
        composable<StatisticsRoute> {
            StatisticsScreen(onBack = { navController.popBackStack() })
        }
    }
}
