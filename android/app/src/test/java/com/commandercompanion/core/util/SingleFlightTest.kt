package com.commandercompanion.core.util

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * Covers [SingleFlight]'s dedup behavior — the fix for `SessionManager.refreshAccessToken()`
 * triggering more than one real `/auth/refresh` call when several requests 401 at once (see the
 * class doc on [SingleFlight] for why that matters: the backend rotates the refresh token, so a
 * second concurrent call would look like a replay and get the whole session family revoked).
 */
@OptIn(ExperimentalCoroutinesApi::class)
class SingleFlightTest {

    @Test
    fun `varias llamadas concurrentes colapsan en una sola ejecucion de block`() = runTest {
        val singleFlight = SingleFlight<Int>(this)
        var executions = 0
        val gate = CompletableDeferred<Int>()

        val callers = List(5) {
            async {
                singleFlight.run {
                    executions++
                    gate.await()
                }
            }
        }
        // Lets all 5 callers reach and suspend on the same in-flight execution before it resolves.
        advanceUntilIdle()
        assertEquals("block debería haberse ejecutado una sola vez", 1, executions)

        gate.complete(42)
        val results = callers.awaitAll()

        assertEquals(1, executions)
        assertTrue(results.all { it == 42 })
    }

    @Test
    fun `una llamada tras completarse la anterior vuelve a ejecutar block, no queda cacheado`() = runTest {
        val singleFlight = SingleFlight<Int>(this)
        var executions = 0

        val first = singleFlight.run { executions++; 1 }
        val second = singleFlight.run { executions++; 2 }

        assertEquals(1, first)
        assertEquals(2, second)
        assertEquals(2, executions)
    }

    @Test
    fun `si block lanza, la excepcion se propaga y la siguiente llamada reintenta en vez de quedar bloqueada`() =
        runTest {
            val singleFlight = SingleFlight<Int>(CoroutineScope(SupervisorJob()))
            var executions = 0

            val failure = runCatching {
                singleFlight.run {
                    executions++
                    throw IllegalStateException("boom")
                }
            }

            assertTrue(failure.isFailure)
            assertEquals(1, executions)

            val retried = singleFlight.run { executions++; 7 }

            assertEquals(7, retried)
            assertEquals(2, executions)
        }
}
