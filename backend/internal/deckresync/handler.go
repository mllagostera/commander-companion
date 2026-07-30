package deckresync

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para deckresync.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo deckresync.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/decks/resync-all", h.StartResyncAll)
	router.Get("/decks/resync-all/:jobId", h.GetJobStatus)
}

// StartResyncAll dispara en background la resincronización de todos los decks con
// moxfield_id del usuario autenticado. Responde 202: el job queda in_progress, el
// progreso se consulta con GetJobStatus.
func (h *Handler) StartResyncAll(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.StartResyncAll(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// GetJobStatus devuelve el estado de un job de resync, acotado al usuario autenticado.
func (h *Handler) GetJobStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetJobStatus(c.Context(), userID, c.Params("jobId"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
