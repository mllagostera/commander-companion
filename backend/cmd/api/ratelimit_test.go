package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	testBFFSecret = "0123456789abcdef0123456789abcdef"
	loginPath     = "/auth/login"
)

// testRequest is one request against a test app: who sends it (the browser IP
// the BFF vouches for, empty for none) and what it carries.
type testRequest struct {
	path      string
	browserIP string
	body      string
}

// send returns the response's status and headers; the body is closed here.
func send(t *testing.T, app *fiber.App, r testRequest) (int, http.Header) {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, r.path, http.NoBody)
	if r.body != "" {
		req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, r.path, strings.NewReader(r.body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	if r.browserIP != "" {
		req.Header.Set(common.BFFSecretHeader, testBFFSecret)
		req.Header.Set(common.BFFClientIPHeader, r.browserIP)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, resp.Header
}

func status(t *testing.T, app *fiber.App, r testRequest) int {
	t.Helper()
	code, _ := send(t, app, r)
	return code
}

func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: common.ErrorHandler})
	app.Use(common.ResolveClientIP(common.ClientIPConfig{BFFSharedSecret: testBFFSecret}))
	return app
}

func noContent(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }

// --- Per-IP budgets ---------------------------------------------------------

// newIPLimitedApp mounts two per-IP limiters of 2 requests each, wired the way
// registerModules wires credentials and refresh.
func newIPLimitedApp() *fiber.App {
	app := newTestApp()
	credentials := newRateLimiter(rateLimit{max: 2, window: time.Minute, key: clientIPKey, message: "credentials"})
	refresh := newRateLimiter(rateLimit{max: 2, window: time.Minute, key: clientIPKey, message: "refresh"})
	app.Post(loginPath, credentials, noContent)
	app.Post("/auth/refresh", refresh, noContent)
	return app
}

func TestRateLimit_RefreshDoesNotSpendTheLoginBudget(t *testing.T) {
	app := newIPLimitedApp()
	refresh := testRequest{path: "/auth/refresh"}

	for range 2 {
		if got := status(t, app, refresh); got != http.StatusNoContent {
			t.Fatalf("refresh status = %d, want 204", got)
		}
	}
	if got := status(t, app, refresh); got != http.StatusTooManyRequests {
		t.Fatalf("third refresh status = %d, want 429", got)
	}
	if got := status(t, app, testRequest{path: loginPath}); got != http.StatusNoContent {
		t.Fatalf("login after exhausting refresh: status = %d, want 204", got)
	}
}

func TestRateLimit_BrowsersBehindTheBFFHaveSeparateBuckets(t *testing.T) {
	app := newIPLimitedApp()
	login := func(ip string) testRequest { return testRequest{path: loginPath, browserIP: ip} }

	for range 2 {
		status(t, app, login("203.0.113.7"))
	}
	if got := status(t, app, login("203.0.113.7")); got != http.StatusTooManyRequests {
		t.Fatalf("third login from the same browser: status = %d, want 429", got)
	}
	// Same TCP peer (the BFF), different browser: its own budget.
	if got := status(t, app, login("203.0.113.8")); got != http.StatusNoContent {
		t.Fatalf("login from another browser: status = %d, want 204", got)
	}
}

// --- Failed logins per account ----------------------------------------------

// newLoginApp mounts the production failed-login limiter in front of a fake
// login that accepts only the password "right".
func newLoginApp() *fiber.App {
	app := newTestApp()
	app.Post(loginPath, newAPIRateLimits().failedLogin, func(c *fiber.Ctx) error {
		if strings.Contains(string(c.Body()), `"password":"right"`) {
			return c.SendStatus(fiber.StatusOK)
		}
		return fiber.NewError(fiber.StatusUnauthorized, "invalid credentials")
	})
	return app
}

func loginAttempt(email, password, fromIP string) testRequest {
	return testRequest{
		path:      loginPath,
		browserIP: fromIP,
		body:      `{"email":"` + email + `","password":"` + password + `"}`,
	}
}

// browserIPFor gives every attempt a different source, the distributed attack
// a per-IP limit can't see.
func browserIPFor(i int) string { return "198.51.100." + string(rune('0'+i%10)) }

func TestFailedLoginLimit_BlocksOneAccountAcrossManyIPs(t *testing.T) {
	app := newLoginApp()

	for i := range failedLoginRateLimitMax {
		got := status(t, app, loginAttempt("victim@example.com", "wrong", browserIPFor(i)))
		if got != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d: status = %d, want 401", i+1, got)
		}
	}
	code, header := send(t, app, loginAttempt("victim@example.com", "wrong", "203.0.113.99"))
	if code != http.StatusTooManyRequests {
		t.Fatalf("attempt past the limit from a fresh IP: status = %d, want 429", code)
	}
	if header.Get(fiber.HeaderRetryAfter) == "" {
		t.Error("429 without a Retry-After header")
	}
	// Even the right password is refused while throttled: otherwise a throttled
	// attacker would still learn when a guess is right.
	got := status(t, app, loginAttempt("victim@example.com", "right", "203.0.113.99"))
	if got != http.StatusTooManyRequests {
		t.Fatalf("right password while throttled: status = %d, want 429", got)
	}
}

func TestFailedLoginLimit_KeysByNormalizedEmail(t *testing.T) {
	app := newLoginApp()

	for range failedLoginRateLimitMax {
		status(t, app, loginAttempt("Victim@Example.com", "wrong", ""))
	}
	if got := status(t, app, loginAttempt("  victim@example.com ", "wrong", "")); got != http.StatusTooManyRequests {
		t.Fatalf("same email, different case/spacing: status = %d, want 429 (one bucket)", got)
	}
	if got := status(t, app, loginAttempt("someone-else@example.com", "wrong", "")); got != http.StatusUnauthorized {
		t.Fatalf("another account: status = %d, want 401 (its own bucket)", got)
	}
}

func TestFailedLoginLimit_SuccessfulLoginsAreFree(t *testing.T) {
	app := newLoginApp()

	for i := range failedLoginRateLimitMax * 2 {
		if got := status(t, app, loginAttempt("user@example.com", "right", "")); got != http.StatusOK {
			t.Fatalf("successful login %d: status = %d, want 200", i+1, got)
		}
	}
	// Successes spent nothing, so the whole failure budget is still there.
	for i := range failedLoginRateLimitMax {
		if got := status(t, app, loginAttempt("user@example.com", "wrong", "")); got != http.StatusUnauthorized {
			t.Fatalf("failed attempt %d after successes: status = %d, want 401", i+1, got)
		}
	}
}

func TestEmailKey_FallsBackToTheClientIP(t *testing.T) {
	tooLong := strings.Repeat("a", maxEmailKeyLength) + "@example.com"
	for name, body := range map[string]string{
		"no body":          "",
		"not JSON":         "email=victim@example.com",
		"no email":         `{"password":"x"}`,
		"blank email":      `{"email":"   "}`,
		"oversized email":  `{"email":"` + tooLong + `"}`,
		"email not string": `{"email":42}`,
	} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			var key string
			app.Post("/", func(c *fiber.Ctx) error {
				key = emailKey(c)
				return nil
			})
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			_ = resp.Body.Close()
			if !strings.HasPrefix(key, "ip:") {
				t.Errorf("key = %q, want the client-IP fallback", key)
			}
		})
	}
}

// --- Moxfield budgets per user ------------------------------------------------

// asUser stands in for auth.RequireAuth: it puts the X-Test-User header's value
// where RequireAuth puts the authenticated user ID.
func asUser(c *fiber.Ctx) error {
	if id := c.Get("X-Test-User"); id != "" {
		c.Locals(common.UserIDKey, id)
	}
	return c.Next()
}

func sendAs(t *testing.T, app *fiber.App, path, userID string) int {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, http.NoBody)
	req.Header.Set("X-Test-User", userID)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestMoxfieldDeckLimit_IsPerUserAndSharedByImportAndSync(t *testing.T) {
	limits := newAPIRateLimits()
	app := newTestApp()
	app.Use(asUser)
	app.Post("/decks/import/moxfield", limits.moxfieldDeck, noContent)
	app.Post("/sync/moxfield", limits.moxfieldDeck, noContent)

	for i := range moxfieldDeckRateLimitMax {
		path := "/decks/import/moxfield"
		if i%2 == 1 {
			path = "/sync/moxfield"
		}
		if got := sendAs(t, app, path, "alice"); got != http.StatusNoContent {
			t.Fatalf("request %d: status = %d, want 204", i+1, got)
		}
	}
	if got := sendAs(t, app, "/sync/moxfield", "alice"); got != http.StatusTooManyRequests {
		t.Fatalf("past the shared budget: status = %d, want 429", got)
	}
	if got := sendAs(t, app, "/decks/import/moxfield", "bob"); got != http.StatusNoContent {
		t.Fatalf("another user: status = %d, want 204 (own bucket)", got)
	}
}

func TestMoxfieldBulkLimit_CountsOnlyJobsStarted(t *testing.T) {
	limits := newAPIRateLimits()
	app := newTestApp()
	app.Use(asUser)
	app.Post("/moxfield-import", limits.moxfieldBulk, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusAccepted)
	})
	app.Post("/decks/resync-all", limits.moxfieldBulk, func(*fiber.Ctx) error {
		return fiber.NewError(fiber.StatusConflict, "a resync is already in progress")
	})

	// Refusals (409: a job already running) cost nothing...
	for range moxfieldBulkRateLimitMax * 2 {
		if got := sendAs(t, app, "/decks/resync-all", "alice"); got != http.StatusConflict {
			t.Fatalf("refused start: status = %d, want 409", got)
		}
	}
	// ...so the whole budget of started jobs is still there.
	for i := range moxfieldBulkRateLimitMax {
		if got := sendAs(t, app, "/moxfield-import", "alice"); got != http.StatusAccepted {
			t.Fatalf("job %d: status = %d, want 202", i+1, got)
		}
	}
	if got := sendAs(t, app, "/moxfield-import", "alice"); got != http.StatusTooManyRequests {
		t.Fatalf("job past the budget: status = %d, want 429", got)
	}
}

// --- outcomeLimiter internals ---------------------------------------------------

func TestOutcomeLimiter_WindowResetsAndExpiredBucketsAreSwept(t *testing.T) {
	l := newOutcomeLimiter(rateLimit{max: 2, window: time.Minute, onlyFailures: true})
	start := l.lastSweep

	l.add("a", start)
	l.add("a", start)
	if wait, blocked := l.blocked("a", start.Add(10*time.Second)); !blocked || wait != 50*time.Second {
		t.Fatalf("after 2 failures: blocked=%v wait=%v, want blocked with 50s left", blocked, wait)
	}
	if _, blocked := l.blocked("a", start.Add(time.Minute)); blocked {
		t.Fatal("still blocked once the window has passed")
	}

	// A write a window later sweeps every expired bucket, not just its own.
	l.add("b", start.Add(time.Minute))
	if _, ok := l.buckets["a"]; ok {
		t.Error("expired bucket for \"a\" was not swept")
	}
	if b := l.buckets["b"]; b == nil || b.count != 1 {
		t.Errorf("bucket for \"b\" = %+v, want a fresh one with count 1", b)
	}
}

func TestOutcomeLimiter_CountsDomainErrorsByTheirStatus(t *testing.T) {
	// The real login handler returns common.MapError(...) of a *DomainError; the
	// limiter must see it as the 401 ErrorHandler will send, not as a 200.
	app := newTestApp()
	limit := newRateLimiter(rateLimit{max: 1, window: time.Minute, key: clientIPKey, onlyFailures: true, message: "x"})
	app.Post("/login", limit, func(*fiber.Ctx) error { return common.Unauthorized("invalid credentials") })

	if got := status(t, app, testRequest{path: "/login"}); got != http.StatusUnauthorized {
		t.Fatalf("first attempt: status = %d, want 401", got)
	}
	if got := status(t, app, testRequest{path: "/login"}); got != http.StatusTooManyRequests {
		t.Fatalf("second attempt: status = %d, want 429 (the 401 was counted)", got)
	}
}

// --- Emails per address --------------------------------------------------------

func emailRequest(path, email string) testRequest {
	return testRequest{path: path, body: `{"email":"` + email + `","locale":"es"}`}
}

func TestAccountEmailLimit_IsPerAddressAndSharedByResetAndResend(t *testing.T) {
	limits := newAPIRateLimits()
	app := newTestApp()
	// Both endpoints answer 204 whether or not the account exists, and every
	// request counts, successes included.
	app.Post("/auth/forgot-password", limits.accountEmail, noContent)
	app.Post("/auth/resend-verification", limits.accountEmail, noContent)

	for i := range accountEmailRateLimitMax {
		path := "/auth/forgot-password"
		if i%2 == 1 {
			path = "/auth/resend-verification"
		}
		if got := status(t, app, emailRequest(path, "Inbox@Example.com")); got != http.StatusNoContent {
			t.Fatalf("email %d: status = %d, want 204", i+1, got)
		}
	}

	code, header := send(t, app, emailRequest("/auth/resend-verification", " inbox@example.com"))
	if code != http.StatusTooManyRequests {
		t.Fatalf("email past the shared budget, same address in another case: status = %d, want 429", code)
	}
	if header.Get(fiber.HeaderRetryAfter) == "" {
		t.Error("429 without a Retry-After header")
	}
	if got := status(t, app, emailRequest("/auth/forgot-password", "other@example.com")); got != http.StatusNoContent {
		t.Fatalf("another address: status = %d, want 204 (own bucket)", got)
	}
}
