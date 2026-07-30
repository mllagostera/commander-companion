package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para usuarios.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// RegisterRoutes registra los endpoints públicos del módulo users/auth. rateLimit es
// el mismo middleware por IP que protege los demás endpoints públicos de auth (ver
// auth.Handler.RegisterPublicRoutes): register es público y hace un hash bcrypt por
// request, así que también hay que acotarlo.
func (h *Handler) RegisterRoutes(router fiber.Router, rateLimit fiber.Handler) {
	router.Post("/auth/register", rateLimit, h.Register)
	router.Post("/auth/verify-email", rateLimit, h.VerifyEmail)
	router.Post("/auth/resend-verification", rateLimit, h.ResendVerification)
}

// RegisterProtectedRoutes registra los endpoints de usuarios que requieren sesión.
func (h *Handler) RegisterProtectedRoutes(router fiber.Router) {
	router.Get("/users/search", h.SearchUsers)
	router.Patch("/users/:id", h.UpdateProfile)
	router.Post("/users/:id/password", h.ChangePassword)
}

// SearchUsers busca usuarios por username (parcial) o email (exacto) — para invitar a
// un playgroup sin conocer el UUID de la otra persona (ver docs/decisions y
// internal/playgroups.AddMember).
func (h *Handler) SearchUsers(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.SearchUsers(c.Context(), userID, c.Query("q"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// Register maneja la petición de registro.
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Validación muy básica
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing required fields")
	}

	res, err := h.svc.RegisterUser(c.Context(), req)
	if err != nil {
		return common.MapError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

// VerifyEmail confirma la cuenta a partir del token mandado por mail. Es POST (no GET)
// a propósito: el link del mail lleva a una página del cliente web, que es quien hace
// este POST con el token en el body — así nunca queda en una query string que el
// logger de acceso del servidor loguearía en texto plano.
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	var req VerifyEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.Token == "" {
		return fiber.NewError(fiber.StatusBadRequest, "token is required")
	}

	if err := h.svc.VerifyEmail(c.Context(), req.Token); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ResendVerification manda un nuevo mail de verificación si corresponde. Siempre
// responde 204 (ver users.Service.ResendVerification: no revela si el email existe).
func (h *Handler) ResendVerification(c *fiber.Ctx) error {
	var req ResendVerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email is required")
	}

	if err := h.svc.ResendVerification(c.Context(), req.Email); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateProfile actualiza el perfil propio: username (opcional) y moxfield_username.
// Acotado a que :id sea el propio usuario autenticado — 404 si no, mismo criterio de
// "no revelar" que decks/playgroups, para no confirmar la existencia de otros IDs.
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if c.Params("id") != userID {
		return common.MapError(ErrUserNotFound)
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	var res *UserResponse
	var err error

	if req.Username != nil {
		if res, err = h.svc.UpdateUsername(c.Context(), userID, *req.Username); err != nil {
			return common.MapError(err)
		}
	}

	if req.MoxfieldUsername != nil {
		if res, err = h.svc.UpdateMoxfieldUsername(c.Context(), userID, *req.MoxfieldUsername); err != nil {
			return common.MapError(err)
		}
	}

	if res == nil {
		// Ningún campo mandado: no hay nada que actualizar, pero igual devolvemos el
		// estado actual en vez de un error — un PATCH vacío no es inválido, es un no-op.
		if res, err = h.svc.GetUser(c.Context(), userID); err != nil {
			return common.MapError(err)
		}
	}

	return c.JSON(res)
}

// ChangePassword cambia el password propio, tras validar el actual. Acotado a que
// :id sea el propio usuario autenticado, mismo criterio que UpdateProfile.
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if c.Params("id") != userID {
		return common.MapError(ErrUserNotFound)
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing required fields")
	}

	if err := h.svc.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		return common.MapError(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
