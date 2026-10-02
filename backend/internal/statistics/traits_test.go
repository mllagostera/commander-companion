package statistics_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

func setDeckTraits(t *testing.T, pool *pgxpool.Pool, userID, deckID string, req decks.UpdateDeckRequest) {
	t.Helper()
	svc := decks.NewService(pool, noopMoxfieldClient{})
	if _, err := svc.UpdateDeck(context.Background(), userID, deckID, req); err != nil {
		t.Fatalf("UpdateDeck() error = %v", err)
	}
}

func finishedGames(t *testing.T, g *twoPlayerGame, filter common.DeckTraitFilter) []statistics.FinishedGameResponse {
	t.Helper()
	page, err := g.stats.ListFinishedGames(context.Background(), common.PageRequest{Limit: 10}, g.user1.ID, filter)
	if err != nil {
		t.Fatalf("ListFinishedGames() error = %v", err)
	}
	return page.Items
}

// setupFinishedTraitsGame plays and finishes a game that user1 wins, then
// gives user1's deck bracket 2 and colors WU. The seats were added before the
// deck had traits, so this is the backfill path.
func setupFinishedTraitsGame(t *testing.T, pool *pgxpool.Pool) *twoPlayerGame {
	t.Helper()
	g := setupTwoPlayerGame(t, pool, "irrelevant", "")
	setDeckTraits(t, pool, g.user1.ID, g.deck1ID, decks.UpdateDeckRequest{
		Bracket: json.RawMessage("2"), ColorIdentity: json.RawMessage(`["U","W"]`),
	})
	mustRecordElimination(t, g.actions, g.gameID, g.user1.ID, g.player1ID, g.player2ID)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)
	return g
}

// assertSeatSnapshots checks user1's seat carries (2, WU) and the opponent's is unknown.
func assertSeatSnapshots(t *testing.T, g *twoPlayerGame, players []statistics.FinishedGamePlayerResponse) {
	t.Helper()
	for i := range players {
		p := &players[i]
		own := p.UserID == g.user1.ID
		switch {
		case own && (p.DeckBracket == nil || *p.DeckBracket != 2 || len(p.DeckColorIdentity) != 2):
			t.Errorf("own seat = bracket %v colors %v, want the snapshot (2, WU)", p.DeckBracket, p.DeckColorIdentity)
		case !own && (p.DeckBracket != nil || p.DeckColorIdentity != nil):
			t.Errorf("opponent seat = bracket %v colors %v, want unknown", p.DeckBracket, p.DeckColorIdentity)
		}
	}
}

func TestFinishedGames_FilterBySeatSnapshot(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupFinishedTraitsGame(t, pool)

	cases := map[string]struct {
		filter common.DeckTraitFilter
		want   int
	}{
		"no filter":      {common.DeckTraitFilter{}, 1},
		"bracket 2":      {common.DeckTraitFilter{Brackets: []int16{2}}, 1},
		"bracket 4":      {common.DeckTraitFilter{Brackets: []int16{4}}, 0},
		"exact WU":       {common.DeckTraitFilter{Colors: []string{"W", "U"}, ColorMode: common.ColorModeExact}, 1},
		"exact B":        {common.DeckTraitFilter{Colors: []string{"B"}, ColorMode: common.ColorModeExact}, 0},
		"within WUB":     {common.DeckTraitFilter{Colors: []string{"W", "U", "B"}, ColorMode: common.ColorModeWithin}, 1},
		"includes WUB":   {common.DeckTraitFilter{Colors: []string{"W", "U", "B"}, ColorMode: common.ColorModeIncludes}, 0},
		"colorless only": {common.DeckTraitFilter{Colors: []string{}, ColorMode: common.ColorModeExact}, 0},
	}
	for name, tc := range cases {
		if got := len(finishedGames(t, g, tc.filter)); got != tc.want {
			t.Errorf("%s: %d games, want %d", name, got, tc.want)
		}
	}

	assertSeatSnapshots(t, g, finishedGames(t, g, common.DeckTraitFilter{})[0].Players)
}

// assertGamesPerBracket checks how many of user1's finished games each bracket filter returns.
func assertGamesPerBracket(t *testing.T, g *twoPlayerGame, want map[int16]int) {
	t.Helper()
	for bracket, n := range want {
		if got := len(finishedGames(t, g, common.DeckTraitFilter{Brackets: []int16{bracket}})); got != n {
			t.Errorf("bracket %d: %d games, want %d", bracket, got, n)
		}
	}
}

// A backfilled seat (its deck had no bracket when played) is only a guess, so
// it follows later corrections; a seat that got its value when it sat down
// (AddGamePlayer) never moves.
func TestFinishedGames_BackfilledSeatsFollowTheDeck_SeatedOnesDont(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupFinishedTraitsGame(t, pool) // game 1, backfilled with bracket 2

	// A typo corrected: the backfilled game follows it.
	setDeckTraits(t, pool, g.user1.ID, g.deck1ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("5")})
	setDeckTraits(t, pool, g.user1.ID, g.deck1ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("4")})
	assertGamesPerBracket(t, g, map[int16]int{2: 0, 5: 0, 4: 1})

	// Game 2 sits down with the deck already at bracket 4.
	second := mustCreateGame(t, g.games, g.user1.ID, "")
	p1 := mustJoinReturningPlayerID(t, g.games, second.ID, g.user1.ID, g.deck1ID)
	p2 := mustJoinReturningPlayerID(t, g.games, second.ID, g.user2.ID, g.deck2ID)
	mustStartGame(t, g.games, second.ID, g.user1.ID)
	mustRecordElimination(t, g.actions, second.ID, g.user1.ID, p1, p2)
	mustFinishGame(t, g.games, second.ID, g.user1.ID)

	// The deck is retuned: game 1 (a guess) moves, game 2 (what was played) stays.
	setDeckTraits(t, pool, g.user1.ID, g.deck1ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("1")})
	assertGamesPerBracket(t, g, map[int16]int{1: 1, 4: 1, 2: 0})

	// The colors were never touched again: game 1 still has the backfilled WU.
	if got := len(finishedGames(t, g, common.DeckTraitFilter{
		Colors: []string{"W", "U"}, ColorMode: common.ColorModeExact,
	})); got != 2 {
		t.Errorf("exact WU: %d games, want 2 (both seats have WU)", got)
	}
}

func TestGetBreakdown_ByBracket(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupFinishedTraitsGame(t, pool)

	res, err := g.stats.GetBreakdown(context.Background(), g.user1.ID, statistics.GroupByBracket)
	if err != nil {
		t.Fatalf("GetBreakdown(bracket) error = %v", err)
	}
	if len(res.Items) != 1 || *res.Items[0].Bracket != 2 || res.Items[0].GamesPlayed != 1 || res.Items[0].GamesWon != 1 {
		t.Fatalf("GetBreakdown(bracket) = %+v, want one bracket-2 group with 1 game, 1 win", res.Items)
	}

	if _, err := g.stats.GetBreakdown(context.Background(), g.user1.ID, "deck"); err == nil {
		t.Fatal("GetBreakdown(deck) error = nil, want invalid group_by")
	}
}

// The loser's deck never got traits: their only group is the unknown one,
// serialized with just the color_identity key, as null.
func TestGetBreakdown_UnknownColorIdentityGroup(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupFinishedTraitsGame(t, pool)

	res, err := g.stats.GetBreakdown(context.Background(), g.user2.ID, statistics.GroupByColorIdentity)
	if err != nil {
		t.Fatalf("GetBreakdown(color_identity) error = %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("GetBreakdown(color_identity) = %+v, want one group", res.Items)
	}
	raw, err := json.Marshal(res.Items[0])
	if err != nil || string(raw) != `{"color_identity":null,"games_played":1,"games_won":0}` {
		t.Fatalf("item JSON = %s (%v), want only the color_identity key, null, with no wins", raw, err)
	}
}
