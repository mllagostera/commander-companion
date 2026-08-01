package com.commandercompanion.presentation.screens.game

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.toUserMessage
import com.commandercompanion.data.remote.ws.GameSocketEvent
import com.commandercompanion.data.repository.GameRepository
import com.commandercompanion.data.repository.LocalSeat
import com.commandercompanion.data.repository.LocalSeatResult
import com.commandercompanion.data.repository.RemoteGameSession
import com.commandercompanion.data.repository.SeatAssignment
import com.commandercompanion.data.session.AccessTokenProvider
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.theme.colorForKey
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject

@HiltViewModel
class GameViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val gameRepository: GameRepository,
    private val accessTokenProvider: AccessTokenProvider
) : ViewModel() {

    private val gameId: String = checkNotNull(savedStateHandle["gameId"])
    private val playerConfigs = decodePlayerConfigs(checkNotNull(savedStateHandle["playersEncoded"]))
    private val startingPlayerSeat: Int = savedStateHandle["startingPlayerSeat"] ?: -1

    /** Group chosen in Group mode (`PlayerSetupScreen`), or null in Casual mode. */
    private val playgroupId: String? = savedStateHandle["playgroupId"]

    /**
     * Backend game mirrored by this device, or null if it couldn't be created / doesn't apply
     * (Casual mode, or Group mode with no seat assigned). See [GameRepository]: any
     * seat present in [RemoteGameSession.seatPlayerIds] has a real `GamePlayer`, not just the
     * authenticated user's — Group mode can seat several teammates at once
     * (proxy-join, see the backend's ADR-0013).
     */
    private var remoteSession: RemoteGameSession? = null

    /**
     * Serializes remote operations: their order matters and can't be left to how two loose
     * coroutines happen to interleave. A `LifeChange` arriving after `finish` would be
     * rejected with 409 ("game not active"), and an action emitted before the bootstrap
     * finishes wouldn't yet have a `GamePlayer` to attribute itself to.
     *
     * kotlinx's `Mutex` is FIFO, so calls come out in the same order the UI generated them.
     */
    private val remoteMutex = Mutex()

    /** Collects [GameRepository.observeGameEvents] while the remote game is [RemoteSyncStatus.Synced]. */
    private var socketJob: Job? = null

    private val _state = mutableStateOf(
        GameState(
            players = playerConfigs.mapIndexed { index, config ->
                PlayerState(
                    id = index + 1,
                    name = config.name,
                    color = colorForKey(config.colorKey),
                    mulligans = config.mulligans
                )
            },
            startingPlayerId = startingPlayerSeat.takeIf { it >= 0 }?.plus(1)
        )
    )
    val state: State<GameState> = _state

    init {
        persistNewGame()
        bootstrapRemoteGame()
    }

    private fun persistNewGame() {
        val seats = _state.value.players.map { player ->
            LocalSeat(
                seatIndex = player.id - 1,
                name = player.name,
                colorKey = playerConfigs[player.id - 1].colorKey,
                life = player.life,
                mulligans = player.mulligans
            )
        }
        viewModelScope.launch { gameRepository.persistNewLocalGame(gameId, seats) }
    }

    /**
     * Creates the game on the backend and seats every seat that ended up assigned to a
     * real user with a chosen deck (`PlayerConfig.assignedUserId`/`deckId`, set by
     * `PlayerSetupScreen` in Group mode).
     *
     * Deliberately best-effort: if it fails (no network, no session) the game is still
     * playable locally, and only the reason is reflected in [GameState.remoteSync].
     */
    private fun bootstrapRemoteGame() {
        val assignments = playerConfigs.mapIndexedNotNull { index, config ->
            val userId = config.assignedUserId
            val deckId = config.deckId
            if (userId != null && deckId != null) SeatAssignment(index, userId, deckId) else null
        }
        launchRemote {
            gameRepository.bootstrapRemoteGame(playgroupId, assignments).fold(
                onSuccess = { session ->
                    remoteSession = session
                    updateRemoteSync(
                        when {
                            session == null -> RemoteSyncState(
                                status = RemoteSyncStatus.Disabled,
                                message = "Nadie quedó asignado: la partida se juega solo en este dispositivo"
                            )
                            session.isActive -> {
                                observeGameSocket(session.gameId)
                                RemoteSyncState(status = RemoteSyncStatus.Synced, gameId = session.gameId)
                            }
                            else -> RemoteSyncState(
                                status = RemoteSyncStatus.WaitingForPlayers,
                                message = "Esperando a que se una otro jugador para iniciarla en el servidor",
                                gameId = session.gameId
                            )
                        }
                    )
                },
                onFailure = { error -> reportRemoteFailure(error) }
            )
        }
    }

    fun adjustLife(playerId: Int, amount: Int) {
        if (_state.value.isFinished) return
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == playerId) {
                    player.copy(life = player.life + amount)
                } else {
                    player
                }
            }
        )
        mirrorLifeChange(playerId, amount)
        checkForGameOver()
    }

    fun adjustCommanderDamage(targetPlayerId: Int, attackerId: Int, amount: Int) {
        if (_state.value.isFinished) return
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == targetPlayerId) {
                    val currentDamage = player.commanderDamage[attackerId] ?: 0
                    val newDamage = (currentDamage + amount).coerceAtLeast(0)
                    player.copy(
                        life = player.life - amount, // Combat damage from a commander also reduces life
                        commanderDamage = player.commanderDamage + (attackerId to newDamage)
                    )
                } else {
                    player
                }
            }
        )
        mirrorCommanderDamage(attackerId, targetPlayerId, amount)
        checkForGameOver()
    }

    fun adjustPoison(playerId: Int, amount: Int) {
        if (_state.value.isFinished) return
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == playerId) {
                    player.copy(poison = (player.poison + amount).coerceAtLeast(0))
                } else {
                    player
                }
            }
        )
        mirrorPoisonChange(playerId, amount)
        checkForGameOver()
    }

    fun nextTurn() {
        _state.value = _state.value.copy(currentTurn = _state.value.currentTurn + 1)
    }

    private fun checkForGameOver() {
        val alive = _state.value.players.filter { it.isAlive() }
        if (alive.size == 1 && _state.value.players.size > 1) {
            finishGame(winnerId = alive.first().id)
        }
    }

    fun finishGame(winnerId: Int? = null) {
        if (_state.value.isFinished) return
        val resolvedWinnerId = winnerId ?: _state.value.players
            .filter { it.isAlive() }
            .maxByOrNull { it.life }
            ?.takeIf { player -> _state.value.players.count { it.life == player.life } == 1 }
            ?.id

        _state.value = _state.value.copy(isFinished = true, winnerId = resolvedWinnerId)
        persistGameResult(resolvedWinnerId)
        finishRemoteGame()
    }

    /** Alive = not eliminated (see [isEliminated], shared with the tracker UI). */
    private fun PlayerState.isAlive(): Boolean = !isEliminated()

    private fun persistGameResult(winnerId: Int?) {
        val results = _state.value.players.map { player ->
            LocalSeatResult(
                seatIndex = player.id - 1,
                finalLife = player.life,
                won = player.id == winnerId
            )
        }
        viewModelScope.launch { gameRepository.persistLocalResult(gameId, results) }
    }

    /**
     * Mirrors [playerId]'s life change; no-op if that seat has no real `GamePlayer` or
     * the remote game isn't active.
     *
     * The session is read INSIDE the serialized block: if the user touches life while the
     * bootstrap is still in flight, the action waits for it to finish instead of being dropped.
     */
    private fun mirrorLifeChange(playerId: Int, amount: Int) {
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = session.seatPlayerIds[playerId - 1] ?: return@launchRemote
            gameRepository.recordLifeChange(session, remotePlayerId, amount)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /**
     * Mirrors commander damage from [attackerId] against [targetPlayerId]; no-op if EITHER of
     * the two seats has no real `GamePlayer` (attributing damage to someone else's `GamePlayer`,
     * or sending it without a real actor, would corrupt another user's statistics).
     */
    private fun mirrorCommanderDamage(attackerId: Int, targetPlayerId: Int, amount: Int) {
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val attackerPlayerId = session.seatPlayerIds[attackerId - 1] ?: return@launchRemote
            val defenderPlayerId = session.seatPlayerIds[targetPlayerId - 1] ?: return@launchRemote
            gameRepository.recordCommanderDamage(session, attackerPlayerId, defenderPlayerId, amount)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Mirrors a poison counter change of [playerId]; no-op if it has no real `GamePlayer`. */
    private fun mirrorPoisonChange(playerId: Int, amount: Int) {
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = session.seatPlayerIds[playerId - 1] ?: return@launchRemote
            gameRepository.recordPoisonChange(session, remotePlayerId, amount)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Finishes the remote game, which triggers the server-side statistics recalculation. */
    private fun finishRemoteGame() {
        socketJob?.cancel()
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            gameRepository.finishGame(session.gameId)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /**
     * Subscribes to live updates for [remoteGameId] (see `GameRepository.observeGameEvents`/ADR-0005).
     * Only makes sense once the remote game is `active` — `pending`-state transitions
     * (join/leave/start) aren't broadcast, so connecting any earlier would just wait for nothing.
     */
    private fun observeGameSocket(remoteGameId: String) {
        socketJob?.cancel()
        socketJob = viewModelScope.launch {
            gameRepository.observeGameEvents(remoteGameId) { accessTokenProvider.currentAccessToken() }
                .collect { event -> handleSocketEvent(event) }
        }
    }

    /**
     * Every `GamePlayer` this device mirrors actions for is one of ITS OWN local seats — Casual
     * mode has none, Group mode proxy-joins its own group's seats (see `GameRepository`,
     * ADR-0013). There's no flow yet for a second device to join an already-created remote game
     * (the "Complete end-to-end flow" gap in Stage 5 of `docs/roadmap/TASKS.md`), so **every**
     * `game_action` this socket can currently receive for this room is necessarily the server's
     * echo of an action this same device already recorded and applied synchronously to
     * [GameState] before mirroring it — there's nothing new to reconcile, and applying it again
     * would double-count the change. `GameFinished`/`Connected`/`Disconnected` need no handling
     * either: this device already knows locally when its own game finishes, and reconnection is
     * fully handled inside `GameSocketClient`.
     *
     * Kept as an explicit no-op (rather than not subscribing at all) so the connection,
     * authentication and reconnection machinery run for real against the backend today, ready to
     * start reconciling other seats' actions the moment that missing join flow exists.
     */
    private fun handleSocketEvent(event: GameSocketEvent) {
        when (event) {
            is GameSocketEvent.ActionReceived,
            GameSocketEvent.GameFinished,
            GameSocketEvent.Connected,
            is GameSocketEvent.Disconnected -> Unit
        }
    }

    private fun activeSession(): RemoteGameSession? = remoteSession?.takeIf { it.isActive }

    /** Queues a remote operation respecting emission order (see [remoteMutex]). */
    private fun launchRemote(block: suspend () -> Unit) {
        viewModelScope.launch { remoteMutex.withLock { block() } }
    }

    private fun reportRemoteFailure(error: Throwable) {
        updateRemoteSync(
            RemoteSyncState(
                status = RemoteSyncStatus.Failed,
                message = (error as? ApiError)?.toUserMessage()
                    ?: "No se pudo sincronizar la partida con el servidor",
                gameId = remoteSession?.gameId
            )
        )
    }

    private fun updateRemoteSync(remoteSync: RemoteSyncState) {
        _state.value = _state.value.copy(remoteSync = remoteSync)
    }
}
