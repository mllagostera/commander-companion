package inbound

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// EventEmailReceived is the only Resend webhook event this handler acts on.
const EventEmailReceived = "email.received"

// forwardTimeout bounds the three Resend calls one forward makes. It stays under
// the time Resend waits for the webhook's answer: past that, Resend retries
// anyway, and the idempotency key keeps the retry from sending a second copy.
const forwardTimeout = 12 * time.Second

// Handler receives Resend's webhooks.
type Handler struct {
	secret    string
	forwarder Forwarder
	now       func() time.Time
}

// NewHandler builds the webhook handler. secret is the webhook's signing secret.
func NewHandler(secret string, forwarder Forwarder) *Handler {
	return &Handler{secret: secret, forwarder: forwarder, now: time.Now}
}

// RegisterRoutes registers POST /webhooks/resend-inbound. It is public: what
// authenticates the caller is the signature, checked before anything else.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Post("/webhooks/resend-inbound", h.ResendInbound)
}

type webhookEvent struct {
	Type string `json:"type"`
	Data struct {
		EmailID string `json:"email_id"`
	} `json:"data"`
}

// ResendInbound verifies the webhook and forwards the email it announces.
// Answers: 401 for a bad signature, 204 for events it ignores, 502 when the
// forward fails, so Resend retries it on its backoff schedule.
func (h *Handler) ResendInbound(c *fiber.Ctx) error {
	body := c.Body()
	err := VerifySignature(h.secret,
		c.Get("svix-id"), c.Get("svix-timestamp"), c.Get("svix-signature"), body, h.now())
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	}

	var event webhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid webhook payload")
	}
	if event.Type != EventEmailReceived {
		return c.SendStatus(fiber.StatusNoContent)
	}
	if event.Data.EmailID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "email.received without email_id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()
	if err := h.forwarder.Forward(ctx, event.Data.EmailID); err != nil {
		log.Printf("inbound: could not forward received email %s: %v", event.Data.EmailID, err)
		status := fiber.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = fiber.StatusGatewayTimeout
		}
		return fiber.NewError(status, "could not forward the email")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
