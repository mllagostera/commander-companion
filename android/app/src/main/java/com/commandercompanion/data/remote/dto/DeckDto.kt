package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs de `/decks`, siguiendo los schemas `Deck`/`CreateDeckRequest`/`ImportMoxfieldRequest`
 * de `docs/api/openapi.yaml` (y los structs reales de `backend/internal/decks/dto.go`).
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

/** [url] acepta tanto la URL completa (`https://moxfield.com/decks/{id}`) como el ID público pelado. */
@Serializable
data class ImportMoxfieldRequest(
    val url: String
)
