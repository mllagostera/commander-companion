// Command mcp runs the Commander Companion MCP server over stdio, so an AI
// client (Claude Desktop, Claude Code, ...) can query one user's games, decks
// and statistics through the REST API. See ADR-0030 and backend/README.md.
//
// Configuration, from the environment:
//
//	COMMANDER_API_URL   API root including /api/v1 (default http://localhost:8080/api/v1)
//	COMMANDER_EMAIL     account email
//	COMMANDER_PASSWORD  account password
//
// stdout carries the MCP protocol, so every log line goes to stderr.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usuario/commander-companion-backend/internal/mcpserver"
)

const (
	defaultAPIURL  = "http://localhost:8080/api/v1"
	logoutTimeout  = 5 * time.Second
	unknownVersion = "dev"
)

func main() {
	log.SetOutput(os.Stderr)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apiURL := os.Getenv("COMMANDER_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	client, err := mcpserver.NewClient(&mcpserver.ClientConfig{
		BaseURL:  apiURL,
		Email:    os.Getenv("COMMANDER_EMAIL"),
		Password: os.Getenv("COMMANDER_PASSWORD"),
	})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Revoke the session on the way out, so each run leaves no refresh token
	// alive behind it. A fresh context: ctx is already canceled by then.
	defer func() {
		logoutCtx, cancel := context.WithTimeout(context.Background(), logoutTimeout)
		defer cancel()
		client.Logout(logoutCtx)
	}()

	server := mcpserver.NewServer(client, version())
	return server.Run(ctx, &mcp.StdioTransport{})
}

// version is the VCS revision the binary was built from, when Go recorded it.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownVersion
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return unknownVersion
}
