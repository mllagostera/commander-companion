package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const bracketsURI = "commander://reference/brackets"

// bracketsReference explains the vocabulary the tools' answers use, so the
// model doesn't guess what a bracket or a win means here.
const bracketsReference = `# Commander Companion: what the numbers mean

## Brackets (Wizards of the Coast's 1-5 Commander power scale)

1. Exhibition: ultra-casual, theme first.
2. Core: roughly a precon; no game changers, no mass land denial, no early two-card combos.
3. Upgraded: tuned beyond a precon; up to three game changers, no early two-card combos.
4. Optimized: high power, no restrictions beyond the banned list.
5. cEDH: competitive, built for the tournament metagame.

A deck's bracket comes from Moxfield on import and resync unless the owner set it
by hand. Missing means unknown.

## Color identity

WUBRG letters (White, blUe, Black, Red, Green) in that order. "colorless" means an
empty identity; "unknown" means it was never recorded.

## Snapshots

Every seat of a game records the deck's bracket and color identity as they were when
the game was played. query_games and get_breakdown filter and group on that snapshot,
not on the deck's current value, so a deck retuned from bracket 2 to 3 keeps its old
games at 2. list_decks and get_deck show the current value.

## Wins and statistics

A game counts once it is finished. The winner is its sole survivor; everyone else lost.
win_rate_pct is games_won / games_played as a percentage. Turn lengths only exist for
turns played with the turn timer.
`

func registerResources(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:         bracketsURI,
		Name:        "brackets",
		Title:       "Brackets, colors and how stats are counted",
		Description: "Reference for the vocabulary the tools use.",
		MIMEType:    "text/markdown",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: bracketsURI, MIMEType: "text/markdown", Text: bracketsReference,
		}}}, nil
	})
}
