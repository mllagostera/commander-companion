package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.PlaygroupDto
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Acceso a los grupos de juego (`playgroups`) del usuario autenticado. Usado por el modo
 * Grupo de `PlayerSetupScreen`: elegir un grupo, asignar sus miembros a los asientos, y ver
 * los decks de un compañero para un proxy-join (ver ADR-0013 del backend).
 */
@Singleton
class PlaygroupRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun listPlaygroups(): Result<List<PlaygroupDto>> = apiCall { api.listPlaygroups() }

    suspend fun getPlaygroup(playgroupId: String): Result<PlaygroupDto> =
        apiCall { api.getPlaygroup(playgroupId) }

    suspend fun getMemberDecks(playgroupId: String, userId: String): Result<List<DeckDto>> =
        apiCall { api.getMemberDecks(playgroupId, userId) }
}
