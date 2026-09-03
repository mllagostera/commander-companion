package com.commandercompanion.domain.usecase

import com.commandercompanion.domain.model.GameAction
import com.commandercompanion.domain.model.GameActionType
import com.commandercompanion.domain.model.amount
import javax.inject.Inject

/**
 * Reconstructs the per-seat, per-attacker-seat commander damage breakdown by replaying a game's
 * `CommanderDamage` actions — `GamePlayerResponse` only exposes each player's current totals, not
 * that breakdown, so `GameViewModel` needs this when a device joins an already-in-progress game
 * (see `JoinGameScreen`).
 */
class ReplayCommanderDamageUseCase @Inject constructor() {

    /** [seatByPlayerId] maps a `GamePlayer.id` to its 1-based seat id (`PlayerState.id`). */
    operator fun invoke(actions: List<GameAction>, seatByPlayerId: Map<String, Int>): Map<Int, Map<Int, Int>> {
        val damage = mutableMapOf<Int, MutableMap<Int, Int>>()
        actions.filter { it.actionType == GameActionType.COMMANDER_DAMAGE }.forEach { action ->
            val attackerSeat = seatByPlayerId[action.actorId] ?: return@forEach
            val targetSeat = action.targetId?.let { seatByPlayerId[it] } ?: return@forEach
            val amount = action.amount ?: return@forEach
            val perOpponent = damage.getOrPut(targetSeat) { mutableMapOf() }
            perOpponent[attackerSeat] = (perOpponent[attackerSeat] ?: 0) + amount
        }
        return damage
    }
}
