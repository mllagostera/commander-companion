package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/config"
	"github.com/usuario/commander-companion-backend/internal/inbound"
)

const inboundWebhookPath = "/api/v1/webhooks/resend-inbound"

// postInbound sends an unsigned webhook through the real router and returns the
// status and body. No request reaches the database: the handler stops at the
// signature, and registering the modules only stores the (nil) pool.
func postInbound(t *testing.T, cfg *config.Config) (status int, body string) {
	t.Helper()
	app := fiber.New(serverConfig(defaultServerTimeouts()))
	registerModules(app, &common.DB{}, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, inboundWebhookPath,
		strings.NewReader(`{"type":"email.received","data":{"email_id":"em_1"}}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// The protected group's RequireAuth runs for every /api/v1 route registered
// after it; the webhook has no JWT, so it must be mounted before. Asserting on
// the body tells the handler's own 401 apart from RequireAuth's.
func TestInboundWebhookIsNotBehindRequireAuth(t *testing.T) {
	cfg := &config.Config{
		Auth: auth.Config{JWTSecret: []byte("test-secret")},
		Inbound: inbound.Config{ //nolint:gosec // throwaway test values, not real secrets
			WebhookSecret: "whsec_dGVzdA==",
			APIKey:        "re_test",
			ForwardTo:     "me@example.com",
			ForwardFrom:   "privacidad@tapeandocartones.es",
		},
	}

	status, body := postInbound(t, cfg)
	if status != fiber.StatusUnauthorized || !strings.Contains(body, inbound.ErrInvalidSignature.Error()) {
		t.Fatalf("POST %s = %d %q, want the webhook handler's own signature error", inboundWebhookPath, status, body)
	}
}

func TestInboundWebhookAbsentWhenDisabled(t *testing.T) {
	cfg := &config.Config{Auth: auth.Config{JWTSecret: []byte("test-secret")}}

	status, body := postInbound(t, cfg)
	if strings.Contains(body, inbound.ErrInvalidSignature.Error()) {
		t.Fatalf("POST %s = %d %q, want no webhook route when inbound is not configured", inboundWebhookPath, status, body)
	}
}
