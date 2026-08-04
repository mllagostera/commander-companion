package com.commandercompanion.presentation.screens.game

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.toUserMessage
import com.commandercompanion.data.remote.dto.GameActionDto
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.GameDto
import com.commandercompanion.data.remote.dto.amount
import com.commandercompanion.data.remote.ws.GameSocketEvent
import com.commandercompanion.data.session.AccessTokenProvider
import com.commandercompanion.domain.model.LocalSeat
import com.commandercompanion.domain.model.LocalSeatResult
import com.commandercompanion.domain.model.PlayerOutcome
import com.commandercompanion.domain.model.RemoteGameSession
import com.commandercompanion.domain.model.SeatAssignment
import com.commandercompanion.domain.repository.GameRepository
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.domain.usecase.ReplayCommanderDamageUseCase
import com.commandercompanion.domain.usecase.ResolveGameOutcomeUseCase
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.theme.PlayerColorPalette
import com.commandercompanion.presentation.theme.colorForKey
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject

private const val HTTP_CONFLICT = 409

@HiltViewModel
class GameViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val gameRepository: GameRepository,
    private val playgroupRepository: PlaygroupRepository,
    private val accessTokenProvider: AccessTokenProvider,
    private val resolveGameOutcomeUseCase: ResolveGameOutcomeUseCase,
    private val replayCommanderDamageUseCase: ReplayCommanderDamageUseCase
) : ViewModel() {

    private val gameId: String = checkNotNull(savedStateHandle["gameId"])

    /**
     * Set only by [com.commandercompanion.presentation.navigation.JoinedGameTrackerRoute]: the
     * `GamePlayer.id` this device got back from `POST /games/{id}/join` when joining SOMEONE
     * ELSE's already-created remote game (see `JoinGameScreen`), as opposed to hosting a new
     * pass-and-play session (`GameTrackerRoute`, which carries `playersEncoded` instead). Its
     * presence is what tells the two modes apart.
     */
    private val localPlayerId: String? = savedStateHandle["localPlayerId"]
    private val joinedMode: Boolean = localPlayerId != null

    private val playerConfigs: List<PlayerConfig> = if (joinedMode) {
        emptyList()
    } else {
        decodePlayerConfigs(checkNotNull(savedStateHandle["playersEncoded"]))
    }
    private val startingPlayerSeat: Int = savedStateHandle["startingPlayerSeat"] ?: -1

    /** Group chosen in Group mode (`PlayerSetupScreen`), or null in Casual mode. */
    private val playgroupId: String? = savedStateHandle["playgroupId"]

    /**
     * Backend game mirrored by this device, or null if it couldn't be created/joined. See
     * [GameRepository]: in host mode, [RemoteGameSession.seatPlayerIds] only has THIS device's own
     * seats (self- or proxy-joined, ADR-0013); in joined mode ([joinedMode]) it has EVERY seat of
     * the table, since this device also needs to reconcile the other seats' live actions — see
     * [ownedSeatIds] for which of those this device is allowed to mirror changes for.
     */
    private var remoteSession: RemoteGameSession? = null

    /**
     * Seats THIS device is the source of truth for and mirrors local UI edits from — every seat in
     * host mode (it created all of them), or just [GameState.localSeatId] in joined mode. An
     * incoming `game_action` for a seat in this set is this device's own echo (already applied
     * synchronously before mirroring it); for any other seat present in [RemoteGameSession] it's a
     * genuine update from whichever device actually controls it, see [applyRemoteAction].
     */
    private var ownedSeatIds: Set<Int> = emptySet()

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
        if (joinedMode) {
            GameState(remoteSync = RemoteSyncState(status = RemoteSyncStatus.Connecting))
        } else {
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
        }
    )
    val state: State<GameState> = _state

    init {
        if (joinedMode) {
            initJoinedGame()
        } else {
            persistNewGame()
            bootstrapRemoteGame()
        }
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
     * `PreGameScreen` in Group mode).
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
        ownedSeatIds = assignments.map { it.seatIndex + 1 }.toSet()
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

    /**
     * Joined-mode counterpart of [bootstrapRemoteGame]: instead of creating a game, fetches the
     * one this device just joined via `JoinGameScreen` (`GET /games/{id}`) and renders every seat
     * already there — usernames resolved from the game's playgroup members, commander damage
     * reconstructed by replaying `GET /games/{id}/timeline` (the backend only exposes each
     * player's current totals, not the per-opponent breakdown `GamePlayerResponse` lacks).
     */
    private fun initJoinedGame() {
        viewModelScope.launch {
            val game = gameRepository.getGame(gameId).getOrElse { error ->
                reportRemoteFailure(error)
                return@launch
            }

            val seatPlayerIds = game.players.mapIndexed { index, player -> index to player.id }.toMap()
            val localSeatId = game.players.indexOfFirst { it.id == localPlayerId }
                .takeIf { it >= 0 }
                ?.plus(1)
            ownedSeatIds = setOfNotNull(localSeatId)
            remoteSession = RemoteGameSession(gameId = game.id, seatPlayerIds = seatPlayerIds, status = game.status)

            val usernames = game.playgroupId
                ?.let { gamePlaygroupId -> playgroupRepository.getPlaygroup(gamePlaygroupId).getOrNull() }
                ?.members
                ?.associate { it.userId to it.username }
                ?: emptyMap()
            val commanderDamageBySeat = replayCommanderDamage(game)

            val players = game.players.mapIndexed { index, player ->
                val seatId = index + 1
                PlayerState(
                    id = seatId,
                    name = usernames[player.userId] ?: "Jugador $seatId",
                    color = colorForKey(PlayerColorPalette[index % PlayerColorPalette.size].first),
                    life = player.lifeTotal,
                    poison = player.poisonCounters,
                    commanderDamage = commanderDamageBySeat[seatId] ?: emptyMap()
                )
            }
            _state.value = _state.value.copy(players = players, localSeatId = localSeatId)

            updateRemoteSync(
                if (remoteSession?.isActive == true) {
                    observeGameSocket(game.id)
                    RemoteSyncState(status = RemoteSyncStatus.Synced, gameId = game.id)
                } else {
                    RemoteSyncState(
                        status = RemoteSyncStatus.WaitingForPlayers,
                        message = "Esperando a que se una otro jugador para iniciarla en el servidor",
                        gameId = game.id
                    )
                }
            )
        }
    }

    /** Replays the `CommanderDamage` actions of [game]'s timeline into a per-seat, per-attacker-seat map. */
    private suspend fun replayCommanderDamage(game: GameDto): Map<Int, Map<Int, Int>> {
        val seatByPlayerId = game.players.mapIndexed { index, player -> player.id to (index + 1) }.toMap()
        val actions = gameRepository.timeline(game.id).getOrElse { return emptyMap() }
        return replayCommanderDamageUseCase(actions, seatByPlayerId)
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

    /** Advances the turn counter and hands the ring highlight to the next seat, wrapping around. */
    fun nextTurn() {
        val seatIds = _state.value.players.map { it.id }
        val currentIndex = seatIds.indexOf(_state.value.currentTurnPlayerId)
        val nextPlayerId = if (currentIndex == -1) seatIds.firstOrNull() else seatIds[(currentIndex + 1) % seatIds.size]
        _state.value = _state.value.copy(currentTurn = _state.value.currentTurn + 1, currentTurnPlayerId = nextPlayerId)
    }

    private fun checkForGameOver() {
        val winnerId = resolveGameOutcomeUseCase.automaticWinner(_state.value.players.toOutcomes()) ?: return
        finishGame(winnerId = winnerId)
    }

    fun finishGame(winnerId: Int? = null) {
        if (_state.value.isFinished) return
        val resolvedWinnerId = resolveGameOutcomeUseCase.resolveWinner(_state.value.players.toOutcomes(), winnerId)

        _state.value = _state.value.copy(isFinished = true, winnerId = resolvedWinnerId)
        persistGameResult(resolvedWinnerId)
        finishRemoteGame()
    }

    /** Alive = not eliminated (see [isEliminated], shared with the tracker UI). */
    private fun PlayerState.isAlive(): Boolean = !isEliminated()

    private fun List<PlayerState>.toOutcomes(): List<PlayerOutcome> =
        map { PlayerOutcome(id = it.id, life = it.life, isAlive = it.isAlive()) }

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

    /**
     * Finishes the remote game, which triggers the server-side statistics recalculation. In joined
     * mode more than one device can independently reach "only one player left standing" from its
     * own view of the table (updates arrive with network lag); a 409 here just means someone else's
     * `finish` already won that race, not a real failure.
     */
    private fun finishRemoteGame() {
        socketJob?.cancel()
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            gameRepository.finishGame(session.gameId)
                .onFailure { error ->
                    val someoneElseAlreadyFinishedIt = error is ApiError.Http && error.code == HTTP_CONFLICT
                    if (!someoneElseAlreadyFinishedIt) reportRemoteFailure(error)
                }
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
     * `Connected`/`Disconnected` need no handling: reconnection is fully handled inside
     * `GameSocketClient`. `game_action` events for a seat in [ownedSeatIds] are this device's own
     * echo (see [applyRemoteAction]); a `game_finished` broadcast is only new information if THIS
     * device hasn't already reached that conclusion on its own (see [finishRemoteGame]'s note on
     * the multi-device race).
     */
    private fun handleSocketEvent(event: GameSocketEvent) {
        when (event) {
            is GameSocketEvent.ActionReceived -> applyRemoteAction(event.action)
            GameSocketEvent.GameFinished -> applyRemoteGameFinished()
            GameSocketEvent.Connected, is GameSocketEvent.Disconnected -> Unit
        }
    }

    /**
     * Reconciles a `game_action` broadcast for a seat this device does NOT own (see [ownedSeatIds])
     * into [GameState] — the live-sync half of joining someone else's game (see `JoinGameScreen`).
     * Unknown actor/target `GamePlayer` ids (not present in [RemoteGameSession.seatPlayerIds], e.g.
     * a host-mode device receiving a proxy-joined teammate's own echo under a different seat) are
     * silently ignored, same as an owned seat's echo.
     */
    private fun applyRemoteAction(action: GameActionDto) {
        val session = remoteSession ?: return
        val actorSeatId = seatIdForPlayer(session, action.actorId) ?: return
        if (actorSeatId in ownedSeatIds) return

        when (action.actionType) {
            GameActionType.LIFE_CHANGE -> action.amount?.let { applyLifeDelta(actorSeatId, it) }
            GameActionType.POISON_COUNTER -> action.amount?.let { applyPoisonDelta(actorSeatId, it) }
            GameActionType.COMMANDER_DAMAGE -> {
                val targetSeatId = action.targetId?.let { seatIdForPlayer(session, it) }
                val amount = action.amount
                if (targetSeatId != null && amount != null) {
                    applyCommanderDamageDelta(targetSeatId = targetSeatId, attackerSeatId = actorSeatId, amount = amount)
                }
            }
            else -> Unit
        }
    }

    private fun seatIdForPlayer(session: RemoteGameSession, remotePlayerId: String): Int? =
        session.seatPlayerIds.entries.firstOrNull { it.value == remotePlayerId }?.key?.plus(1)

    private fun applyLifeDelta(seatId: Int, amount: Int) {
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == seatId) player.copy(life = player.life + amount) else player
            }
        )
    }

    private fun applyPoisonDelta(seatId: Int, amount: Int) {
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == seatId) player.copy(poison = (player.poison + amount).coerceAtLeast(0)) else player
            }
        )
    }

    private fun applyCommanderDamageDelta(targetSeatId: Int, attackerSeatId: Int, amount: Int) {
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == targetSeatId) {
                    val current = player.commanderDamage[attackerSeatId] ?: 0
                    player.copy(
                        life = player.life - amount,
                        commanderDamage = player.commanderDamage + (attackerSeatId to (current + amount).coerceAtLeast(0))
                    )
                } else {
                    player
                }
            }
        )
    }

    /** A `game_finished` broadcast only matters if this device hasn't already finished locally. */
    private fun applyRemoteGameFinished() {
        if (_state.value.isFinished) return
        socketJob?.cancel()
        val winnerId = _state.value.players.filter { it.isAlive() }.singleOrNull()?.id
        _state.value = _state.value.copy(isFinished = true, winnerId = winnerId)
        persistGameResult(winnerId)
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
