package websocket

import (
	"sync"
	"time"

	fiberws "github.com/gofiber/websocket/v2"
)

// sendBufferSize is the capacity of each connection's outgoing buffer. If a client
// doesn't read fast enough and the buffer fills up, new messages are dropped for
// that specific connection (see Hub.Broadcast and ADR-0005).
const sendBufferSize = 16

// Default heartbeat timings (see ADR-0005, "Heartbeat / application-level
// ping-pong"). A client that goes silent without a clean TCP close — the common
// case for a phone losing signal — would otherwise leave its Hub entry, readLoop,
// and writePump goroutines running forever: writePump sends a ping every
// pingPeriod, and readLoop's read deadline (refreshed on every pong, real or
// control-frame-implicit) closes the connection if none arrives within pongWait.
const (
	// pongWait is how long readLoop waits, after each ping, for a pong (or any
	// other message) before giving up on the connection. Also the initial read
	// deadline, in case the client goes quiet before the first ping is even due.
	pongWait = 60 * time.Second
	// pingPeriod is how often writePump sends a ping frame — 90% of pongWait, so at
	// least one ping/pong round-trip fits before the read deadline expires.
	pingPeriod = 54 * time.Second
	// writeWait bounds how long a single write (a queued message or a ping) may take.
	writeWait = 10 * time.Second
)

// Client wraps a real WebSocket connection (gofiber/websocket/v2) and satisfies the
// Conn interface the Hub needs.
type Client struct {
	conn   *fiberws.Conn
	gameID string
	userID string

	send chan []byte
	once sync.Once
	// wg tracks writePump's goroutine (started by startWritePump) so wait can block
	// until it has actually returned — not just been signaled to stop. This matters
	// because gofiber/websocket recycles the *fiberws.Conn back to its sync.Pool the
	// moment the connection handler function returns; if writePump were still touching
	// conn after that (its next queued write, or ping tick), that's a data race against
	// whatever new connection the pool hands that same object to next.
	wg sync.WaitGroup

	// Heartbeat timings, copied from the package defaults in newClient. Kept as
	// per-instance fields (not the package constants directly) so tests can shrink
	// them and exercise the timeout paths without a 60s wait.
	pongWait   time.Duration
	pingPeriod time.Duration
	writeWait  time.Duration
}

// newClient creates a Client for conn, already authenticated as userID and subscribed to gameID.
func newClient(conn *fiberws.Conn, gameID, userID string) *Client {
	return &Client{
		conn:       conn,
		gameID:     gameID,
		userID:     userID,
		send:       make(chan []byte, sendBufferSize),
		pongWait:   pongWait,
		pingPeriod: pingPeriod,
		writeWait:  writeWait,
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

// startWritePump runs writePump in its own goroutine, tracked by wg so wait can block
// until it has actually finished. Callers must eventually call wait (after Close) to
// avoid handing the underlying conn back to gofiber's pool while writePump might still
// be using it — see wg's doc.
func (c *Client) startWritePump() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.writePump()
	}()
}

// wait blocks until writePump (started via startWritePump) has returned. Call after
// Close, which is what makes writePump return in the first place.
func (c *Client) wait() {
	c.wg.Wait()
}

// writePump drains the outgoing channel and writes each message to the socket, and
// sends a ping frame every pingPeriod (see readLoop's deadline, which a pong reply
// refreshes). It runs for as long as the connection lasts (see startWritePump). It
// ends, closing the connection so readLoop (blocked reading) unblocks too, when
// Close() closes the channel or any write/ping fails (dead connection).
func (c *Client) writePump() {
	ticker := time.NewTicker(c.pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(fiberws.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			deadline := time.Now().Add(c.writeWait)
			if err := c.conn.WriteControl(fiberws.PingMessage, nil, deadline); err != nil {
				return
			}
		}
	}
}

// readLoop blocks reading from the socket until the connection closes, fails, or goes
// silent for longer than pongWait (see writePump's ping and the pong handler below).
// There's no client→server channel in this pass (see ADR-0005): any incoming message
// is discarded, and reading is only used to detect the disconnect (and process control
// frames, as required by the WebSocket protocol).
func (c *Client) readLoop() {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
