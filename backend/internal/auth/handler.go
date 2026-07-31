package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contains the HTTP transport dependencies for auth.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes registers the auth endpoints that don't require a valid access
// token. rateLimit applies only to the ones that issue tokens (login, google,
// refresh): they're the only ones without a JWT in front and the most expensive to
// process (bcrypt, verifying the id_token against Google), so they're the natural
// target for brute force. Logout is left unlimited: it's idempotent, cheap, and doesn't leak information.
func (h *Handler) RegisterPublicRoutes(router fiber.Router, rateLimit fiber.Handler) {
	router.Post("/auth/login", rateLimit, h.Login)
	router.Post("/auth/google", rateLimit, h.GoogleLogin)
	router.Post("/auth/refresh", rateLimit, h.Refresh)
	router.Post("/auth/logout", h.Logout)
}

// RegisterProtectedRoutes registers the auth endpoints that require a valid access token.
func (h *Handler) RegisterProtectedRoutes(router fiber.Router) {
	router.Get("/auth/me", h.Me)
}

// Login authenticates a user with email and password.
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GoogleLogin authenticates (or registers) a user from a Google id_token.
func (h *Handler) GoogleLogin(c *fiber.Ctx) error {
	var req GoogleLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.IDToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "id_token is required")
	}

	res, err := h.svc.GoogleLogin(c.Context(), req.IDToken)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// Refresh renews the access token from a valid refresh token.
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// Logout invalidates the given refresh token.
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req LogoutRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.svc.Logout(c.Context(), req.RefreshToken); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Me returns the authenticated user's profile.
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)

	res, err := h.svc.Me(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
