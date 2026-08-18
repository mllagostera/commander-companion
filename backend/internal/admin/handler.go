package admin

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// defaultActivityDaysBack is how many days GetDailyActivity looks back when the
// caller doesn't pass a `days` query param.
const defaultActivityDaysBack = 30

// Handler holds the HTTP transport dependencies for the admin dashboard.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers the admin dashboard's endpoints. router is expected to
// already be scoped under /admin with auth.RequireAdmin applied (see cmd/api/main.go) —
// routes here are relative to that prefix.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/users", h.ListUsers)
	router.Get("/users/:id", h.GetUser)
	router.Patch("/users/:id/status", h.UpdateUserStatus)
	router.Get("/stats/overview", h.GetOverviewStats)
	router.Get("/stats/activity", h.GetDailyActivity)
}

// ListUsers returns a paginated, optionally search-filtered list of users.
func (h *Handler) ListUsers(c *fiber.Ctx) error {
	page, err := common.ParsePageRequest(c)
	if err != nil {
		return common.MapError(err)
	}

	res, err := h.svc.ListUsers(c.Context(), page, c.Query("search"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetUser returns a single user's admin-facing profile.
func (h *Handler) GetUser(c *fiber.Ctx) error {
	res, err := h.svc.GetUserDetail(c.Context(), c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// UpdateUserStatus activates/deactivates a user's account.
func (h *Handler) UpdateUserStatus(c *fiber.Ctx) error {
	var req UpdateUserStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	callerID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.UpdateUserStatus(c.Context(), callerID, c.Params("id"), req.IsActive)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetOverviewStats returns the global counts shown on the admin home page.
func (h *Handler) GetOverviewStats(c *fiber.Ctx) error {
	res, err := h.svc.GetOverviewStats(c.Context())
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetDailyActivity returns the historical series for the admin dashboard's
// activity chart. `days` defaults to 30 and is clamped to
// [1, maxActivityDaysBack] by the service, same "clamp, don't error" approach
// as internal/common/pagination.go's limit param.
func (h *Handler) GetDailyActivity(c *fiber.Ctx) error {
	days := defaultActivityDaysBack
	if raw := c.Query("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "days must be a positive integer")
		}
		days = parsed
	}

	res, err := h.svc.GetDailyActivity(c.Context(), days)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
