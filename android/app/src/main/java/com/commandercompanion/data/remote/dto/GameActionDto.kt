package com.commandercompanion.data.remote.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

/**
 * DTOs de `/games/{id}/actions` y `/games/{id}/timeline`.
 *
 * `payload` va como [JsonObject] crudo porque el backend lo modela como `map[string]interface{}`
 * y su forma depende del `action_type` (hoy solo `{ "amount": <int> }` para los tipos numéricos).
 */

/**
 * Vocabulario fijo de `action_type` que valida el backend
 * (`backend/internal/game-actions/service.go` + `CHECK` constraint en la migración `00004`).
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
 * [actorId] y [targetId] son IDs de `GamePlayer` (no de usuario). Si [targetId] es null la acción
 * afecta al propio actor.
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

/** Payload estándar de las acciones numéricas: `{ "amount": <int> }`. */
fun amountPayload(amount: Int): JsonObject = buildJsonObject { put("amount", amount) }

/** Lee `payload.amount` de una acción del timeline; null si no aplica a ese `action_type`. */
val GameActionDto.amount: Int?
    get() = (payload?.get("amount") as? JsonPrimitive)?.intOrNull
