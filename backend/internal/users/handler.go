package users

import (
	"github.com/gofiber/fiber/v2"
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

// RegisterRoutes registra todos los endpoints del módulo users/auth.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/auth/register", h.Register)
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
		return err // Será manejado por el ErrorHandler global
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}
