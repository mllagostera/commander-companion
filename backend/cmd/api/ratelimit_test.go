package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const testBFFSecret = "0123456789abcdef0123456789abcdef"

// newRateLimitedApp mounts two limiters of 2 requests each, wired the way
// registerModules wires credentials and refresh.
func newRateLimitedApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: common.ErrorHandler})
	app.Use(common.ResolveClientIP(common.ClientIPConfig{BFFSharedSecret: testBFFSecret}))
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }
	credentials := newRateLimiter(2, "credentials")
	refresh := newRateLimiter(2, "refresh")
	app.Post("/auth/login", credentials, ok)
	app.Post("/auth/refresh", refresh, ok)
	return app
}

func post(t *testing.T, app *fiber.App, path, browserIP string) int {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, http.NoBody)
	if browserIP != "" {
		req.Header.Set(common.BFFSecretHeader, testBFFSecret)
		req.Header.Set(common.BFFClientIPHeader, browserIP)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestRateLimit_RefreshDoesNotSpendTheLoginBudget(t *testing.T) {
	app := newRateLimitedApp()

	for range 2 {
		if got := post(t, app, "/auth/refresh", ""); got != http.StatusNoContent {
			t.Fatalf("refresh status = %d, want 204", got)
		}
	}
	if got := post(t, app, "/auth/refresh", ""); got != http.StatusTooManyRequests {
		t.Fatalf("third refresh status = %d, want 429", got)
	}
	if got := post(t, app, "/auth/login", ""); got != http.StatusNoContent {
		t.Fatalf("login after exhausting refresh: status = %d, want 204", got)
	}
}

func TestRateLimit_BrowsersBehindTheBFFHaveSeparateBuckets(t *testing.T) {
	app := newRateLimitedApp()

	for range 2 {
		post(t, app, "/auth/login", "203.0.113.7")
	}
	if got := post(t, app, "/auth/login", "203.0.113.7"); got != http.StatusTooManyRequests {
		t.Fatalf("third login from the same browser: status = %d, want 429", got)
	}
	// Same TCP peer (the BFF), different browser: its own budget.
	if got := post(t, app, "/auth/login", "203.0.113.8"); got != http.StatusNoContent {
		t.Fatalf("login from another browser: status = %d, want 204", got)
	}
}
