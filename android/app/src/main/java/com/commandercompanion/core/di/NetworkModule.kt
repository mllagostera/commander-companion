package com.commandercompanion.core.di

import com.commandercompanion.BuildConfig
import com.commandercompanion.data.remote.api.AuthApi
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.interceptor.AuthAuthenticator
import com.commandercompanion.data.remote.interceptor.AuthInterceptor
import com.commandercompanion.data.remote.ws.GameSocketClient
import com.commandercompanion.data.remote.ws.OkHttpGameSocketClient
import com.commandercompanion.data.session.AccessTokenProvider
import com.commandercompanion.data.session.SessionManager
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import java.util.concurrent.TimeUnit
import javax.inject.Qualifier
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit

/** Client/Retrofit WITHOUT the session interceptor — used by [AuthApi] (login/google/refresh/logout). */
@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class UnauthenticatedClient

/** Client/Retrofit WITH the Bearer interceptor + refresh authenticator — used by [CommanderApi]. */
@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class AuthenticatedClient

/**
 * Client used for the live-sync WebSocket ([GameSocketClient]) — no Bearer interceptor (the
 * token travels in the first WebSocket message, not an HTTP header, see ADR-0005) and no read
 * timeout (an idle-but-alive room would otherwise be killed by the same 15s timeout the REST
 * clients use).
 */
@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class WebSocketOkHttpClient

/**
 * Network module: Retrofit + OkHttp to talk to the Go backend.
 *
 * Base URL configurable via `API_BASE_URL` (see `app/build.gradle.kts`), default
 * `http://10.0.2.2:8080/` (the Android emulator's alias for the host's localhost).
 *
 * There are deliberately TWO HTTP clients: [AuthApi] must never go through the session
 * interceptor/authenticator (its endpoints are public, and using the authenticated client for
 * the refresh itself would cause a recursion). [CommanderApi] does need the automatic Bearer +
 * refresh-on-401.
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
    @WebSocketOkHttpClient
    fun provideWebSocketOkHttpClient(): OkHttpClient =
        OkHttpClient.Builder()
            .addInterceptor(loggingInterceptor())
            .connectTimeout(15, TimeUnit.SECONDS)
            .readTimeout(0, TimeUnit.MILLISECONDS)
            .pingInterval(20, TimeUnit.SECONDS)
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

    @Provides
    @Singleton
    fun provideGameSocketClient(impl: OkHttpGameSocketClient): GameSocketClient = impl

    @Provides
    @Singleton
    fun provideAccessTokenProvider(sessionManager: SessionManager): AccessTokenProvider =
        AccessTokenProvider { sessionManager.currentAccessToken() }
}
