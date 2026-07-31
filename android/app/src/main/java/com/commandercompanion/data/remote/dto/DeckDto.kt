package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for `/decks`, following the `Deck`/`CreateDeckRequest`/`ImportMoxfieldRequest` schemas
 * from `docs/api/openapi.yaml` (and the real structs from `backend/internal/decks/dto.go`).
 */

@Serializable
data class DeckDto(
    val id: String,
    @SerialName("user_id") val userId: String,
    val name: String,
    val commander: String,
    @SerialName("moxfield_id") val moxfieldId: String? = null
)

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
