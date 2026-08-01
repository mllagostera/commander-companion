package com.commandercompanion.domain.usecase

import com.commandercompanion.data.remote.dto.CreateActionRequest
import com.commandercompanion.data.remote.dto.GameActionType
import com.commandercompanion.data.remote.dto.amountPayload
import com.commandercompanion.testing.gameActionDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class ReplayCommanderDamageUseCaseTest {

    private val useCase = ReplayCommanderDamageUseCase()

    /** seat 1 = gp-a, seat 2 = gp-b, seat 3 = gp-c. */
    private val seatByPlayerId = mapOf("gp-a" to 1, "gp-b" to 2, "gp-c" to 3)

    private fun commanderDamage(actorId: String, targetId: String?, amount: Int) = gameActionDto(
        gameId = "game-1",
        request = CreateActionRequest(
            actorId = actorId,
            targetId = targetId,
            actionType = GameActionType.COMMANDER_DAMAGE,
            payload = amountPayload(amount)
        )
    )

    @Test
    fun `acumula el dano repetido del mismo atacante sobre el mismo objetivo`() {
        val actions = listOf(
            commanderDamage(actorId = "gp-a", targetId = "gp-b", amount = 5),
            commanderDamage(actorId = "gp-a", targetId = "gp-b", amount = 3)
        )

        val result = useCase(actions, seatByPlayerId)

        assertEquals(8, result[2]!![1])
    }

    @Test
    fun `distingue el dano por atacante dentro del mismo objetivo`() {
        val actions = listOf(
            commanderDamage(actorId = "gp-a", targetId = "gp-c", amount = 4),
            commanderDamage(actorId = "gp-b", targetId = "gp-c", amount = 9)
        )

        val result = useCase(actions, seatByPlayerId)

        assertEquals(mapOf(1 to 4, 2 to 9), result[3])
    }

    @Test
    fun `distingue el dano por objetivo`() {
        val actions = listOf(
            commanderDamage(actorId = "gp-a", targetId = "gp-b", amount = 5),
            commanderDamage(actorId = "gp-a", targetId = "gp-c", amount = 7)
        )

        val result = useCase(actions, seatByPlayerId)

        assertEquals(5, result[2]!![1])
        assertEquals(7, result[3]!![1])
    }

    @Test
    fun `ignora acciones que no son CommanderDamage`() {
        val actions = listOf(
            gameActionDto(
                gameId = "game-1",
                request = CreateActionRequest(actorId = "gp-a", actionType = GameActionType.LIFE_CHANGE, payload = amountPayload(-5))
            )
        )

        assertTrue(useCase(actions, seatByPlayerId).isEmpty())
    }

    @Test
    fun `ignora acciones de un actor desconocido`() {
        val actions = listOf(commanderDamage(actorId = "gp-fantasma", targetId = "gp-b", amount = 5))

        assertTrue(useCase(actions, seatByPlayerId).isEmpty())
    }

    @Test
    fun `ignora CommanderDamage sin target`() {
        val actions = listOf(commanderDamage(actorId = "gp-a", targetId = null, amount = 5))

        assertTrue(useCase(actions, seatByPlayerId).isEmpty())
    }

    @Test
    fun `ignora acciones sin amount en el payload`() {
        val malformed = gameActionDto(
            gameId = "game-1",
            request = CreateActionRequest(actorId = "gp-a", targetId = "gp-b", actionType = GameActionType.COMMANDER_DAMAGE, payload = null)
        )

        assertTrue(useCase(listOf(malformed), seatByPlayerId).isEmpty())
    }

    @Test
    fun `sin acciones devuelve un mapa vacio`() {
        assertTrue(useCase(emptyList(), seatByPlayerId).isEmpty())
    }
}
