package com.commandercompanion

import android.app.Application
import coil3.ComponentRegistry
import coil3.ImageLoader
import coil3.PlatformContext
import coil3.SingletonImageLoader
import coil3.Uri
import coil3.network.okhttp.OkHttpNetworkFetcherFactory
import dagger.hilt.android.HiltAndroidApp
import okhttp3.OkHttpClient

@HiltAndroidApp
class CommanderCompanionApp : Application(), SingletonImageLoader.Factory {
    override fun onCreate() {
        super.onCreate()
    }

    /**
     * Deck art thumbnails are hotlinked from Scryfall's CDN, which rejects OkHttp's default
     * User-Agent as bot traffic (returns HTTP 400) — a descriptive one is required, matching
     * Scryfall's own API etiquette guidelines.
     */
    override fun newImageLoader(context: PlatformContext): ImageLoader {
        val registry = ComponentRegistry.Builder()
            .add(
                OkHttpNetworkFetcherFactory(
                    callFactory = {
                        OkHttpClient.Builder()
                            .addNetworkInterceptor { chain ->
                                chain.proceed(
                                    chain.request().newBuilder()
                                        .header("User-Agent", "CommanderCompanion-Android/1.0")
                                        .build()
                                )
                            }
                            .build()
                    }
                ),
                Uri::class
            )
            .build()
        return ImageLoader.Builder(context)
            .components(registry)
            .build()
    }
}
