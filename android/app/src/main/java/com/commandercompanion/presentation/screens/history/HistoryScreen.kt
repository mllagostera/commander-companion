package com.commandercompanion.presentation.screens.history

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.presentation.theme.colorForKey
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HistoryScreen(
    onBack: () -> Unit,
    viewModel: HistoryViewModel = hiltViewModel()
) {
    val games by viewModel.games.collectAsState()

    Column(modifier = Modifier.fillMaxSize()) {
        TopAppBar(
            title = { Text("Historial de partidas") },
            navigationIcon = {
                TextButton(onClick = onBack) { Text("<") }
            }
        )

        if (games.isEmpty()) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text("Todavía no hay partidas registradas", style = MaterialTheme.typography.bodyLarge)
            }
        } else {
            LazyColumn(modifier = Modifier.fillMaxSize().padding(8.dp)) {
                items(games, key = { it.game.id }) { entry ->
                    GameHistoryCard(entry)
                }
            }
        }
    }
}

@Composable
private fun GameHistoryCard(entry: GameWithPlayers) {
    val dateFormat = remember { SimpleDateFormat("dd/MM/yyyy HH:mm", Locale.getDefault()) }
    val winner = entry.players.firstOrNull { it.won }

    Card(modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Text(
                    text = dateFormat.format(Date(entry.game.startTime)),
                    style = MaterialTheme.typography.labelMedium
                )
                Text(
                    text = if (entry.game.status == "FINISHED") "Finalizada" else "En curso",
                    style = MaterialTheme.typography.labelMedium
                )
            }
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = winner?.let { "Ganó: ${it.name}" } ?: "${entry.game.playerCount} jugadores",
                style = MaterialTheme.typography.titleMedium
            )
            Spacer(modifier = Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                entry.players.sortedBy { it.seatIndex }.forEach { player ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(
                            modifier = Modifier
                                .size(12.dp)
                                .clip(CircleShape)
                                .background(colorForKey(player.colorKey))
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text("${player.name}: ${player.finalLife}", style = MaterialTheme.typography.bodySmall)
                    }
                }
            }
        }
    }
}
