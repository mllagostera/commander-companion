package deckresync

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for deckresync.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the deckresync module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/decks/resync-all", h.StartResyncAll)
	router.Get("/decks/resync-all/:jobId", h.GetJobStatus)
}

// StartResyncAll triggers in the background the resynchronization of all decks
// with moxfield_id belonging to the authenticated user. Responds 202: the job is
// left in_progress, progress is queried with GetJobStatus.
func (h *Handler) StartResyncAll(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.StartResyncAll(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// GetJobStatus returns the status of a resync job, restricted to the authenticated user.
func (h *Handler) GetJobStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetJobStatus(c.Context(), userID, c.Params("jobId"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
