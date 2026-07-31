package websocket_test

import (
	"sync"
	"testing"

	"github.com/usuario/commander-companion-backend/internal/websocket"
)

// fakeConn implements websocket.Conn without opening any real socket, so the Hub
// can be tested purely in memory (it doesn't need Postgres or a network connection,
// unlike the integration tests of other modules in internal/testutil).
type fakeConn struct {
	mu       sync.Mutex
	received [][]byte
	closed   bool
}

func (c *fakeConn) Send(message []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.received = append(c.received, message)
}

func (c *fakeConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}

func (c *fakeConn) messages() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.received))
	copy(out, c.received)
	return out
}

func (c *fakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func TestHub_RegisterAndBroadcast_DeliversToSameRoomOnly(t *testing.T) {
	hub := websocket.NewHub()

	gameA1 := &fakeConn{}
	gameA2 := &fakeConn{}
	gameB1 := &fakeConn{}

	hub.Register("game-a", gameA1)
	hub.Register("game-a", gameA2)
	hub.Register("game-b", gameB1)

	hub.Broadcast("game-a", []byte("hello-a"))

	if got := gameA1.messages(); len(got) != 1 || string(got[0]) != "hello-a" {
		t.Fatalf("gameA1.messages() = %q, want [\"hello-a\"]", got)
	}
	if got := gameA2.messages(); len(got) != 1 || string(got[0]) != "hello-a" {
		t.Fatalf("gameA2.messages() = %q, want [\"hello-a\"]", got)
	}
	if got := gameB1.messages(); len(got) != 0 {
		t.Fatalf("gameB1.messages() = %q, want no messages (different game_id)", got)
	}
}

func TestHub_Broadcast_UnknownGameID_IsNoop(t *testing.T) {
	hub := websocket.NewHub()

	// Must not panic or error when broadcasting to a room that never existed.
	hub.Broadcast("no-such-game", []byte("hello"))

	if size := hub.RoomSize("no-such-game"); size != 0 {
		t.Fatalf("RoomSize() = %d, want 0", size)
	}
}

func TestHub_Unregister_StopsDeliveryAndCleansUpEmptyRoom(t *testing.T) {
	hub := websocket.NewHub()
	conn := &fakeConn{}

	hub.Register("game-a", conn)
	if size := hub.RoomSize("game-a"); size != 1 {
		t.Fatalf("RoomSize() tras Register = %d, want 1", size)
	}

	hub.Unregister("game-a", conn)
	if size := hub.RoomSize("game-a"); size != 0 {
		t.Fatalf("RoomSize() tras Unregister = %d, want 0 (sala vacía debe limpiarse)", size)
	}

	hub.Broadcast("game-a", []byte("should-not-arrive"))
	if got := conn.messages(); len(got) != 0 {
		t.Fatalf("messages() tras Unregister = %q, want none", got)
	}
}

func TestHub_Unregister_UnknownConn_IsNoop(t *testing.T) {
	hub := websocket.NewHub()
	registered := &fakeConn{}
	other := &fakeConn{}

	hub.Register("game-a", registered)

	// Unregistering a connection that was never in the room must not affect the ones
	// that are.
	hub.Unregister("game-a", other)

	if size := hub.RoomSize("game-a"); size != 1 {
		t.Fatalf("RoomSize() = %d, want 1 (unregister de conn ajena no debe afectar la sala)", size)
	}
}

func TestHub_RegisterSameConnTwice_CountsOnce(t *testing.T) {
	hub := websocket.NewHub()
	conn := &fakeConn{}

	hub.Register("game-a", conn)
	hub.Register("game-a", conn)

	if size := hub.RoomSize("game-a"); size != 1 {
		t.Fatalf("RoomSize() = %d, want 1 (registrar la misma conexión dos veces no debe duplicarla)", size)
	}
}

func TestHub_CloseRoom_ClosesAllConnectionsAndRemovesRoom(t *testing.T) {
	hub := websocket.NewHub()
	conn1 := &fakeConn{}
	conn2 := &fakeConn{}
	otherRoomConn := &fakeConn{}

	hub.Register("game-a", conn1)
	hub.Register("game-a", conn2)
	hub.Register("game-b", otherRoomConn)

	hub.CloseRoom("game-a")

	if !conn1.isClosed() || !conn2.isClosed() {
		t.Fatalf("CloseRoom() no cerró todas las conexiones de la sala: conn1.closed=%v conn2.closed=%v",
			conn1.isClosed(), conn2.isClosed())
	}
	if otherRoomConn.isClosed() {
		t.Fatalf("CloseRoom(\"game-a\") no debe cerrar conexiones de otra sala")
	}
	if size := hub.RoomSize("game-a"); size != 0 {
		t.Fatalf("RoomSize(\"game-a\") tras CloseRoom = %d, want 0", size)
	}

	// Broadcasting to an already closed room must do nothing (and not panic).
	hub.Broadcast("game-a", []byte("too-late"))
	if got := conn1.messages(); len(got) != 0 {
		t.Fatalf("messages() tras CloseRoom = %q, want none", got)
	}
}
