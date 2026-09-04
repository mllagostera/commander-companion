package com.commandercompanion.core.util

import java.io.IOException
import kotlinx.coroutines.CancellationException
import retrofit2.HttpException

/**
 * Normalized error from an API call.
 *
 * Exists so repositories always return the same failure type and `ViewModel`s don't have to
 * know about Retrofit/OkHttp.
 *
 * The `message` of each subclass is a *developer* string (logs, stack traces) and is never
 * shown to anyone — see [ApiFailure] for what reaches the screen.
 */
sealed class ApiError(message: String, cause: Throwable? = null) : Exception(message, cause) {

    /** The server responded, but with an error code (4xx/5xx). */
    class Http(val code: Int, cause: Throwable? = null) : ApiError("HTTP $code", cause)

    /** Couldn't reach the server (no network, timeout, DNS, etc.). */
    class Network(cause: IOException) : ApiError("network unreachable", cause)

    /** Anything else: JSON parsing, a programming error, etc. */
    class Unexpected(cause: Throwable) : ApiError(cause.message ?: "unexpected failure", cause)
}

/**
 * What went wrong, for the screen to turn into a string resource.
 *
 * Same reasoning as `FriendsError`: the app ships `values/`, `values-en/` and `values-ca/`, so
 * a literal built down here would be untranslatable. It is a sealed interface rather than an
 * enum only because [Server] has to carry the status code the message interpolates.
 */
sealed interface ApiFailure {
    data object Network : ApiFailure
    data object SessionExpired : ApiFailure
    data object Forbidden : ApiFailure
    data object NotFound : ApiFailure
    data object Conflict : ApiFailure
    data class Server(val code: Int) : ApiFailure
    data object Unexpected : ApiFailure
}

/** Generic mapping; a screen with something better to say for a code refines it itself. */
fun ApiError.toFailure(): ApiFailure = when (this) {
    is ApiError.Network -> ApiFailure.Network
    is ApiError.Unexpected -> ApiFailure.Unexpected
    is ApiError.Http -> when (code) {
        401 -> ApiFailure.SessionExpired
        403 -> ApiFailure.Forbidden
        404 -> ApiFailure.NotFound
        409 -> ApiFailure.Conflict
        else -> ApiFailure.Server(code)
    }
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
