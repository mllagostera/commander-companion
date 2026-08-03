package moxfieldimport

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for moxfieldimport.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the moxfieldimport module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/moxfield-import", h.StartImport)
	router.Get("/moxfield-import", h.GetLatestJobStatus)
	router.Get("/moxfield-import/:jobId", h.GetJobStatus)
}

// StartImport triggers a background bulk import for the authenticated user.
// Responds 202: the job is left pending/in_progress, progress is queried with
// GetJobStatus.
func (h *Handler) StartImport(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.StartImport(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// GetJobStatus returns the status of an import job, restricted to the authenticated user.
func (h *Handler) GetJobStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetJobStatus(c.Context(), userID, c.Params("jobId"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetLatestJobStatus returns the authenticated user's most recently started
// import job, whatever its status. Lets the frontend resume tracking a job
// (e.g. after navigating away and back to the settings page) without having
// kept the job ID around. 404 if the user never started an import.
func (h *Handler) GetLatestJobStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetLatestJobStatus(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
