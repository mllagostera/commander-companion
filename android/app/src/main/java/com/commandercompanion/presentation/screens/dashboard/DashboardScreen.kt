package com.commandercompanion.presentation.screens.dashboard

import androidx.compose.foundation.layout.*
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel

@Composable
fun DashboardScreen(
    onNewGame: () -> Unit,
    onViewHistory: () -> Unit,
    onLogout: () -> Unit,
    viewModel: DashboardViewModel = hiltViewModel()
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text(
            text = "Commander Companion",
            style = MaterialTheme.typography.displayLarge
        )
        Spacer(modifier = Modifier.height(32.dp))
        Button(
            onClick = onNewGame,
            modifier = Modifier.fillMaxWidth().height(64.dp)
        ) {
            Text("NEW GAME", style = MaterialTheme.typography.titleLarge)
        }
        Spacer(modifier = Modifier.height(16.dp))
        OutlinedButton(
            onClick = onViewHistory,
            modifier = Modifier.fillMaxWidth().height(48.dp)
        ) {
            Text("HISTORIAL")
        }
        Spacer(modifier = Modifier.height(24.dp))
        TextButton(onClick = { viewModel.logout(onLogout) }) {
            Text("Cerrar sesión")
        }
    }
}
