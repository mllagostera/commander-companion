package com.commandercompanion.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.PlayerResultEntity

@Database(entities = [GameEntity::class, PlayerResultEntity::class], version = 1, exportSchema = false)
abstract class CommanderDatabase : RoomDatabase() {
    abstract val gameDao: GameDao
}
