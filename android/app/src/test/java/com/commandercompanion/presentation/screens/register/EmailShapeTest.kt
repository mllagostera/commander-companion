package com.commandercompanion.presentation.screens.register

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class EmailShapeTest {

    @Test
    fun `accepts ordinary addresses, surrounding whitespace included`() {
        listOf(
            "user@example.com",
            "  User@Example.COM  ",
            "first.last+tag@mail.co.uk"
        ).forEach { assertTrue("expected valid: '$it'", isPlausibleEmail(it)) }
    }

    @Test
    fun `rejects a bare username and other malformed input`() {
        listOf(
            "vansidgg",
            "",
            "user@",
            "@example.com",
            "user@localhost",
            "a b@example.com",
            "a@b@example.com"
        ).forEach { assertFalse("expected invalid: '$it'", isPlausibleEmail(it)) }
    }
}
