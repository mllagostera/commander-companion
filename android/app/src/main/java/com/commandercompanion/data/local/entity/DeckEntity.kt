package com.commandercompanion.data.local.entity

import androidx.room.Entity
import androidx.room.PrimaryKey

/**
 * Local cache mirror of [com.commandercompanion.domain.model.Deck], keyed by the same
 * backend `id` — see `DeckRepository`'s offline-first `listDecks()`.
 */
@Entity(tableName = "decks")
data class DeckEntity(
    @PrimaryKey val id: String,
    val userId: String,
    val name: String,
    val commander: String,
    val moxfieldId: String? = null,
    val imageUrl: String? = null
)
