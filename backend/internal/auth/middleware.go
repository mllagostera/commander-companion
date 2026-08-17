package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const bearerPrefix = "Bearer "

// RequireAuth validates the Bearer token from the Authorization header and stores the
// authenticated user's ID in the request context under common.UserIDKey.
func RequireAuth(secret []byte) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(header, bearerPrefix) {
			return fiber.NewError(fiber.StatusUnauthorized, "Missing or invalid Authorization header")
		}

		userID, err := parseAccessToken(secret, strings.TrimPrefix(header, bearerPrefix))
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid or expired token")
		}

		c.Locals(common.UserIDKey, userID)
		return c.Next()
	}
}

// RequireAdmin builds on RequireAuth's already-authenticated user (must be chained
// after it, so common.UserIDKey is already set) and additionally requires
// is_admin = true. It's looked up fresh from the DB on every request rather than
// trusted from a JWT claim, so demoting an admin takes effect immediately instead of
// leaving a stale-privilege window until their token expires — see ADR-0018.
func RequireAdmin(usersSvc users.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, _ := c.Locals(common.UserIDKey).(string)

		isAdmin, err := usersSvc.IsAdmin(c.Context(), userID)
		if err != nil {
			return common.MapError(err)
		}
		if !isAdmin {
			return fiber.NewError(fiber.StatusForbidden, "admin access required")
		}

		return c.Next()
	}
}
