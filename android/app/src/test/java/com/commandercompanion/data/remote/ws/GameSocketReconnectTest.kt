package com.commandercompanion.data.remote.ws

import com.commandercompanion.domain.model.GameSocketEvent
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** Covers [reconnectingGameSocketFlow]'s retry/backoff behavior against a fake `connectOnce`. */
@OptIn(ExperimentalCoroutinesApi::class)
class GameSocketReconnectTest {

    @Test
    fun `reintenta con backoff exponencial mientras nunca llega a autenticar`() = runTest {
        val scheduler = testScheduler
        val attemptTimes = mutableListOf<Long>()

        val job = launch {
            reconnectingGameSocketFlow(accessToken = { "token" }) { _ ->
                attemptTimes += scheduler.currentTime
                emptyFlow()
            }.collect { }
        }

        advanceTimeBy(7_001)
        runCurrent()
        job.cancel()

        assertEquals(listOf(0L, 1_000L, 3_000L, 7_000L), attemptTimes.take(4))
    }

    @Test
    fun `el backoff vuelve al valor inicial despues de una conexion exitosa`() = runTest {
        val scheduler = testScheduler
        val attemptTimes = mutableListOf<Long>()
        var attempt = 0

        val job = launch {
            reconnectingGameSocketFlow(accessToken = { "token" }) { _ ->
                attemptTimes += scheduler.currentTime
                attempt++
                // The 2nd attempt gets far enough to authenticate, then the room drops it.
                if (attempt == 2) flowOf(GameSocketEvent.Connected) else emptyFlow()
            }.collect { }
        }

        advanceTimeBy(2_001)
        runCurrent()
        job.cancel()

        // 1st retry after 1000ms (initial backoff); the 2nd attempt authenticates, so the 3rd
        // one also waits only 1000ms — not the 2000ms it would take without the reset.
        assertEquals(listOf(0L, 1_000L, 2_000L), attemptTimes)
    }

    @Test
    fun `no reintenta despues de GameFinished`() = runTest {
        var attempts = 0

        val job = launch {
            reconnectingGameSocketFlow(accessToken = { "token" }) { _ ->
                attempts++
                flowOf(GameSocketEvent.Connected, GameSocketEvent.GameFinished)
            }.collect { }
        }

        advanceTimeBy(60_000)
        runCurrent()

        assertTrue(job.isCompleted)
        assertEquals(1, attempts)
    }

    @Test
    fun `sin token disponible tambien hace backoff en vez de reintentar en bucle cerrado`() = runTest {
        val scheduler = testScheduler
        val attemptTimes = mutableListOf<Long>()
        var hasToken = false

        val job = launch {
            reconnectingGameSocketFlow(accessToken = { if (hasToken) "token" else null }) { _ ->
                attemptTimes += scheduler.currentTime
                emptyFlow()
            }.collect { }
        }

        advanceTimeBy(1_001)
        runCurrent()
        hasToken = true
        advanceTimeBy(2_001)
        runCurrent()
        job.cancel()

        // No-token backoff ticks at t=0 and t=1000 (1000ms, then doubled to 2000ms) never call
        // connectOnce at all; the token becomes available in between, so the first real
        // connection attempt only happens once that 2nd backoff tick's delay elapses, at t=3000.
        assertEquals(listOf(3_000L), attemptTimes)
    }
}
