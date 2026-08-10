package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
)

// MembershipChecker is what websocket needs from games to authorize a
// connection: whether userID may access gameID — the same boundary as
// games.GetGame (playgroup membership, or a seat in the game — see
// ADR-0013). Implemented by games.Service, injected here without this
// package depending on internal/games directly (same pattern as
// games.Broadcaster/gameactions.Broadcaster, which go the other way).
type MembershipChecker interface {
	CanAccessGame(ctx context.Context, gameID, userID string) (bool, error)
}

// authMessageTimeout is how long the server waits, after accepting the upgrade, for
// the first message ({"type":"auth","token":"..."}) to arrive before closing the connection.
const authMessageTimeout = 10 * time.Second

const authMessageType = "auth"

// authMessage is the only message the server expects to receive from a client: the
// first one, and always in this format (see ADR-0005, authentication section).
type authMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// RegisterRoutes registers the WebSocket route for live game
// synchronization. Unlike the rest of the API, this route is public (it doesn't carry
// auth.RequireAuth): the HTTP upgrade handshake is done by the browser itself, which
// can't attach the Authorization header, so authentication happens in a message
// sent after the connection is established (see ADR-0005).
func RegisterRoutes(router fiber.Router, hub *Hub, jwtSecret []byte, membership MembershipChecker) {
	router.Use("/ws/games/:id", func(c *fiber.Ctx) error {
		if !fiberws.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		if _, err := common.ParseUUID(c.Params("id")); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid game id")
		}
		return c.Next()
	})

	router.Get("/ws/games/:id", fiberws.New(func(conn *fiberws.Conn) {
		handleConnection(hub, jwtSecret, membership, conn.Params("id"), conn)
	}))
}

// handleConnection handles the full lifecycle of a WebSocket connection:
// authentication, authorization, registration in the Hub, and
// reading/writing until it disconnects.
func handleConnection(hub *Hub, jwtSecret []byte, membership MembershipChecker, gameID string, conn *fiberws.Conn) {
	userID, ok := authenticate(conn, jwtSecret, gameID)
	if !ok {
		_ = conn.Close()
		return
	}

	// A valid JWT only proves who the caller is, not that they may watch this
	// particular game's live feed — that requires the same check as
	// GET /games/{id} (see MembershipChecker).
	allowed, err := membership.CanAccessGame(context.Background(), gameID, userID)
	if err != nil || !allowed {
		rejectAuth(conn, gameID, "not authorized to access this game")
		_ = conn.Close()
		return
	}

	client := newClient(conn, gameID, userID)
	hub.Register(gameID, client)
	defer func() {
		hub.Unregister(gameID, client)
		client.Close()
		// Wait for writePump to actually exit before this function returns and
		// gofiber recycles conn back to its pool — see Client.wg's doc.
		client.wait()
	}()

	client.startWritePump()
	client.Send(connectedEnvelope(gameID))

	client.readLoop()
}

// authenticate waits for the initial auth message and validates the JWT it carries. Returns
// the user ID and true if it's valid; otherwise it has already written an error-type
// Envelope and a policy-violation close frame (code 1008) to the socket, and returns false.
func authenticate(conn *fiberws.Conn, jwtSecret []byte, gameID string) (userID string, ok bool) {
	if err := conn.SetReadDeadline(time.Now().Add(authMessageTimeout)); err != nil {
		return "", false
	}

	_, raw, err := conn.ReadMessage()
	if err != nil {
		rejectAuth(conn, gameID, "timed out waiting for auth message")
		return "", false
	}

	var msg authMessage
	if unmarshalErr := json.Unmarshal(raw, &msg); unmarshalErr != nil || msg.Type != authMessageType || msg.Token == "" {
		rejectAuth(conn, gameID, `expected {"type":"auth","token":"..."} as the first message`)
		return "", false
	}

	userID, err = auth.VerifyAccessToken(jwtSecret, msg.Token)
	if err != nil {
		rejectAuth(conn, gameID, "invalid or expired token")
		return "", false
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return "", false
	}
	return userID, true
}

// rejectAuth sends an error Envelope followed by a policy-violation close frame
// (RFC 6455 1008). Write errors are deliberately ignored: if the socket is already
// broken, the caller is going to close it anyway.
func rejectAuth(conn *fiberws.Conn, gameID, reason string) {
	_ = conn.WriteMessage(fiberws.TextMessage, errEnvelope(gameID, reason))
	deadline := time.Now().Add(authMessageTimeout)
	_ = conn.WriteControl(fiberws.CloseMessage, fiberws.FormatCloseMessage(fiberws.ClosePolicyViolation, reason), deadline)
}
