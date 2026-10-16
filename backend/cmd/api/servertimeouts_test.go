package main

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	realws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	fiberws "github.com/gofiber/websocket/v2"
)

// shortTimeout stands in for every server timeout so the tests below take a
// second instead of a minute; the behavior under test doesn't depend on the
// exact durations.
const shortTimeout = 300 * time.Millisecond

// waitLongerThanTimeouts outlasts shortTimeout.
const waitLongerThanTimeouts = time.Second

// startTimeoutTestServer serves serverConfig(shortTimeouts) on a loopback port
// with one plain route and one WebSocket route that echoes every message.
func startTimeoutTestServer(t *testing.T) string {
	t.Helper()

	cfg := serverConfig(serverTimeouts{Read: shortTimeout, Write: shortTimeout, Idle: shortTimeout})
	cfg.DisableStartupMessage = true
	app := fiber.New(cfg)
	app.Get("/", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Use("/ws", func(c *fiber.Ctx) error {
		if !fiberws.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		return c.Next()
	})
	app.Get("/ws", fiberws.New(func(conn *fiberws.Conn) {
		for {
			kind, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(kind, msg); err != nil {
				return
			}
		}
	}))

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = app.Listener(ln) }()
	t.Cleanup(func() { _ = app.Shutdown() })
	return ln.Addr().String()
}

func TestServerConfig_DropsAClientThatNeverFinishesItsRequest(t *testing.T) {
	addr := startTimeoutTestServer(t)

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Half a request line, then silence: the slowloris pattern.
	if _, werr := conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")); werr != nil {
		t.Fatalf("write: %v", werr)
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * waitLongerThanTimeouts))
	_, err = io.ReadAll(conn)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("server kept the half-sent request open; ReadTimeout is not applied")
	}
}

func TestServerConfig_DoesNotCutAnIdleWebSocket(t *testing.T) {
	addr := startTimeoutTestServer(t)

	conn, resp, err := realws.DefaultDialer.Dial("ws://"+addr+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = resp.Body.Close()
	defer func() { _ = conn.Close() }()

	// Stay silent past every server timeout, as a game screen does between events.
	time.Sleep(waitLongerThanTimeouts)

	if err := conn.WriteMessage(realws.TextMessage, []byte("still there?")); err != nil {
		t.Fatalf("write after idling: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(waitLongerThanTimeouts))
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "still there?" {
		t.Fatalf("WebSocket was cut by the server timeouts: msg=%q err=%v", msg, err)
	}
}
