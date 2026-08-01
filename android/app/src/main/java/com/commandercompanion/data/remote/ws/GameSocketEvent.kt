package com.commandercompanion.data.remote.ws

import com.commandercompanion.data.remote.dto.GameActionDto

/**
 * Events emitted by [GameSocketClient] for a single game room, translated from the server's
 * envelope (see the backend's ADR-0005: `docs/decisions/0005-websocket-protocol.md`).
 */
sealed class GameSocketEvent {

    /** Sent once, right after the connection authenticates successfully. */
    object Connected : GameSocketEvent()

    /**
     * A `game_actions` action (any of the 7 action types) was just recorded for this game.
     * `payload` is exactly the `GameActionResponse` the REST API already returns, so a client
     * that knows how to render the timeline already knows how to interpret this too.
     */
    data class ActionReceived(val action: GameActionDto) : GameSocketEvent()

    /**
     * The game ended. Carries no state of its own — the server closes the room right after
     * broadcasting this, and the client is expected to reconcile via REST
     * (`GET /games/{id}`, the `/statistics` endpoints) rather than trust this channel for the
     * final state.
     */
    object GameFinished : GameSocketEvent()

    /** The connection dropped (or never authenticated) and a reconnection attempt is queued. */
    data class Disconnected(val willRetry: Boolean) : GameSocketEvent()
}
