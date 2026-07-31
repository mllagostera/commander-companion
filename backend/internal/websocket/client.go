package websocket

import (
	"sync"

	fiberws "github.com/gofiber/websocket/v2"
)

// sendBufferSize is the capacity of each connection's outgoing buffer. If a client
// doesn't read fast enough and the buffer fills up, new messages are dropped for
// that specific connection (see Hub.Broadcast and ADR-0005).
const sendBufferSize = 16

// Client wraps a real WebSocket connection (gofiber/websocket/v2) and satisfies the
// Conn interface the Hub needs.
type Client struct {
	conn   *fiberws.Conn
	gameID string
	userID string

	send chan []byte
	once sync.Once
}

// newClient creates a Client for conn, already authenticated as userID and subscribed to gameID.
func newClient(conn *fiberws.Conn, gameID, userID string) *Client {
	return &Client{
		conn:   conn,
		gameID: gameID,
		userID: userID,
		send:   make(chan []byte, sendBufferSize),
	}
}

// Send implements Conn: it queues message for the write goroutine (writePump). If
// the buffer is full, it drops the message instead of blocking the caller (which, for
// game_action, is game-actions.RecordAction's own HTTP request: it must never
// wait on a slow WebSocket client).
func (c *Client) Send(message []byte) {
	select {
	case c.send <- message:
	default:
	}
}

// Close implements Conn: it closes the outgoing channel (which makes writePump end) and
// the underlying connection. Safe to call more than once.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

// writePump drains the outgoing channel and writes each message to the socket. It runs in
// its own goroutine for as long as the connection lasts; it ends when Close() closes the
// channel, or on any write error (connection dropped on the other end).
func (c *Client) writePump() {
	for msg := range c.send {
		if err := c.conn.WriteMessage(fiberws.TextMessage, msg); err != nil {
			return
		}
	}
}

// readLoop blocks reading from the socket until the connection closes or fails. There's no
// client→server channel in this pass (see ADR-0005): any incoming message is
// discarded, and reading is only used to detect the disconnect (and process
// control frames, as required by the WebSocket protocol).
func (c *Client) readLoop() {
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
