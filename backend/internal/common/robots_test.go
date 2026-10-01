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

func TestRegisterRobotsRoute_DisallowsEverything(t *testing.T) {
	app := fiber.New()
	common.RegisterRobotsRoute(app)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/robots.txt", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got, want := string(body), "User-agent: *\nDisallow: /\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestNoIndex_SetsHeaderOnSuccessAndError(t *testing.T) {
	app := fiber.New()
	app.Use(common.NoIndex)
	app.Get("/ok", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	for _, path := range []string{"/ok", "/missing"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s): %v", path, err)
		}
		_ = resp.Body.Close()

		if got := resp.Header.Get("X-Robots-Tag"); got != "noindex, nofollow" {
			t.Errorf("%s: X-Robots-Tag = %q, want %q", path, got, "noindex, nofollow")
		}
	}
}
