// Package websocket implements live game synchronization over WebSocket
// (see ADR-0005 in docs/decisions/). The Hub keeps, in this process's memory, the
// active connections grouped by game_id and rebroadcasts messages to all
// connections of a game. There's no external pub/sub: if the backend runs on more than
// one replica, two players connected to different replicas won't see each other (a
// limitation documented in the ADR, not an oversight).
package websocket

import "sync"

// Conn is the minimal surface the Hub needs from a client connection: queue a
// message to send it asynchronously, and close it. The real implementation
// (Client, in client.go) wraps a *websocket.Conn from gofiber/websocket/v2; the Hub's
// tests use a lightweight fake that doesn't open any real socket.
type Conn interface {
	// Send queues message for asynchronous delivery. It's best-effort: if the
	// connection's internal buffer is full, the implementation may drop the message instead of
	// blocking (see ADR-0005).
	Send(message []byte)
	// Close closes the connection. Must be safe to call more than once.
	Close()
}

// Hub groups the active connections by game_id and allows broadcasting messages to them.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[Conn]struct{}
}

// NewHub creates an empty Hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[Conn]struct{})}
}

// Register adds conn to gameID's room.
func (h *Hub) Register(gameID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[gameID] == nil {
		h.rooms[gameID] = make(map[Conn]struct{})
	}
	h.rooms[gameID][conn] = struct{}{}
}

// Unregister removes conn from gameID's room. Does nothing if conn wasn't
// registered (e.g. if the whole room was already closed by CloseRoom).
func (h *Hub) Unregister(gameID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[gameID]
	if !ok {
		return
	}
	delete(clients, conn)
	if len(clients) == 0 {
		delete(h.rooms, gameID)
	}
}

// Broadcast sends message to all connections registered in gameID's room. It doesn't
// block or return an error: delivery to each connection is Conn.Send's responsibility
// (see its doc, and ADR-0005's "out of scope" section on delivery guarantees).
func (h *Hub) Broadcast(gameID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.rooms[gameID] {
		conn.Send(message)
	}
}

// CloseRoom closes all active connections for gameID and removes the room. Used
// when the game finishes: there can no longer be any game_actions to rebroadcast (see
// ADR-0005, connection lifecycle).
func (h *Hub) CloseRoom(gameID string) {
	h.mu.Lock()
	clients := h.rooms[gameID]
	delete(h.rooms, gameID)
	h.mu.Unlock()

	for conn := range clients {
		conn.Close()
	}
}

// RoomSize returns the number of active connections in gameID's room (0 if there
// are none, including a game with no registered room). Intended for tests and
// introspection.
func (h *Hub) RoomSize(gameID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.rooms[gameID])
}
