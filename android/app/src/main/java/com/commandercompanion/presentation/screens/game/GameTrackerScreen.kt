package com.commandercompanion.presentation.screens.game

import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.commandercompanion.presentation.components.PlayerCard
import com.commandercompanion.presentation.theme.ManaWhite
import com.commandercompanion.presentation.theme.ManaBlue
import com.commandercompanion.presentation.theme.ManaBlack
import com.commandercompanion.presentation.theme.ManaRed

@Composable
fun GameTrackerScreen(
    gameId: String
) {
    Column(modifier = Modifier.fillMaxSize()) {
        Text(
            text = "Game: $gameId",
            style = MaterialTheme.typography.titleLarge,
            modifier = Modifier.padding(16.dp),
            color = MaterialTheme.colorScheme.onBackground
        )

        // Mock 4 players layout
        Row(modifier = Modifier.weight(1f)) {
            PlayerCard(
                playerName = "Player 1",
                lifeTotal = 40,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                cardColor = ManaWhite
            )
            PlayerCard(
                playerName = "Player 2",
                lifeTotal = 40,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                cardColor = ManaBlue
            )
        }
        Row(modifier = Modifier.weight(1f)) {
            PlayerCard(
                playerName = "Player 3",
                lifeTotal = 40,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                cardColor = ManaBlack
            )
            PlayerCard(
                playerName = "Player 4",
                lifeTotal = 40,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                cardColor = ManaRed
            )
        }
    }
}
