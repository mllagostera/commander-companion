package com.commandercompanion.data.local.dao

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import com.commandercompanion.data.local.entity.DeckEntity

@Dao
interface DeckDao {
    @Query("SELECT * FROM decks")
    suspend fun getAll(): List<DeckEntity>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(deck: DeckEntity)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAll(decks: List<DeckEntity>)

    @Query("DELETE FROM decks")
    suspend fun clear()

    @Query("DELETE FROM decks WHERE id = :deckId")
    suspend fun deleteById(deckId: String)
}
