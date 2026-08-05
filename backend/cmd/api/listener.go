package main

import "net"

// noDelayListener wraps a net.Listener to disable Nagle's algorithm (TCP_NODELAY) on
// every accepted connection. fasthttp (which Fiber runs on) doesn't do this itself
// (see https://github.com/valyala/fasthttp/issues/241): on a connection kept alive
// across several requests, a client write split into two TCP segments (headers, then a
// JSON body) stalls for ~40ms waiting on Nagle plus the peer's delayed ACK before the
// server sees the body. Confirmed locally: a POST with a JSON body over a reused
// keep-alive connection took ~45ms versus ~5-7ms for the same request over a fresh
// connection, or for a bodyless GET on that same reused connection.
type noDelayListener struct {
	net.Listener
}

func (l *noDelayListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
	}
	return conn, nil
}
