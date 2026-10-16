package main

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// The API's rate limits. docs/api/openapi.yaml (info.description) lists them
// for clients; ADR-0024 and ADR-0025 explain the keys.
const (
	// Public endpoints that take a credential or create an account (login,
	// google, register, verify-email, password reset, ...). 20 a minute per
	// client IP leaves slack for a human mistyping a password but cuts
	// credential stuffing from one source short.
	credentialsRateLimitMax    = 20
	credentialsRateLimitWindow = time.Minute

	// Failed POST /auth/login attempts per account (the submitted email). The
	// per-IP limit above can't see an attacker spread over many IPs; this can.
	// See ADR-0025 for the lockout trade-off it accepts.
	failedLoginRateLimitMax    = 10
	failedLoginRateLimitWindow = 15 * time.Minute

	// POST /auth/refresh, in its own bucket so automatic session renewal (several
	// devices behind one NAT) can't use up the login budget, nor the other way
	// round. Looser because guessing a refresh token is not a realistic attack
	// (256 random bits, see auth.newRefreshTokenPlain): it only bounds load.
	refreshRateLimitMax    = 60
	refreshRateLimitWindow = time.Minute

	// GET /users/search does an exact-email lookup; without a bound an account
	// could check an arbitrary list of addresses (2026-08-01 security audit).
	searchRateLimitMax    = 20
	searchRateLimitWindow = time.Minute

	// One-deck Moxfield calls per user: import and sync share it. Every request
	// counts, since even a 404 already cost a call to Moxfield.
	moxfieldDeckRateLimitMax    = 20
	moxfieldDeckRateLimitWindow = 10 * time.Minute

	// Moxfield bulk jobs (import, resync-all) started per user. Each makes one
	// Moxfield call per deck; this bounds back-to-back jobs, which the
	// one-job-at-a-time guard alone allows indefinitely.
	moxfieldBulkRateLimitMax    = 5
	moxfieldBulkRateLimitWindow = time.Hour

	// maxEmailKeyLength is the longest email used as a rate-limit key: RFC 5321's
	// limit for a forward path. Anything longer can't be a real address and must
	// not mint a counter of its own.
	maxEmailKeyLength = 254
)

// rateLimit describes one rate-limited budget.
type rateLimit struct {
	max    int
	window time.Duration
	// key picks the bucket a request counts against.
	key func(*fiber.Ctx) string
	// onlyFailures counts only responses >= 400 (a successful request is given
	// back); onlySuccesses counts only responses < 400.
	onlyFailures  bool
	onlySuccesses bool
	// message is the 429's error message.
	message string
}

// newRateLimiter builds the middleware for rl. Each call owns its own counters:
// two routes share a budget only by sharing the returned handler.
//
// The counters live in the process's memory: enough for a single instance (the
// current deployment); with several replicas they would need a shared store.
func newRateLimiter(rl rateLimit) fiber.Handler {
	if rl.onlyFailures || rl.onlySuccesses {
		return newOutcomeLimiter(rl).handle
	}
	return limiter.New(limiter.Config{
		Max:          rl.max,
		Expiration:   rl.window,
		KeyGenerator: rl.key,
		LimitReached: func(_ *fiber.Ctx) error {
			// Returning the error (instead of writing the response here) makes it
			// go through common.ErrorHandler, so a 429 has the same body as any
			// other API error. The limiter has already set Retry-After.
			return tooManyRequests(rl)
		},
	})
}

func tooManyRequests(rl rateLimit) error {
	return fiber.NewError(fiber.StatusTooManyRequests, rl.message)
}

// outcomeLimiter is a fixed-window limiter that counts a request only once its
// outcome is known, for the budgets that spend only on failures or only on
// successes.
//
// Fiber's limiter has options for this (SkipSuccessfulRequests and
// SkipFailedRequests), but they read the response status right after the
// handler returns, before common.ErrorHandler has turned a returned error into
// a response. Every handler here returns its errors, so to Fiber's limiter a
// failed login looks like a 200 and would never be counted. This reads the
// status the error will become instead.
type outcomeLimiter struct {
	rl rateLimit

	mu        sync.Mutex
	buckets   map[string]*outcomeBucket
	lastSweep time.Time
}

type outcomeBucket struct {
	count   int
	resetAt time.Time
}

func newOutcomeLimiter(rl rateLimit) *outcomeLimiter {
	return &outcomeLimiter{rl: rl, buckets: map[string]*outcomeBucket{}, lastSweep: time.Now()}
}

func (l *outcomeLimiter) handle(c *fiber.Ctx) error {
	key := l.rl.key(c)

	if wait, blocked := l.blocked(key, time.Now()); blocked {
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(wait.Seconds()))))
		return tooManyRequests(l.rl)
	}

	err := c.Next()

	failed := responseStatus(c, err) >= fiber.StatusBadRequest
	if (l.rl.onlyFailures && failed) || (l.rl.onlySuccesses && !failed) {
		l.add(key, time.Now())
	}
	return err
}

// blocked reports whether key has used up its budget in the current window,
// and how long until that window resets.
func (l *outcomeLimiter) blocked(key string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || !now.Before(b.resetAt) {
		return 0, false
	}
	return b.resetAt.Sub(now), b.count >= l.rl.max
}

func (l *outcomeLimiter) add(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)
	b, ok := l.buckets[key]
	if !ok || !now.Before(b.resetAt) {
		b = &outcomeBucket{resetAt: now.Add(l.rl.window)}
		l.buckets[key] = b
	}
	b.count++
}

// sweep drops expired buckets at most once per window, so keys that are never
// seen again (one-off emails in a guessing run) don't accumulate. Callers hold mu.
func (l *outcomeLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.rl.window {
		return
	}
	for key, b := range l.buckets {
		if !now.Before(b.resetAt) {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}

// responseStatus is the status the response to this request ends up with:
// the one common.ErrorHandler will give a returned error, else the one
// already set.
func responseStatus(c *fiber.Ctx, err error) int {
	if err == nil {
		return c.Response().StatusCode()
	}
	var fiberErr *fiber.Error
	if errors.As(common.MapError(err), &fiberErr) {
		return fiberErr.Code
	}
	return fiber.StatusInternalServerError
}

// clientIPKey keys by the caller's IP. It is common.ClientIP, not
// fiber.Ctx.IP(): behind the web BFF or an edge proxy the TCP peer is the same
// address for every user (see ADR-0024).
func clientIPKey(c *fiber.Ctx) string {
	return "ip:" + common.ClientIP(c)
}

// loginAccountKey keys a login attempt by the email it targets, trimmed and
// lowercased, whether or not that account exists. A body without a usable
// email falls back to the client IP: the request fails validation anyway, and
// an arbitrary string must not mint an unbounded number of counters.
func loginAccountKey(c *fiber.Ctx) string {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(c.Body(), &body); err == nil {
		email := strings.ToLower(strings.TrimSpace(body.Email))
		if email != "" && len(email) <= maxEmailKeyLength {
			return "email:" + email
		}
	}
	return clientIPKey(c)
}

// userKey keys by the authenticated user. Only valid behind auth.RequireAuth,
// which sets the user ID; without one it falls back to the client IP rather
// than putting every such request in a single shared bucket.
func userKey(c *fiber.Ctx) string {
	if userID, ok := c.Locals(common.UserIDKey).(string); ok && userID != "" {
		return "user:" + userID
	}
	return clientIPKey(c)
}

// apiRateLimits are the rate-limiting middlewares, built once so that the
// routes sharing a budget share the same handler.
type apiRateLimits struct {
	credentials  fiber.Handler
	failedLogin  fiber.Handler
	refresh      fiber.Handler
	search       fiber.Handler
	moxfieldDeck fiber.Handler
	moxfieldBulk fiber.Handler
}

func newAPIRateLimits() apiRateLimits {
	return apiRateLimits{
		credentials: newRateLimiter(rateLimit{
			max: credentialsRateLimitMax, window: credentialsRateLimitWindow, key: clientIPKey,
			message: "too many authentication requests, try again later",
		}),
		failedLogin: newRateLimiter(rateLimit{
			max: failedLoginRateLimitMax, window: failedLoginRateLimitWindow, key: loginAccountKey,
			onlyFailures: true,
			message:      "too many failed login attempts for this account, try again later",
		}),
		refresh: newRateLimiter(rateLimit{
			max: refreshRateLimitMax, window: refreshRateLimitWindow, key: clientIPKey,
			message: "too many session refresh requests, try again later",
		}),
		search: newRateLimiter(rateLimit{
			max: searchRateLimitMax, window: searchRateLimitWindow, key: clientIPKey,
			message: "too many search requests, try again later",
		}),
		moxfieldDeck: newRateLimiter(rateLimit{
			max: moxfieldDeckRateLimitMax, window: moxfieldDeckRateLimitWindow, key: userKey,
			message: "too many Moxfield requests, try again later",
		}),
		moxfieldBulk: newRateLimiter(rateLimit{
			max: moxfieldBulkRateLimitMax, window: moxfieldBulkRateLimitWindow, key: userKey,
			onlySuccesses: true,
			message:       "too many Moxfield bulk jobs started, try again later",
		}),
	}
}
