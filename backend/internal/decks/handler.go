package decks

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler contains the HTTP transport dependencies for decks.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all endpoints of the decks module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/decks", h.ListDecks)
	router.Post("/decks", h.CreateDeck)
	router.Post("/decks/import/moxfield", h.ImportMoxfield)
	router.Get("/decks/:id", h.GetDeck)
	router.Delete("/decks/:id", h.DeleteDeck)
}

// CreateDeck handles the manual creation of a deck.
func (h *Handler) CreateDeck(c *fiber.Ctx) error {
	var req CreateDeckRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.CreateDeck(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ImportMoxfield imports a public Moxfield deck as a new deck for the authenticated user.
func (h *Handler) ImportMoxfield(c *fiber.Ctx) error {
	var req ImportMoxfieldRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ImportFromMoxfield(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListDecks returns a page of the authenticated user's decks. Accepts the
// `cursor` and `limit` query params (see internal/common/pagination.go).
func (h *Handler) ListDecks(c *fiber.Ctx) error {
	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListDecks(c.Context(), userID, page)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetDeck returns the detail of a deck belonging to the authenticated user.
func (h *Handler) GetDeck(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetDeck(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// DeleteDeck deletes a deck belonging to the authenticated user.
func (h *Handler) DeleteDeck(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.DeleteDeck(c.Context(), userID, c.Params("id")); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
