package com.commandercompanion.domain.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

/**
 * Actions recorded against a game (`/games/{id}/actions`, `/games/{id}/timeline`).
 *
 * `payload` is a raw [JsonObject] because the backend models it as
 * `map[string]interface{}` and its shape depends on `action_type` (today only
 * `{ "amount": <int> }` for numeric types). See [Deck] for why these types live
 * in `domain/` while keeping their serialization annotations.
 */

/**
 * Fixed `action_type` vocabulary validated by the backend
 * (`backend/internal/game-actions/service.go` + `CHECK` constraint in migration `00004`).
 */
object GameActionType {
    const val LIFE_CHANGE = "LifeChange"
    const val COMBAT_DAMAGE = "CombatDamage"
    const val COMMANDER_DAMAGE = "CommanderDamage"
    const val POISON_COUNTER = "PoisonCounter"
    const val TURN_START = "TurnStart"
    const val TURN_END = "TurnEnd"
    const val ELIMINATION = "Elimination"
}

/**
 * An action about to be recorded. [actorId] and [targetId] are [GamePlayer] ids
 * (not user ids); a null [targetId] means the action affects the actor itself.
 *
 * Unlike the other request bodies this one is named by [com.commandercompanion.domain.repository.GameRepository],
 * so it belongs to the domain rather than to `data/remote/dto/`.
 */
@Serializable
data class NewGameAction(
    @SerialName("actor_id") val actorId: String,
    @SerialName("target_id") val targetId: String? = null,
    @SerialName("action_type") val actionType: String,
    val payload: JsonObject? = null
)

@Serializable
data class GameAction(
    val id: String,
    @SerialName("game_id") val gameId: String,
    @SerialName("actor_id") val actorId: String,
    @SerialName("target_id") val targetId: String? = null,
    @SerialName("action_type") val actionType: String,
    val payload: JsonObject? = null,
    @SerialName("created_at") val createdAt: String
)

/** Standard payload for numeric actions: `{ "amount": <int> }`. */
fun amountPayload(amount: Int): JsonObject = buildJsonObject { put("amount", amount) }

/** Reads `payload.amount` from a timeline action; null if it doesn't apply to that `action_type`. */
val GameAction.amount: Int?
    get() = (payload?.get("amount") as? JsonPrimitive)?.intOrNull
