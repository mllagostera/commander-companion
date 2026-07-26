package com.commandercompanion.presentation.screens.setup

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.presentation.theme.PlayerColorPalette
import java.util.UUID

private const val MIN_PLAYERS = 2
private const val MAX_PLAYERS = 6

@Composable
fun PlayerSetupScreen(
    onStartGame: (gameId: String, playersEncoded: String) -> Unit
) {
    var playerCount by remember { mutableIntStateOf(4) }
    val names = remember {
        mutableStateListOf(*Array(MAX_PLAYERS) { "Jugador ${it + 1}" })
    }
    val colorKeys = remember {
        mutableStateListOf(*Array(MAX_PLAYERS) { PlayerColorPalette[it % PlayerColorPalette.size].first })
    }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        Text("Nueva partida", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(16.dp))

        Text("Jugadores", style = MaterialTheme.typography.titleSmall)
        Row(
            modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            (MIN_PLAYERS..MAX_PLAYERS).forEach { count ->
                FilterChip(
                    selected = playerCount == count,
                    onClick = { playerCount = count },
                    label = { Text(count.toString()) }
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(modifier = Modifier.weight(1f)) {
            items(playerCount) { index ->
                PlayerConfigRow(
                    name = names[index],
                    onNameChange = { names[index] = it },
                    selectedColorKey = colorKeys[index],
                    onColorSelected = { colorKeys[index] = it }
                )
            }
        }

        Button(
            onClick = {
                val configs = (0 until playerCount).map { index ->
                    PlayerConfig(
                        name = names[index].ifBlank { "Jugador ${index + 1}" },
                        colorKey = colorKeys[index]
                    )
                }
                onStartGame(UUID.randomUUID().toString(), encodePlayerConfigs(configs))
            },
            modifier = Modifier.fillMaxWidth().height(56.dp)
        ) {
            Text("EMPEZAR PARTIDA", style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Composable
private fun PlayerConfigRow(
    name: String,
    onNameChange: (String) -> Unit,
    selectedColorKey: String,
    onColorSelected: (String) -> Unit
) {
    Column(modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        OutlinedTextField(
            value = name,
            onValueChange = onNameChange,
            label = { Text("Nombre") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth()
        )
        Spacer(modifier = Modifier.height(4.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            PlayerColorPalette.forEach { (key, color) ->
                ColorSwatch(
                    color = color,
                    selected = key == selectedColorKey,
                    onClick = { onColorSelected(key) }
                )
            }
        }
    }
}

@Composable
private fun ColorSwatch(color: Color, selected: Boolean, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(32.dp)
            .clip(CircleShape)
            .background(color)
            .border(
                width = if (selected) 3.dp else 0.dp,
                color = MaterialTheme.colorScheme.primary,
                shape = CircleShape
            )
            .clickable(onClick = onClick)
    )
}
