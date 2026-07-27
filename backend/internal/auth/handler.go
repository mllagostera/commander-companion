package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para auth.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes registra los endpoints de auth que no requieren un access
// token vigente. rateLimit se aplica solo a los que emiten tokens (login, google,
// refresh): son los únicos sin JWT por delante y los más caros de procesar (bcrypt,
// verificación del id_token contra Google), así que son el blanco natural de fuerza
// bruta. Logout queda sin límite: es idempotente, barato y no filtra información.
func (h *Handler) RegisterPublicRoutes(router fiber.Router, rateLimit fiber.Handler) {
	router.Post("/auth/login", rateLimit, h.Login)
	router.Post("/auth/google", rateLimit, h.GoogleLogin)
	router.Post("/auth/refresh", rateLimit, h.Refresh)
	router.Post("/auth/logout", h.Logout)
}

// RegisterProtectedRoutes registra los endpoints de auth que requieren un access token vigente.
func (h *Handler) RegisterProtectedRoutes(router fiber.Router) {
	router.Get("/auth/me", h.Me)
}

// Login autentica a un usuario con email y password.
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

// GoogleLogin autentica (o registra) a un usuario a partir de un id_token de Google.
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

// Refresh renueva el access token a partir de un refresh token vigente.
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

// Logout invalida el refresh token indicado.
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

// Me devuelve el perfil del usuario autenticado.
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)

	res, err := h.svc.Me(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
