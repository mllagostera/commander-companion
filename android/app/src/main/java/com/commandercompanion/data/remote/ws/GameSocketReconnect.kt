package com.commandercompanion.data.remote.ws

import com.commandercompanion.domain.model.GameSocketEvent
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow

internal const val RECONNECT_INITIAL_BACKOFF_MS = 1_000L
internal const val RECONNECT_MAX_BACKOFF_MS = 30_000L

/**
 * Wraps a single-connection attempt ([connectOnce]) with reconnection and exponential backoff.
 *
 * Kept as a standalone function (not a method on [OkHttpGameSocketClient]) so the retry/backoff
 * behavior can be unit tested against a fake [connectOnce] without a real socket — see
 * `GameSocketReconnectTest`.
 *
 * Backoff resets to [RECONNECT_INITIAL_BACKOFF_MS] after any attempt that got far enough to see a
 * [GameSocketEvent.Connected] (a real server, even if it drops the connection moments later
 * counts as "reachable" for backoff purposes); it keeps doubling, up to
 * [RECONNECT_MAX_BACKOFF_MS], while attempts fail before ever authenticating (no token, no
 * network, server unreachable).
 */
internal fun reconnectingGameSocketFlow(
    accessToken: suspend () -> String?,
    initialBackoffMs: Long = RECONNECT_INITIAL_BACKOFF_MS,
    maxBackoffMs: Long = RECONNECT_MAX_BACKOFF_MS,
    connectOnce: suspend (token: String) -> Flow<GameSocketEvent>
): Flow<GameSocketEvent> = flow {
    // Delay for the NEXT retry after an attempt that never even saw Connected; doubles only
    // after being spent, so the very first retry always waits exactly [initialBackoffMs], not
    // double it.
    var backoffMs = initialBackoffMs

    while (true) {
        val token = accessToken()
        if (token == null) {
            emit(GameSocketEvent.Disconnected(willRetry = true))
            delay(backoffMs)
            backoffMs = (backoffMs * 2).coerceAtMost(maxBackoffMs)
            continue
        }

        var sawConnected = false
        var finished = false
        connectOnce(token).collect { event ->
            if (event is GameSocketEvent.Connected) sawConnected = true
            if (event is GameSocketEvent.GameFinished) finished = true
            emit(event)
        }

        if (finished) return@flow

        val delayMs = if (sawConnected) initialBackoffMs else backoffMs
        emit(GameSocketEvent.Disconnected(willRetry = true))
        delay(delayMs)
        backoffMs = if (sawConnected) initialBackoffMs else (backoffMs * 2).coerceAtMost(maxBackoffMs)
    }
}
