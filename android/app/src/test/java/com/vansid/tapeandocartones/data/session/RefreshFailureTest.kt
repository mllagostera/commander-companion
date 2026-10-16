package com.vansid.tapeandocartones.data.session

import com.vansid.tapeandocartones.testing.httpException
import java.io.IOException
import java.net.SocketTimeoutException
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RefreshFailureTest {

    @Test
    fun `a rejected or revoked refresh token ends the session`() {
        assertTrue(isRefreshRejected(httpException(401)))
    }

    @Test
    fun `a deactivated account ends the session`() {
        assertTrue(isRefreshRejected(httpException(403)))
    }

    @Test
    fun `no network keeps the session`() {
        assertFalse(isRefreshRejected(IOException("no route to host")))
        assertFalse(isRefreshRejected(SocketTimeoutException("timeout")))
    }

    @Test
    fun `rate limiting and server errors keep the session`() {
        assertFalse(isRefreshRejected(httpException(429)))
        assertFalse(isRefreshRejected(httpException(500)))
        assertFalse(isRefreshRejected(httpException(503)))
    }

    @Test
    fun `anything else keeps the session`() {
        assertFalse(isRefreshRejected(IllegalStateException("unexpected body")))
    }
}
