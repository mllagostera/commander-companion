package common

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// healthCheckTimeout acota cuánto puede tardar el ping a Postgres antes de
// considerar el health check en sí mismo colgado (p. ej. pool exhausto).
const healthCheckTimeout = 2 * time.Second

// RegisterHealthRoute registra GET /health: liveness del proceso más un ping a
// Postgres, pensado para monitores externos (UptimeRobot). Vive fuera de
// /api/v1 y sin auth.RequireAuth: un monitor externo no tiene JWT, y no debe
// competir con el rate limit de los endpoints de auth.
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
