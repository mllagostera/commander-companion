package com.commandercompanion.presentation.screens.game

import androidx.compose.ui.graphics.Color

data class PlayerState(
    val id: Int,
    val name: String,
    val life: Int = 40,
    val color: Color,
    val commanderDamage: Map<Int, Int> = emptyMap() // Key: Opponent ID, Value: Damage received
)

data class GameState(
    val players: List<PlayerState> = emptyList(),
    val currentTurn: Int = 1
)
