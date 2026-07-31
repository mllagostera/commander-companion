package games

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for games.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the games module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/games", h.ListGames)
	router.Post("/games", h.CreateGame)
	router.Get("/games/:id", h.GetGame)
	router.Post("/games/:id/join", h.JoinGame)
	router.Post("/games/:id/leave", h.LeaveGame)
	router.Post("/games/:id/start", h.StartGame)
	router.Post("/games/:id/finish", h.FinishGame)
}

// CreateGame handles the creation of a new game in pending state.
func (h *Handler) CreateGame(c *fiber.Ctx) error {
	var req CreateGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	res, err := h.svc.CreateGame(c.Context(), req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListGames returns a page of the game history. Accepts the `cursor` and
// `limit` query params (see internal/common/pagination.go). With `playgroup_id`,
// however, it returns the complete history (unpaginated) of that group — it requires
// the authenticated user to be a member (see Service.ListGamesForPlaygroup).
func (h *Handler) ListGames(c *fiber.Ctx) error {
	if playgroupID := c.Query("playgroup_id"); playgroupID != "" {
		userID, _ := c.Locals(common.UserIDKey).(string)
		res, err := h.svc.ListGamesForPlaygroup(c.Context(), playgroupID, userID)
		if err != nil {
			return common.MapError(err)
		}
		return c.JSON(res)
	}

	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	res, err := h.svc.ListGames(c.Context(), page)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetGame returns the detail of a game.
func (h *Handler) GetGame(c *fiber.Ctx) error {
	res, err := h.svc.GetGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// JoinGame adds the authenticated user to a game in pending state.
func (h *Handler) JoinGame(c *fiber.Ctx) error {
	var req JoinGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.JoinGame(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// LeaveGame removes the authenticated user from a game in pending state.
func (h *Handler) LeaveGame(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.LeaveGame(c.Context(), c.Params("id"), userID); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// StartGame starts a game, moving it to active state.
func (h *Handler) StartGame(c *fiber.Ctx) error {
	res, err := h.svc.StartGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// FinishGame finishes a game, moving it to finished state.
func (h *Handler) FinishGame(c *fiber.Ctx) error {
	res, err := h.svc.FinishGame(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
