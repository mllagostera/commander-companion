package admin

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

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
