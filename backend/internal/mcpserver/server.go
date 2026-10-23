// Package mcpserver exposes the Commander Companion REST API to an AI client
// as an MCP server, for one user's personal use (ADR-0030).
//
// It is a client of the API, not part of it: every tool calls the endpoints
// docs/api/openapi.yaml documents, as the logged-in user, through the same
// handlers the web and Android clients use. Nothing here touches the database
// or imports another slice, so the contract stays the only way in.
//
// Phase 1 is read-only. Tools aggregate several endpoints into one answer per
// question ("my decks with their record"), resolve names to IDs, page through
// lists themselves, and return only the fields a model can use.
package mcpserver

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName = "commander-companion"
	// pageSize is what the tools ask for per page: the API's maximum.
	pageSize = 100
	// maxPages bounds how far a tool pages through one list, so a question
	// can't turn into an unbounded crawl. 10 pages = 1000 items.
	maxPages = 10
)

const instructions = `Commander Companion holds the user's Magic: The Gathering ` +
	`Commander games, decks, playgroups and tournaments. Every tool is read-only ` +
	`and answers as the logged-in user ("me"). Start with get_overview for ` +
	`context. Decks and playgroups can be named by ID or by name. A game is won ` +
	`by its sole survivor; win_rate_pct is games_won / games_played. Brackets ` +
	`(1-5) and color identity are explained in the resource ` +
	`commander://reference/brackets.`

// ErrAmbiguous and ErrNoMatch are returned when a name doesn't resolve to
// exactly one deck, playgroup or tournament. Their messages list what the
// model can do next.
var (
	ErrAmbiguous = errors.New("ambiguous name")
	ErrNoMatch   = errors.New("no match")
	// ErrInvalidInput is returned for an argument the tool rejects before
	// calling the API.
	ErrInvalidInput = errors.New("invalid input")
)

// NewServer builds the MCP server with every tool, prompt and resource.
func NewServer(c *Client, version string) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Title: "Commander Companion", Version: version},
		&mcp.ServerOptions{Instructions: instructions},
	)
	t := &tools{c: c}
	t.registerStatistics(s)
	t.registerDecks(s)
	t.registerPlaygroups(s)
	t.registerGames(s)
	t.registerTournaments(s)
	registerPrompts(s)
	registerResources(s)
	return s
}

type tools struct {
	c *Client
}

// readOnly is the annotation set every phase-1 tool carries.
func readOnly(title string) *mcp.ToolAnnotations {
	closedWorld := false
	return &mcp.ToolAnnotations{
		Title:          title,
		ReadOnlyHint:   true,
		IdempotentHint: true,
		OpenWorldHint:  &closedWorld,
	}
}

// page is the API's cursor-paginated list envelope.
type page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// visitFunc decides, per item, whether to keep it and whether to stop paging.
type visitFunc[T any] func(item *T) (keep, stop bool)

// fetchPages follows a paginated list until visit says stop, the cursor runs
// out, or maxPages is reached. truncated reports that more items exist than
// were looked at.
func fetchPages[T any](
	ctx context.Context, c *Client, path string, query url.Values, visit visitFunc[T],
) (items []T, truncated bool, err error) {
	q := url.Values{}
	for k, v := range query {
		q[k] = v
	}
	q.Set("limit", strconv.Itoa(pageSize))
	for range maxPages {
		var p page[T]
		if err := c.Get(ctx, path, q, &p); err != nil {
			return nil, false, err
		}
		for i := range p.Items {
			keep, stop := visit(&p.Items[i])
			if keep {
				items = append(items, p.Items[i])
			}
			if stop {
				return items, i < len(p.Items)-1 || p.NextCursor != nil, nil
			}
		}
		if p.NextCursor == nil {
			return items, false, nil
		}
		q.Set("cursor", *p.NextCursor)
	}
	return items, true, nil
}

// keepAll is the visitFunc for "every item".
func keepAll[T any](*T) (keep, stop bool) { return true, false }

// deckFilterQuery turns the bracket/colors/color_mode tool arguments into the
// API's query parameters (BracketFilterParam, ColorsFilterParam, ColorModeParam).
func deckFilterQuery(bracket []int, colors, colorMode string) url.Values {
	q := url.Values{}
	if len(bracket) > 0 {
		parts := make([]string, len(bracket))
		for i, b := range bracket {
			parts[i] = strconv.Itoa(b)
		}
		q.Set("bracket", strings.Join(parts, ","))
	}
	if colors != "" {
		q.Set("colors", colors)
		if colorMode != "" {
			q.Set("color_mode", colorMode)
		}
	}
	return q
}

func isUUID(s string) bool {
	_, err := uuid.Parse(strings.TrimSpace(s))
	return err == nil
}

// matchName picks the one candidate whose name matches ref: an exact
// (case-insensitive) match wins, otherwise a unique substring match.
// The second result lists the candidates when there isn't exactly one.
func matchName[T any](ref string, items []T, names func(*T) []string) (match *T, candidates []T) {
	needle := strings.ToLower(strings.TrimSpace(ref))
	var exact, partial []T
	for i := range items {
		for _, n := range names(&items[i]) {
			hay := strings.ToLower(n)
			if hay == needle {
				exact = append(exact, items[i])
				break
			}
			if strings.Contains(hay, needle) {
				partial = append(partial, items[i])
				break
			}
		}
	}
	switch {
	case len(exact) == 1:
		return &exact[0], nil
	case len(exact) > 1:
		return nil, exact
	case len(partial) == 1:
		return &partial[0], nil
	default:
		return nil, partial
	}
}
