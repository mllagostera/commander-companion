package com.commandercompanion.data.remote.ws

import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Envelope shapes mirror the backend's `internal/websocket/envelope.go` (see ADR-0005). */
class GameSocketEnvelopeTest {

    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `connected se traduce a Connected`() {
        val event = parseEnvelope(json, """{"type":"connected","game_id":"game-1","timestamp":"2026-07-27T10:00:00Z"}""")

        assertEquals(GameSocketEvent.Connected, event)
    }

    @Test
    fun `game_finished se traduce a GameFinished`() {
        val event = parseEnvelope(json, """{"type":"game_finished","game_id":"game-1","timestamp":"2026-07-27T10:00:00Z"}""")

        assertEquals(GameSocketEvent.GameFinished, event)
    }

    @Test
    fun `game_action trae el GameActionResponse completo en el payload`() {
        val text = """
            {
              "type": "game_action",
              "game_id": "game-1",
              "actor_id": "gp-1",
              "payload": {
                "id": "action-1",
                "game_id": "game-1",
                "actor_id": "gp-1",
                "target_id": null,
                "action_type": "LifeChange",
                "payload": {"amount": -3},
                "created_at": "2026-07-27T10:00:00Z"
              },
              "timestamp": "2026-07-27T10:00:01Z"
            }
        """.trimIndent()

        val event = parseEnvelope(json, text)

        assertTrue(event is GameSocketEvent.ActionReceived)
        val action = (event as GameSocketEvent.ActionReceived).action
        assertEquals("action-1", action.id)
        assertEquals("gp-1", action.actorId)
        assertEquals("LifeChange", action.actionType)
        assertEquals(-3, action.amount)
    }

    @Test
    fun `error no se traduce a ningun evento (el servidor ya cierra el socket)`() {
        val event = parseEnvelope(
            json,
            """{"type":"error","game_id":"game-1","payload":{"message":"invalid or expired token"},"timestamp":"2026-07-27T10:00:00Z"}"""
        )

        assertNull(event)
    }

    @Test
    fun `un tipo desconocido se ignora`() {
        val event = parseEnvelope(json, """{"type":"something_new","game_id":"game-1","timestamp":"2026-07-27T10:00:00Z"}""")

        assertNull(event)
    }

    @Test
    fun `JSON invalido no rompe, devuelve null`() {
        val event = parseEnvelope(json, "not json at all")

        assertNull(event)
    }
}
