package com.commandercompanion.core.di

import android.content.Context
import androidx.room.Room
import com.commandercompanion.data.local.CommanderDatabase
import com.commandercompanion.data.local.dao.DeckDao
import com.commandercompanion.data.local.dao.GameDao
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object DatabaseModule {

    @Provides
    @Singleton
    fun provideCommanderDatabase(@ApplicationContext context: Context): CommanderDatabase =
        Room.databaseBuilder(context, CommanderDatabase::class.java, "commander_companion.db")
            .fallbackToDestructiveMigration()
            .build()

    @Provides
    @Singleton
    fun provideGameDao(database: CommanderDatabase): GameDao = database.gameDao

    @Provides
    @Singleton
    fun provideDeckDao(database: CommanderDatabase): DeckDao = database.deckDao
}
