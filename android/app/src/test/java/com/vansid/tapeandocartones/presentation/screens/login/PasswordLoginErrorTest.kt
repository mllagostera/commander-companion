package com.vansid.tapeandocartones.presentation.screens.login

import org.junit.Assert.assertEquals
import org.junit.Test

class PasswordLoginErrorTest {

    @Test
    fun `401 is bad credentials`() {
        assertEquals(LoginError.BadCredentials, passwordLoginError(401, """{"code":401,"message":"invalid email or password"}"""))
    }

    @Test
    fun `403 for an unconfirmed email offers the resend`() {
        assertEquals(
            LoginError.EmailNotConfirmed,
            passwordLoginError(403, """{"code":403,"message":"email not confirmed, check your inbox"}""")
        )
    }

    @Test
    fun `403 for a deactivated account is not mistaken for an unconfirmed email`() {
        assertEquals(
            LoginError.AccountDeactivated,
            passwordLoginError(403, """{"code":403,"message":"account has been deactivated"}""")
        )
    }

    @Test
    fun `403 without a body falls back to unconfirmed email`() {
        assertEquals(LoginError.EmailNotConfirmed, passwordLoginError(403, null))
    }

    @Test
    fun `any other status keeps its code`() {
        assertEquals(LoginError.Unknown(500), passwordLoginError(500, null))
    }
}
