package com.commandercompanion.core.util

import com.commandercompanion.testing.httpException
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test
import java.io.IOException
import java.net.UnknownHostException

/** Mapeo de excepciones de red a [ApiError] — la base del manejo de errores de los repositorios. */
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
     * Clave: si el scope se cancela, tragar la excepción rompería la cancelación estructurada
     * y el llamador seguiría ejecutando lógica de error sobre una pantalla ya destruida.
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
        // HttpException no hereda de IOException, pero el orden de los catch importa:
        // este test falla si alguien reordena los bloques.
        val result = apiCall<String> { throw httpException(500) }

        assertTrue(result.exceptionOrNull() is ApiError.Http)
    }

    @Test
    fun `toUserMessage distingue sesion expirada, conflicto y sin red`() {
        assertEquals(
            "Tu sesión expiró, iniciá sesión de nuevo",
            ApiError.Http(401).toUserMessage()
        )
        assertEquals(
            "La partida no está en un estado válido para esta acción",
            ApiError.Http(409).toUserMessage()
        )
        assertEquals(
            "No se pudo conectar con el servidor",
            ApiError.Network(IOException()).toUserMessage()
        )
        assertEquals(
            "El servidor respondió con un error (500)",
            ApiError.Http(500).toUserMessage()
        )
    }
}
