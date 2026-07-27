package com.commandercompanion.core.di

import com.commandercompanion.BuildConfig
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.interceptor.AuthAuthenticator
import com.commandercompanion.data.remote.interceptor.AuthInterceptor
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import retrofit2.Retrofit
import java.util.concurrent.TimeUnit
import javax.inject.Qualifier
import javax.inject.Singleton

/** Cliente/Retrofit SIN interceptor de sesión — usado por [AuthApi] (login/google/refresh/logout). */
@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class UnauthenticatedClient

/** Cliente/Retrofit CON interceptor de Bearer + authenticator de refresh — usado por [CommanderApi]. */
@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class AuthenticatedClient

/**
 * Módulo de red: Retrofit + OkHttp para hablar con el backend Go.
 *
 * Base URL configurable vía `API_BASE_URL` (ver `app/build.gradle.kts`), default
 * `http://10.0.2.2:8080/` (alias del emulador Android hacia el localhost del host).
 *
 * Hay DOS clientes HTTP a propósito: [AuthApi] nunca debe pasar por el interceptor/authenticator
 * de sesión (sus endpoints son públicos, y usar el cliente autenticado para el propio refresh
 * causaría una recursión). [CommanderApi] sí necesita el Bearer automático + refresh-on-401.
 */
@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {

    @Provides
    @Singleton
    fun provideJson(): Json = Json {
        ignoreUnknownKeys = true
        isLenient = true
        encodeDefaults = true
    }

    private fun loggingInterceptor(): HttpLoggingInterceptor =
        HttpLoggingInterceptor().apply {
            level = if (BuildConfig.DEBUG) {
                HttpLoggingInterceptor.Level.BODY
            } else {
                HttpLoggingInterceptor.Level.NONE
            }
        }

    @Provides
    @Singleton
    @UnauthenticatedClient
    fun provideUnauthenticatedOkHttpClient(): OkHttpClient =
        OkHttpClient.Builder()
            .addInterceptor(loggingInterceptor())
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(15, TimeUnit.SECONDS)
            .build()

    @Provides
    @Singleton
    @AuthenticatedClient
    fun provideAuthenticatedOkHttpClient(
        authInterceptor: AuthInterceptor,
        authAuthenticator: AuthAuthenticator
    ): OkHttpClient =
        OkHttpClient.Builder()
            .addInterceptor(authInterceptor)
            .authenticator(authAuthenticator)
            .addInterceptor(loggingInterceptor())
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(15, TimeUnit.SECONDS)
            .build()

    @Provides
    @Singleton
    @UnauthenticatedClient
    fun provideUnauthenticatedRetrofit(
        json: Json,
        @UnauthenticatedClient client: OkHttpClient
    ): Retrofit = Retrofit.Builder()
        .baseUrl(BuildConfig.API_BASE_URL)
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()

    @Provides
    @Singleton
    @AuthenticatedClient
    fun provideAuthenticatedRetrofit(
        json: Json,
        @AuthenticatedClient client: OkHttpClient
    ): Retrofit = Retrofit.Builder()
        .baseUrl(BuildConfig.API_BASE_URL)
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()

    @Provides
    @Singleton
    fun provideAuthApi(@UnauthenticatedClient retrofit: Retrofit): AuthApi =
        retrofit.create(AuthApi::class.java)

    @Provides
    @Singleton
    fun provideCommanderApi(@AuthenticatedClient retrofit: Retrofit): CommanderApi =
        retrofit.create(CommanderApi::class.java)
}
