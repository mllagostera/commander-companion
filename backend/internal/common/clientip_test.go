package common_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	testBFFSecret = "0123456789abcdef0123456789abcdef"
	// Documentation ranges (RFC 5737): one for the browser behind the BFF,
	// one for a direct client behind the edge proxy.
	browserIP     = "203.0.113.7"
	directIP      = "198.51.100.1"
	trustedHeader = "True-Client-IP"
)

// newClientIPApp echoes the resolved client IP, and whether the BFF secret
// header was still visible to the handler (it must not be).
func newClientIPApp(cfg common.ClientIPConfig) *fiber.App {
	app := fiber.New()
	app.Use(common.ResolveClientIP(cfg))
	app.Get("/", func(c *fiber.Ctx) error {
		if c.Get(common.BFFSecretHeader) != "" || c.Get(common.BFFClientIPHeader) != "" {
			return c.SendString("LEAKED")
		}
		return c.SendString(common.ClientIP(c))
	})
	return app
}

func resolvedIP(t *testing.T, app *fiber.App, headers map[string]string) string {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// app.Test connects from 0.0.0.0, which is therefore the "TCP peer" fallback.
const peerIP = "0.0.0.0"

func TestResolveClientIP(t *testing.T) {
	tests := []struct {
		name    string
		cfg     common.ClientIPConfig
		headers map[string]string
		want    string
	}{
		{
			name: "no configuration uses the peer address",
			headers: map[string]string{
				common.BFFSecretHeader:   testBFFSecret,
				common.BFFClientIPHeader: browserIP,
				trustedHeader:            directIP,
			},
			want: peerIP,
		},
		{
			name: "BFF with the right secret is trusted",
			cfg:  common.ClientIPConfig{BFFSharedSecret: testBFFSecret},
			headers: map[string]string{
				common.BFFSecretHeader:   testBFFSecret,
				common.BFFClientIPHeader: browserIP,
			},
			want: browserIP,
		},
		{
			name: "wrong secret is ignored",
			cfg:  common.ClientIPConfig{BFFSharedSecret: testBFFSecret},
			headers: map[string]string{
				common.BFFSecretHeader:   "not-the-secret",
				common.BFFClientIPHeader: browserIP,
			},
			want: peerIP,
		},
		{
			name:    "client IP header without a secret is ignored",
			cfg:     common.ClientIPConfig{BFFSharedSecret: testBFFSecret},
			headers: map[string]string{common.BFFClientIPHeader: browserIP},
			want:    peerIP,
		},
		{
			name: "BFF wins over the trusted header",
			cfg:  common.ClientIPConfig{BFFSharedSecret: testBFFSecret, TrustedHeader: trustedHeader},
			headers: map[string]string{
				common.BFFSecretHeader:   testBFFSecret,
				common.BFFClientIPHeader: browserIP,
				trustedHeader:            directIP,
			},
			want: browserIP,
		},
		{
			name:    "trusted header is used for direct clients",
			cfg:     common.ClientIPConfig{TrustedHeader: trustedHeader},
			headers: map[string]string{trustedHeader: directIP},
			want:    directIP,
		},
		{
			name:    "a list in the trusted header is rejected",
			cfg:     common.ClientIPConfig{TrustedHeader: trustedHeader},
			headers: map[string]string{trustedHeader: directIP + ", 10.0.0.1"},
			want:    peerIP,
		},
		{
			name: "malformed BFF IP falls through to the trusted header",
			cfg:  common.ClientIPConfig{BFFSharedSecret: testBFFSecret, TrustedHeader: trustedHeader},
			headers: map[string]string{
				common.BFFSecretHeader:   testBFFSecret,
				common.BFFClientIPHeader: "not-an-ip",
				trustedHeader:            directIP,
			},
			want: directIP,
		},
		{
			name: "IPv4-mapped IPv6 is canonicalized",
			cfg:  common.ClientIPConfig{BFFSharedSecret: testBFFSecret},
			headers: map[string]string{
				common.BFFSecretHeader:   testBFFSecret,
				common.BFFClientIPHeader: "::ffff:" + browserIP,
			},
			want: browserIP,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvedIP(t, newClientIPApp(tc.cfg), tc.headers)
			if got == "LEAKED" {
				t.Fatal("BFF headers were still visible after ResolveClientIP")
			}
			if got != tc.want {
				t.Errorf("client IP = %q, want %q", got, tc.want)
			}
		})
	}
}
