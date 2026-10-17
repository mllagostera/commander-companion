package com.vansid.tapeandocartones.presentation.navigation

import com.vansid.tapeandocartones.data.session.AccessTokenProvider
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class StartupViewModelTest {

    private val dispatcher = StandardTestDispatcher()

    @Before
    fun setUp() {
        Dispatchers.setMain(dispatcher)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun `a stored session opens on the dashboard instead of asking to sign in again`() = runTest(dispatcher) {
        val vm = StartupViewModel(AccessTokenProvider { "access-token" })
        advanceUntilIdle()

        assertEquals(DashboardRoute, vm.startDestination.value)
    }

    @Test
    fun `no stored session opens on the login`() = runTest(dispatcher) {
        val vm = StartupViewModel(AccessTokenProvider { null })
        advanceUntilIdle()

        assertEquals(LoginRoute, vm.startDestination.value)
    }

    @Test
    fun `nothing is chosen before the session has been read`() = runTest(dispatcher) {
        val vm = StartupViewModel(AccessTokenProvider { "access-token" })

        assertNull(vm.startDestination.value)
    }
}
