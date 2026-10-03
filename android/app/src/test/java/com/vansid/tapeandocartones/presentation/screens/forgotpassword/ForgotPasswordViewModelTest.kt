package com.vansid.tapeandocartones.presentation.screens.forgotpassword

import com.vansid.tapeandocartones.data.remote.api.AuthApi
import com.vansid.tapeandocartones.data.remote.dto.ForgotPasswordRequest
import com.vansid.tapeandocartones.data.remote.dto.GoogleLoginRequest
import com.vansid.tapeandocartones.data.remote.dto.LoginRequest
import com.vansid.tapeandocartones.data.remote.dto.LogoutRequest
import com.vansid.tapeandocartones.data.remote.dto.RefreshRequest
import com.vansid.tapeandocartones.data.remote.dto.RegisterRequest
import com.vansid.tapeandocartones.data.remote.dto.ResendVerificationRequest
import com.vansid.tapeandocartones.data.remote.dto.TokenResponse
import com.vansid.tapeandocartones.data.remote.dto.UserDto
import com.vansid.tapeandocartones.testing.httpException
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/** Only [forgotPassword] is exercised here; the rest fail loudly if a test ever reaches them. */
private class FakeAuthApi : AuthApi {
    val forgotPasswordRequests = mutableListOf<ForgotPasswordRequest>()
    var onForgotPassword: () -> Unit = {}

    override suspend fun forgotPassword(request: ForgotPasswordRequest) {
        forgotPasswordRequests += request
        onForgotPassword()
    }

    override suspend fun login(request: LoginRequest): TokenResponse = error("not used")
    override suspend fun register(request: RegisterRequest): UserDto = error("not used")
    override suspend fun resendVerification(request: ResendVerificationRequest) = error("not used")
    override suspend fun loginWithGoogle(request: GoogleLoginRequest): TokenResponse = error("not used")
    override suspend fun refresh(request: RefreshRequest): TokenResponse = error("not used")
    override suspend fun logout(request: LogoutRequest) = error("not used")
    override suspend fun me(bearerToken: String): UserDto = error("not used")
}

@OptIn(ExperimentalCoroutinesApi::class)
class ForgotPasswordViewModelTest {

    private val dispatcher = StandardTestDispatcher()
    private val api = FakeAuthApi()

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun `sends the trimmed email and the app language`() = runTest(dispatcher) {
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("  ana@example.com ", "ca")
        advanceUntilIdle()

        assertEquals(listOf(ForgotPasswordRequest("ana@example.com", "ca")), api.forgotPasswordRequests)
        val state = vm.uiState.value
        assertEquals("ana@example.com", state.sentTo)
        assertFalse(state.isLoading)
        assertNull(state.error)
    }

    @Test
    fun `a blank email never reaches the backend`() = runTest(dispatcher) {
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("   ", "es")
        advanceUntilIdle()

        assertEquals(ForgotPasswordError.EmptyEmail, vm.uiState.value.error)
        assertTrue(api.forgotPasswordRequests.isEmpty())
    }

    @Test
    fun `a bare username is rejected before calling the backend`() = runTest(dispatcher) {
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("ana", "es")
        advanceUntilIdle()

        assertEquals(ForgotPasswordError.InvalidEmail, vm.uiState.value.error)
        assertTrue(api.forgotPasswordRequests.isEmpty())
    }

    @Test
    fun `the per-IP rate limit has its own message`() = runTest(dispatcher) {
        api.onForgotPassword = { throw httpException(429) }
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("ana@example.com", "es")
        advanceUntilIdle()

        assertEquals(ForgotPasswordError.TooManyRequests, vm.uiState.value.error)
        assertNull(vm.uiState.value.sentTo)
    }

    @Test
    fun `any other status keeps its code`() = runTest(dispatcher) {
        api.onForgotPassword = { throw httpException(500) }
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("ana@example.com", "es")
        advanceUntilIdle()

        assertEquals(ForgotPasswordError.Unknown(500), vm.uiState.value.error)
    }

    @Test
    fun `no connection is a network error`() = runTest(dispatcher) {
        api.onForgotPassword = { throw IOException("offline") }
        val vm = ForgotPasswordViewModel(api)

        vm.requestReset("ana@example.com", "es")
        advanceUntilIdle()

        assertEquals(ForgotPasswordError.Network, vm.uiState.value.error)
        assertFalse(vm.uiState.value.isLoading)
    }
}
