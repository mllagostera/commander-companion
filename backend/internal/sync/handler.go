package sync

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
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

// SyncMoxfield re-sincroniza un deck del usuario autenticado con Moxfield. Responde
// 200 y no 202 porque el sync ya se aplicó dentro del request; ver el doc del paquete.
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

// GetStatus devuelve el estado de sincronización de un deck del usuario
// autenticado, identificado por el query param `moxfield_id`.
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetSyncStatus(c.Context(), userID, c.Query("moxfield_id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
