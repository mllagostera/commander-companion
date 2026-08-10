package websocket

import (
	"context"
	"net"
	"testing"
	"time"

	realws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"
)

// startHeartbeatTestServer spins up a real fiber app on a loopback TCP port whose
// only route upgrades to a WebSocket and hands the raw *fiberws.Conn to handle. It
// exists so writePump/readLoop can be exercised against an actual socket (they take a
// concrete *fiberws.Conn, not the Conn interface Hub uses, so a fake won't do).
func startHeartbeatTestServer(t *testing.T, handle func(conn *fiberws.Conn)) string {
	t.Helper()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use("/ws", func(c *fiber.Ctx) error {
		if !fiberws.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		return c.Next()
	})
	app.Get("/ws", fiberws.New(handle))

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	return "ws://" + ln.Addr().String() + "/ws"
}

func dialHeartbeatTestClient(t *testing.T, url string) *realws.Conn {
	t.Helper()
	conn, _, err := realws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestClient_ReadLoop_ClosesSilentConnectionAfterPongWait covers the gap flagged in
// docs/roadmap/TASKS.md and ADR-0005: a client that goes silent without a clean TCP
// close (e.g. a phone losing signal) must not leave its Hub entry and goroutines
// running forever. Here "silent" is simulated by a real client that connects and then
// never reads again — with no read pump running client-side, the server's pings never
// get answered — and readLoop must give up once pongWait elapses.
func TestClient_ReadLoop_ClosesSilentConnectionAfterPongWait(t *testing.T) {
	readLoopReturned := make(chan struct{})

	url := startHeartbeatTestServer(t, func(conn *fiberws.Conn) {
		c := newClient(conn, "game-1", "user-1")
		c.pongWait = 150 * time.Millisecond
		c.pingPeriod = 40 * time.Millisecond
		c.writeWait = 50 * time.Millisecond

		c.startWritePump()
		c.readLoop()
		c.Close()
		c.wait()
		close(readLoopReturned)
	})

	// Connect and go silent: no ReadMessage/NextReader call ever follows, so pings
	// from the server are never answered.
	dialHeartbeatTestClient(t, url)

	select {
	case <-readLoopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop never returned for a silently-dead client — the pongWait deadline was not enforced")
	}
}

// TestClient_ReadLoop_StaysAliveWhileClientRespondsToPings is the counterpart: a
// client that keeps its read pump running (so it answers pings with pongs, same as a
// real app/browser) must not be spuriously disconnected between ping periods.
func TestClient_ReadLoop_StaysAliveWhileClientRespondsToPings(t *testing.T) {
	readLoopReturned := make(chan struct{})

	url := startHeartbeatTestServer(t, func(conn *fiberws.Conn) {
		c := newClient(conn, "game-1", "user-1")
		c.pongWait = 150 * time.Millisecond
		c.pingPeriod = 40 * time.Millisecond
		c.writeWait = 50 * time.Millisecond

		c.startWritePump()
		c.readLoop()
		c.Close()
		c.wait()
		close(readLoopReturned)
	})

	client := dialHeartbeatTestClient(t, url)

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			// Blocks until a frame arrives; the default ping handler answers pings
			// with a pong before returning control here, refreshing readLoop's deadline.
			if _, _, err := client.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Comfortably longer than pongWait, but only reachable without disconnecting if
	// several ping/pong round-trips actually happened in between.
	time.Sleep(400 * time.Millisecond)

	select {
	case <-readLoopReturned:
		t.Fatal("readLoop returned even though the client kept answering pings — spurious disconnect")
	default:
	}
}
