package com.commandercompanion.presentation.screens.game

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.ApiFailure
import com.commandercompanion.core.util.toFailure
import com.commandercompanion.data.session.AccessTokenProvider
import com.commandercompanion.domain.model.Game
import com.commandercompanion.domain.model.GameAction
import com.commandercompanion.domain.model.GameActionType
import com.commandercompanion.domain.model.GameSocketEvent
import com.commandercompanion.domain.model.LocalSeat
import com.commandercompanion.domain.model.LocalSeatResult
import com.commandercompanion.domain.model.PlayerOutcome
import com.commandercompanion.domain.model.RemoteGameSession
import com.commandercompanion.domain.model.SeatAssignment
import com.commandercompanion.domain.model.amount
import com.commandercompanion.domain.repository.GameRepository
import com.commandercompanion.domain.repository.PlaygroupRepository
import com.commandercompanion.domain.usecase.ReplayCommanderDamageUseCase
import com.commandercompanion.domain.usecase.ResolveGameOutcomeUseCase
import com.commandercompanion.presentation.navigation.PlayerConfig
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.theme.PlayerColorPalette
import com.commandercompanion.presentation.theme.colorForKey
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

private const val HTTP_CONFLICT = 409
private const val NANOS_PER_MILLI = 1_000_000L

/** Mirrors the backend's cap on one `POST /games/{id}/actions/undo` (game-actions' maxUndoBatch). */
private const val MAX_UNDO_BATCH = 1000

/**
 * One local effect of a player action, kept so [GameViewModel.undo] can apply its inverse. Inverses
 * rather than snapshots of the whole table: in joined mode other devices' updates keep arriving in
 * between, and restoring a snapshot would wipe them out.
 */
private sealed interface UndoEffect {
    data class Life(val seatId: Int, val amount: Int) : UndoEffect
    data class Poison(val seatId: Int, val amount: Int) : UndoEffect
    data class Commander(val targetId: Int, val attackerId: Int, val amount: Int) : UndoEffect
    data class DamageTotals(val sourceId: Int?, val targetId: Int, val amount: Int) : UndoEffect

    /** [fromId] passed the turn to [toId]; [closedTurnMs] is the turn it closed (null before the first turn). */
    data class TurnPassed(val fromId: Int?, val toId: Int, val closedTurnMs: Long?, val fromLongestBefore: Long) : UndoEffect

    /** The game ended, closing [turnOwnerId]'s turn of [closedTurnMs] (see [GameViewModel.confirmFinish]). */
    data class Finished(
        val turnOwnerId: Int?,
        val closedTurnMs: Long?,
        val longestBefore: Long,
        val clockWasRunning: Boolean
    ) : UndoEffect
}

/**
 * One thing the player did on the tracker: its local effects, in order, and the ids of the backend
 * actions it produced -- filled in as each one is recorded, which always happens before an undo of
 * the step reaches the backend, because both go through the same FIFO queue (see remoteMutex).
 */
private class UndoStep(val label: UndoLabel) {
    val effects = mutableListOf<UndoEffect>()
    val remoteActionIds = mutableListOf<String>()
}

/** The last turn of a finished game, whose TurnEnd waits for [GameViewModel.confirmFinish]. */
private data class PendingFinish(val lastTurnPlayerId: Int?, val lastTurnMs: Long?)

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

    /** Undoable steps, oldest first (see [UndoStep]). */
    private val undoSteps = ArrayDeque<UndoStep>()

    /** The step being recorded while a player action runs (see [recordStep]); null otherwise. */
    private var openStep: UndoStep? = null

    /** Every backend action this device recorded and hasn't undone yet -- what [resetLives] reverts. */
    private val liveRemoteActionIds = LinkedHashSet<String>()

    /** The game-ending turn, held back from the backend until [confirmFinish] (see [finishLocally]). */
    private var pendingFinish: PendingFinish? = null

    private var undoNoticeSeq = 0L

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
                        mulligans = config.mulligans,
                        deckImageUrl = config.deckImageUrl
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
                            session == null -> RemoteSyncState(status = RemoteSyncStatus.Disabled)
                            session.isActive -> {
                                observeGameSocket(session.gameId)
                                RemoteSyncState(status = RemoteSyncStatus.Synced, gameId = session.gameId)
                            }
                            else -> RemoteSyncState(
                                status = RemoteSyncStatus.WaitingForPlayers,
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
            loadJoinedDeckArt(game)

            updateRemoteSync(
                if (remoteSession?.isActive == true) {
                    observeGameSocket(game.id)
                    RemoteSyncState(status = RemoteSyncStatus.Synced, gameId = game.id)
                } else {
                    RemoteSyncState(
                        status = RemoteSyncStatus.WaitingForPlayers,
                        gameId = game.id
                    )
                }
            )
        }
    }

    /**
     * Paints each seat with its deck's art, AFTER the table is already on screen.
     *
     * Joined mode has no `PlayerConfig` to carry the art the way host mode does (the route only
     * knows the game id), and `GamePlayer` exposes `deck_id` but no image, so it is resolved the
     * same way the usernames above are: through the game's playgroup, one
     * `GET /playgroups/{id}/members/{userId}/decks` per distinct seated user. That fan-out runs in
     * its own coroutine on purpose — the seats must not wait on it to appear, and a failure just
     * leaves them on their flat colour, like a guest's.
     *
     * If it ever becomes visible, the fix is a `deck_image_url` on `GamePlayerResponse` (additive,
     * non-breaking), not a blocking fetch here.
     */
    private fun loadJoinedDeckArt(game: Game) {
        val gamePlaygroupId = game.playgroupId ?: return
        viewModelScope.launch {
            val artByDeckId = game.players.map { it.userId }.distinct()
                .flatMap { userId ->
                    playgroupRepository.getMemberDecks(gamePlaygroupId, userId).getOrDefault(emptyList())
                }
                .mapNotNull { deck -> deck.imageUrl?.let { url -> deck.id to url } }
                .toMap()
            if (artByDeckId.isEmpty()) return@launch

            val deckIdBySeat = game.players.mapIndexed { index, player -> (index + 1) to player.deckId }.toMap()
            _state.value = _state.value.copy(
                players = _state.value.players.map { player ->
                    player.copy(deckImageUrl = artByDeckId[deckIdBySeat[player.id]] ?: player.deckImageUrl)
                }
            )
        }
    }

    /** Replays the `CommanderDamage` actions of [game]'s timeline into a per-seat, per-attacker-seat map. */
    private suspend fun replayCommanderDamage(game: Game): Map<Int, Map<Int, Int>> {
        val seatByPlayerId = game.players.mapIndexed { index, player -> player.id to (index + 1) }.toMap()
        val actions = gameRepository.timeline(game.id).getOrElse { return emptyMap() }
        return replayCommanderDamageUseCase(actions, seatByPlayerId)
    }

    fun adjustLife(playerId: Int, amount: Int) = recordStep(UndoLabel.LifeChange(nameOf(playerId), amount)) {
        changeLife(playerId, amount, damageSourceId = null)
    }

    /**
     * Applies a life change to [playerId]. When it's damage dealt by [damageSourceId], the summary
     * totals are updated in the SAME state write, so a lethal hit is already counted by the time
     * [checkForGameOver] flips the game to finished.
     */
    private fun changeLife(playerId: Int, amount: Int, damageSourceId: Int?) {
        if (_state.value.isFinished) return
        val wasEliminated = isSeatEliminated(playerId)
        val players = _state.value.players.map { player ->
            if (player.id == playerId) {
                player.copy(life = player.life + amount)
            } else {
                player
            }
        }
        _state.value = _state.value.copy(
            players = if (damageSourceId != null) players.withDamageTotals(damageSourceId, playerId, -amount) else players
        )
        addUndoEffect(UndoEffect.Life(playerId, amount))
        if (damageSourceId != null) addUndoEffect(UndoEffect.DamageTotals(damageSourceId, playerId, -amount))
        if (damageSourceId != null) {
            mirrorCombatDamage(attackerId = damageSourceId, targetPlayerId = playerId, amount = -amount)
            mirrorEliminationIfLethal(sourceId = damageSourceId, targetId = playerId, wasEliminated = wasEliminated)
        } else {
            mirrorLifeChange(playerId, amount)
        }
        checkForGameOver()
    }

    private fun isSeatEliminated(seatId: Int): Boolean =
        _state.value.players.firstOrNull { it.id == seatId }?.isEliminated() ?: false

    /**
     * Adds [amount] damage from [sourceId] (null when unknown, e.g. a remote life change) to
     * [targetId] into both seats' summary totals. A negative [amount] is a correction.
     */
    private fun List<PlayerState>.withDamageTotals(sourceId: Int?, targetId: Int, amount: Int): List<PlayerState> =
        map { player ->
            when (player.id) {
                targetId -> player.copy(damageTaken = (player.damageTaken + amount).coerceAtLeast(0))
                sourceId -> player.copy(damageDealt = (player.damageDealt + amount).coerceAtLeast(0))
                else -> player
            }
        }

    /** The commander grid's "+": [attackerId]'s commander deals [amount] to [targetPlayerId]. */
    fun adjustCommanderDamage(targetPlayerId: Int, attackerId: Int, amount: Int) {
        if (amount <= 0 || targetPlayerId == attackerId) return
        recordStep(UndoLabel.Damage(nameOf(attackerId), nameOf(targetPlayerId), amount, commander = true)) {
            applyCommanderDamage(targetPlayerId, attackerId, amount)
        }
    }

    private fun applyCommanderDamage(targetPlayerId: Int, attackerId: Int, amount: Int) {
        if (_state.value.isFinished) return
        val wasEliminated = isSeatEliminated(targetPlayerId)
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
            }.withDamageTotals(sourceId = attackerId, targetId = targetPlayerId, amount = amount)
        )
        addUndoEffect(UndoEffect.Commander(targetPlayerId, attackerId, amount))
        addUndoEffect(UndoEffect.DamageTotals(attackerId, targetPlayerId, amount))
        mirrorCommanderDamage(attackerId, targetPlayerId, amount)
        mirrorEliminationIfLethal(sourceId = attackerId, targetId = targetPlayerId, wasEliminated = wasEliminated)
        checkForGameOver()
    }

    /**
     * [sourceId] deals [amount] damage to [targetId] -- what the tracker's drag from one seat to
     * another records. Commander damage also counts towards the 21-point rule against that attacker
     * (and costs life, see [adjustCommanderDamage]); plain damage only costs life. A seat can't
     * damage itself, and a non-positive amount is nothing to record.
     */
    fun dealDamage(sourceId: Int, targetId: Int, amount: Int, commander: Boolean) {
        if (amount <= 0 || sourceId == targetId) return
        recordStep(UndoLabel.Damage(nameOf(sourceId), nameOf(targetId), amount, commander)) {
            if (commander) {
                applyCommanderDamage(targetPlayerId = targetId, attackerId = sourceId, amount = amount)
            } else {
                changeLife(playerId = targetId, amount = -amount, damageSourceId = sourceId)
            }
        }
    }

    /** [playerId] gains [amount] life -- a drag that starts and ends on the same seat. */
    fun gainLife(playerId: Int, amount: Int) {
        if (amount <= 0) return
        adjustLife(playerId = playerId, amount = amount)
    }

    fun adjustPoison(playerId: Int, amount: Int) = recordStep(UndoLabel.Poison(nameOf(playerId), amount)) {
        if (_state.value.isFinished) return@recordStep
        val before = _state.value.players.firstOrNull { it.id == playerId }?.poison ?: return@recordStep
        // Counters never go below 0: a "-" at 0 changes nothing, so there's nothing to record or undo.
        val delta = (before + amount).coerceAtLeast(0) - before
        if (delta == 0) return@recordStep
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == playerId) player.copy(poison = player.poison + delta) else player
            }
        )
        addUndoEffect(UndoEffect.Poison(playerId, delta))
        mirrorPoisonChange(playerId, delta)
        checkForGameOver()
    }

    /**
     * Advances the turn counter and hands the ring highlight to the next seat that is still alive,
     * going CLOCKWISE around the table ([clockwiseSeats]) rather than down the seat list: with four
     * seats the list order jumps from the top-right quadrant to the bottom-left one, which is not
     * where the turn goes at a real table.
     */
    fun nextTurn() = recordStep(UndoLabel.PassTurn(_state.value.currentTurnPlayerId?.let(::nameOf).orEmpty())) {
        passTurn()
    }

    private fun passTurn() {
        if (_state.value.isFinished) return
        val ring = clockwiseSeats(_state.value.players)
        if (ring.isEmpty()) return
        // -1 (no current seat yet) lands on the first seat of the ring, same as before.
        val currentIndex = ring.indexOfFirst { it.id == _state.value.currentTurnPlayerId }
        // Walking the ring at most once bounds the search: on a table where every seat is
        // eliminated there is nobody to skip to, and the turn just moves on to the next seat
        // instead of looping forever. The game normally finishes before that (one seat left
        // standing ends it), so this is the corner case, not the common path.
        val nextPlayerId = (1..ring.size).asSequence()
            .map { step -> ring[(currentIndex + step).mod(ring.size)] }
            .firstOrNull { it.isAlive() }
            ?.id
            ?: ring[(currentIndex + 1).mod(ring.size)].id
        // The outgoing seat banks its turn; the incoming one's clock starts now (if it was running).
        val wasRunning = _state.value.turnClockRunningSince != null
        val outgoingPlayerId = _state.value.currentTurnPlayerId
        val longestBefore = _state.value.players.firstOrNull { it.id == outgoingPlayerId }?.longestTurnMs ?: 0
        val (closed, turnDurationMs) = closeTurn(settleTurnClock(_state.value))
        _state.value = closed.copy(
            currentTurn = _state.value.currentTurn + 1,
            currentTurnPlayerId = nextPlayerId,
            turnClockRunningSince = if (wasRunning) nowMs() else null
        )
        addUndoEffect(UndoEffect.TurnPassed(outgoingPlayerId, nextPlayerId, turnDurationMs, longestBefore))
        if (outgoingPlayerId != null && turnDurationMs != null) mirrorTurnEnd(outgoingPlayerId, turnDurationMs)
        mirrorTurnStart(nextPlayerId)
    }

    /**
     * Monotonic milliseconds for the turn clocks. Not wall-clock time, which jumps when the device
     * syncs its clock; a test swaps it for a fake clock.
     */
    internal var nowMs: () -> Long = { System.nanoTime() / NANOS_PER_MILLI }

    /** Starts the first turn's clock, once the starter draw has landed. Later calls are no-ops. */
    fun startTurnClock() {
        val current = _state.value
        if (current.turnClockStarted || current.isFinished) return
        _state.value = current.copy(turnClockStarted = true, turnClockRunningSince = nowMs())
        current.currentTurnPlayerId?.let { mirrorTurnStart(it) }
    }

    /** Stops the running clock, banking the time so far (the game is paused). */
    fun pauseTurnClock() {
        _state.value = settleTurnClock(_state.value)
    }

    /** Restarts the clock stopped by [pauseTurnClock]; nothing to do before the first turn. */
    fun resumeTurnClock() {
        val current = _state.value
        if (!current.turnClockStarted || current.isFinished || current.turnClockRunningSince != null) return
        _state.value = current.copy(turnClockRunningSince = nowMs())
    }

    /** [playerId]'s time on its own turns, including the turn in progress if it is theirs. */
    fun turnTimeOf(playerId: Int): Long {
        val current = _state.value
        val banked = current.players.firstOrNull { it.id == playerId }?.turnTimeMs ?: 0
        val runningSince = current.turnClockRunningSince
        return if (runningSince != null && playerId == current.currentTurnPlayerId) {
            banked + (nowMs() - runningSince).coerceAtLeast(0)
        } else {
            banked
        }
    }

    /** Adds the running stretch to the turn owner's total (and to the turn in progress) and stops the clock. */
    private fun settleTurnClock(state: GameState): GameState {
        val runningSince = state.turnClockRunningSince ?: return state
        val elapsed = (nowMs() - runningSince).coerceAtLeast(0)
        return state.copy(
            players = state.players.map { player ->
                if (player.id == state.currentTurnPlayerId) player.copy(turnTimeMs = player.turnTimeMs + elapsed) else player
            },
            turnClockRunningSince = null,
            currentTurnMs = state.currentTurnMs + elapsed
        )
    }

    /**
     * Ends the turn in progress of an already-settled [state] (see [settleTurnClock]): counts it for
     * its owner and keeps it if it's their longest. Returns the turn's length too, or null when no
     * turn was running yet (before the first turn's clock started) -- there's nothing to count then.
     */
    private fun closeTurn(state: GameState): Pair<GameState, Long?> {
        val ownerId = state.currentTurnPlayerId
        if (!state.turnClockStarted || ownerId == null) return state to null
        val duration = state.currentTurnMs
        val closed = state.copy(
            players = state.players.map { player ->
                if (player.id == ownerId) {
                    player.copy(turnsTaken = player.turnsTaken + 1, longestTurnMs = maxOf(player.longestTurnMs, duration))
                } else {
                    player
                }
            },
            currentTurnMs = 0
        )
        return closed to duration
    }

    /**
     * Resets life/poison/commander damage for every current seat without ending the game — same
     * seats and turn order, fresh counters. On the backend it undoes every action this device
     * recorded (commander damage can't be taken back any other way: it's never negative), so the
     * restarted game starts from a clean log, and the first turn starts again. Not undoable itself.
     */
    fun resetLives() {
        if (_state.value.isFinished) return
        val previous = _state.value.players
        // A fresh game starts the clocks from zero too, running again if they were.
        val wasRunning = _state.value.turnClockRunningSince != null
        _state.value = _state.value.copy(
            players = previous.map {
                it.copy(
                    life = STARTING_LIFE,
                    poison = 0,
                    commanderDamage = emptyMap(),
                    damageDealt = 0,
                    damageTaken = 0,
                    turnTimeMs = 0,
                    turnsTaken = 0,
                    longestTurnMs = 0
                )
            },
            currentTurn = 1,
            currentTurnPlayerId = _state.value.startingPlayerId,
            turnClockRunningSince = if (wasRunning) nowMs() else null,
            currentTurnMs = 0
        )
        undoSteps.clear()
        publishUndoState()
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            // Read here, not when resetLives ran: actions still queued ahead of this one add theirs.
            val ids = liveRemoteActionIds.toList().asReversed()
            for (chunk in ids.chunked(MAX_UNDO_BATCH)) {
                gameRepository.undoActions(session, chunk)
                    .onSuccess { liveRemoteActionIds.removeAll(chunk.toSet()) }
                    .onFailure { error -> reportRemoteFailure(error) }
            }
        }
        _state.value.currentTurnPlayerId?.takeIf { _state.value.turnClockStarted }?.let { mirrorTurnStart(it) }
    }

    private fun checkForGameOver() {
        val winnerId = resolveGameOutcomeUseCase.automaticWinner(_state.value.players.toOutcomes()) ?: return
        finishLocally(winnerId = winnerId)
    }

    /** "Finish game" from the pause menu. */
    fun finishGame(winnerId: Int? = null) = recordStep(UndoLabel.FinishGame) { finishLocally(winnerId) }

    /**
     * Ends the game on this device only: the summary shows, but the result isn't saved and the
     * backend isn't told until [confirmFinish] -- so a mistaken lethal hit (or a mistaken "finish")
     * can still be undone from the summary. The finish belongs to the step that caused it, and
     * undoing that step reopens the game (see [UndoEffect.Finished]).
     */
    private fun finishLocally(winnerId: Int?) {
        if (_state.value.isFinished) return
        val resolvedWinnerId = resolveGameOutcomeUseCase.resolveWinner(_state.value.players.toOutcomes(), winnerId)

        // The turn the game ended on counts as a finished turn too.
        val wasRunning = _state.value.turnClockRunningSince != null
        val lastTurnPlayerId = _state.value.currentTurnPlayerId
        val longestBefore = _state.value.players.firstOrNull { it.id == lastTurnPlayerId }?.longestTurnMs ?: 0
        val (closed, lastTurnMs) = closeTurn(settleTurnClock(_state.value))
        _state.value = closed.copy(isFinished = true, winnerId = resolvedWinnerId)
        addUndoEffect(UndoEffect.Finished(lastTurnPlayerId, lastTurnMs, longestBefore, wasRunning))
        pendingFinish = PendingFinish(lastTurnPlayerId, lastTurnMs)
    }

    /**
     * Makes the finish shown on the summary final: saves the result and finishes the remote game
     * (which recalculates the statistics). From here on nothing can be undone. The last turn's
     * TurnEnd is queued before the remote `finish`, which would reject any later action with 409.
     * A no-op when there's no finish pending (already confirmed, or finished by another device).
     */
    fun confirmFinish() {
        val pending = pendingFinish ?: return
        pendingFinish = null
        undoSteps.clear()
        publishUndoState()
        if (pending.lastTurnPlayerId != null && pending.lastTurnMs != null) {
            mirrorTurnEnd(pending.lastTurnPlayerId, pending.lastTurnMs)
        }
        persistGameResult(_state.value.winnerId)
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
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = session.seatPlayerIds[playerId - 1] ?: return@launchRemote
            gameRepository.recordLifeChange(session, remotePlayerId, amount)
                .track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /**
     * Mirrors commander damage from [attackerId] against [targetPlayerId]; no-op if the defender has
     * no real `GamePlayer`, and only a life change when the attacker isn't this device's to act for
     * (attributing damage to someone else's `GamePlayer` would corrupt another user's statistics).
     */
    private fun mirrorCommanderDamage(attackerId: Int, targetPlayerId: Int, amount: Int) {
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val defenderPlayerId = session.seatPlayerIds[targetPlayerId - 1] ?: return@launchRemote
            // Joined mode: another device's commander can't be this device's actor. The defender's
            // life is kept in sync with a LifeChange, though the 21-point count stays local.
            val attackerPlayerId = ownedRemotePlayerId(session, attackerId)
            val result = if (attackerPlayerId != null) {
                gameRepository.recordCommanderDamage(session, attackerPlayerId, defenderPlayerId, amount)
            } else {
                gameRepository.recordLifeChange(session, defenderPlayerId, -amount)
            }
            result.track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Mirrors a poison counter change of [playerId]; no-op if it has no real `GamePlayer`. */
    private fun mirrorPoisonChange(playerId: Int, amount: Int) {
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = session.seatPlayerIds[playerId - 1] ?: return@launchRemote
            gameRepository.recordPoisonChange(session, remotePlayerId, amount)
                .track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /**
     * The `GamePlayer` of [seatId] if THIS device may act on its behalf -- the backend only accepts
     * an action whose actor is the caller's own (or proxy-joined) seat. In joined mode the session
     * knows every seat, but only [ownedSeatIds] may be an actor.
     */
    private fun ownedRemotePlayerId(session: RemoteGameSession, seatId: Int): String? =
        session.seatPlayerIds[seatId - 1]?.takeIf { seatId in ownedSeatIds }

    /**
     * Mirrors plain damage from [attackerId] to [targetPlayerId] as `CombatDamage`, which the
     * backend credits to the attacker as damage dealt. When the attacker can't be the actor (a guest
     * seat, or another device's seat in joined mode) it falls back to a `LifeChange` on the target,
     * so the target's life still stays in sync even though nobody gets credit for the damage.
     */
    private fun mirrorCombatDamage(attackerId: Int, targetPlayerId: Int, amount: Int) {
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val defenderPlayerId = session.seatPlayerIds[targetPlayerId - 1] ?: return@launchRemote
            val attackerPlayerId = ownedRemotePlayerId(session, attackerId)
            val result = if (attackerPlayerId != null) {
                gameRepository.recordCombatDamage(session, attackerPlayerId, defenderPlayerId, amount)
            } else {
                gameRepository.recordLifeChange(session, defenderPlayerId, -amount)
            }
            result.track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /**
     * Credits [sourceId] with eliminating [targetId] on the backend when the damage just applied
     * took [targetId] out. The backend eliminates the player on its own when life hits 0, but that
     * auto-elimination has no actor, so without this explicit `Elimination` nobody gets the kill.
     * Queued before [finishRemoteGame]'s `finish`, which would reject any later action with 409.
     */
    private fun mirrorEliminationIfLethal(sourceId: Int, targetId: Int, wasEliminated: Boolean) {
        if (wasEliminated || !isSeatEliminated(targetId)) return
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val actorPlayerId = ownedRemotePlayerId(session, sourceId) ?: return@launchRemote
            val targetRemotePlayerId = session.seatPlayerIds[targetId - 1] ?: return@launchRemote
            gameRepository.recordElimination(session, actorPlayerId, targetRemotePlayerId)
                .track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Records the end of [playerId]'s turn and its length, for the backend's turn-time statistics. */
    private fun mirrorTurnEnd(playerId: Int, durationMs: Long) {
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = ownedRemotePlayerId(session, playerId) ?: return@launchRemote
            gameRepository.recordTurnEnd(session, remotePlayerId, durationMs)
                .track(step)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Records the start of [playerId]'s turn, which is what the backend counts as the game's turns. */
    private fun mirrorTurnStart(playerId: Int) {
        val step = openStep
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            val remotePlayerId = ownedRemotePlayerId(session, playerId) ?: return@launchRemote
            gameRepository.recordTurnStart(session, remotePlayerId)
                .track(step)
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
            is GameSocketEvent.ActionUndone -> applyRemoteUndo(event.action)
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
    private fun applyRemoteAction(action: GameAction) {
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
            GameActionType.COMBAT_DAMAGE -> {
                val targetSeatId = action.targetId?.let { seatIdForPlayer(session, it) }
                val amount = action.amount
                if (targetSeatId != null && amount != null) {
                    applyCombatDamageDelta(targetSeatId = targetSeatId, attackerSeatId = actorSeatId, amount = amount)
                }
            }
            else -> Unit
        }
    }

    private fun seatIdForPlayer(session: RemoteGameSession, remotePlayerId: String): Int? =
        session.seatPlayerIds.entries.firstOrNull { it.value == remotePlayerId }?.key?.plus(1)

    /** A remote `LifeChange` carries no source, so a loss only counts towards [PlayerState.damageTaken]. */
    private fun applyLifeDelta(seatId: Int, amount: Int) {
        val players = _state.value.players.map { player ->
            if (player.id == seatId) player.copy(life = player.life + amount) else player
        }
        _state.value = _state.value.copy(
            players = if (amount < 0) players.withDamageTotals(sourceId = null, targetId = seatId, amount = -amount) else players
        )
    }

    private fun applyPoisonDelta(seatId: Int, amount: Int) {
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == seatId) player.copy(poison = (player.poison + amount).coerceAtLeast(0)) else player
            }
        )
    }

    private fun applyCombatDamageDelta(targetSeatId: Int, attackerSeatId: Int, amount: Int) {
        _state.value = _state.value.copy(
            players = _state.value.players.map { player ->
                if (player.id == targetSeatId) player.copy(life = player.life - amount) else player
            }.withDamageTotals(sourceId = attackerSeatId, targetId = targetSeatId, amount = amount)
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
            }.withDamageTotals(sourceId = attackerSeatId, targetId = targetSeatId, amount = amount)
        )
    }

    /** A `game_finished` broadcast only matters if this device hasn't already finished locally. */
    private fun applyRemoteGameFinished() {
        if (_state.value.isFinished) return
        socketJob?.cancel()
        val winnerId = _state.value.players.filter { it.isAlive() }.singleOrNull()?.id
        // Already finished remotely: the last turn is only counted locally, there's no TurnEnd to
        // send, and nothing can be undone any more.
        _state.value = closeTurn(settleTurnClock(_state.value)).first.copy(isFinished = true, winnerId = winnerId)
        undoSteps.clear()
        pendingFinish = null
        publishUndoState()
        persistGameResult(winnerId)
    }

    // ------------------------------------------------------------------ undo

    /**
     * Runs a player action as one undoable step: every effect it causes -- including a lethal hit's
     * elimination and the end of the game -- is recorded against [label] and undone together.
     * Reentrant: an action that runs inside another (the finish a lethal hit triggers) joins the
     * outer step. A step that ended up changing nothing isn't kept.
     */
    private inline fun recordStep(label: UndoLabel, action: () -> Unit) {
        if (openStep != null) {
            action()
            return
        }
        val step = UndoStep(label)
        openStep = step
        try {
            action()
        } finally {
            openStep = null
        }
        if (step.effects.isNotEmpty()) {
            undoSteps.addLast(step)
            publishUndoState()
        }
    }

    private fun addUndoEffect(effect: UndoEffect) {
        openStep?.effects?.add(effect)
    }

    /** Remembers a recorded backend action, for undo ([step]) and for [resetLives]. */
    private fun Result<GameAction>.track(step: UndoStep?): Result<GameAction> = onSuccess { action ->
        liveRemoteActionIds += action.id
        step?.remoteActionIds?.add(action.id)
    }

    private fun publishUndoState() {
        _state.value = _state.value.copy(canUndo = undoSteps.isNotEmpty())
    }

    private fun nameOf(seatId: Int): String = _state.value.players.firstOrNull { it.id == seatId }?.name.orEmpty()

    /**
     * Undoes the most recent step this device recorded -- on the summary too, where the step that
     * ended the game reopens it. Only this device's own steps: in joined mode, the other seats undo
     * theirs, and the change reaches this device through [applyRemoteUndo].
     */
    fun undo() {
        val step = undoSteps.removeLastOrNull() ?: return
        revertStep(step)
    }

    /**
     * The commander grid's "-": undoes the latest step in which [attackerId]'s commander hit
     * [targetPlayerId], even if something else happened after it (its effects are independent of
     * later ones). Commander damage is never negative on the backend, so this is how it goes down.
     */
    fun undoCommanderDamage(targetPlayerId: Int, attackerId: Int) {
        if (_state.value.isFinished) return
        val step = undoSteps.lastOrNull { candidate ->
            candidate.effects.none { it is UndoEffect.TurnPassed || it is UndoEffect.Finished } &&
                candidate.effects.any { it is UndoEffect.Commander && it.targetId == targetPlayerId && it.attackerId == attackerId }
        } ?: return
        undoSteps.remove(step)
        revertStep(step)
    }

    private fun revertStep(step: UndoStep) {
        var state = _state.value
        for (effect in step.effects.asReversed()) state = revertEffect(state, effect)
        undoNoticeSeq += 1
        _state.value = state.copy(canUndo = undoSteps.isNotEmpty(), lastUndone = UndoNotice(undoNoticeSeq, step.label))
        if (step.effects.any { it is UndoEffect.Finished }) pendingFinish = null

        launchRemote {
            // Read here, not when undo ran: the step's own actions were queued ahead of this block.
            val ids = step.remoteActionIds.asReversed().toList()
            if (ids.isEmpty()) return@launchRemote
            val session = activeSession() ?: return@launchRemote
            gameRepository.undoActions(session, ids)
                .onSuccess { liveRemoteActionIds.removeAll(ids.toSet()) }
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    private fun revertEffect(state: GameState, effect: UndoEffect): GameState = when (effect) {
        is UndoEffect.Life -> state.mapPlayer(effect.seatId) { it.copy(life = it.life - effect.amount) }
        is UndoEffect.Poison -> state.mapPlayer(effect.seatId) { it.copy(poison = (it.poison - effect.amount).coerceAtLeast(0)) }
        is UndoEffect.Commander -> state.mapPlayer(effect.targetId) { player ->
            val current = player.commanderDamage[effect.attackerId] ?: 0
            player.copy(
                life = player.life + effect.amount,
                commanderDamage = player.commanderDamage + (effect.attackerId to (current - effect.amount).coerceAtLeast(0))
            )
        }
        is UndoEffect.DamageTotals ->
            state.copy(players = state.players.withDamageTotals(effect.sourceId, effect.targetId, -effect.amount))
        is UndoEffect.TurnPassed -> revertTurnPass(state, effect)
        is UndoEffect.Finished -> revertFinish(state, effect)
    }

    private inline fun GameState.mapPlayer(seatId: Int, transform: (PlayerState) -> PlayerState): GameState =
        copy(players = players.map { if (it.id == seatId) transform(it) else it })

    /**
     * Hands the turn back to whoever passed it. The time the next seat has run on its turn since
     * then goes back to the one who passed, whose closed turn reopens with it (it never ended).
     */
    private fun revertTurnPass(state: GameState, effect: UndoEffect.TurnPassed): GameState {
        val wasRunning = state.turnClockRunningSince != null
        val settled = settleTurnClock(state)
        val sinceThePass = settled.currentTurnMs
        val players = settled.players.map { player ->
            var p = player
            if (p.id == effect.toId) p = p.copy(turnTimeMs = p.turnTimeMs - sinceThePass)
            if (p.id == effect.fromId) {
                p = p.copy(turnTimeMs = p.turnTimeMs + sinceThePass)
                if (effect.closedTurnMs != null) {
                    p = p.copy(turnsTaken = p.turnsTaken - 1, longestTurnMs = effect.fromLongestBefore)
                }
            }
            p
        }
        return settled.copy(
            players = players,
            currentTurn = settled.currentTurn - 1,
            currentTurnPlayerId = effect.fromId,
            currentTurnMs = (effect.closedTurnMs ?: 0) + sinceThePass,
            turnClockRunningSince = if (wasRunning) nowMs() else null
        )
    }

    /** Reopens a game finished locally: its last turn goes on, and its clock runs again if it was. */
    private fun revertFinish(state: GameState, effect: UndoEffect.Finished): GameState {
        val reopened = if (effect.closedTurnMs != null && effect.turnOwnerId != null) {
            state.mapPlayer(effect.turnOwnerId) {
                it.copy(turnsTaken = it.turnsTaken - 1, longestTurnMs = effect.longestBefore)
            }
        } else {
            state
        }
        return reopened.copy(
            isFinished = false,
            winnerId = null,
            currentTurnMs = effect.closedTurnMs ?: reopened.currentTurnMs,
            turnClockRunningSince = if (effect.clockWasRunning) nowMs() else null
        )
    }

    /**
     * Reverts an `action_undone` broadcast for a seat this device does NOT own -- the counterpart of
     * [applyRemoteAction]. This device's own undos were already reverted locally by [undo].
     */
    private fun applyRemoteUndo(action: GameAction) {
        val session = remoteSession ?: return
        val actorSeatId = seatIdForPlayer(session, action.actorId) ?: return
        if (actorSeatId in ownedSeatIds) return
        val amount = action.amount ?: return
        val targetSeatId = action.targetId?.let { seatIdForPlayer(session, it) }

        _state.value = when (action.actionType) {
            GameActionType.LIFE_CHANGE -> _state.value.mapPlayer(actorSeatId) { it.copy(life = it.life - amount) }
                .let { reverted ->
                    // A remote loss counted as damage taken (see applyLifeDelta); its undo takes it back.
                    if (amount < 0) reverted.copy(players = reverted.players.withDamageTotals(null, actorSeatId, amount)) else reverted
                }
            GameActionType.POISON_COUNTER -> revertEffect(_state.value, UndoEffect.Poison(actorSeatId, amount))
            GameActionType.COMBAT_DAMAGE -> targetSeatId?.let { target ->
                revertEffect(
                    revertEffect(_state.value, UndoEffect.DamageTotals(actorSeatId, target, amount)),
                    UndoEffect.Life(target, -amount)
                )
            } ?: return
            GameActionType.COMMANDER_DAMAGE -> targetSeatId?.let { target ->
                revertEffect(
                    revertEffect(_state.value, UndoEffect.DamageTotals(actorSeatId, target, amount)),
                    UndoEffect.Commander(target, actorSeatId, amount)
                )
            } ?: return
            else -> return
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
                failure = (error as? ApiError)?.toFailure() ?: ApiFailure.Unexpected,
                gameId = remoteSession?.gameId
            )
        )
    }

    private fun updateRemoteSync(remoteSync: RemoteSyncState) {
        _state.value = _state.value.copy(remoteSync = remoteSync)
    }
}
