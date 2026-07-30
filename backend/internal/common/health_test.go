package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

func TestRegisterHealthRoute_DBReachable(t *testing.T) {
	db := &common.DB{Pool: testutil.DB(t)}

	app := fiber.New()
	common.RegisterHealthRoute(app, db)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestRegisterHealthRoute_DBUnreachable(t *testing.T) {
	pool := testutil.DB(t)
	pool.Close() // simula Postgres caído/inalcanzable sin depender de infra externa

	db := &common.DB{Pool: pool}
	app := fiber.New()
	common.RegisterHealthRoute(app, db)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}
