package tournaments

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for tournaments.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the tournaments module.
// /tournaments/lookup and /tournaments/join are registered before
// /tournaments/:id so the router matches them as static segments, same
// precedent as decks.Handler putting /decks/import/moxfield before /decks/:id.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/tournaments", h.ListTournaments)
	router.Post("/tournaments", h.CreateTournament)
	router.Get("/tournaments/lookup", h.LookupByCode)
	router.Post("/tournaments/join", h.JoinTournament)
	router.Get("/tournaments/:id", h.GetTournament)
	router.Post("/tournaments/:id/participants", h.AddGuestParticipant)
	router.Post("/tournaments/:id/start", h.StartTournament)
	router.Post("/tournaments/:id/tables/:tableId/result", h.RecordTableResult)
	router.Post("/tournaments/:id/rounds/next", h.AdvanceRound)
}

// CreateTournament handles the creation of a new standalone tournament, with
// the caller as organizer.
func (h *Handler) CreateTournament(c *fiber.Ctx) error {
	var req CreateTournamentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.CreateTournament(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListTournaments returns a page of tournaments the authenticated user
// organizes or participates in. Accepts the `cursor` and `limit` query
// params (see internal/common/pagination.go).
func (h *Handler) ListTournaments(c *fiber.Ctx) error {
	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListTournaments(c.Context(), userID, page)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetTournament returns the full detail of a tournament, if the authenticated
// user organizes it or is a registered participant.
func (h *Handler) GetTournament(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetTournament(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// JoinTournament self-registers the authenticated user for a tournament by
// its join code, with one of their own decks.
func (h *Handler) JoinTournament(c *fiber.Ctx) error {
	var req JoinTournamentRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.JoinTournament(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// AddGuestParticipant registers a participant with no account. Organizer-only.
func (h *Handler) AddGuestParticipant(c *fiber.Ctx) error {
	var req AddGuestParticipantRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.AddGuestParticipant(c.Context(), userID, c.Params("id"), req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// StartTournament locks the roster and seats round 1. Organizer-only.
func (h *Handler) StartTournament(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.StartTournament(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// RecordTableResult sets a table's finish order for the current round. Organizer-only.
func (h *Handler) RecordTableResult(c *fiber.Ctx) error {
	var req RecordTableResultRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.RecordTableResult(c.Context(), userID, c.Params("id"), c.Params("tableId"), req)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// AdvanceRound finishes the current round and seats the next one, or
// finishes the tournament if it was the last round. Organizer-only.
func (h *Handler) AdvanceRound(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.AdvanceRound(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// LookupByCode resolves a tournament by its join code -- the "enter the code
// in the app" endpoint. Query param `code`.
func (h *Handler) LookupByCode(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.LookupByCode(c.Context(), userID, c.Query("code"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
