package com.commandercompanion.data.local.entity

import androidx.room.Entity
import androidx.room.Index
import androidx.room.PrimaryKey

@Entity(tableName = "player_results", indices = [Index("gameId")])
data class PlayerResultEntity(
    @PrimaryKey(autoGenerate = true)
    val id: Int = 0,
    val gameId: String,
    val seatIndex: Int,
    val name: String,
    val colorKey: String,
    val finalLife: Int,
    val mulligans: Int = 0,
    val won: Boolean = false
)
