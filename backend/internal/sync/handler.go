package sync

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contains the HTTP transport dependencies for sync.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all endpoints of the sync module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/sync/moxfield", h.SyncMoxfield)
	router.Get("/sync/status", h.GetStatus)
}

// SyncMoxfield re-syncs a deck of the authenticated user with Moxfield. Responds
// 200 and not 202 because the sync was already applied within the request; see the package doc.
func (h *Handler) SyncMoxfield(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.TriggerSync(c.Context(), userID, req.MoxfieldID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetStatus returns the sync state of a deck belonging to the authenticated
// user, identified by the `moxfield_id` query param.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetSyncStatus(c.Context(), userID, c.Query("moxfield_id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
