package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The prompts are canned analyses a client shows as slash commands. They only
// phrase the request; the model gathers the data with the tools.

func registerPrompts(s *mcp.Server) {
	s.AddPrompt(&mcp.Prompt{
		Name:        "weekly-recap",
		Title:       "Weekly recap",
		Description: "Summary of the games played in the last 7 days.",
	}, staticPrompt("Weekly recap",
		"Use get_overview, then query_games with since set to 7 days ago. Summarize my week of "+
			"Commander: games played and won, which decks I played and how they did, who I played "+
			"against most, and anything notable (long games, streaks, a deck that keeps losing). "+
			"Keep it short and concrete, with numbers."))

	s.AddPrompt(&mcp.Prompt{
		Name:        "deck-review",
		Title:       "Deck review",
		Description: "How one deck performs, compared with the rest of my decks.",
		Arguments: []*mcp.PromptArgument{{
			Name: "deck", Description: "deck name, commander or ID", Required: true,
		}},
	}, deckReviewPrompt)

	s.AddPrompt(&mcp.Prompt{
		Name:        "rivalries",
		Title:       "Rivalries",
		Description: "Who I play against most, who beats me, and who I beat.",
	}, staticPrompt("Rivalries",
		"Use get_head_to_head and, for the top three opponents, query_games filtered by that "+
			"opponent. Tell me who my most frequent opponent is, my archenemy (who eliminates me "+
			"most), who I eliminate most, and which of their commanders give me trouble."))
}

func staticPrompt(description, text string) mcp.PromptHandler {
	return func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return userPrompt(description, text), nil
	}
}

func deckReviewPrompt(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	deck := req.Params.Arguments["deck"]
	if deck == "" {
		return nil, fmt.Errorf("%w: the deck argument is required", ErrInvalidInput)
	}
	return userPrompt("Deck review", fmt.Sprintf(
		"Use get_deck for %q and list_decks for context. Review it: record and win rate against my "+
			"average, how it does by playgroup and opponent in its recent games, whether its bracket "+
			"matches the tables it plays at, and one or two concrete conclusions.", deck)), nil
}

func userPrompt(description, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: description,
		Messages:    []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}},
	}
}
