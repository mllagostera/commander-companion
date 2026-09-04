package com.commandercompanion.core.util

import com.commandercompanion.testing.httpException
import java.io.IOException
import java.net.UnknownHostException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

/** Mapping of network exceptions to [ApiError] — the basis of the repositories' error handling. */
class ApiCallTest {

    @Test
    fun `devuelve success con el valor cuando no hay error`() = runTest {
        val result = apiCall { "ok" }

        assertEquals("ok", result.getOrNull())
    }

    @Test
    fun `mapea HttpException a ApiError Http conservando el codigo`() = runTest {
        val result = apiCall<String> { throw httpException(409) }

        val error = result.exceptionOrNull()
        assertTrue("esperaba ApiError.Http, fue $error", error is ApiError.Http)
        assertEquals(409, (error as ApiError.Http).code)
    }

    @Test
    fun `mapea IOException a ApiError Network`() = runTest {
        val result = apiCall<String> { throw UnknownHostException("sin dns") }

        assertTrue(result.exceptionOrNull() is ApiError.Network)
    }

    @Test
    fun `mapea cualquier otra excepcion a ApiError Unexpected`() = runTest {
        val result = apiCall<String> { throw IllegalStateException("json roto") }

        assertTrue(result.exceptionOrNull() is ApiError.Unexpected)
    }

    /**
     * Key: if the scope gets cancelled, swallowing the exception would break structured
     * cancellation and the caller would keep running error logic on an already-destroyed screen.
     */
    @Test
    fun `relanza CancellationException en vez de convertirla en failure`() = runTest {
        try {
            apiCall<String> { throw CancellationException("scope cancelado") }
            fail("apiCall debería haber relanzado la CancellationException")
        } catch (e: CancellationException) {
            assertEquals("scope cancelado", e.message)
        }
    }

    @Test
    fun `HttpException no se confunde con un error de red`() = runTest {
        // HttpException doesn't inherit from IOException, but the order of the catch blocks matters:
        // this test fails if someone reorders them.
        val result = apiCall<String> { throw httpException(500) }

        assertTrue(result.exceptionOrNull() is ApiError.Http)
    }

    /**
     * Asserts the [ApiFailure] case, not prose: the wording lives in `strings.xml` in three
     * locales, so a string assertion here would only pin down one of them.
     */
    @Test
    fun `toFailure distingue sesion expirada, conflicto y sin red`() {
        assertEquals(ApiFailure.SessionExpired, ApiError.Http(401).toFailure())
        assertEquals(ApiFailure.Forbidden, ApiError.Http(403).toFailure())
        assertEquals(ApiFailure.NotFound, ApiError.Http(404).toFailure())
        assertEquals(ApiFailure.Conflict, ApiError.Http(409).toFailure())
        assertEquals(ApiFailure.Network, ApiError.Network(IOException()).toFailure())
        assertEquals(ApiFailure.Unexpected, ApiError.Unexpected(IllegalStateException()).toFailure())
    }

    /** The status code survives into the message the screen interpolates. */
    @Test
    fun `un codigo sin caso propio conserva el numero`() {
        assertEquals(ApiFailure.Server(500), ApiError.Http(500).toFailure())
    }
}
