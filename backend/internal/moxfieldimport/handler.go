package moxfieldimport

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contiene las dependencias del transporte HTTP para moxfieldimport.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo moxfieldimport.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/moxfield-import", h.StartImport)
	router.Get("/moxfield-import/:jobId", h.GetJobStatus)
}

// StartImport dispara un import masivo en background para el usuario autenticado.
// Responde 202: el job queda pending/in_progress, el progreso se consulta con
// GetJobStatus.
func (h *Handler) StartImport(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.StartImport(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusAccepted).JSON(res)
}

// GetJobStatus devuelve el estado de un job de import, acotado al usuario autenticado.
func (h *Handler) GetJobStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetJobStatus(c.Context(), userID, c.Params("jobId"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
