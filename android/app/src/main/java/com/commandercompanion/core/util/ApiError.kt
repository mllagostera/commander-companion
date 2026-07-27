package com.commandercompanion.core.util

import kotlinx.coroutines.CancellationException
import retrofit2.HttpException
import java.io.IOException

/**
 * Error normalizado de una llamada a la API.
 *
 * Existe para que los repositorios devuelvan siempre el mismo tipo de fallo y los `ViewModel`
 * no tengan que conocer Retrofit/OkHttp (hoy `LoginViewModel` cachea `HttpException`/`IOException`
 * a mano en cada método; todo lo nuevo pasa por acá).
 */
sealed class ApiError(message: String, cause: Throwable? = null) : Exception(message, cause) {

    /** El servidor respondió, pero con un código de error (4xx/5xx). */
    class Http(val code: Int, cause: Throwable? = null) : ApiError("HTTP $code", cause)

    /** No se pudo hablar con el servidor (sin red, timeout, DNS, etc.). */
    class Network(cause: IOException) : ApiError("Sin conexión con el servidor", cause)

    /** Cualquier otra cosa: parseo de JSON, error de programación, etc. */
    class Unexpected(cause: Throwable) : ApiError(cause.message ?: "Error inesperado", cause)
}

/** Mensaje listo para mostrar en la UI. Genérico a propósito: cada pantalla puede afinarlo. */
fun ApiError.toUserMessage(): String = when (this) {
    is ApiError.Http -> when (code) {
        401 -> "Tu sesión expiró, iniciá sesión de nuevo"
        403 -> "No tenés permiso para hacer esto"
        404 -> "No se encontró en el servidor"
        409 -> "La partida no está en un estado válido para esta acción"
        else -> "El servidor respondió con un error ($code)"
    }
    is ApiError.Network -> "No se pudo conectar con el servidor"
    is ApiError.Unexpected -> "Ocurrió un error inesperado"
}

/**
 * Envuelve una llamada de red y traduce sus excepciones a [ApiError].
 *
 * `CancellationException` se **re-lanza** en vez de convertirse en `Result.failure`: si el scope
 * de la corrutina se canceló (el usuario salió de la pantalla), tragarla rompería la cancelación
 * estructurada y el llamador seguiría ejecutando código de "error" sobre una pantalla muerta.
 */
suspend fun <T> apiCall(block: suspend () -> T): Result<T> =
    try {
        Result.success(block())
    } catch (e: CancellationException) {
        throw e
    } catch (e: HttpException) {
        Result.failure(ApiError.Http(e.code(), e))
    } catch (e: IOException) {
        Result.failure(ApiError.Network(e))
    } catch (e: Throwable) {
        Result.failure(ApiError.Unexpected(e))
    }
