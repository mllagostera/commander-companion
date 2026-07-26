package decks

import (
	"github.com/gofiber/fiber/v2"
)

// Handler contiene las dependencias del transporte HTTP para decks.
type Handler struct {
	svc Service
}

// NewHandler inicializa y retorna un nuevo Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registra todos los endpoints del módulo decks.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/decks", h.ListDecks)
	router.Post("/decks", h.CreateDeck)
	router.Get("/decks/:id", h.GetDeck)
	router.Delete("/decks/:id", h.DeleteDeck)
}

// CreateDeck maneja la creación manual de un deck.
func (h *Handler) CreateDeck(c *fiber.Ctx) error {
	var req CreateDeckRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	dummyUserID := "dummy-user-id"
	res, err := h.svc.CreateDeck(c.Context(), dummyUserID, req)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListDecks devuelve los decks del usuario autenticado.
func (h *Handler) ListDecks(c *fiber.Ctx) error {
	dummyUserID := "dummy-user-id"
	res, err := h.svc.ListDecks(c.Context(), dummyUserID)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// GetDeck devuelve el detalle de un deck.
func (h *Handler) GetDeck(c *fiber.Ctx) error {
	res, err := h.svc.GetDeck(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// DeleteDeck elimina un deck.
func (h *Handler) DeleteDeck(c *fiber.Ctx) error {
	if err := h.svc.DeleteDeck(c.Context(), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
