package com.vansid.tapeandocartones.core.util

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LocalNetworkHostTest {

    @Test
    fun `the emulator's alias for the host machine is on the local network`() {
        assertTrue(isLocalNetworkUrl("http://10.0.2.2:8080/"))
    }

    @Test
    fun `every private, link-local and CGNAT block is on the local network`() {
        assertTrue(isLocalNetworkUrl("http://192.168.1.50:8080/"))
        assertTrue(isLocalNetworkUrl("http://172.16.0.1/"))
        assertTrue(isLocalNetworkUrl("http://172.31.255.255/"))
        assertTrue(isLocalNetworkUrl("http://169.254.10.1/"))
        assertTrue(isLocalNetworkUrl("http://100.64.0.1/"))
    }

    @Test
    fun `addresses just outside those blocks are not`() {
        assertFalse(isLocalNetworkUrl("http://172.32.0.1/"))
        assertFalse(isLocalNetworkUrl("http://100.128.0.1/"))
        assertFalse(isLocalNetworkUrl("http://11.0.0.1/"))
    }

    @Test
    fun `a deployed backend is not`() {
        assertFalse(isLocalNetworkUrl("https://api.example.com/"))
        assertFalse(isLocalNetworkUrl("https://8.8.8.8/"))
    }

    @Test
    fun `loopback is not, it is outside Android's definition`() {
        assertFalse(isLocalNetworkUrl("http://localhost:8080/"))
        assertFalse(isLocalNetworkUrl("http://127.0.0.1:8080/"))
    }

    @Test
    fun `garbage is not`() {
        assertFalse(isLocalNetworkUrl("not a url"))
        assertFalse(isLocalNetworkUrl("http://10.0.2/"))
        assertFalse(isLocalNetworkUrl("http://10.0.2.256/"))
    }
}
