package com.commandercompanion.core.util

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * Collapses concurrent callers of [run] into a single execution of [block]: while one call is
 * still in flight, others just await its result instead of starting a new one. [scope] owns the
 * actual execution — pick one with a lifetime independent of any individual caller (e.g. a
 * class-level [CoroutineScope] backed by a `SupervisorJob`), not a scope tied to a single
 * `runBlocking` invocation, or the dedup only works within that one call.
 *
 * Needed for anything that must not run twice at once for the same logical operation even when
 * several coroutines/threads request it concurrently — the motivating case is
 * `SessionManager.refreshAccessToken()`: the backend **rotates** the refresh token on every call
 * (see ADR-0001), so two concurrent refreshes would have the second one present an
 * already-revoked token, which the backend treats as theft and revokes the user's whole session
 * family (`internal/auth/service.go: Refresh`, `docs/roadmap/DECISIONS-LOG.md`) — logging the
 * user out on every device just because two requests happened to 401 at the same time.
 */
class SingleFlight<T>(private val scope: CoroutineScope) {
    private val mutex = Mutex()
    private var inFlight: Deferred<T>? = null

    suspend fun run(block: suspend () -> T): T {
        val deferred = mutex.withLock {
            inFlight ?: scope.async { block() }.also { inFlight = it }
        }
        return try {
            deferred.await()
        } finally {
            mutex.withLock {
                if (inFlight === deferred) inFlight = null
            }
        }
    }
}
