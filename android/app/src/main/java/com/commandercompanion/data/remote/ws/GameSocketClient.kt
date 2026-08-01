package com.commandercompanion.data.remote.ws

import kotlinx.coroutines.flow.Flow

/**
 * Live game synchronization over WebSocket, consuming the backend's protocol (see the backend's
 * `docs/decisions/0005-websocket-protocol.md`, ADR-0005): connection, authentication and
 * reconnection with backoff for a single game room.
 *
 * REST remains the source of truth (no delivery guarantees/replay on this channel, see the ADR)
 * — a collector is expected to already treat [GameSocketEvent.ActionReceived] as a best-effort
 * notification, not the only place it learns about a change.
 */
interface GameSocketClient {

    /**
     * Subscribes to [gameId]'s room. [accessToken] is invoked on every (re)connection attempt —
     * not just once — so a token refreshed meanwhile (e.g. by the authenticated REST client's
     * 401 authenticator, `AuthAuthenticator`) is picked up on the next retry instead of being
     * stuck on a stale one. A null token (no session) is treated the same as a failed connection
     * attempt: back off and try again.
     *
     * The returned Flow keeps reconnecting with exponential backoff until either the collecting
     * coroutine is cancelled or a [GameSocketEvent.GameFinished] is received, after which it
     * completes normally — the server closes the socket itself once a game finishes, so there's
     * nothing left to reconnect for.
     */
    fun connect(gameId: String, accessToken: suspend () -> String?): Flow<GameSocketEvent>
}
