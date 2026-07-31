package com.commandercompanion.presentation.navigation

import org.junit.Assert.assertEquals
import org.junit.Test

class PlayerConfigCodecTest {

    @Test
    fun `round-trip preserva assignedUserId, assignedUsername y deckId de cada asiento`() {
        val configs = listOf(
            PlayerConfig(
                name = "Ana", colorKey = "blue", mulligans = 1,
                assignedUserId = "user-1", assignedUsername = "ana", deckId = "deck-a"
            ),
            PlayerConfig(name = "Beto", colorKey = "red")
        )

        val decoded = decodePlayerConfigs(encodePlayerConfigs(configs))

        assertEquals(configs, decoded)
    }

    @Test
    fun `name with special characters survives encode-decode`() {
        val configs = listOf(PlayerConfig(name = "José | Ana, Beto", colorKey = "green"))

        val decoded = decodePlayerConfigs(encodePlayerConfigs(configs))

        assertEquals("José | Ana, Beto", decoded.single().name)
    }

    @Test
    fun `username con caracteres especiales sobrevive el encode-decode`() {
        val configs = listOf(
            PlayerConfig(
                name = "Ana", colorKey = "blue",
                assignedUserId = "user-1", assignedUsername = "ana|beto,pepe"
            )
        )

        val decoded = decodePlayerConfigs(encodePlayerConfigs(configs))

        assertEquals("ana|beto,pepe", decoded.single().assignedUsername)
    }

    @Test
    fun `sin asignacion ni deckId decodifica a los defaults`() {
        val decoded = decodePlayerConfigs("Ana|blue|0")

        assertEquals(PlayerConfig(name = "Ana", colorKey = "blue"), decoded.single())
    }
}
