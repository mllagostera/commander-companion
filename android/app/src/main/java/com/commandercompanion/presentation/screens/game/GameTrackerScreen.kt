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

        RemoteSyncBanner(state.remoteSync)

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

/**
 * Franja informativa del estado de sincronización con el backend.
 *
 * Deliberadamente no bloquea nada: la partida se juega igual en local, así que solo se muestra
 * cuando hay algo que contar (no en [RemoteSyncStatus.Synced], que es el caso silencioso).
 */
@Composable
private fun RemoteSyncBanner(remoteSync: RemoteSyncState) {
    val label = when (remoteSync.status) {
        RemoteSyncStatus.Connecting -> "Creando la partida en el servidor…"
        RemoteSyncStatus.Synced -> null
        RemoteSyncStatus.Disabled,
        RemoteSyncStatus.WaitingForPlayers,
        RemoteSyncStatus.Failed -> remoteSync.message
    } ?: return

    val container = when (remoteSync.status) {
        RemoteSyncStatus.Failed -> MaterialTheme.colorScheme.errorContainer
        else -> MaterialTheme.colorScheme.surfaceVariant
    }
    val content = when (remoteSync.status) {
        RemoteSyncStatus.Failed -> MaterialTheme.colorScheme.onErrorContainer
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }

    Text(
        text = label,
        style = MaterialTheme.typography.bodySmall,
        color = content,
        modifier = Modifier
            .fillMaxWidth()
            .background(container)
            .padding(horizontal = 12.dp, vertical = 6.dp)
    )
}
