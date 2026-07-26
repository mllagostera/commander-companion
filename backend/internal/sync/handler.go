package sync

import (
	"github.com/gofiber/fiber/v2"
)

// Handler contiene las dependencias del transporte HTTP para sync.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo sync.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/sync/moxfield", h.SyncMoxfield)
	router.Get("/sync/status", h.GetStatus)
}

// SyncMoxfield inicia una sincronización de deck con Moxfield.
func (h *Handler) SyncMoxfield(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.TriggerSync(c.Context(), req.MoxfieldID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// GetStatus devuelve el estado de un job de sincronización.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	jobID := c.Query("job_id")
	res, err := h.svc.GetSyncStatus(c.Context(), jobID)
	if err != nil {
		return err
	}
	return c.JSON(res)
}
