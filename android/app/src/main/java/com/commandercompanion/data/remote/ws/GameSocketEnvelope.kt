package com.commandercompanion.data.remote.ws

import com.commandercompanion.domain.model.GameAction
import com.commandercompanion.domain.model.GameSocketEvent
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.decodeFromJsonElement

private const val EVENT_CONNECTED = "connected"
private const val EVENT_GAME_ACTION = "game_action"
private const val EVENT_GAME_FINISHED = "game_finished"

/** The auth handshake message every client must send as the first frame (see ADR-0005). */
@Serializable
internal data class AuthSocketMessage(val type: String = "auth", val token: String)

/**
 * Mirrors the backend's `Envelope` (`internal/websocket/envelope.go`): the common shape of every
 * message the server sends. `payload` is left as a raw [JsonElement] because its schema depends
 * on `type` (empty for `connected`/`game_finished`, a `GameActionResponse` for `game_action`, an
 * `{"message": "..."}` for `error`, which only ever appears during the auth handshake).
 */
@Serializable
private data class SocketEnvelope(
    val type: String,
    @SerialName("game_id") val gameId: String,
    val payload: JsonElement? = null
)

/**
 * Translates one raw WebSocket text frame into a [GameSocketEvent], or null if it isn't one this
 * client acts on (malformed JSON, an unknown `type`, or `error` — which only appears during the
 * auth handshake, by which point the server is already closing the socket, see ADR-0005).
 *
 * A pure function on purpose: it's the part of [OkHttpGameSocketClient] that's worth unit
 * testing without a real socket.
 */
internal fun parseEnvelope(json: Json, text: String): GameSocketEvent? {
    val envelope = runCatching { json.decodeFromString<SocketEnvelope>(text) }.getOrNull() ?: return null
    return when (envelope.type) {
        EVENT_CONNECTED -> GameSocketEvent.Connected
        EVENT_GAME_FINISHED -> GameSocketEvent.GameFinished
        EVENT_GAME_ACTION -> envelope.payload
            ?.let { payload -> runCatching { json.decodeFromJsonElement<GameAction>(payload) }.getOrNull() }
            ?.let { GameSocketEvent.ActionReceived(it) }
        else -> null
    }
}
