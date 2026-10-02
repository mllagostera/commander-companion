package users

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// touchRecorder is a Service whose only implemented method is TouchLastSeen
// (the rest panic via the nil embedded interface): it's all ActivityTracker calls.
type touchRecorder struct {
	Service
	touched chan string
}

func (r *touchRecorder) TouchLastSeen(_ context.Context, id string) error {
	r.touched <- id
	return nil
}

// newTrackerApp mounts the tracker behind a stand-in for auth.RequireAuth that
// takes the user ID from a header, and returns a clock the test can move.
func newTrackerApp(t *testing.T) (*fiber.App, *touchRecorder, *time.Time) {
	t.Helper()
	rec := &touchRecorder{touched: make(chan string, 10)}
	tracker := NewActivityTracker(rec)
	clock := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return clock }

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if id := c.Get("X-Test-User"); id != "" {
			c.Locals(common.UserIDKey, id)
		}
		return c.Next()
	}, tracker.Middleware())
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	return app, rec, &clock
}

func doRequest(t *testing.T, app *fiber.App, userID string) {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	if userID != "" {
		req.Header.Set("X-Test-User", userID)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	_ = resp.Body.Close()
}

// expectTouches waits for exactly want writes, in any order (each one runs in its
// own goroutine), and then makes sure no extra one shows up.
func expectTouches(t *testing.T, rec *touchRecorder, want ...string) {
	t.Helper()
	pending := make(map[string]int, len(want))
	for _, id := range want {
		pending[id]++
	}
	for range want {
		select {
		case got := <-rec.touched:
			if pending[got] == 0 {
				t.Fatalf("unexpected TouchLastSeen(%q), want %v", got, want)
			}
			pending[got]--
		case <-time.After(time.Second):
			t.Fatalf("TouchLastSeen was not called for all of %v", want)
		}
	}
	select {
	case got := <-rec.touched:
		t.Fatalf("unexpected extra TouchLastSeen(%q)", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestActivityTracker_ThrottlesWritesPerUser(t *testing.T) {
	app, rec, clock := newTrackerApp(t)

	doRequest(t, app, "user-a")
	doRequest(t, app, "user-a")
	doRequest(t, app, "user-b")
	expectTouches(t, rec, "user-a", "user-b")

	*clock = clock.Add(30 * time.Second)
	doRequest(t, app, "user-a")
	expectTouches(t, rec)

	*clock = clock.Add(31 * time.Second)
	doRequest(t, app, "user-a")
	expectTouches(t, rec, "user-a")
}

func TestActivityTracker_IgnoresUnauthenticatedRequests(t *testing.T) {
	app, rec, _ := newTrackerApp(t)

	doRequest(t, app, "")
	expectTouches(t, rec)
}
