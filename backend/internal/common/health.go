package common

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// healthCheckTimeout bounds how long the ping to Postgres can take before
// considering the health check itself stuck (e.g. exhausted pool).
const healthCheckTimeout = 2 * time.Second

// RegisterHealthRoute registers GET /health: process liveness plus a ping to
// Postgres, meant for external monitors (UptimeRobot). It lives outside
// /api/v1 and without auth.RequireAuth: an external monitor has no JWT, and
// it shouldn't compete with the rate limit of the auth endpoints.
func RegisterHealthRoute(app *fiber.App, db *DB) {
	app.Get("/health", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), healthCheckTimeout)
		defer cancel()

		if err := db.Pool.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "error",
				"db":     "unreachable",
			})
		}

		return c.JSON(fiber.Map{
			"status": "ok",
			"db":     "ok",
		})
	})
}
