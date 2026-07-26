package gameactions

import (
	"github.com/gofiber/fiber/v2"
)

// Handler contiene las dependencias del transporte HTTP para game-actions.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo game-actions.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/games/:id/actions", h.CreateAction)
	router.Get("/games/:id/timeline", h.GetTimeline)
}

// CreateAction registra una nueva acción dentro de una partida.
func (h *Handler) CreateAction(c *fiber.Ctx) error {
	var req CreateActionRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.RecordAction(c.Context(), c.Params("id"), req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// GetTimeline devuelve el historial de acciones de una partida.
func (h *Handler) GetTimeline(c *fiber.Ctx) error {
	res, err := h.svc.GetTimeline(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(res)
}
