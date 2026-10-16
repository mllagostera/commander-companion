package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

func TestRequestLogger_SkipsOnlyTheHealthCheck(t *testing.T) {
	var out bytes.Buffer
	app := fiber.New()
	app.Use(newRequestLogger(&out))
	ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }
	app.Get(common.HealthPath, ok)
	app.Get("/api/v1/decks", ok)

	for _, path := range []string{common.HealthPath, "/api/v1/decks"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test(%s): %v", path, err)
		}
		_ = resp.Body.Close()
	}

	logged := out.String()
	if strings.Contains(logged, common.HealthPath) {
		t.Errorf("health check was logged:\n%s", logged)
	}
	if !strings.Contains(logged, "/api/v1/decks") {
		t.Errorf("a regular request was not logged:\n%s", logged)
	}
}
