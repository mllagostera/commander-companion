package com.commandercompanion.domain.usecase

import com.commandercompanion.domain.model.PlayerOutcome
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ResolveGameOutcomeUseCaseTest {

    private val useCase = ResolveGameOutcomeUseCase()

    // ------------------------------------------------------- automaticWinner

    @Test
    fun `un solo jugador vivo entre varios es el ganador automatico`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 20, isAlive = true),
            PlayerOutcome(id = 2, life = 0, isAlive = false)
        )

        assertEquals(1, useCase.automaticWinner(players))
    }

    @Test
    fun `mas de un jugador vivo no termina la partida`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 20, isAlive = true),
            PlayerOutcome(id = 2, life = 15, isAlive = true)
        )

        assertNull(useCase.automaticWinner(players))
    }

    @Test
    fun `una unica configuracion de un jugador nunca se autofinaliza`() {
        val players = listOf(PlayerOutcome(id = 1, life = 40, isAlive = true))

        assertNull(useCase.automaticWinner(players))
    }

    @Test
    fun `sin nadie vivo no hay ganador automatico`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 0, isAlive = false),
            PlayerOutcome(id = 2, life = 0, isAlive = false)
        )

        assertNull(useCase.automaticWinner(players))
    }

    // --------------------------------------------------------- resolveWinner

    @Test
    fun `un ganador explicito se respeta sin mirar el resto`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 1, isAlive = true),
            PlayerOutcome(id = 2, life = 40, isAlive = true)
        )

        assertEquals(2, useCase.resolveWinner(players, explicitWinnerId = 2))
    }

    @Test
    fun `sin ganador explicito gana el vivo con mas vida`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 12, isAlive = true),
            PlayerOutcome(id = 2, life = 30, isAlive = true),
            PlayerOutcome(id = 3, life = 0, isAlive = false)
        )

        assertEquals(2, useCase.resolveWinner(players, explicitWinnerId = null))
    }

    @Test
    fun `un empate de vida con el maximo no corona a nadie`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 30, isAlive = true),
            PlayerOutcome(id = 2, life = 30, isAlive = true)
        )

        assertNull(useCase.resolveWinner(players, explicitWinnerId = null))
    }

    /**
     * La comparación de empate mira la vida de TODOS los jugadores, no solo los vivos: si un
     * jugador eliminado (p. ej. por veneno o daño de comandante) quedó congelado con la misma
     * vida que el único sobreviviente, tampoco hay ganador — coincide con el comportamiento
     * previo de `GameViewModel.finishGame`.
     */
    @Test
    fun `un eliminado con la misma vida que el sobreviviente tambien bloquea el empate`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 15, isAlive = true),
            PlayerOutcome(id = 2, life = 15, isAlive = false)
        )

        assertNull(useCase.resolveWinner(players, explicitWinnerId = null))
    }

    @Test
    fun `sin nadie vivo y sin ganador explicito no hay resolucion`() {
        val players = listOf(
            PlayerOutcome(id = 1, life = 0, isAlive = false),
            PlayerOutcome(id = 2, life = 0, isAlive = false)
        )

        assertNull(useCase.resolveWinner(players, explicitWinnerId = null))
    }
}
