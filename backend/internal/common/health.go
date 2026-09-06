package common

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// healthCheckTimeout bounds how long the ping to Postgres can take before
// considering the health check itself stuck (e.g. exhausted pool).
const healthCheckTimeout = 2 * time.Second

// BuildInfo is what GET /health reports about the running process itself, as
// opposed to its dependencies: which source revision it was built from and
// since when it has been up. Resolving Commit is config's job (see
// config.Config.GitCommit and ADR-0020); this package only serves it.
type BuildInfo struct {
	Commit    string
	StartedAt time.Time
}

// RegisterHealthRoute registers GET /health: process liveness, a ping to
// Postgres, and the build marker, meant for external monitors (UptimeRobot)
// and for anything that has to tell a finished deploy from one still in
// flight. It lives outside /api/v1 and without auth.RequireAuth: an external
// monitor has no JWT, and it shouldn't compete with the rate limit of the auth
// endpoints.
//
// The build fields are returned on the 503 branch too: which build is serving
// does not depend on the database being reachable, and a caller waiting for a
// deploy still needs the answer when Postgres is down.
func RegisterHealthRoute(app *fiber.App, db *DB, build BuildInfo) {
	startedAt := build.StartedAt.UTC().Format(time.RFC3339)

	app.Get("/health", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), healthCheckTimeout)
		defer cancel()

		if err := db.Pool.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":     "error",
				"db":         "unreachable",
				"commit":     build.Commit,
				"started_at": startedAt,
			})
		}

		return c.JSON(fiber.Map{
			"status":     "ok",
			"db":         "ok",
			"commit":     build.Commit,
			"started_at": startedAt,
		})
	})
}
