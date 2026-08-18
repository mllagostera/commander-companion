package com.commandercompanion.core.di

import com.commandercompanion.data.repository.DeckRepositoryImpl
import com.commandercompanion.data.repository.FriendsRepositoryImpl
import com.commandercompanion.data.repository.GameRepositoryImpl
import com.commandercompanion.data.repository.PlaygroupRepositoryImpl
import com.commandercompanion.data.repository.StatisticsRepositoryImpl
import com.commandercompanion.domain.repository.DeckRepository
import com.commandercompanion.domain.repository.FriendsRepository
import com.commandercompanion.domain.repository.GameRepository
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.domain.repository.StatisticsRepository
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Singleton

/** Binds each domain repository interface to its `data/repository` implementation. */
@Module
@InstallIn(SingletonComponent::class)
object RepositoryModule {

    @Provides
    @Singleton
    fun provideGameRepository(impl: GameRepositoryImpl): GameRepository = impl

    @Provides
    @Singleton
    fun provideDeckRepository(impl: DeckRepositoryImpl): DeckRepository = impl

    @Provides
    @Singleton
    fun providePlaygroupRepository(impl: PlaygroupRepositoryImpl): PlaygroupRepository = impl

    @Provides
    @Singleton
    fun provideStatisticsRepository(impl: StatisticsRepositoryImpl): StatisticsRepository = impl

    @Provides
    @Singleton
    fun provideFriendsRepository(impl: FriendsRepositoryImpl): FriendsRepository = impl
}
