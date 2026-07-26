package auth

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/users"
)

// Handler contiene las dependencias del transporte HTTP para auth.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterPublicRoutes registra los endpoints de auth que no requieren un access token vigente.
func (h *Handler) RegisterPublicRoutes(router fiber.Router) {
	router.Post("/auth/login", h.Login)
	router.Post("/auth/google", h.GoogleLogin)
	router.Post("/auth/refresh", h.Refresh)
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
		return mapError(err)
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
		return mapError(err)
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
		return mapError(err)
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
		return mapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Me devuelve el perfil del usuario autenticado.
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)

	res, err := h.svc.Me(c.Context(), userID)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(res)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, users.ErrInvalidCredentials), errors.Is(err, users.ErrGoogleOnlyAccount):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, users.ErrUserNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, users.ErrEmailNotVerified):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInvalidToken):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrGoogleAuthNotConfigured):
		return fiber.NewError(fiber.StatusNotImplemented, err.Error())
	default:
		return err
	}
}
