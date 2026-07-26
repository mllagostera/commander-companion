package com.commandercompanion.data.local.dao

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Transaction
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.GameWithPlayers
import com.commandercompanion.data.local.entity.PlayerResultEntity
import kotlinx.coroutines.flow.Flow

@Dao
interface GameDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertGame(game: GameEntity)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertPlayers(players: List<PlayerResultEntity>)

    @Query("UPDATE games SET status = :status, endTime = :endTime WHERE id = :gameId")
    suspend fun finishGame(gameId: String, status: String, endTime: Long)

    @Query(
        "UPDATE player_results SET finalLife = :finalLife, won = :won " +
            "WHERE gameId = :gameId AND seatIndex = :seatIndex"
    )
    suspend fun updatePlayerResult(gameId: String, seatIndex: Int, finalLife: Int, won: Boolean)

    @Transaction
    @Query("SELECT * FROM games ORDER BY startTime DESC")
    fun getGamesWithPlayers(): Flow<List<GameWithPlayers>>
}
