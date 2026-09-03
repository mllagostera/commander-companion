package com.commandercompanion.domain.model

/**
 * A game played on this device, as the history screen needs it.
 *
 * Unlike the rest of this package these are NOT wire types: they are the domain
 * view of what Room stores (`GameEntity` + `PlayerResultEntity`, joined by
 * `GameWithPlayers`). Room's relation classes carry `@Embedded`/`@Relation` and
 * a schema the UI has no business knowing about, so this one is mapped for real
 * at the data boundary — see `GameRepositoryImpl.observeHistory`.
 *
 * The mapping also turns the persisted `status` string into [isFinished], so no
 * caller compares against the literal `"FINISHED"` any more.
 */
data class PlayedGame(
    val id: String,
    val startedAtEpochMillis: Long,
    val isFinished: Boolean,
    val playerCount: Int,
    val seats: List<PlayedSeat>
)

/** One seat's outcome within a [PlayedGame], ordered by [seatIndex] by the repository. */
data class PlayedSeat(
    val seatIndex: Int,
    val name: String,
    val colorKey: String,
    val finalLife: Int,
    val mulligans: Int,
    val won: Boolean
)
