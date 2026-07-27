package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckStatsDto
import com.commandercompanion.data.remote.dto.PlaygroupStatsDto
import com.commandercompanion.data.remote.dto.UserStatsDto
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Estadísticas agregadas (endpoints bajo `/statistics`).
 *
 * Siempre remotas: el backend las recalcula al finalizar cada partida y no tiene sentido
 * duplicar esa agregación en el cliente. Devuelve ceros (no error) para un usuario/deck que
 * todavía no terminó ninguna partida.
 *
 * TODO: todavía no hay pantalla que la consuma — la UI de estadísticas es Stage 7 de `TASKS.md`,
 * fuera del alcance de esta pasada (que era destrabarla conectando la API).
 */
@Singleton
class StatisticsRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun userStats(): Result<UserStatsDto> = apiCall { api.getUserStats() }

    suspend fun deckStats(deckId: String): Result<DeckStatsDto> = apiCall { api.getDeckStats(deckId) }

    suspend fun playgroupStats(playgroupId: String): Result<PlaygroupStatsDto> =
        apiCall { api.getPlaygroupStats(playgroupId) }
}
