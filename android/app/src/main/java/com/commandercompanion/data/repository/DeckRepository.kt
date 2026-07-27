package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Acceso a los decks del usuario autenticado.
 *
 * Hoy es 100% remoto: los decks viven solo en el backend y no hay entidad Room para ellos. Se
 * modela igual como repositorio para que los `ViewModel` nunca toquen [CommanderApi] directo y
 * para tener un único lugar donde meter caché offline más adelante (Stage 5 de `TASKS.md`,
 * "Room como caché offline-first ... decks propios").
 */
@Singleton
class DeckRepository @Inject constructor(
    private val api: CommanderApi
) {

    suspend fun listDecks(): Result<List<DeckDto>> = apiCall { api.listDecks().items }

    suspend fun getDeck(deckId: String): Result<DeckDto> = apiCall { api.getDeck(deckId) }

    suspend fun createDeck(
        name: String,
        commander: String,
        moxfieldId: String? = null
    ): Result<DeckDto> = apiCall {
        api.createDeck(CreateDeckRequest(name = name, commander = commander, moxfieldId = moxfieldId))
    }

    /** [urlOrPublicId] acepta la URL completa de Moxfield o solo el ID público. */
    suspend fun importFromMoxfield(urlOrPublicId: String): Result<DeckDto> = apiCall {
        api.importMoxfieldDeck(ImportMoxfieldRequest(urlOrPublicId))
    }

    suspend fun deleteDeck(deckId: String): Result<Unit> = apiCall { api.deleteDeck(deckId) }

    /**
     * Primer deck del usuario, o `null` si todavía no tiene ninguno.
     *
     * Lo usa [GameRepository.bootstrapRemoteGame] para poder sentar al usuario en una partida sin
     * que exista todavía una pantalla de selección de deck.
     * TODO: reemplazar por una selección explícita en `PlayerSetupScreen` cuando exista la UI de
     * decks — elegir "el primero" es una simplificación deliberada de esta pasada.
     */
    suspend fun firstDeckId(): Result<String?> = listDecks().map { decks -> decks.firstOrNull()?.id }
}
