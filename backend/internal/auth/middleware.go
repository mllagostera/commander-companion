package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
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
