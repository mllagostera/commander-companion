package com.commandercompanion.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "games")
data class GameEntity(
    @PrimaryKey
    val id: String,
    val startTime: Long,
    val status: String // e.g. "IN_PROGRESS", "FINISHED"
)
