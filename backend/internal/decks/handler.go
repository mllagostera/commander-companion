package decks

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
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
	router.Post("/decks/import/moxfield", h.ImportMoxfield)
	router.Get("/decks/:id", h.GetDeck)
	router.Delete("/decks/:id", h.DeleteDeck)
}

// CreateDeck maneja la creación manual de un deck.
func (h *Handler) CreateDeck(c *fiber.Ctx) error {
	var req CreateDeckRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.CreateDeck(c.Context(), userID, req)
	if err != nil {
		return mapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ImportMoxfield importa un deck público de Moxfield como un deck nuevo del usuario autenticado.
func (h *Handler) ImportMoxfield(c *fiber.Ctx) error {
	var req ImportMoxfieldRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ImportFromMoxfield(c.Context(), userID, req)
	if err != nil {
		return mapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListDecks devuelve los decks del usuario autenticado.
func (h *Handler) ListDecks(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListDecks(c.Context(), userID)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(res)
}

// GetDeck devuelve el detalle de un deck del usuario autenticado.
func (h *Handler) GetDeck(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetDeck(c.Context(), userID, c.Params("id"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(res)
}

// DeleteDeck elimina un deck del usuario autenticado.
func (h *Handler) DeleteDeck(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.DeleteDeck(c.Context(), userID, c.Params("id")); err != nil {
		return mapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func mapError(err error) error {
	if errors.Is(err, ErrDeckNotFound) {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}
	return err
}
