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

// RegisterRoutes registers all endpoints of the statistics module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/statistics/user", h.GetUserStats)
	router.Get("/statistics/deck/:id", h.GetDeckStats)
	router.Get("/statistics/playgroup/:id", h.GetPlaygroupStats)
}

// GetUserStats returns the authenticated user's statistics.
func (h *Handler) GetUserStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetUserStats(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetDeckStats returns the statistics for a deck belonging to the authenticated user.
func (h *Handler) GetDeckStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetDeckStats(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetPlaygroupStats returns the aggregated statistics for a playgroup.
func (h *Handler) GetPlaygroupStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetPlaygroupStats(c.Context(), c.Params("id"), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
