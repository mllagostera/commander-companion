package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Request bodies for `/decks`, following the `CreateDeckRequest`/`ImportMoxfieldRequest`
 * schemas from `docs/api/openapi.yaml` (and the real structs from
 * `backend/internal/decks/dto.go`).
 *
 * The response shape lives in `domain/model/Deck.kt`: the repository interfaces
 * name it, so it belongs to the domain. These bodies stay here because nothing
 * outside `data/` ever names them.
 */

@Serializable
data class CreateDeckRequest(
    val name: String,
    val commander: String,
    @SerialName("moxfield_id") val moxfieldId: String? = null
)

/** [url] accepts either the full URL (`https://moxfield.com/decks/{id}`) or the bare public ID. */
@Serializable
data class ImportMoxfieldRequest(
    val url: String
)
