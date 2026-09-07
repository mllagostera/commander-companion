package common_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

// testCommit is a plausible full SHA: the handler serves whatever config
// resolved, so the test only cares that it reaches the body verbatim.
const testCommit = "0123456789abcdef0123456789abcdef01234567"

// The instant testBuildInfo carries, in the RFC 3339 UTC shape the handler
// must serve it as.
const testStartedAtRFC3339 = "2026-09-06T10:30:00Z"

func testBuildInfo() common.BuildInfo {
	return common.BuildInfo{
		Commit:    testCommit,
		StartedAt: time.Date(2026, 9, 6, 10, 30, 0, 0, time.UTC),
	}
}

type healthBody struct {
	Status    string `json:"status"`
	DB        string `json:"db"`
	Commit    string `json:"commit"`
	StartedAt string `json:"started_at"`
}

func decodeHealth(t *testing.T, r io.Reader) healthBody {
	t.Helper()

	var body healthBody
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		t.Fatalf("decode /health body: %v", err)
	}
	return body
}

// assertBuildMarker checks the half of the response that must not depend on
// the database: which build is answering and since when. A caller polling for
// a finished deploy reads it on both branches (see ADR-0020).
func assertBuildMarker(t *testing.T, body healthBody) {
	t.Helper()

	if body.Commit != testCommit {
		t.Errorf("commit = %q, want %q", body.Commit, testCommit)
	}
	if body.StartedAt != testStartedAtRFC3339 {
		t.Errorf("started_at = %q, want %q (RFC 3339, UTC)", body.StartedAt, testStartedAtRFC3339)
	}
}

func TestRegisterHealthRoute_DBReachable(t *testing.T) {
	db := &common.DB{Pool: testutil.DB(t)}

	app := fiber.New()
	common.RegisterHealthRoute(app, db, testBuildInfo())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := decodeHealth(t, resp.Body)
	if body.Status != "ok" || body.DB != "ok" {
		t.Errorf("status/db = %q/%q, want ok/ok", body.Status, body.DB)
	}
	assertBuildMarker(t, body)
}

func TestRegisterHealthRoute_DBUnreachable(t *testing.T) {
	pool := testutil.DB(t)
	pool.Close() // simulates Postgres being down/unreachable without depending on external infra

	db := &common.DB{Pool: pool}
	app := fiber.New()
	common.RegisterHealthRoute(app, db, testBuildInfo())

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	body := decodeHealth(t, resp.Body)
	if body.Status != "error" || body.DB != "unreachable" {
		t.Errorf("status/db = %q/%q, want error/unreachable", body.Status, body.DB)
	}
	// The point of the marker: a deploy poller still learns which build is up
	// while Postgres is down.
	assertBuildMarker(t, body)
}
