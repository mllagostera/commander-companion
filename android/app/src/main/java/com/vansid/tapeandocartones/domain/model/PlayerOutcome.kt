package com.vansid.tapeandocartones.domain.model

/**
 * A player's alive/life snapshot at the moment a game-over decision needs to be made — decoupled
 * from `PlayerState` (which carries a Compose `Color` and belongs to the tracker's presentation
 * layer) so [com.vansid.tapeandocartones.domain.usecase.ResolveGameOutcomeUseCase] stays framework-free
 * and independently testable.
 */
data class PlayerOutcome(val id: Int, val life: Int, val isAlive: Boolean)
