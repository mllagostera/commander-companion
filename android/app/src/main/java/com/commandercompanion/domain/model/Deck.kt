package com.commandercompanion.domain.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * A deck belonging to the authenticated user, following the `Deck` schema in
 * `docs/api/openapi.yaml` (and `backend/internal/decks/dto.go`).
 *
 * Lives in `domain/` rather than in `data/remote/dto/` so the repository
 * interfaces that traffic in it don't have to reach into the data layer — see
 * `docs/architecture/PROJECT-STRUCTURE.md` §9. It keeps its serialization
 * annotations, so Retrofit deserializes straight into it: the deliberate
 * trade-off is that a rename in the REST contract lands here, in exchange for
 * not maintaining a parallel set of types and mappers. The request bodies
 * (`CreateDeckRequest`, `ImportMoxfieldRequest`) stay in `data/`: nothing in
 * the domain names them.
 */
@Serializable
data class Deck(
    val id: String,
    @SerialName("user_id") val userId: String,
    val name: String,
    val commander: String,
    @SerialName("moxfield_id") val moxfieldId: String? = null,
    @SerialName("image_url") val imageUrl: String? = null
)
