package common

import (
	"crypto/subtle"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// Headers the web client's BFF (Nitro) uses to tell the API which browser it is
// calling on behalf of. Every web request reaches the API from the BFF's own
// address, so without them a per-IP rate limit would put every web user in the
// same bucket (see ADR-0024).
const (
	// BFFSecretHeader carries the shared secret (BFF_SHARED_SECRET) that proves
	// the request comes from the BFF. ResolveClientIP strips it from the request
	// before any later middleware (Sentry, handlers) can see it.
	BFFSecretHeader = "X-BFF-Secret" //nolint:gosec // a header name, not a credential
	// BFFClientIPHeader carries the browser's IP as the BFF saw it. Honored only
	// alongside a valid BFFSecretHeader; anyone else sending it is ignored.
	BFFClientIPHeader = "X-BFF-Client-IP"
)

// clientIPKey is the fiber.Ctx.Locals key ResolveClientIP stores its result under.
const clientIPKey = "client_ip"

// ClientIPConfig says which request data the API may trust to identify the
// caller's IP. The zero value trusts nothing but the TCP peer address.
type ClientIPConfig struct {
	// BFFSharedSecret enables BFFClientIPHeader. Empty disables it: every
	// web request is then keyed by the BFF's address.
	BFFSharedSecret string
	// TrustedHeader names a single-IP header the edge proxy in front of the API
	// overwrites on every request (never one it appends to, like
	// X-Forwarded-For, whose leftmost entry the client controls). Empty uses
	// the TCP peer address.
	TrustedHeader string
}

// ResolveClientIP returns the middleware that works out the caller's IP once
// per request, for ClientIP to read. It must be mounted before anything that
// records request headers (Sentry), because it removes the BFF secret header.
//
// Precedence: the IP the BFF vouches for, then TrustedHeader, then the TCP
// peer address. A malformed value at any step falls through to the next one
// instead of failing the request: this decides a rate-limit bucket, not access.
func ResolveClientIP(cfg ClientIPConfig) fiber.Handler {
	secret := []byte(cfg.BFFSharedSecret)

	return func(c *fiber.Ctx) error {
		sentSecret := c.Get(BFFSecretHeader)
		claimedIP := c.Get(BFFClientIPHeader)
		c.Request().Header.Del(BFFSecretHeader)
		c.Request().Header.Del(BFFClientIPHeader)

		ip := ""
		if len(secret) > 0 && subtle.ConstantTimeCompare([]byte(sentSecret), secret) == 1 {
			ip = parseIP(claimedIP)
		}
		if ip == "" && cfg.TrustedHeader != "" {
			ip = parseIP(c.Get(cfg.TrustedHeader))
		}
		if ip == "" {
			ip = c.IP()
		}

		c.Locals(clientIPKey, ip)
		return c.Next()
	}
}

// ClientIP returns the caller's IP as resolved by ResolveClientIP, or the TCP
// peer address on a route the middleware doesn't cover.
func ClientIP(c *fiber.Ctx) string {
	if ip, ok := c.Locals(clientIPKey).(string); ok && ip != "" {
		return ip
	}
	return c.IP()
}

// parseIP accepts exactly one IP address and returns it in canonical form, or
// "" for anything else (a list, a port, garbage). Canonical form matters: the
// result is a rate-limit key, so "::ffff:1.2.3.4" and "1.2.3.4" must not be two
// different buckets for the same client.
func parseIP(raw string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return addr.Unmap().String()
}
