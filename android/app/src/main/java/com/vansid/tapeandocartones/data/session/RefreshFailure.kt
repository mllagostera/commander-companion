package com.vansid.tapeandocartones.data.session

import retrofit2.HttpException

/**
 * Status codes with which `POST /auth/refresh` rejects the refresh token itself: 401 for an
 * unknown, expired or revoked token (including a reused one, which revokes the whole session
 * family) and 403 for a deactivated account. See `backend/internal/auth/service.go: Refresh`.
 */
private val REJECTED_REFRESH_CODES = setOf(401, 403)

/**
 * Whether a failed refresh means the session is over, as opposed to the refresh merely not
 * getting through.
 *
 * Only the backend saying no ends it. No network, a timeout, a 429 or a 5xx say nothing about
 * the token, which is still valid on the server: logging out on those used to throw away a
 * working 30-day session over a dropped connection mid-game, without revoking it server-side.
 */
internal fun isRefreshRejected(error: Throwable): Boolean =
    error is HttpException && error.code() in REJECTED_REFRESH_CODES
