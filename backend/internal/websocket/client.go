package websocket

import (
	"sync"

	fiberws "github.com/gofiber/websocket/v2"
)

// sendBufferSize es la capacidad del buffer de salida de cada conexión. Si un cliente
// no lee lo bastante rápido y el buffer se llena, los mensajes nuevos se descartan para
// esa conexión puntual (ver Hub.Broadcast y ADR-0005).
const sendBufferSize = 16

// Client envuelve una conexión WebSocket real (gofiber/websocket/v2) y satisface la
// interfaz Conn que necesita el Hub.
type Client struct {
	conn   *fiberws.Conn
	gameID string
	userID string

	send chan []byte
	once sync.Once
}

// newClient crea un Client para conn, ya autenticado como userID y suscripto a gameID.
func newClient(conn *fiberws.Conn, gameID, userID string) *Client {
	return &Client{
		conn:   conn,
		gameID: gameID,
		userID: userID,
		send:   make(chan []byte, sendBufferSize),
	}
}

// Send implementa Conn: encola message para el goroutine de escritura (writePump). Si
// el buffer está lleno, descarta el mensaje en vez de bloquear al llamador (que, para
// game_action, es el propio request HTTP de game-actions.RecordAction: nunca debe
// esperar a un cliente WebSocket lento).
func (c *Client) Send(message []byte) {
	select {
	case c.send <- message:
	default:
	}
}

// Close implementa Conn: cierra el canal de salida (lo que hace terminar a writePump) y
// la conexión subyacente. Seguro de llamar más de una vez.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

// writePump drena el canal de salida y escribe cada mensaje al socket. Corre en su
// propio goroutine mientras dura la conexión; termina cuando Close() cierra el canal, o
// ante cualquier error de escritura (conexión caída del otro lado).
func (c *Client) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(fiberws.TextMessage, msg); err != nil {
			return
		}
	}
}

// readLoop bloquea leyendo del socket hasta que la conexión se cierra o falla. No hay
// canal cliente→servidor en esta pasada (ver ADR-0005): cualquier mensaje entrante se
// descarta, y solo se usa la lectura para detectar la desconexión (y procesar
// control frames, como corresponde al protocolo WebSocket).
func (c *Client) readLoop() {
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
