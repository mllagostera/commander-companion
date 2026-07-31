package com.commandercompanion.core.util

import kotlinx.coroutines.CancellationException
import retrofit2.HttpException
import java.io.IOException

/**
 * Normalized error from an API call.
 *
 * Exists so repositories always return the same failure type and `ViewModel`s don't have to
 * know about Retrofit/OkHttp (today `LoginViewModel` catches `HttpException`/`IOException`
 * by hand in each method; everything new goes through here).
 */
sealed class ApiError(message: String, cause: Throwable? = null) : Exception(message, cause) {

    /** The server responded, but with an error code (4xx/5xx). */
    class Http(val code: Int, cause: Throwable? = null) : ApiError("HTTP $code", cause)

    /** Couldn't reach the server (no network, timeout, DNS, etc.). */
    class Network(cause: IOException) : ApiError("Sin conexión con el servidor", cause)

    /** Anything else: JSON parsing, a programming error, etc. */
    class Unexpected(cause: Throwable) : ApiError(cause.message ?: "Error inesperado", cause)
}

/** Message ready to show in the UI. Deliberately generic: each screen can refine it. */
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
 * Wraps a network call and translates its exceptions into [ApiError].
 *
 * `CancellationException` is **re-thrown** instead of becoming a `Result.failure`: if the
 * coroutine's scope was cancelled (the user left the screen), swallowing it would break
 * structured cancellation and the caller would keep running "error" code on a dead screen.
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
