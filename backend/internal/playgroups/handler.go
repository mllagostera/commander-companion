package playgroups

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for playgroups.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the playgroups module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/playgroups", h.ListPlaygroups)
	router.Post("/playgroups", h.CreatePlaygroup)
	router.Get("/playgroups/:id", h.GetPlaygroup)
	router.Patch("/playgroups/:id", h.UpdatePlaygroup)
	router.Post("/playgroups/:id/members", h.AddMember)
	router.Get("/playgroups/:id/members/:userId/decks", h.ListMemberDecks)
}

// CreatePlaygroup handles the creation of a new playgroup.
func (h *Handler) CreatePlaygroup(c *fiber.Ctx) error {
	var req CreatePlaygroupRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.CreatePlaygroup(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListPlaygroups returns the authenticated user's groups. Backward compatible:
// without `cursor`/`limit`, it keeps returning the full list with members
// populated (same response shape existing clients already parse as a bare
// array). With either query param present, it opts into a page instead (see
// Service.ListPlaygroupsPage) — same "response shape depends on a query
// param" pattern already used by games.Handler.ListGames for playgroup_id.
func (h *Handler) ListPlaygroups(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)

	if c.Query("cursor") != "" || c.Query("limit") != "" {
		page, err := common.ParsePageRequest(c)
		if err != nil {
			return common.MapError(err)
		}
		res, err := h.svc.ListPlaygroupsPage(c.Context(), page, userID)
		if err != nil {
			return common.MapError(err)
		}
		return c.JSON(res)
	}

	res, err := h.svc.ListPlaygroups(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// GetPlaygroup returns the detail of a group, if the authenticated user is a member.
func (h *Handler) GetPlaygroup(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.GetPlaygroup(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// UpdatePlaygroup renames the given playgroup.
func (h *Handler) UpdatePlaygroup(c *fiber.Ctx) error {
	var req UpdatePlaygroupRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.UpdatePlaygroup(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// AddMember adds a member to the given playgroup.
func (h *Handler) AddMember(c *fiber.Ctx) error {
	var req AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.AddMember(c.Context(), c.Params("id"), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListMemberDecks returns the decks of a group member, if the requester is
// also a member (see ADR-0013: this is what enables picking their deck in a
// proxy-join).
func (h *Handler) ListMemberDecks(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListMemberDecks(c.Context(), c.Params("id"), userID, c.Params("userId"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}
