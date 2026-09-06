package com.commandercompanion.presentation.screens.game

/**
 * Where each seat physically sits at the table, in one place.
 *
 * The tracker grid, the commander-damage grids and the turn order all have to agree on it: the
 * seats are NOT laid out in list order, so anything derived from the list order alone (the turn
 * ring used to be) travels across the table instead of around it.
 */

/**
 * Splits the table the way it is seated: the first half sits "at the top" (rotated 180°), the rest
 * below. Both the seat grid and each seat's commander-damage grid use this, so a seat occupies the
 * same relative position in either — which is what makes the damage grid readable at a glance.
 */
fun <T> seatRows(seats: List<T>): Pair<List<T>, List<T>> {
    val topCount = (seats.size + 1) / 2
    return seats.take(topCount) to seats.drop(topCount)
}

/**
 * The seats in the order someone walking clockwise around the table meets them: the top row left
 * to right, then the bottom row **right to left**, which is where the ring comes back down the
 * other side. With four seats that is 1, 2, 4, 3 — seat 3 sits under seat 1, not next to seat 2.
 *
 * Used by [GameViewModel.nextTurn] and by the starter draw, so passing the turn follows the
 * quadrants the players actually see rather than the order the seats were created in.
 */
fun <T> clockwiseSeats(seats: List<T>): List<T> {
    val (top, bottom) = seatRows(seats)
    return top + bottom.reversed()
}
