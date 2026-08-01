package com.commandercompanion.domain.model

import com.commandercompanion.data.remote.dto.GameStatus

/** A local tracker seat, as configured by the user in `PlayerSetupScreen`. */
data class LocalSeat(
    val seatIndex: Int,
    val name: String,
    val colorKey: String,
    val life: Int,
    val mulligans: Int
)

/** Final result of a local seat, to persist when the game ends. */
data class LocalSeatResult(
    val seatIndex: Int,
    val finalLife: Int,
    val won: Boolean
)

/**
 * A seat to remotely seat in the bootstrap: which user ([userId]) and with which deck
 * ([deckId]). The backend decides whether it's a self-join or a proxy-join by comparing [userId]
 * against the authenticated user (see the backend's ADR-0013) — Android never needs to know
 * "which seat am I", just pass the assignment as it ended up in `PlayerSetupScreen`.
 */
data class SeatAssignment(val seatIndex: Int, val userId: String, val deckId: String)

/**
 * A backend game in which this device seated one or more seats with a real
 * `GamePlayer` — its own and, in Group mode, those of proxy-joined teammates. [seatPlayerIds]
 * maps the local seat index (0-based, the same as `PlayerConfig`/`PlayerSetupScreen`) to its
 * `GamePlayer`'s ID — the `actor_id`/`target_id` expected by `POST /games/{id}/actions` (which is
 * NOT the `user_id`).
 */
data class RemoteGameSession(
    val gameId: String,
    val seatPlayerIds: Map<Int, String>,
    val status: String
) {
    val isActive: Boolean get() = status == GameStatus.ACTIVE
}
