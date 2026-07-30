package com.commandercompanion.presentation.screens.dashboard

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.presentation.components.AppLogoMark
import com.commandercompanion.presentation.components.AppScreenBackground
import com.commandercompanion.presentation.components.GradientButton
import com.commandercompanion.presentation.components.GradientOutlineButton
import com.commandercompanion.presentation.components.GradientTitle
import com.commandercompanion.presentation.theme.StatusDanger

@Composable
fun DashboardScreen(
    onNewGame: () -> Unit,
    onViewHistory: () -> Unit,
    onLogout: () -> Unit,
    viewModel: DashboardViewModel = hiltViewModel()
) {
    AppScreenBackground {
        Column(
            modifier = Modifier.fillMaxSize().padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            AppLogoMark(width = 42.dp, height = 58.dp)
            Spacer(modifier = Modifier.height(20.dp))
            GradientTitle(text = "Commander Companion", fontSize = 26.sp)
            Spacer(modifier = Modifier.height(24.dp))

            GradientButton(text = "NUEVA PARTIDA", onClick = onNewGame) {
                Text(
                    "NUEVA PARTIDA",
                    color = MaterialTheme.colorScheme.background,
                    fontWeight = FontWeight.Bold,
                    fontSize = 15.sp,
                    letterSpacing = 0.5.sp
                )
            }
            Spacer(modifier = Modifier.height(14.dp))
            GradientOutlineButton(text = "HISTORIAL", onClick = onViewHistory)
            Spacer(modifier = Modifier.height(20.dp))
            Text(
                text = "Cerrar sesión",
                color = StatusDanger,
                fontSize = 13.sp,
                modifier = Modifier.clickable { viewModel.logout(onLogout) }
            )
        }
    }
}
