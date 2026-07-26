package com.commandercompanion.presentation.screens.pregame

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.navigation.encodePlayerConfigs
import com.commandercompanion.presentation.theme.colorForKey
import kotlin.random.Random

@Composable
fun PreGameScreen(
    playersEncoded: String,
    onContinue: (playersEncoded: String, startingPlayerSeat: Int) -> Unit
) {
    val configs = remember { decodePlayerConfigs(playersEncoded) }
    val mulligans = remember { mutableStateListOf(*IntArray(configs.size) { 0 }.toTypedArray()) }
    var startingSeat by remember { mutableIntStateOf(-1) }

    Column(modifier = Modifier.fillMaxSize().padding(16.dp)) {
        Text("Antes de empezar", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(24.dp))

        Text("¿Quién empieza?", style = MaterialTheme.typography.titleSmall)
        Spacer(modifier = Modifier.height(8.dp))
        Card(
            modifier = Modifier.fillMaxWidth().height(80.dp),
            colors = CardDefaults.cardColors(
                containerColor = if (startingSeat >= 0) {
                    colorForKey(configs[startingSeat].colorKey)
                } else {
                    MaterialTheme.colorScheme.surfaceVariant
                }
            )
        ) {
            Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                Text(
                    text = if (startingSeat >= 0) "Empieza ${configs[startingSeat].name}" else "Sin sortear todavía",
                    style = MaterialTheme.typography.titleMedium
                )
            }
        }
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedButton(
            onClick = { startingSeat = Random.nextInt(configs.size) },
            modifier = Modifier.fillMaxWidth()
        ) {
            Text("SORTEAR")
        }

        Spacer(modifier = Modifier.height(24.dp))
        Text("Mulligans", style = MaterialTheme.typography.titleSmall)
        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(modifier = Modifier.weight(1f)) {
            items(configs.size) { index ->
                MulliganRow(
                    config = configs[index],
                    mulligans = mulligans[index],
                    onIncrement = { mulligans[index] = mulligans[index] + 1 },
                    onDecrement = { mulligans[index] = (mulligans[index] - 1).coerceAtLeast(0) }
                )
            }
        }

        Button(
            onClick = {
                val updatedConfigs = configs.mapIndexed { index, config ->
                    config.copy(mulligans = mulligans[index])
                }
                onContinue(encodePlayerConfigs(updatedConfigs), startingSeat)
            },
            modifier = Modifier.fillMaxWidth().height(56.dp)
        ) {
            Text("EMPEZAR PARTIDA", style = MaterialTheme.typography.titleMedium)
        }
    }
}

@Composable
private fun MulliganRow(
    config: PlayerConfig,
    mulligans: Int,
    onIncrement: () -> Unit,
    onDecrement: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Box(
            modifier = Modifier
                .size(20.dp)
                .clip(CircleShape)
                .background(colorForKey(config.colorKey))
        )
        Spacer(modifier = Modifier.width(12.dp))
        Text(config.name, style = MaterialTheme.typography.bodyLarge, modifier = Modifier.weight(1f))

        Row(verticalAlignment = Alignment.CenterVertically) {
            StepperButton(label = "-", onClick = onDecrement)
            Text(
                text = mulligans.toString(),
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(horizontal = 16.dp)
            )
            StepperButton(label = "+", onClick = onIncrement)
        }
    }
}

@Composable
private fun StepperButton(label: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .size(32.dp)
            .clip(RoundedCornerShape(8.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center
    ) {
        Text(label, style = MaterialTheme.typography.titleMedium)
    }
}
