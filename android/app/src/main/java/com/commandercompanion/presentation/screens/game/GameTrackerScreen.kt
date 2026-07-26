package com.commandercompanion.presentation.screens.game

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.presentation.components.PlayerCard

@Composable
fun GameTrackerScreen(
    onFinish: () -> Unit,
    viewModel: GameViewModel = hiltViewModel()
) {
    val state by viewModel.state
    var showFinishConfirm by remember { mutableStateOf(false) }

    Column(modifier = Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        // Turn Counter Header
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(8.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Button(onClick = { viewModel.previousTurn() }) {
                Text("<")
            }

            Text(
                text = "Turn: ${state.currentTurn}",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground
            )

            Row {
                Button(onClick = { viewModel.nextTurn() }) {
                    Text(">")
                }
                Spacer(modifier = Modifier.width(8.dp))
                OutlinedButton(onClick = { showFinishConfirm = true }) {
                    Text("Finalizar")
                }
            }
        }

        // Dynamic player grid: rows of up to 2 players, works for 2-6 players
        Column(modifier = Modifier.weight(1f)) {
            state.players.chunked(2).forEach { row ->
                Row(modifier = Modifier.weight(1f)) {
                    row.forEach { player ->
                        PlayerCard(
                            playerState = player,
                            otherPlayers = state.players.filter { it.id != player.id },
                            onLifeChange = { viewModel.adjustLife(player.id, it) },
                            onCommanderDamageChange = { attackerId, amount ->
                                viewModel.adjustCommanderDamage(player.id, attackerId, amount)
                            },
                            isStartingPlayer = player.id == state.startingPlayerId,
                            modifier = Modifier.weight(1f).fillMaxHeight()
                        )
                    }
                }
            }
        }
    }

    if (showFinishConfirm && !state.isFinished) {
        AlertDialog(
            onDismissRequest = { showFinishConfirm = false },
            title = { Text("¿Finalizar partida?") },
            text = { Text("Se registrará la vida actual de cada jugador en el historial.") },
            confirmButton = {
                TextButton(onClick = {
                    showFinishConfirm = false
                    viewModel.finishGame()
                }) { Text("Finalizar") }
            },
            dismissButton = {
                TextButton(onClick = { showFinishConfirm = false }) { Text("Cancelar") }
            }
        )
    }

    if (state.isFinished) {
        val winnerName = state.players.firstOrNull { it.id == state.winnerId }?.name
        AlertDialog(
            onDismissRequest = onFinish,
            title = { Text(if (winnerName != null) "¡$winnerName gana!" else "Partida finalizada") },
            text = {
                Column {
                    state.players.forEach { player ->
                        Text("${player.name}: ${player.life} vida")
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = onFinish) { Text("Volver al inicio") }
            }
        )
    }
}
