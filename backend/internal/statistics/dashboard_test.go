package statistics_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/decks"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

// The dashboard's own list caps (see the dashboard* constants in service.go).
// Duplicated here rather than exported: a test that reads the same constant it
// checks would pass even if the endpoint started returning the whole collection.
const (
	wantDashboardDecks     = 4
	wantDashboardPlaygroup = 3
	wantDashboardGames     = 4
)

// dashboardFixture is an account to run the dashboard against: the caller, one
// rival who shares every game with them, and a playgroup to hold the games.
type dashboardFixture struct {
	statsSvc      statistics.Service
	gamesSvc      games.Service
	actionsSvc    gameactions.Service
	decksSvc      decks.Service
	playgroupsSvc playgroups.Service
	usersSvc      users.Service
	caller        *users.UserResponse
	rival         *users.UserResponse
	playgroupID   string
}

func setupDashboard(t *testing.T, pool *pgxpool.Pool) *dashboardFixture {
	t.Helper()
	ctx := context.Background()

	f := &dashboardFixture{
		statsSvc:      statistics.NewService(pool, playgroups.NewService(pool)),
		decksSvc:      decks.NewService(pool, noopMoxfieldClient{}),
		playgroupsSvc: playgroups.NewService(pool),
		usersSvc:      testutil.NewUsersService(pool),
		actionsSvc:    gameactions.NewService(pool, noopBroadcaster{}),
	}
	f.gamesSvc = games.NewService(pool, f.statsSvc, noopBroadcaster{}, f.playgroupsSvc)

	f.caller = f.registerUser(t, "caller")
	f.rival = f.registerUser(t, "rival")
	f.playgroupID = f.createPlaygroup(t, "Friday night")

	if _, err := f.playgroupsSvc.AddMember(ctx, f.playgroupID, f.caller.ID, playgroups.AddMemberRequest{
		UserID: f.rival.ID,
	}); err != nil {
		t.Fatalf("adding rival to the playgroup: %v", err)
	}
	return f
}

func (f *dashboardFixture) registerUser(t *testing.T, name string) *users.UserResponse {
	t.Helper()
	user, err := f.usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: name + "-" + t.Name(),
		Email:    name + "-" + t.Name() + "@example.com",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering %s: %v", name, err)
	}
	return user
}

func (f *dashboardFixture) createPlaygroup(t *testing.T, name string) string {
	t.Helper()
	pg, err := f.playgroupsSvc.CreatePlaygroup(context.Background(), f.caller.ID, playgroups.CreatePlaygroupRequest{
		Name: name,
	})
	if err != nil {
		t.Fatalf("creating playgroup %q: %v", name, err)
	}
	return pg.ID
}

func (f *dashboardFixture) createDeck(t *testing.T, ownerID, name string) string {
	t.Helper()
	deck, err := f.decksSvc.CreateDeck(context.Background(), ownerID, decks.CreateDeckRequest{
		Name: name, Commander: name + " commander",
	})
	if err != nil {
		t.Fatalf("creating deck %q: %v", name, err)
	}
	return deck.ID
}

// playGame runs one game to completion between the caller and the rival. The
// loser is eliminated by 40 combat damage (the auto-elimination at 0 life), so
// the survivor is the winner under the same sole-survivor rule the dashboard's
// queries use.
func (f *dashboardFixture) playGame(t *testing.T, callerDeckID, rivalDeckID string, callerWins bool) {
	t.Helper()
	game := mustCreateGame(t, f.gamesSvc, f.caller.ID, f.playgroupID)
	callerSeat := mustJoinReturningPlayerID(t, f.gamesSvc, game.ID, f.caller.ID, callerDeckID)
	rivalSeat := mustJoinReturningPlayerID(t, f.gamesSvc, game.ID, f.rival.ID, rivalDeckID)
	mustStartGame(t, f.gamesSvc, game.ID, f.caller.ID)

	if callerWins {
		mustRecordCombatDamage(t, f.actionsSvc, game.ID, f.caller.ID, callerSeat, rivalSeat)
	} else {
		mustRecordCombatDamage(t, f.actionsSvc, game.ID, f.rival.ID, rivalSeat, callerSeat)
	}
	mustFinishGame(t, f.gamesSvc, game.ID, f.caller.ID)
}

func mustGetDashboard(t *testing.T, svc statistics.Service, userID string) *statistics.DashboardResponse {
	t.Helper()
	res, err := svc.GetDashboard(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetDashboard() error = %v, want nil", err)
	}
	return res
}

func TestGetDashboard_FreshAccount_ReturnsZeroesAndEmptyLists(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)

	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "brand-new", Email: "brand-new@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}

	res := mustGetDashboard(t, statistics.NewService(pool, playgroups.NewService(pool)), user.ID)

	for name, got := range map[string]int32{
		"Stats.GamesPlayed": res.Stats.GamesPlayed,
		"Stats.GamesWon":    res.Stats.GamesWon,
		"TotalDecks":        res.TotalDecks,
		"TotalPlaygroups":   res.TotalPlaygroups,
		"Streak":            res.Streak,
	} {
		if got != 0 {
			t.Errorf("GetDashboard().%s = %d, want 0 for a fresh account", name, got)
		}
	}

	// Empty, not nil: a nil slice serializes as null, and the client iterates
	// these without a guard.
	for name, isNil := range map[string]bool{
		"Decks":       res.Decks == nil,
		"Playgroups":  res.Playgroups == nil,
		"RecentGames": res.RecentGames == nil,
	} {
		if isNil {
			t.Errorf("GetDashboard().%s is nil, want an empty slice", name)
		}
	}

	for name, length := range map[string]int{
		"Decks":       len(res.Decks),
		"Playgroups":  len(res.Playgroups),
		"RecentGames": len(res.RecentGames),
	} {
		if length != 0 {
			t.Errorf("GetDashboard().%s has %d entries, want none", name, length)
		}
	}

	if res.BestDeck != nil {
		t.Errorf("GetDashboard().BestDeck = %+v, want nil with no games played", res.BestDeck)
	}
	// Nil, not false: "no games yet" has to be distinguishable from a losing streak.
	if res.StreakWon != nil {
		t.Errorf("GetDashboard().StreakWon = %v, want nil with no games played", *res.StreakWon)
	}
}

func TestGetDashboard_RecentGameIsResolvedFromTheCallersSeat(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	callerDeck := f.createDeck(t, f.caller.ID, "Caller deck")
	rivalDeck := f.createDeck(t, f.rival.ID, "Rival deck")
	f.playGame(t, callerDeck, rivalDeck, true)

	res := mustGetDashboard(t, f.statsSvc, f.caller.ID)

	if len(res.RecentGames) != 1 {
		t.Fatalf("GetDashboard().RecentGames = %d entries, want 1", len(res.RecentGames))
	}
	game := res.RecentGames[0]
	if !game.Won {
		t.Error("RecentGames[0].Won = false, want true (the caller was the sole survivor)")
	}
	if game.Deck == nil {
		t.Fatalf("RecentGames[0].Deck = nil, want the caller's own deck (%s)", callerDeck)
	}
	if game.Deck.ID != callerDeck {
		t.Errorf("RecentGames[0].Deck = %q, want the caller's own deck (%s)", game.Deck.ID, callerDeck)
	}
	if len(game.Opponents) != 1 {
		t.Fatalf("RecentGames[0].Opponents = %v, want exactly one name", game.Opponents)
	}
	if game.Opponents[0] != f.rival.Username {
		t.Errorf("RecentGames[0].Opponents = %v, want [%s]", game.Opponents, f.rival.Username)
	}
	if game.PlaygroupName == nil {
		t.Fatal("RecentGames[0].PlaygroupName = nil, want the group the game was played in")
	}
	if *game.PlaygroupName != "Friday night" {
		t.Errorf("RecentGames[0].PlaygroupName = %q, want \"Friday night\"", *game.PlaygroupName)
	}
}

func TestGetDashboard_RecentGameFlipsForTheOtherSeat(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	callerDeck := f.createDeck(t, f.caller.ID, "Caller deck")
	rivalDeck := f.createDeck(t, f.rival.ID, "Rival deck")
	f.playGame(t, callerDeck, rivalDeck, true)

	// The same row read from the losing seat: their result, their deck -- not a
	// copy of the winner's view of the game.
	res := mustGetDashboard(t, f.statsSvc, f.rival.ID)

	if len(res.RecentGames) != 1 {
		t.Fatalf("the loser's RecentGames = %d entries, want 1", len(res.RecentGames))
	}
	if res.RecentGames[0].Won {
		t.Error("the loser's RecentGames[0].Won = true, want false")
	}
	if res.RecentGames[0].Deck == nil {
		t.Fatalf("the loser's RecentGames[0].Deck = nil, want their own deck (%s)", rivalDeck)
	}
	if res.RecentGames[0].Deck.ID != rivalDeck {
		t.Errorf("the loser's RecentGames[0].Deck = %q, want their own (%s)",
			res.RecentGames[0].Deck.ID, rivalDeck)
	}
}

func TestGetDashboard_TotalsCountBeyondTheListedSlices(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	const (
		deckCount      = wantDashboardDecks + 2
		playgroupCount = wantDashboardPlaygroup + 2 // + the one setupDashboard already made
	)
	for i := range deckCount {
		f.createDeck(t, f.caller.ID, fmt.Sprintf("Deck %d", i))
	}
	for i := range playgroupCount - 1 {
		f.createPlaygroup(t, fmt.Sprintf("Group %d", i))
	}

	res := mustGetDashboard(t, f.statsSvc, f.caller.ID)

	if res.TotalDecks != deckCount {
		t.Fatalf("GetDashboard().TotalDecks = %d, want %d (the real total, not the listed slice)",
			res.TotalDecks, deckCount)
	}
	if res.TotalPlaygroups != playgroupCount {
		t.Fatalf("GetDashboard().TotalPlaygroups = %d, want %d", res.TotalPlaygroups, playgroupCount)
	}
	if len(res.Decks) != wantDashboardDecks {
		t.Fatalf("GetDashboard().Decks = %d entries, want it capped at %d", len(res.Decks), wantDashboardDecks)
	}
	if len(res.Playgroups) != wantDashboardPlaygroup {
		t.Fatalf("GetDashboard().Playgroups = %d entries, want it capped at %d",
			len(res.Playgroups), wantDashboardPlaygroup)
	}
}

func TestGetDashboard_RecentGamesAreCappedAndMostRecentFirst(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	callerDeck := f.createDeck(t, f.caller.ID, "Caller deck")
	rivalDeck := f.createDeck(t, f.rival.ID, "Rival deck")
	for range wantDashboardGames + 2 {
		f.playGame(t, callerDeck, rivalDeck, true)
	}

	res := mustGetDashboard(t, f.statsSvc, f.caller.ID)

	if len(res.RecentGames) != wantDashboardGames {
		t.Fatalf("GetDashboard().RecentGames = %d entries, want it capped at %d",
			len(res.RecentGames), wantDashboardGames)
	}
	if res.Stats.GamesPlayed != wantDashboardGames+2 {
		t.Fatalf("GetDashboard().Stats.GamesPlayed = %d, want all %d games counted even though only %d are listed",
			res.Stats.GamesPlayed, wantDashboardGames+2, wantDashboardGames)
	}
}

func TestGetDashboard_StreakCountsBackFromTheMostRecentGame(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	callerDeck := f.createDeck(t, f.caller.ID, "Caller deck")
	rivalDeck := f.createDeck(t, f.rival.ID, "Rival deck")

	// Oldest to newest: a loss, then two wins. The streak is the two wins --
	// the older loss is what ends it, and must not be counted.
	f.playGame(t, callerDeck, rivalDeck, false)
	f.playGame(t, callerDeck, rivalDeck, true)
	f.playGame(t, callerDeck, rivalDeck, true)

	res := mustGetDashboard(t, f.statsSvc, f.caller.ID)

	if res.StreakWon == nil || !*res.StreakWon {
		t.Fatalf("GetDashboard().StreakWon = %v, want a winning streak", res.StreakWon)
	}
	if res.Streak != 2 {
		t.Fatalf("GetDashboard().Streak = %d, want 2 (the loss before them ends it)", res.Streak)
	}

	// The rival's games are the mirror image, so their streak is two losses.
	rivalView := mustGetDashboard(t, f.statsSvc, f.rival.ID)
	if rivalView.StreakWon == nil || *rivalView.StreakWon {
		t.Fatalf("the rival's StreakWon = %v, want a losing streak (false, not nil)", rivalView.StreakWon)
	}
	if rivalView.Streak != 2 {
		t.Fatalf("the rival's Streak = %d, want 2", rivalView.Streak)
	}
}

func TestGetDashboard_BestDeckIsTheHighestWinRateAmongPlayedDecks(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	f := setupDashboard(t, pool)

	rivalDeck := f.createDeck(t, f.rival.ID, "Rival deck")
	sharpDeck := f.createDeck(t, f.caller.ID, "Sharp") // 2 games, 2 wins
	blunt := f.createDeck(t, f.caller.ID, "Blunt")     // 3 games, 1 win
	f.createDeck(t, f.caller.ID, "Never sleeved")      // no games at all

	f.playGame(t, sharpDeck, rivalDeck, true)
	f.playGame(t, sharpDeck, rivalDeck, true)
	f.playGame(t, blunt, rivalDeck, true)
	f.playGame(t, blunt, rivalDeck, false)
	f.playGame(t, blunt, rivalDeck, false)

	res := mustGetDashboard(t, f.statsSvc, f.caller.ID)

	if res.BestDeck == nil {
		t.Fatal("GetDashboard().BestDeck = nil, want the 100% win rate deck")
	}
	if res.BestDeck.ID != sharpDeck {
		t.Fatalf("GetDashboard().BestDeck = %q (%d/%d), want Sharp -- the best win rate, not the most played",
			res.BestDeck.Name, res.BestDeck.GamesWon, res.BestDeck.GamesPlayed)
	}
	if res.BestDeck.GamesPlayed != 2 || res.BestDeck.GamesWon != 2 {
		t.Fatalf("GetDashboard().BestDeck record = %d/%d, want 2/2",
			res.BestDeck.GamesWon, res.BestDeck.GamesPlayed)
	}

	// Most played first, so the deck with the losing record leads the strip.
	if len(res.Decks) == 0 || res.Decks[0].ID != blunt {
		t.Fatalf("GetDashboard().Decks[0] = %+v, want the most played deck (Blunt)", res.Decks)
	}
}
