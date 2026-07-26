package statistics

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para statistics.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo statistics.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/statistics/user", h.GetUserStats)
	router.Get("/statistics/deck/:id", h.GetDeckStats)
	router.Get("/statistics/playgroup/:id", h.GetPlaygroupStats)
}

// GetUserStats devuelve las estadísticas del usuario autenticado.
func (h *Handler) GetUserStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetUserStats(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// GetDeckStats devuelve las estadísticas de un deck.
func (h *Handler) GetDeckStats(c *fiber.Ctx) error {
	id := c.Params("id")
	res, err := h.svc.GetDeckStats(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// GetPlaygroupStats devuelve las estadísticas de un grupo de juego.
func (h *Handler) GetPlaygroupStats(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"message": "Not implemented yet"})
}
