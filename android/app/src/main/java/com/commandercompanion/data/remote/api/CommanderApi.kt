package com.commandercompanion.data.remote.api

import retrofit2.http.GET

interface CommanderApi {
    @GET("api/v1/health")
    suspend fun checkHealth(): String
}
