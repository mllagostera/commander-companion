package users

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	// activityWriteInterval is the minimum gap between two last_seen_at writes for
	// the same user. Well below the admin overview's online window (see
	// GetAdminOverviewStats), so throttling never makes an active user look offline.
	activityWriteInterval = time.Minute
	// activityWriteTimeout bounds the background UPDATE, which runs detached from
	// the request's context (that one is done as soon as the response is sent).
	activityWriteTimeout = 5 * time.Second
)

// ActivityTracker keeps users.last_seen_at up to date from authenticated
// requests, which is what the admin overview's online_users counts.
//
// It writes at most once per activityWriteInterval per user, tracked in memory:
// with several API instances each one throttles on its own, which only means a
// few extra writes, never a missed one. The write happens in a goroutine so it
// adds no latency to the request, and a failed write is only logged — presence
// is best-effort and must never fail a real request.
type ActivityTracker struct {
	svc Service
	now func() time.Time

	mu        sync.Mutex
	lastWrite map[string]time.Time
	lastSweep time.Time
}

// NewActivityTracker builds a tracker that writes through svc.TouchLastSeen.
func NewActivityTracker(svc Service) *ActivityTracker {
	return &ActivityTracker{
		svc:       svc,
		now:       time.Now,
		lastWrite: make(map[string]time.Time),
	}
}

// Middleware records the request's user as seen. It must be chained after
// auth.RequireAuth, which sets common.UserIDKey; without it, it does nothing.
func (t *ActivityTracker) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if userID, ok := c.Locals(common.UserIDKey).(string); ok && userID != "" {
			// Cloned because it outlives the request (map key, goroutine) and Fiber
			// may hand out strings backed by buffers it reuses for the next request.
			userID = strings.Clone(userID)
			if t.shouldWrite(userID) {
				go t.write(userID)
			}
		}
		return c.Next()
	}
}

// shouldWrite reports whether userID's last write is old enough to write again,
// and if so claims the slot (records now) so concurrent requests don't all write.
func (t *ActivityTracker) shouldWrite(userID string) bool {
	now := t.now()

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.lastWrite[userID]; ok && now.Sub(last) < activityWriteInterval {
		return false
	}
	t.lastWrite[userID] = now
	t.sweep(now)
	return true
}

// sweep drops entries that no longer throttle anything, so the map holds only
// users seen in the last interval instead of every user since startup. Runs at
// most once per interval; must be called with mu held.
func (t *ActivityTracker) sweep(now time.Time) {
	if now.Sub(t.lastSweep) < activityWriteInterval {
		return
	}
	t.lastSweep = now
	for id, last := range t.lastWrite {
		if now.Sub(last) >= activityWriteInterval {
			delete(t.lastWrite, id)
		}
	}
}

func (t *ActivityTracker) write(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), activityWriteTimeout)
	defer cancel()
	if err := t.svc.TouchLastSeen(ctx, userID); err != nil {
		log.Printf("users: recording activity for %s: %v", userID, err)
	}
}
