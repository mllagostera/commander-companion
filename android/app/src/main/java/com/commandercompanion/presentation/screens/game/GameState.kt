package com.commandercompanion.presentation.screens.game

import androidx.compose.ui.graphics.Color

/** Reglas de eliminación de Commander, compartidas por [GameViewModel] y la UI del tracker. */
const val COMMANDER_DAMAGE_LETHAL = 21
const val POISON_LETHAL = 10

data class PlayerState(
    val id: Int,
    val name: String,
    val life: Int = 40,
    val color: Color,
    val mulligans: Int = 0,
    val poison: Int = 0,
    val commanderDamage: Map<Int, Int> = emptyMap() // Key: Opponent ID, Value: Damage received
)

/** Vivo = vida positiva, sin 21+ de daño de un mismo comandante y menos de 10 contadores de veneno. */
fun PlayerState.isEliminated(): Boolean =
    life <= 0 || poison >= POISON_LETHAL || commanderDamage.values.any { it >= COMMANDER_DAMAGE_LETHAL }

/** Estado del espejo de la partida contra el backend (ver `GameRepository`). */
enum class RemoteSyncStatus {
    /** Todavía creando la partida remota / uniéndose a ella. */
    Connecting,

    /** No se intenta sincronizar (p. ej. el usuario no tiene decks). Partida 100% local. */
    Disabled,

    /** Partida creada y unida, pero en `pending`: falta que se una un segundo jugador. */
    WaitingForPlayers,

    /** Partida `active` en el backend: los cambios del asiento local se registran allá. */
    Synced,

    /** Falló la sincronización. La partida sigue funcionando en local. */
    Failed
}

data class RemoteSyncState(
    val status: RemoteSyncStatus = RemoteSyncStatus.Connecting,
    val message: String? = null,
    val gameId: String? = null
)

data class GameState(
    val players: List<PlayerState> = emptyList(),
    val currentTurn: Int = 1,
    val startingPlayerId: Int? = null,
    val isFinished: Boolean = false,
    val winnerId: Int? = null,
    val remoteSync: RemoteSyncState = RemoteSyncState()
)
