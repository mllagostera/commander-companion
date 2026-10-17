package com.vansid.tapeandocartones.core.util

import java.net.URI

/**
 * Whether [url]'s host is an IPv4 literal on what Android 17 calls the local network: the
 * RFC 1918 blocks, link-local and CGNAT (see developer.android.com,
 * privacy-and-security/local-network-definition). Reaching one from an app targeting API 37
 * needs the `ACCESS_LOCAL_NETWORK` runtime permission.
 *
 * Only IP literals are recognized, because that is what a development backend URL is
 * (`10.0.2.2` on the emulator, a `192.168.x.x` LAN address on a phone): a hostname would need a
 * DNS lookup to classify, and a deployed backend is never on the local network. Loopback is not
 * in Android's definition, so `localhost` is not local here either.
 */
fun isLocalNetworkUrl(url: String): Boolean {
    val host = runCatching { URI(url).host }.getOrNull() ?: return false
    val octets = host.split('.')
    if (octets.size != IPV4_OCTETS) return false
    val bytes = octets.map { it.toIntOrNull()?.takeIf { b -> b in 0..MAX_OCTET } ?: return false }
    val (a, b) = bytes
    return a == 10 ||
        (a == 172 && b in 16..31) ||
        (a == 192 && b == 168) ||
        (a == 169 && b == 254) ||
        (a == 100 && b in 64..127)
}

private const val IPV4_OCTETS = 4
private const val MAX_OCTET = 255
