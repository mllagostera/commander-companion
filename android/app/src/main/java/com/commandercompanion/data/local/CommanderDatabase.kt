package com.commandercompanion.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameEntity

@Database(entities = [GameEntity::class], version = 1, exportSchema = false)
abstract class CommanderDatabase : RoomDatabase() {
    abstract val gameDao: GameDao
}
