package com.commandercompanion.data.remote.ws

import com.commandercompanion.BuildConfig
import com.commandercompanion.core.di.WebSocketOkHttpClient
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import javax.inject.Inject

private const val NORMAL_CLOSURE_CODE = 1000

/**
 * Real [GameSocketClient]: opens `GET {API_BASE_URL}api/v1/ws/games/{gameId}` with OkHttp (the
 * upgrade works over a plain `http(s)://` URL — OkHttp's `WebSocket` doesn't need a `ws(s)://`
 * scheme), sends the `{"type":"auth","token":"..."}` handshake message as soon as the socket
 * opens (see ADR-0005), and translates every text frame via [parseEnvelope].
 *
 * [okHttpClient] must have no read timeout (an idle-but-alive room would otherwise be killed by
 * the same 15s timeout the REST clients use) — see the `@WebSocketOkHttpClient`-qualified
 * provider in `NetworkModule`.
 */
class OkHttpGameSocketClient @Inject constructor(
    @WebSocketOkHttpClient private val okHttpClient: OkHttpClient,
    private val json: Json
) : GameSocketClient {

    override fun connect(gameId: String, accessToken: suspend () -> String?): Flow<GameSocketEvent> =
        reconnectingGameSocketFlow(accessToken) { token -> connectOnce(gameId, token) }

    private fun connectOnce(gameId: String, token: String): Flow<GameSocketEvent> = callbackFlow {
        val request = Request.Builder()
            .url("${BuildConfig.API_BASE_URL}api/v1/ws/games/$gameId")
            .build()
        val authMessage = json.encodeToString(AuthSocketMessage(token = token))

        val listener = object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                webSocket.send(authMessage)
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                parseEnvelope(json, text)?.let { trySend(it) }
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                close()
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                close()
            }
        }

        val webSocket = okHttpClient.newWebSocket(request, listener)
        awaitClose { webSocket.close(NORMAL_CLOSURE_CODE, null) }
    }
}
