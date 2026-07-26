package com.commandercompanion.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "games")
data class GameEntity(
    @PrimaryKey
    val id: String,
    val startTime: Long,
    val endTime: Long? = null,
    val status: String, // "IN_PROGRESS" | "FINISHED"
    val playerCount: Int
)
