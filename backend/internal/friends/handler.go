package friends

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for friends.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all the endpoints of the friends module.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/friends/requests", h.SendFriendRequest)
	router.Get("/friends/requests", h.ListRequests)
	router.Post("/friends/requests/:id/accept", h.AcceptFriendRequest)
	router.Post("/friends/requests/:id/reject", h.RejectFriendRequest)
	router.Delete("/friends/requests/:id", h.CancelFriendRequest)
	router.Get("/friends", h.ListFriends)
	router.Delete("/friends/:userId", h.RemoveFriend)
}

// SendFriendRequest sends a friend request to req.AddresseeID.
func (h *Handler) SendFriendRequest(c *fiber.Ctx) error {
	var req SendFriendRequestRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.SendFriendRequest(c.Context(), userID, req)
	if err != nil {
		return common.MapError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(res)
}

// ListRequests returns the authenticated user's pending requests, in the
// direction given by the `direction` query param (`incoming` or `outgoing`,
// default `incoming`).
func (h *Handler) ListRequests(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)

	if c.Query("direction") == "outgoing" {
		res, err := h.svc.ListOutgoingRequests(c.Context(), userID)
		if err != nil {
			return common.MapError(err)
		}
		return c.JSON(res)
	}

	res, err := h.svc.ListIncomingRequests(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// AcceptFriendRequest accepts a pending request addressed to the authenticated user.
func (h *Handler) AcceptFriendRequest(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.AcceptFriendRequest(c.Context(), userID, c.Params("id"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// RejectFriendRequest rejects a pending request addressed to the authenticated user.
func (h *Handler) RejectFriendRequest(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.RejectFriendRequest(c.Context(), userID, c.Params("id")); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// CancelFriendRequest cancels a pending request sent by the authenticated user.
func (h *Handler) CancelFriendRequest(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.CancelFriendRequest(c.Context(), userID, c.Params("id")); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListFriends returns the authenticated user's accepted friendships.
func (h *Handler) ListFriends(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.ListFriends(c.Context(), userID)
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// RemoveFriend removes the friendship between the authenticated user and :userId.
func (h *Handler) RemoveFriend(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if err := h.svc.RemoveFriend(c.Context(), userID, c.Params("userId")); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
