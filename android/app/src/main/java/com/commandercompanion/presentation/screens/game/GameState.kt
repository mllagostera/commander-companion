package com.commandercompanion.presentation.screens.game

import androidx.compose.ui.graphics.Color

data class PlayerState(
    val id: Int,
    val name: String,
    val life: Int = 40,
    val color: Color,
    val mulligans: Int = 0,
    val commanderDamage: Map<Int, Int> = emptyMap() // Key: Opponent ID, Value: Damage received
)

data class GameState(
    val players: List<PlayerState> = emptyList(),
    val currentTurn: Int = 1,
    val startingPlayerId: Int? = null,
    val isFinished: Boolean = false,
    val winnerId: Int? = null
)
