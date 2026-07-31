package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

/**
 * DTOs for `/games/{id}/actions` and `/games/{id}/timeline`.
 *
 * `payload` is a raw [JsonObject] because the backend models it as `map[string]interface{}`
 * and its shape depends on `action_type` (today only `{ "amount": <int> }` for numeric types).
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
 * [actorId] and [targetId] are `GamePlayer` IDs (not user IDs). If [targetId] is null the action
 * affects the actor itself.
 */
@Serializable
data class CreateActionRequest(
    @SerialName("actor_id") val actorId: String,
    @SerialName("target_id") val targetId: String? = null,
    @SerialName("action_type") val actionType: String,
    val payload: JsonObject? = null
)

@Serializable
data class GameActionDto(
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
val GameActionDto.amount: Int?
    get() = (payload?.get("amount") as? JsonPrimitive)?.intOrNull
