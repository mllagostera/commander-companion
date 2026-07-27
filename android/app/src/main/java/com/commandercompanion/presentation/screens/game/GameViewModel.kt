package com.commandercompanion.presentation.screens.game

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.core.util.ApiError
import com.commandercompanion.core.util.toUserMessage
import com.commandercompanion.data.repository.GameRepository
import com.commandercompanion.data.repository.LocalSeat
import com.commandercompanion.data.repository.LocalSeatResult
import com.commandercompanion.data.repository.RemoteGameSession
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.theme.colorForKey
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject

@HiltViewModel
class GameViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val gameRepository: GameRepository
) : ViewModel() {

    private val gameId: String = checkNotNull(savedStateHandle["gameId"])
    private val playerConfigs = decodePlayerConfigs(checkNotNull(savedStateHandle["playersEncoded"]))
    private val startingPlayerSeat: Int = savedStateHandle["startingPlayerSeat"] ?: -1

    /**
     * Partida del backend espejada por este dispositivo, o null si no se pudo / no corresponde.
     * Ver la nota de modelo en [GameRepository]: solo el asiento local tiene identidad remota.
     */
    private var remoteSession: RemoteGameSession? = null

    /**
     * Serializa las operaciones remotas: su orden importa y no puede quedar librado a cómo se
     * intercalen dos corrutinas sueltas. Un `LifeChange` que llegue después del `finish` sería
     * rechazado con 409 ("la partida no está activa"), y una acción emitida antes de que termine
     * el bootstrap no tendría todavía `GamePlayer` al que atribuirse.
     *
     * `Mutex` de kotlinx es FIFO, así que las llamadas salen en el mismo orden en que la UI las
     * generó.
     */
    private val remoteMutex = Mutex()

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
     * Crea la partida en el backend y sienta al usuario autenticado.
     *
     * Best-effort a propósito: si falla (sin red, sin sesión, sin decks) la partida sigue siendo
     * jugable en local y solo se refleja el motivo en [GameState.remoteSync].
     */
    private fun bootstrapRemoteGame() {
        launchRemote {
            gameRepository.bootstrapRemoteGame().fold(
                onSuccess = { session ->
                    remoteSession = session
                    updateRemoteSync(
                        when {
                            session == null -> RemoteSyncState(
                                status = RemoteSyncStatus.Disabled,
                                message = "No tenés decks todavía: la partida se juega solo en este dispositivo"
                            )
                            session.isActive -> RemoteSyncState(
                                status = RemoteSyncStatus.Synced,
                                gameId = session.gameId
                            )
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
        if (playerId == LOCAL_SEAT_PLAYER_ID) {
            mirrorLifeChange(amount)
        }
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
        // TODO: no se espeja como `CommanderDamage` en el backend a propósito. Esa acción atribuye
        //  el daño al `actor_id`, y desde un solo dispositivo el atacante (otro asiento local) no
        //  tiene `GamePlayer` propio: mandarla con el asiento local como actor le acreditaría
        //  `total_commander_damage_dealt` ajeno en las estadísticas. Requiere que cada jugador
        //  entre desde su propio cliente (o un mapeo asiento→usuario), fuera de alcance acá.
        checkForGameOver()
    }

    fun nextTurn() {
        _state.value = _state.value.copy(currentTurn = _state.value.currentTurn + 1)
    }

    fun previousTurn() {
        if (_state.value.currentTurn > 1) {
            _state.value = _state.value.copy(currentTurn = _state.value.currentTurn - 1)
        }
    }

    private fun checkForGameOver() {
        val alive = _state.value.players.filter { it.life > 0 }
        if (alive.size == 1 && _state.value.players.size > 1) {
            finishGame(winnerId = alive.first().id)
        }
    }

    fun finishGame(winnerId: Int? = null) {
        if (_state.value.isFinished) return
        val resolvedWinnerId = winnerId ?: _state.value.players
            .filter { it.life > 0 }
            .maxByOrNull { it.life }
            ?.takeIf { player -> _state.value.players.count { it.life == player.life } == 1 }
            ?.id

        _state.value = _state.value.copy(isFinished = true, winnerId = resolvedWinnerId)
        persistGameResult(resolvedWinnerId)
        finishRemoteGame()
    }

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
     * Espeja el cambio de vida del asiento local; no-op si la partida remota no está activa.
     *
     * La sesión se lee DENTRO del bloque serializado: si el usuario toca la vida mientras el
     * bootstrap sigue en vuelo, la acción espera a que termine en vez de descartarse.
     */
    private fun mirrorLifeChange(amount: Int) {
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            gameRepository.recordLifeChange(session, amount)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    /** Finaliza la partida remota, lo que dispara el recálculo de estadísticas server-side. */
    private fun finishRemoteGame() {
        launchRemote {
            val session = activeSession() ?: return@launchRemote
            gameRepository.finishGame(session.gameId)
                .onFailure { error -> reportRemoteFailure(error) }
        }
    }

    private fun activeSession(): RemoteGameSession? = remoteSession?.takeIf { it.isActive }

    /** Encola una operación remota respetando el orden de emisión (ver [remoteMutex]). */
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

    private companion object {
        /**
         * El primer asiento configurado representa al usuario autenticado de este dispositivo.
         * Es el único que puede tener un `GamePlayer` en el backend (ver [GameRepository]).
         */
        const val LOCAL_SEAT_PLAYER_ID = 1
    }
}
