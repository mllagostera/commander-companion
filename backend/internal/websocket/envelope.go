package websocket

import (
	"encoding/json"
	"time"
)

// Tipos de evento del sobre (envelope) que el servidor envía por WebSocket. Ver
// ADR-0005 para el detalle de payload de cada uno.
const (
	// EventConnected se envía una única vez, justo después de que la conexión
	// autentica correctamente.
	EventConnected = "connected"
	// EventGameAction retransmite una acción de game_actions recién registrada
	// (payload = GameActionResponse, sin excepción de action_type).
	EventGameAction = "game_action"
	// EventGameFinished avisa que la partida terminó; no lleva estado, es un aviso
	// para que el cliente reconcilie por REST (ver ADR-0005, "REST sigue siendo la
	// fuente de verdad").
	EventGameFinished = "game_finished"
	// EventError se usa únicamente durante el handshake de autenticación.
	EventError = "error"
)

// Envelope es el sobre común de todo mensaje que el servidor envía por WebSocket.
type Envelope struct {
	Type      string          `json:"type"`
	GameID    string          `json:"game_id"`
	ActorID   string          `json:"actor_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp string          `json:"timestamp"`
}

// errorPayload es el payload de un Envelope de tipo EventError.
type errorPayload struct {
	Message string `json:"message"`
}

// encodeEnvelope arma y serializa un Envelope. payload puede ser nil (se omite del
// JSON resultante).
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

// connectedEnvelope arma el mensaje de ack enviado tras autenticar con éxito. El
// payload es nil, así que encodeEnvelope no puede fallar en la práctica (solo falla al
// serializar el payload) — el chequeo de error es por prolijidad, no porque se espere
// que dispare.
func connectedEnvelope(gameID string) []byte {
	msg, err := encodeEnvelope(EventConnected, gameID, "", nil)
	if err != nil {
		return nil
	}
	return msg
}

// errEnvelope arma un mensaje de error (solo se usa durante el handshake de auth). El
// payload es un struct trivial de un solo string, por lo que tampoco puede fallar en
// la práctica.
func errEnvelope(gameID, message string) []byte {
	msg, err := encodeEnvelope(EventError, gameID, "", errorPayload{Message: message})
	if err != nil {
		return nil
	}
	return msg
}
