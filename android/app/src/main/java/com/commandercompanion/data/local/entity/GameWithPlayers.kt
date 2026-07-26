package com.commandercompanion.data.local.entity

import androidx.room.Embedded
import androidx.room.Relation

data class GameWithPlayers(
    @Embedded val game: GameEntity,
    @Relation(parentColumn = "id", entityColumn = "gameId")
    val players: List<PlayerResultEntity>
)
