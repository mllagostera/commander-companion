package com.commandercompanion.data.local

import androidx.room.Database
import androidx.room.RoomDatabase
import com.commandercompanion.data.local.dao.DeckDao
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.DeckEntity
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.PlayerResultEntity

@Database(
    entities = [GameEntity::class, PlayerResultEntity::class, DeckEntity::class],
    version = 4,
    exportSchema = false
)
abstract class CommanderDatabase : RoomDatabase() {
    abstract val gameDao: GameDao
    abstract val deckDao: DeckDao
}
