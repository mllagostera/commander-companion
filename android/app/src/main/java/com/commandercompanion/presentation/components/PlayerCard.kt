package com.commandercompanion.presentation.components

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.commandercompanion.presentation.screens.game.PlayerState

@Composable
fun PlayerCard(
    playerState: PlayerState,
    otherPlayers: List<PlayerState>,
    onLifeChange: (Int) -> Unit,
    onCommanderDamageChange: (Int, Int) -> Unit,
    modifier: Modifier = Modifier
) {
    var showCommanderDamage by remember { mutableStateOf(false) }

    Surface(
        modifier = modifier
            .padding(4.dp)
            .clip(RoundedCornerShape(16.dp))
            .clickable { showCommanderDamage = !showCommanderDamage },
        color = playerState.color
    ) {
        Box(modifier = Modifier.fillMaxSize()) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(8.dp),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally
            ) {
                Text(
                    text = playerState.name,
                    style = MaterialTheme.typography.titleMedium,
                    color = Color.Black.copy(alpha = 0.7f)
                )

                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.Center
                ) {
                    IconButton(onClick = { onLifeChange(-1) }) {
                        Text("-", fontSize = 24.sp, fontWeight = FontWeight.Bold, color = Color.Black)
                    }

                    Text(
                        text = playerState.life.toString(),
                        style = MaterialTheme.typography.displayLarge.copy(
                            fontWeight = FontWeight.Bold,
                            fontSize = 64.sp
                        ),
                        color = Color.Black
                    )

                    IconButton(onClick = { onLifeChange(1) }) {
                        Text("+", fontSize = 24.sp, fontWeight = FontWeight.Bold, color = Color.Black)
                    }
                }

                if (!showCommanderDamage) {
                    Text(
                        text = "Commander Damage",
                        style = MaterialTheme.typography.labelSmall,
                        color = Color.Black.copy(alpha = 0.5f)
                    )
                }
            }

            if (showCommanderDamage) {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = Color.Black.copy(alpha = 0.8f)
                ) {
                    Column(
                        modifier = Modifier.padding(8.dp),
                        horizontalAlignment = Alignment.CenterHorizontally
                    ) {
                        Text(
                            "Commander Damage",
                            color = Color.White,
                            style = MaterialTheme.typography.titleSmall
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        LazyVerticalGrid(
                            columns = GridCells.Fixed(3),
                            modifier = Modifier.fillMaxSize(),
                            horizontalArrangement = Arrangement.spacedBy(4.dp),
                            verticalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            items(otherPlayers) { attacker ->
                                val damage = playerState.commanderDamage[attacker.id] ?: 0
                                CommanderDamageItem(
                                    attackerColor = attacker.color,
                                    damage = damage,
                                    onIncrement = { onCommanderDamageChange(attacker.id, 1) },
                                    onDecrement = { onCommanderDamageChange(attacker.id, -1) }
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
fun CommanderDamageItem(
    attackerColor: Color,
    damage: Int,
    onIncrement: () -> Unit,
    onDecrement: () -> Unit
) {
    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = Modifier.background(Color.White.copy(alpha = 0.1f), RoundedCornerShape(8.dp)).padding(4.dp)
    ) {
        Box(
            modifier = Modifier
                .size(20.dp)
                .clip(CircleShape)
                .background(attackerColor)
        )
        Text(
            text = damage.toString(),
            color = Color.White,
            style = MaterialTheme.typography.titleMedium
        )
        Row {
            IconButton(onClick = onDecrement, modifier = Modifier.size(24.dp)) {
                Text("-", color = Color.White, fontWeight = FontWeight.Bold)
            }
            IconButton(onClick = onIncrement, modifier = Modifier.size(24.dp)) {
                Text("+", color = Color.White, fontWeight = FontWeight.Bold)
            }
        }
    }
}
