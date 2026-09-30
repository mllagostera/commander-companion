package com.vansid.tapeandocartones.core.di

import com.vansid.tapeandocartones.data.repository.DeckRepositoryImpl
import com.vansid.tapeandocartones.data.repository.FriendsRepositoryImpl
import com.vansid.tapeandocartones.data.repository.GameRepositoryImpl
import com.vansid.tapeandocartones.data.repository.PlaygroupRepositoryImpl
import com.vansid.tapeandocartones.data.repository.StatisticsRepositoryImpl
import com.vansid.tapeandocartones.domain.repository.DeckRepository
import com.vansid.tapeandocartones.domain.repository.FriendsRepository
import com.vansid.tapeandocartones.domain.repository.GameRepository
import com.vansid.tapeandocartones.domain.repository.PlaygroupRepository
import com.vansid.tapeandocartones.domain.repository.StatisticsRepository
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
