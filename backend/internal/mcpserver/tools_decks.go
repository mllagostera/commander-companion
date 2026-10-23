package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// deckRecentGames is how many of a deck's games get_deck returns, found among
// the user's last deckGamesScanned finished games.
const (
	deckRecentGames  = 10
	deckGamesScanned = 2 * pageSize
)

func (t *tools) registerDecks(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_decks",
		Description: "Every deck of the user with its bracket, color identity and record, " +
			"most played first. Optionally filtered by bracket and color identity.",
		Annotations: readOnly("List decks"),
	}, t.listDecks)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_deck",
		Description: "One deck of the user, by ID, name or commander: its record, highest " +
			"life total, commander damage dealt and its most recent games.",
		Annotations: readOnly("Deck detail"),
	}, t.getDeck)
}

// --- list_decks -----------------------------------------------------------

// ListDecksInput is list_decks' arguments.
type ListDecksInput struct {
	Bracket   []int  `json:"bracket,omitempty" jsonschema:"only decks in one of these brackets (1-5)"`
	Colors    string `json:"colors,omitempty" jsonschema:"color identity: WUBRG letters (e.g. UB) or C for colorless"`
	ColorMode string `json:"color_mode,omitempty" jsonschema:"how colors match: exact (default), includes or within"`
}

// ListDecksOutput is list_decks' answer.
type ListDecksOutput struct {
	Count     int          `json:"count"`
	Decks     []DeckRecord `json:"decks"`
	Truncated bool         `json:"truncated" jsonschema:"the user has more decks than were listed"`
}

func (t *tools) listDecks(
	ctx context.Context, _ *mcp.CallToolRequest, in ListDecksInput,
) (*mcp.CallToolResult, ListDecksOutput, error) {
	query := deckFilterQuery(in.Bracket, in.Colors, in.ColorMode)
	decks, truncated, err := fetchPages(ctx, t.c, pathDecks, query, keepAll[apiDeck])
	if err != nil {
		return nil, ListDecksOutput{}, err
	}
	var stats []apiDeckStats
	if err := t.c.Get(ctx, pathDeckStatistics, nil, &stats); err != nil {
		return nil, ListDecksOutput{}, err
	}
	byDeck := make(map[string]apiDeckStats, len(stats))
	for _, st := range stats {
		byDeck[st.DeckID] = st
	}

	out := ListDecksOutput{Decks: make([]DeckRecord, 0, len(decks)), Truncated: truncated}
	for i := range decks {
		st := byDeck[decks[i].ID]
		out.Decks = append(out.Decks, deckRecord(&decks[i], st.GamesPlayed, st.GamesWon))
	}
	slices.SortStableFunc(out.Decks, func(a, b DeckRecord) int {
		return cmp.Or(cmp.Compare(b.GamesPlayed, a.GamesPlayed), strings.Compare(a.Name, b.Name))
	})
	out.Count = len(out.Decks)
	return nil, out, nil
}

func deckRecord(d *apiDeck, played, won int) DeckRecord {
	return DeckRecord{
		ID:            d.ID,
		Name:          d.Name,
		Commander:     d.Commander,
		Bracket:       d.Bracket,
		ColorIdentity: d.ColorIdentity,
		GamesPlayed:   played,
		GamesWon:      won,
		WinRatePct:    winRate(won, played),
	}
}

// --- get_deck -------------------------------------------------------------

// GetDeckInput is get_deck's arguments.
type GetDeckInput struct {
	Deck string `json:"deck" jsonschema:"deck ID, deck name or commander name (partial names work if unique)"`
}

// DeckDetail is get_deck's answer.
type DeckDetail struct {
	DeckRecord
	HighestLifeTotalAchieved  int          `json:"highest_life_total_achieved"`
	TotalCommanderDamageDealt int          `json:"total_commander_damage_dealt"`
	RecentGames               []RecentGame `json:"recent_games" jsonschema:"up to 10, among the user's last 200 games"`
}

func (t *tools) getDeck(
	ctx context.Context, _ *mcp.CallToolRequest, in GetDeckInput,
) (*mcp.CallToolResult, DeckDetail, error) {
	deck, err := t.resolveDeck(ctx, in.Deck)
	if err != nil {
		return nil, DeckDetail{}, err
	}
	var st apiDeckStats
	if statsErr := t.c.Get(ctx, withID(pathOneDeckStatistics, deck.ID), nil, &st); statsErr != nil {
		return nil, DeckDetail{}, statsErr
	}
	recent, err := t.recentGamesWithDeck(ctx, deck.ID)
	if err != nil {
		return nil, DeckDetail{}, err
	}
	return nil, DeckDetail{
		DeckRecord:                deckRecord(deck, st.GamesPlayed, st.GamesWon),
		HighestLifeTotalAchieved:  st.HighestLifeTotalAchieved,
		TotalCommanderDamageDealt: st.TotalCommanderDamageDealt,
		RecentGames:               recent,
	}, nil
}

// resolveDeck finds one of the user's decks by ID, name or commander.
func (t *tools) resolveDeck(ctx context.Context, ref string) (*apiDeck, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("%w: deck is required", ErrInvalidInput)
	}
	if isUUID(ref) {
		var d apiDeck
		if err := t.c.Get(ctx, withID(pathDeck, strings.TrimSpace(ref)), nil, &d); err != nil {
			return nil, err
		}
		return &d, nil
	}
	decks, _, err := fetchPages(ctx, t.c, pathDecks, nil, keepAll[apiDeck])
	if err != nil {
		return nil, err
	}
	match, candidates := matchName(ref, decks, func(d *apiDeck) []string { return []string{d.Name, d.Commander} })
	if match != nil {
		return match, nil
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no deck is named %q or has it as commander; call list_decks", ErrNoMatch, ref)
	}
	names := make([]string, 0, len(candidates))
	for i := range candidates {
		names = append(names, fmt.Sprintf("%s (%s, id %s)", candidates[i].Name, candidates[i].Commander, candidates[i].ID))
	}
	return nil, fmt.Errorf("%w: %q matches %d decks: %s; ask which one, or pass its id",
		ErrAmbiguous, ref, len(names), strings.Join(names, "; "))
}

// recentGamesWithDeck returns the user's latest games played with deckID.
// There is no per-deck filter in the API, so it scans the finished-games
// history, which is already enriched with each seat's deck.
func (t *tools) recentGamesWithDeck(ctx context.Context, deckID string) ([]RecentGame, error) {
	myID, _, err := t.c.Me(ctx)
	if err != nil {
		return nil, err
	}
	scanned := 0
	games, _, err := fetchPages(ctx, t.c, pathFinishedGames, nil, func(g *FinishedGame) (keep, stop bool) {
		scanned++
		keep = seatOf(g, myID).DeckID == deckID
		return keep, scanned >= deckGamesScanned
	})
	if err != nil {
		return nil, err
	}
	out := []RecentGame{}
	for i := range games {
		if len(out) == deckRecentGames {
			break
		}
		out = append(out, recentGame(&games[i], myID))
	}
	return out, nil
}

// recentGame folds a finished game into the caller's point of view.
func recentGame(g *FinishedGame, myID string) RecentGame {
	mine := seatOf(g, myID)
	rg := RecentGame{
		ID:            g.ID,
		PlaygroupName: g.PlaygroupName,
		FinishedAt:    g.FinishedAt,
		Won:           mine.Won,
		DeckName:      mine.DeckName,
		Commander:     mine.Commander,
		Opponents:     []string{},
	}
	for i := range g.Players {
		if g.Players[i].UserID != myID {
			rg.Opponents = append(rg.Opponents, g.Players[i].Username)
		}
	}
	return rg
}
