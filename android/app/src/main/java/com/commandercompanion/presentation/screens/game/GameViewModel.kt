package com.commandercompanion.presentation.screens.game

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.commandercompanion.data.local.dao.GameDao
import com.commandercompanion.data.local.entity.GameEntity
import com.commandercompanion.data.local.entity.PlayerResultEntity
import com.commandercompanion.presentation.navigation.decodePlayerConfigs
import com.commandercompanion.presentation.theme.colorForKey
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class GameViewModel @Inject constructor(
    savedStateHandle: SavedStateHandle,
    private val gameDao: GameDao
) : ViewModel() {

    private val gameId: String = checkNotNull(savedStateHandle["gameId"])
    private val playerConfigs = decodePlayerConfigs(checkNotNull(savedStateHandle["playersEncoded"]))
    private val startingPlayerSeat: Int = savedStateHandle["startingPlayerSeat"] ?: -1

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
    }

    private fun persistNewGame() {
        val players = _state.value.players
        viewModelScope.launch {
            gameDao.insertGame(
                GameEntity(
                    id = gameId,
                    startTime = System.currentTimeMillis(),
                    status = "IN_PROGRESS",
                    playerCount = players.size
                )
            )
            gameDao.insertPlayers(
                players.map { player ->
                    PlayerResultEntity(
                        gameId = gameId,
                        seatIndex = player.id - 1,
                        name = player.name,
                        colorKey = playerConfigs[player.id - 1].colorKey,
                        finalLife = player.life,
                        mulligans = player.mulligans
                    )
                }
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
    }

    private fun persistGameResult(winnerId: Int?) {
        val players = _state.value.players
        viewModelScope.launch {
            gameDao.finishGame(gameId, status = "FINISHED", endTime = System.currentTimeMillis())
            players.forEach { player ->
                gameDao.updatePlayerResult(
                    gameId = gameId,
                    seatIndex = player.id - 1,
                    finalLife = player.life,
                    won = player.id == winnerId
                )
            }
        }
    }
}
