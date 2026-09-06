package com.commandercompanion.presentation.screens.game

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * The seating ring the tracker draws and the turn order walk. Pure functions on purpose: the bug
 * they fix (the turn crossing the table diagonally) is a layout question, and answering it needed
 * no Compose.
 */
class SeatLayoutTest {

    @Test
    fun `an even table splits into two equal rows`() {
        assertEquals(listOf(1, 2) to listOf(3, 4), seatRows(listOf(1, 2, 3, 4)))
    }

    @Test
    fun `an odd table seats the extra player on the top row`() {
        assertEquals(listOf(1, 2, 3) to listOf(4, 5), seatRows(listOf(1, 2, 3, 4, 5)))
    }

    @Test
    fun `four seats go around the table, not down the list`() {
        // Seat 3 sits under seat 1, so it comes last: 1 and 2 on top, 4 and 3 coming back.
        assertEquals(listOf(1, 2, 4, 3), clockwiseSeats(listOf(1, 2, 3, 4)))
    }

    @Test
    fun `six seats come back along the bottom row`() {
        assertEquals(listOf(1, 2, 3, 6, 5, 4), clockwiseSeats(listOf(1, 2, 3, 4, 5, 6)))
    }

    @Test
    fun `two and three seats keep the order they already had`() {
        assertEquals(listOf(1, 2), clockwiseSeats(listOf(1, 2)))
        assertEquals(listOf(1, 2, 3), clockwiseSeats(listOf(1, 2, 3)))
    }

    @Test
    fun `five seats close the ring through the bottom row`() {
        assertEquals(listOf(1, 2, 3, 5, 4), clockwiseSeats(listOf(1, 2, 3, 4, 5)))
    }

    @Test
    fun `an empty table has no ring`() {
        assertEquals(emptyList<Int>(), clockwiseSeats(emptyList<Int>()))
    }
}
