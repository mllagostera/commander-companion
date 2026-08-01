package com.commandercompanion.domain.usecase

import com.commandercompanion.domain.model.PlayerOutcome
import javax.inject.Inject

/**
 * Commander's "who won" rules, extracted from `GameViewModel` so they're testable without a
 * `ViewModel`/`SavedStateHandle`: when a game ends on its own (exactly one player left standing)
 * and, when it's finished manually, who gets credited as the winner.
 */
class ResolveGameOutcomeUseCase @Inject constructor() {

    /**
     * The game ends itself the moment only one player remains alive — but only among 2+
     * configured players; a single-player session (if one were ever created) never auto-finishes.
     * Returns that player's id, or null if the game should keep going.
     */
    fun automaticWinner(players: List<PlayerOutcome>): Int? {
        if (players.size <= 1) return null
        return players.filter { it.isAlive }.singleOrNull()?.id
    }

    /**
     * Resolves who won when a game is finished. [explicitWinnerId] (already known, e.g. from
     * [automaticWinner] or a manual pick) always wins. Otherwise, the sole alive player with the
     * strictly highest life total — among ALL players, not just the alive ones, so a tie with an
     * eliminated player's frozen life total also blocks a winner instead of picking one arbitrarily.
     * Null if nobody is alive or there's a tie.
     */
    fun resolveWinner(players: List<PlayerOutcome>, explicitWinnerId: Int? = null): Int? {
        if (explicitWinnerId != null) return explicitWinnerId
        val topAlive = players.filter { it.isAlive }.maxByOrNull { it.life } ?: return null
        val tiedForTopLife = players.count { it.life == topAlive.life }
        return topAlive.id.takeIf { tiedForTopLife == 1 }
    }
}
