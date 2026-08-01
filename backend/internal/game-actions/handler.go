package gameactions

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for game-actions.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the game-actions module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/games/:id/actions", h.CreateAction)
	router.Get("/games/:id/timeline", h.GetTimeline)
}

// CreateAction records a new action within a game.
func (h *Handler) CreateAction(c *fiber.Ctx) error {
	var req CreateActionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.RecordAction(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// GetTimeline returns the action history of a game.
func (h *Handler) GetTimeline(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetTimeline(c.Context(), c.Params("id"), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
