package com.commandercompanion.data.repository

import com.commandercompanion.core.util.apiCall
import com.commandercompanion.data.local.dao.DeckDao
import com.commandercompanion.data.local.entity.DeckEntity
import com.commandercompanion.data.remote.api.CommanderApi
import com.commandercompanion.data.remote.dto.CreateDeckRequest
import com.commandercompanion.data.remote.dto.DeckDto
import com.commandercompanion.data.remote.dto.ImportMoxfieldRequest
import com.commandercompanion.domain.repository.DeckRepository
import javax.inject.Inject

/**
 * [DeckRepository] implementation.
 *
 * [listDecks] is network-first with a Room cache as a fallback (see [DeckDao]): every successful
 * fetch follows `next_cursor` until exhausted (`GET /decks` only returns 20 at a time — stopping
 * at the first page silently hid decks past it, e.g. for a user with a large bulk Moxfield
 * import) and fully replaces the cache with the complete list, and a failed fetch falls back to
 * whatever was cached last instead of leaving `JoinGameScreen`'s deck picker or the statistics
 * screen's per-deck list empty just because of a network blip. An empty cache still propagates the
 * original error — there's nothing useful to show either way.
 */
class DeckRepositoryImpl @Inject constructor(
    private val api: CommanderApi,
    private val deckDao: DeckDao
) : DeckRepository {

    override suspend fun listDecks(): Result<List<DeckDto>> {
        val networkResult = fetchAllPages()
        return networkResult.fold(
            onSuccess = { decks ->
                deckDao.clear()
                deckDao.insertAll(decks.map { it.toEntity() })
                Result.success(decks)
            },
            onFailure = { error ->
                val cached = deckDao.getAll().map { it.toDto() }
                if (cached.isNotEmpty()) Result.success(cached) else Result.failure(error)
            }
        )
    }

    private suspend fun fetchAllPages(): Result<List<DeckDto>> {
        val all = mutableListOf<DeckDto>()
        var cursor: String? = null
        do {
            val page = apiCall { api.listDecks(cursor) }.getOrElse { return Result.failure(it) }
            all += page.items
            cursor = page.nextCursor
        } while (cursor != null)
        return Result.success(all)
    }

    override suspend fun getDeck(deckId: String): Result<DeckDto> = apiCall { api.getDeck(deckId) }

    override suspend fun createDeck(
        name: String,
        commander: String,
        moxfieldId: String?
    ): Result<DeckDto> = apiCall {
        api.createDeck(CreateDeckRequest(name = name, commander = commander, moxfieldId = moxfieldId))
    }.onSuccess { deck -> deckDao.insert(deck.toEntity()) }

    override suspend fun importFromMoxfield(urlOrPublicId: String): Result<DeckDto> = apiCall {
        api.importMoxfieldDeck(ImportMoxfieldRequest(urlOrPublicId))
    }.onSuccess { deck -> deckDao.insert(deck.toEntity()) }

    override suspend fun deleteDeck(deckId: String): Result<Unit> =
        apiCall { api.deleteDeck(deckId) }.onSuccess { deckDao.deleteById(deckId) }
}

private fun DeckDto.toEntity() = DeckEntity(id = id, userId = userId, name = name, commander = commander, moxfieldId = moxfieldId, imageUrl = imageUrl)

private fun DeckEntity.toDto() = DeckDto(id = id, userId = userId, name = name, commander = commander, moxfieldId = moxfieldId, imageUrl = imageUrl)
