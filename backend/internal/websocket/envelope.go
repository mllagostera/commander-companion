package websocket

import (
	"encoding/json"
	"time"
)

// Event types for the envelope that the server sends over WebSocket. See
// ADR-0005 for the payload details of each one.
const (
	// EventConnected is sent exactly once, right after the connection
	// authenticates successfully.
	EventConnected = "connected"
	// EventGameAction rebroadcasts a game_actions action that was just recorded
	// (payload = GameActionResponse, with no exception for action_type).
	EventGameAction = "game_action"
	// EventGameFinished notifies that the game has ended; it carries no state, it's a
	// heads-up for the client to reconcile via REST (see ADR-0005, "REST remains the
	// source of truth").
	EventGameFinished = "game_finished"
	// EventError is used only during the authentication handshake.
	EventError = "error"
)

// Envelope is the common envelope for every message the server sends over WebSocket.
type Envelope struct {
	Type      string          `json:"type"`
	GameID    string          `json:"game_id"`
	ActorID   string          `json:"actor_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp string          `json:"timestamp"`
}

// errorPayload is the payload of an EventError-type Envelope.
type errorPayload struct {
	Message string `json:"message"`
}

// encodeEnvelope builds and serializes an Envelope. payload can be nil (it's omitted
// from the resulting JSON).
func encodeEnvelope(eventType, gameID, actorID string, payload interface{}) ([]byte, error) {
	env := Envelope{
		Type:      eventType,
		GameID:    gameID,
		ActorID:   actorID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		env.Payload = raw
	}

	return json.Marshal(env)
}

// connectedEnvelope builds the ack message sent after authenticating successfully. The
// payload is nil, so encodeEnvelope can't fail in practice (it only fails while
// serializing the payload) — the error check is for tidiness, not because it's expected
// to trigger.
func connectedEnvelope(gameID string) []byte {
	msg, err := encodeEnvelope(EventConnected, gameID, "", nil)
	if err != nil {
		return nil
	}
	return msg
}

// errEnvelope builds an error message (only used during the auth handshake). The
// payload is a trivial single-string struct, so it can't fail in practice
// either.
func errEnvelope(gameID, message string) []byte {
	msg, err := encodeEnvelope(EventError, gameID, "", errorPayload{Message: message})
	if err != nil {
		return nil
	}
	return msg
}
