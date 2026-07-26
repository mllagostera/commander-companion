package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const bearerPrefix = "Bearer "

// RequireAuth valida el Bearer token del header Authorization y guarda el ID del
// usuario autenticado en el contexto de la request bajo common.UserIDKey.
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
