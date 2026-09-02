package statistics

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for statistics.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all endpoints of the statistics module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/statistics/user", h.GetUserStats)
	router.Get("/statistics/decks", h.ListDeckStats)
	router.Get("/statistics/deck/:id", h.GetDeckStats)
	router.Get("/statistics/playgroup/:id", h.GetPlaygroupStats)
	router.Get("/statistics/playgroups", h.ListPlaygroupGameCounts)
	router.Get("/statistics/opponents", h.ListOpponentStats)
	router.Get("/statistics/games", h.ListFinishedGames)
	router.Get("/statistics/dashboard", h.GetDashboard)
}

// GetDashboard returns everything the web dashboard renders, in one request.
func (h *Handler) GetDashboard(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetDashboard(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetUserStats returns the authenticated user's statistics.
func (h *Handler) GetUserStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetUserStats(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetDeckStats returns the statistics for a deck belonging to the authenticated user.
func (h *Handler) GetDeckStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetDeckStats(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// ListDeckStats returns the statistics of every deck owned by the authenticated
// user in one call, instead of one GetDeckStats request per deck.
func (h *Handler) ListDeckStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListDeckStats(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetPlaygroupStats returns the aggregated statistics for a playgroup.
func (h *Handler) GetPlaygroupStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetPlaygroupStats(c.Context(), c.Params("id"), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// ListPlaygroupGameCounts returns, for every playgroup the authenticated user
// belongs to, how many finished games they've played within it.
func (h *Handler) ListPlaygroupGameCounts(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListPlaygroupGameCounts(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// ListOpponentStats returns the authenticated user's head-to-head record
// against every other user they've shared a finished game with.
func (h *Handler) ListOpponentStats(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListOpponentStats(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// ListFinishedGames returns a page of the authenticated user's finished-games
// history, enriched with each seat's username/deck. Accepts the `cursor` and
// `limit` query params (see internal/common/pagination.go).
func (h *Handler) ListFinishedGames(c *fiber.Ctx) error {
	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListFinishedGames(c.Context(), page, userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
