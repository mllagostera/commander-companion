package users

import (
	"github.com/gofiber/fiber/v2"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// Handler holds the HTTP transport dependencies for users.
type Handler struct {
	svc Service
}

// NewHandler initializes and returns a new Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{
		svc: svc,
	}
}

// RegisterRoutes registers the public endpoints of the users/auth module. rateLimit is
// the same per-IP middleware that protects the other public auth endpoints (see
// auth.Handler.RegisterPublicRoutes): register is public and does a bcrypt hash per
// request, so it also needs to be bounded.
func (h *Handler) RegisterRoutes(router fiber.Router, rateLimit fiber.Handler) {
	router.Post("/auth/register", rateLimit, h.Register)
	router.Post("/auth/verify-email", rateLimit, h.VerifyEmail)
	router.Post("/auth/resend-verification", rateLimit, h.ResendVerification)
}

// RegisterProtectedRoutes registers the user endpoints that require a session.
// searchRateLimit bounds GET /users/search specifically: it does an exact-email
// lookup (see SearchUsers) that could otherwise be used to check an arbitrary
// list of addresses at whatever rate an authenticated account wants.
func (h *Handler) RegisterProtectedRoutes(router fiber.Router, searchRateLimit fiber.Handler) {
	router.Get("/users/search", searchRateLimit, h.SearchUsers)
	router.Patch("/users/:id", h.UpdateProfile)
	router.Post("/users/:id/password", h.ChangePassword)
}

// SearchUsers searches users by username (partial) or email (exact) — to invite
// someone to a playgroup without knowing the other person's UUID (see docs/decisions and
// internal/playgroups.AddMember).
func (h *Handler) SearchUsers(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	res, err := h.svc.SearchUsers(c.Context(), userID, c.Query("q"))
	if err != nil {
		return common.MapError(err)
	}
	return c.JSON(res)
}

// Register handles the registration request.
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Very basic validation
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing required fields")
	}

	res, err := h.svc.RegisterUser(c.Context(), req)
	if err != nil {
		return common.MapError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

// VerifyEmail confirms the account from the token sent by mail. It's POST (not GET)
// on purpose: the mail link leads to a page on the web client, which is what makes
// this POST with the token in the body — this way it never ends up in a query string
// that the server's access logger would log in plain text.
func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
	var req VerifyEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.Token == "" {
		return fiber.NewError(fiber.StatusBadRequest, "token is required")
	}

	if err := h.svc.VerifyEmail(c.Context(), req.Token); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ResendVerification sends a new verification email if applicable. It always
// responds 204 (see users.Service.ResendVerification: it doesn't reveal whether the email exists).
func (h *Handler) ResendVerification(c *fiber.Ctx) error {
	var req ResendVerificationRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.Email == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email is required")
	}

	if err := h.svc.ResendVerification(c.Context(), req.Email); err != nil {
		return common.MapError(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateProfile updates the caller's own profile: username (optional) and moxfield_username.
// Restricted to :id being the authenticated user themselves — 404 if not, same "don't
// reveal" criteria as decks/playgroups, so as not to confirm the existence of other IDs.
func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if c.Params("id") != userID {
		return common.MapError(ErrUserNotFound)
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	var res *UserResponse
	var err error

	if req.Username != nil {
		if res, err = h.svc.UpdateUsername(c.Context(), userID, *req.Username); err != nil {
			return common.MapError(err)
		}
	}

	if req.MoxfieldUsername != nil {
		if res, err = h.svc.UpdateMoxfieldUsername(c.Context(), userID, *req.MoxfieldUsername); err != nil {
			return common.MapError(err)
		}
	}

	if res == nil {
		// No field sent: there's nothing to update, but we still return the
		// current state instead of an error — an empty PATCH isn't invalid, it's a no-op.
		if res, err = h.svc.GetUser(c.Context(), userID); err != nil {
			return common.MapError(err)
		}
	}

	return c.JSON(res)
}

// ChangePassword changes the caller's own password, after validating the current one. Restricted
// to :id being the authenticated user themselves, same criteria as UpdateProfile.
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	userID, _ := c.Locals(common.UserIDKey).(string)
	if c.Params("id") != userID {
		return common.MapError(ErrUserNotFound)
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing required fields")
	}

	if err := h.svc.ChangePassword(c.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		return common.MapError(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
