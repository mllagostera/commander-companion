package websocket

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"

	"github.com/usuario/commander-companion-backend/internal/auth"
	"github.com/usuario/commander-companion-backend/internal/common"
)

// authMessageTimeout es cuánto espera el servidor, tras aceptar el upgrade, a que
// llegue el primer mensaje ({"type":"auth","token":"..."}) antes de cerrar la conexión.
const authMessageTimeout = 10 * time.Second

const authMessageType = "auth"

// authMessage es el único mensaje que el servidor espera recibir de un cliente: el
// primero, y siempre con este formato (ver ADR-0005, sección de autenticación).
type authMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// RegisterRoutes registra la ruta de WebSocket para sincronización en vivo de
// partidas. A diferencia del resto de la API, esta ruta es pública (no lleva
// auth.RequireAuth): el handshake HTTP de upgrade lo hace el propio navegador, que no
// puede adjuntar el header Authorization, así que la autenticación ocurre en un mensaje
// posterior a la conexión (ver ADR-0005).
func RegisterRoutes(router fiber.Router, hub *Hub, jwtSecret []byte) {
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
		handleConnection(hub, jwtSecret, conn.Params("id"), conn)
	}))
}

// handleConnection maneja el ciclo de vida completo de una conexión WebSocket:
// autenticación, registro en el Hub, y lectura/escritura hasta que se desconecta.
func handleConnection(hub *Hub, jwtSecret []byte, gameID string, conn *fiberws.Conn) {
	userID, ok := authenticate(conn, jwtSecret, gameID)
	if !ok {
		_ = conn.Close()
		return
	}

	client := newClient(conn, gameID, userID)
	hub.Register(gameID, client)
	defer func() {
		hub.Unregister(gameID, client)
		client.Close()
	}()

	go client.writePump()
	client.Send(connectedEnvelope(gameID))

	client.readLoop()
}

// authenticate espera el mensaje inicial de auth y valida el JWT que trae. Devuelve el
// user ID y true si es válido; si no, ya dejó escrito en el socket un Envelope de tipo
// error y un close frame de política violada (código 1008), y devuelve false.
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

// rejectAuth envía un Envelope de error seguido de un close frame de política violada
// (RFC 6455 1008). Los errores de escritura se ignoran deliberadamente: si el socket ya
// está roto, el llamador va a cerrarlo igual.
func rejectAuth(conn *fiberws.Conn, gameID, reason string) {
	_ = conn.WriteMessage(fiberws.TextMessage, errEnvelope(gameID, reason))
	deadline := time.Now().Add(authMessageTimeout)
	_ = conn.WriteControl(fiberws.CloseMessage, fiberws.FormatCloseMessage(fiberws.ClosePolicyViolation, reason), deadline)
}
