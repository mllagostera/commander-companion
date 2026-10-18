package inbound_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/inbound"
)

const webhookPath = "/webhooks/resend-inbound"

var errResendDown = errors.New("resend is down")

type stubForwarder struct {
	ids []string
	err error
}

func (s *stubForwarder) Forward(_ context.Context, emailID string) error {
	s.ids = append(s.ids, emailID)
	return s.err
}

func newApp(fw inbound.Forwarder) *fiber.App {
	app := fiber.New()
	inbound.NewHandler(testSecret(), fw).RegisterRoutes(app)
	return app
}

func webhookRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, webhookPath, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func signedRequest(t *testing.T, body, key string) *http.Request {
	t.Helper()
	ts := time.Now().Unix()
	req := webhookRequest(t, body)
	req.Header.Set("svix-id", testMsgID)
	req.Header.Set("svix-timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("svix-signature", sign(key, ts, []byte(body)))
	return req
}

func TestResendInbound(t *testing.T) {
	received := `{"type":"email.received","created_at":"2026-10-18T10:00:00Z",` +
		`"data":{"email_id":"em_123","from":"ana@example.com"}}`

	tests := []struct {
		name       string
		req        func(t *testing.T) *http.Request
		forwardErr error
		wantStatus int
		wantIDs    int
	}{
		{
			name:       "forwards a received email",
			req:        func(t *testing.T) *http.Request { return signedRequest(t, received, testKey) },
			wantStatus: fiber.StatusNoContent,
			wantIDs:    1,
		},
		{
			name:       "refuses a bad signature without forwarding",
			req:        func(t *testing.T) *http.Request { return signedRequest(t, received, "not-the-secret") },
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "refuses an unsigned request",
			req:        func(t *testing.T) *http.Request { return webhookRequest(t, received) },
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name: "ignores other events",
			req: func(t *testing.T) *http.Request {
				return signedRequest(t, `{"type":"email.delivered","data":{"email_id":"em_9"}}`, testKey)
			},
			wantStatus: fiber.StatusNoContent,
		},
		{
			name: "rejects email.received without an id",
			req: func(t *testing.T) *http.Request {
				return signedRequest(t, `{"type":"email.received","data":{}}`, testKey)
			},
			wantStatus: fiber.StatusBadRequest,
		},
		{
			name:       "answers 502 when forwarding fails so Resend retries",
			req:        func(t *testing.T) *http.Request { return signedRequest(t, received, testKey) },
			forwardErr: errResendDown,
			wantStatus: fiber.StatusBadGateway,
			wantIDs:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw := &stubForwarder{err: tt.forwardErr}
			resp, err := newApp(fw).Test(tt.req(t))
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if len(fw.ids) != tt.wantIDs {
				t.Fatalf("forwarded %v, want %d call(s)", fw.ids, tt.wantIDs)
			}
			if tt.wantIDs == 1 && fw.ids[0] != "em_123" {
				t.Errorf("forwarded email %q, want em_123", fw.ids[0])
			}
		})
	}
}
