package statistics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "test-password-123"

// noopMoxfieldClient satisfies decks.MoxfieldClient without hitting the real API;
// these tests never import decks from Moxfield, they only need to create their own decks.
type noopMoxfieldClient struct{}

func (noopMoxfieldClient) GetDeck(_ context.Context, _ string) (*moxfield.Deck, error) {
	return nil, nil //nolint:nilnil // stub never invoked in these tests
}

// noopBroadcaster satisfies both games.Broadcaster and gameactions.Broadcaster without
// actually broadcasting anything: these tests don't exercise internal/websocket, they
// just need the dependency to be present to be able to construct the services.
type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastGameFinished(_ string)                              {}
func (noopBroadcaster) BroadcastAction(_ string, _ *gameactions.GameActionResponse) {}

func truncateStatsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games"/"playgroups" clean up game_players/game_actions/playgroup_members via CASCADE;
	// "users" cleans up decks and the statistics summaries (FK to users/decks).
	testutil.Truncate(t, pool, "games", "playgroups", "users")
}

// twoPlayerGame groups all the services and IDs of a 2-player game ready to
// be played, to avoid repeating the setup in every test.
type twoPlayerGame struct {
	games     games.Service
	actions   gameactions.Service
	stats     statistics.Service
	gameID    string
	user1     *users.UserResponse
	deck1ID   string
	player1ID string
	user2     *users.UserResponse
	deck2ID   string
	player2ID string
}

func setupTwoPlayerGame(t *testing.T, pool *pgxpool.Pool, creatorID, playgroupID string) *twoPlayerGame {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	statsSvc := statistics.NewService(pool, playgroups.NewService(pool))
	gamesSvc := games.NewService(pool, statsSvc, noopBroadcaster{}, playgroups.NewService(pool))
	actionsSvc := gameactions.NewService(pool, noopBroadcaster{})

	user1, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "p1-" + t.Name(), Email: "p1-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user 1: %v", err)
	}
	user2, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "p2-" + t.Name(), Email: "p2-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user 2: %v", err)
	}

	deck1, err := decksSvc.CreateDeck(ctx, user1.ID, decks.CreateDeckRequest{Name: "D1", Commander: "C1"})
	if err != nil {
		t.Fatalf("creating deck 1: %v", err)
	}
	deck2, err := decksSvc.CreateDeck(ctx, user2.ID, decks.CreateDeckRequest{Name: "D2", Commander: "C2"})
	if err != nil {
		t.Fatalf("creating deck 2: %v", err)
	}

	game := mustCreateGame(t, gamesSvc, creatorID, playgroupID)
	p1, err := gamesSvc.JoinGame(ctx, game.ID, user1.ID, games.JoinGameRequest{DeckID: deck1.ID})
	if err != nil {
		t.Fatalf("JoinGame(1) error = %v", err)
	}
	p2, err := gamesSvc.JoinGame(ctx, game.ID, user2.ID, games.JoinGameRequest{DeckID: deck2.ID})
	if err != nil {
		t.Fatalf("JoinGame(2) error = %v", err)
	}
	mustStartGame(t, gamesSvc, game.ID, user1.ID)

	return &twoPlayerGame{
		games: gamesSvc, actions: actionsSvc, stats: statsSvc,
		gameID: game.ID,
		user1:  user1, deck1ID: deck1.ID, player1ID: p1.ID,
		user2: user2, deck2ID: deck2.ID, player2ID: p2.ID,
	}
}

func mustCreateGame(t *testing.T, svc games.Service, creatorID, playgroupID string) *games.GameResponse {
	t.Helper()
	game, err := svc.CreateGame(context.Background(), creatorID, games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	return game
}

func mustJoin(t *testing.T, svc games.Service, gameID, userID, deckID string) {
	t.Helper()
	_, err := svc.JoinGame(context.Background(), gameID, userID, games.JoinGameRequest{DeckID: deckID})
	if err != nil {
		t.Fatalf("JoinGame(%s) error = %v", userID, err)
	}
}

func mustStartGame(t *testing.T, svc games.Service, gameID, userID string) {
	t.Helper()
	_, err := svc.StartGame(context.Background(), gameID, userID)
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}
}

func mustFinishGame(t *testing.T, svc games.Service, gameID, userID string) {
	t.Helper()
	_, err := svc.FinishGame(context.Background(), gameID, userID)
	if err != nil {
		t.Fatalf("FinishGame() error = %v, want nil", err)
	}
}

func mustRecordCombatDamage(
	t *testing.T, svc gameactions.Service, gameID, callerID, actorID, targetID string, amount float64,
) {
	t.Helper()
	_, err := svc.RecordAction(context.Background(), gameID, callerID, gameactions.CreateActionRequest{
		ActorID: actorID, TargetID: targetID, ActionType: "CombatDamage",
		Payload: map[string]interface{}{"amount": amount},
	})
	if err != nil {
		t.Fatalf("RecordAction(CombatDamage) error = %v", err)
	}
}

func mustRecordElimination(t *testing.T, svc gameactions.Service, gameID, callerID, actorID, targetID string) {
	t.Helper()
	_, err := svc.RecordAction(context.Background(), gameID, callerID, gameactions.CreateActionRequest{
		ActorID: actorID, TargetID: targetID, ActionType: "Elimination",
	})
	if err != nil {
		t.Fatalf("RecordAction(Elimination) error = %v", err)
	}
}

func mustJoinReturningPlayerID(t *testing.T, svc games.Service, gameID, userID, deckID string) string {
	t.Helper()
	p, err := svc.JoinGame(context.Background(), gameID, userID, games.JoinGameRequest{DeckID: deckID})
	if err != nil {
		t.Fatalf("JoinGame(%s) error = %v", userID, err)
	}
	return p.ID
}

func mustGetUserStats(t *testing.T, svc statistics.Service, userID string) *statistics.UserStatsResponse {
	t.Helper()
	stats, err := svc.GetUserStats(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserStats() error = %v, want nil", err)
	}
	return stats
}

func mustGetDeckStats(t *testing.T, svc statistics.Service, userID, deckID string) *statistics.DeckStatsResponse {
	t.Helper()
	stats, err := svc.GetDeckStats(context.Background(), userID, deckID)
	if err != nil {
		t.Fatalf("GetDeckStats() error = %v, want nil", err)
	}
	return stats
}

func createFounder(t *testing.T, pool *pgxpool.Pool) *users.UserResponse {
	t.Helper()
	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "founder-" + t.Name(), Email: "founder-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering playgroup founder: %v", err)
	}
	return user
}

func TestGetUserStats_NoGamesYet_ReturnsZeroValues(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)

	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "fresh-user", Email: "fresh-user@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}

	res := mustGetUserStats(t, statistics.NewService(pool, playgroups.NewService(pool)), user.ID)
	if res.GamesPlayed != 0 || res.GamesWon != 0 {
		t.Fatalf("GetUserStats() for a user with no games = %+v, want everything zero", res)
	}
}

func TestGetDeckStats_OwnedByAnotherUser_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	owner, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "deck-owner", Email: "deck-owner@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}
	intruder, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "deck-intruder", Email: "deck-intruder@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}
	deck, err := decksSvc.CreateDeck(ctx, owner.ID, decks.CreateDeckRequest{Name: "D", Commander: "C"})
	if err != nil {
		t.Fatalf("creating deck: %v", err)
	}

	svc := statistics.NewService(pool, playgroups.NewService(pool))
	_, err = svc.GetDeckStats(ctx, intruder.ID, deck.ID)
	if !errors.Is(err, statistics.ErrDeckNotFound) {
		t.Fatalf("GetDeckStats() for someone else's deck: error = %v, want ErrDeckNotFound", err)
	}
}

func TestRecalculateForGame_WinnerGetsCreditForWinAndDamage(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupTwoPlayerGame(t, pool, "irrelevant", "")

	// player1 eliminates player2 with pure CombatDamage; player1 survives at full life.
	mustRecordCombatDamage(t, g.actions, g.gameID, g.user1.ID, g.player1ID, g.player2ID, 40)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)

	winnerStats := mustGetUserStats(t, g.stats, g.user1.ID)
	if winnerStats.GamesPlayed != 1 || winnerStats.GamesWon != 1 {
		t.Fatalf("GetUserStats(winner) = %+v, want games_played=1 games_won=1", winnerStats)
	}
	if winnerStats.TotalDamageDealt != 40 {
		t.Fatalf("GetUserStats(winner).TotalDamageDealt = %d, want 40", winnerStats.TotalDamageDealt)
	}

	loserStats := mustGetUserStats(t, g.stats, g.user2.ID)
	if loserStats.GamesPlayed != 1 || loserStats.GamesWon != 0 {
		t.Fatalf("GetUserStats(loser) = %+v, want games_played=1 games_won=0", loserStats)
	}

	winnerDeckStats := mustGetDeckStats(t, g.stats, g.user1.ID, g.deck1ID)
	if winnerDeckStats.GamesWon != 1 || winnerDeckStats.HighestLifeTotal != 40 {
		t.Fatalf("GetDeckStats(winner) = %+v, want games_won=1 highest_life_total=40", winnerDeckStats)
	}

	loserDeckStats := mustGetDeckStats(t, g.stats, g.user2.ID, g.deck2ID)
	if loserDeckStats.HighestLifeTotal != 40 {
		t.Fatalf(
			"GetDeckStats(loser).HighestLifeTotal = %d, want 40 (never gained extra life)",
			loserDeckStats.HighestLifeTotal,
		)
	}
}

func TestRecalculateForGame_NoSurvivors_NoWinnerCredited(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupTwoPlayerGame(t, pool, "irrelevant", "")

	// Game manually cut short with nobody eliminated: 2 survivors, no winner.
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)

	stats1 := mustGetUserStats(t, g.stats, g.user1.ID)
	stats2 := mustGetUserStats(t, g.stats, g.user2.ID)
	if stats1.GamesPlayed != 1 || stats1.GamesWon != 0 || stats2.GamesPlayed != 1 || stats2.GamesWon != 0 {
		t.Fatalf("with no clear winner, nobody should get games_won: p1=%+v p2=%+v", stats1, stats2)
	}
}

func TestRecalculateForGame_AccumulatesAcrossGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	g1 := setupTwoPlayerGame(t, pool, "irrelevant", "")
	mustFinishGame(t, g1.games, g1.gameID, g1.user1.ID)

	// setupTwoPlayerGame creates new users on every call; to test real
	// accumulation you need to play twice with the SAME user, so the
	// second game is built by hand reusing g1.user1/g1.deck1ID.
	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	statsSvc := statistics.NewService(pool, playgroups.NewService(pool))
	gamesSvc := games.NewService(pool, statsSvc, noopBroadcaster{}, playgroups.NewService(pool))

	opponent, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "opponent-" + t.Name(), Email: "opponent-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering opponent: %v", err)
	}
	opponentDeck, err := decksSvc.CreateDeck(ctx, opponent.ID, decks.CreateDeckRequest{Name: "OD", Commander: "OC"})
	if err != nil {
		t.Fatalf("creating opponent's deck: %v", err)
	}

	game2 := mustCreateGame(t, gamesSvc, "irrelevant", "")
	mustJoin(t, gamesSvc, game2.ID, g1.user1.ID, g1.deck1ID)
	mustJoin(t, gamesSvc, game2.ID, opponent.ID, opponentDeck.ID)
	mustStartGame(t, gamesSvc, game2.ID, g1.user1.ID)
	mustFinishGame(t, gamesSvc, game2.ID, g1.user1.ID)

	stats := mustGetUserStats(t, statsSvc, g1.user1.ID)
	if stats.GamesPlayed != 2 {
		t.Fatalf("GetUserStats() after 2 games by the same user: games_played = %d, want 2", stats.GamesPlayed)
	}
}

// setupListDeckStatsFixture plays one finished game (player1 wins with a
// combat-damage kill) and adds a second, never-played deck for user1, so
// ListDeckStats has both a played and an unplayed deck to report on.
func setupListDeckStatsFixture(t *testing.T, pool *pgxpool.Pool) (*twoPlayerGame, *decks.DeckResponse) {
	t.Helper()
	ctx := context.Background()
	g := setupTwoPlayerGame(t, pool, "irrelevant", "")

	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	unplayedDeck, err := decksSvc.CreateDeck(ctx, g.user1.ID, decks.CreateDeckRequest{Name: "D3", Commander: "C3"})
	if err != nil {
		t.Fatalf("creating unplayed deck: %v", err)
	}

	mustRecordCombatDamage(t, g.actions, g.gameID, g.user1.ID, g.player1ID, g.player2ID, 40)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)
	return g, unplayedDeck
}

// TestListDeckStats_ReturnsEveryDeckIncludingUnplayed is the regression test
// for the web dashboard's N+1: before ListDeckStats existed, showing stats
// for every deck meant one GetDeckStats call per deck. It must return every
// deck the user owns in one call, with zeros for a deck never played (same
// as GetDeckStats).
func TestListDeckStats_ReturnsEveryDeckIncludingUnplayed(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g, unplayedDeck := setupListDeckStatsFixture(t, pool)

	all, err := g.stats.ListDeckStats(context.Background(), g.user1.ID)
	if err != nil {
		t.Fatalf("ListDeckStats() error = %v, want nil", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListDeckStats() returned %d decks, want 2 (played + unplayed)", len(all))
	}

	statsByDeckID := make(map[string]statistics.DeckStatsResponse, len(all))
	for _, s := range all {
		statsByDeckID[s.DeckID] = s
	}

	played := statsByDeckID[g.deck1ID]
	if played.GamesWon != 1 || played.HighestLifeTotal != 40 {
		t.Fatalf("ListDeckStats()[played] = %+v, want games_won=1 highest_life_total=40", played)
	}
	unplayed := statsByDeckID[unplayedDeck.ID]
	if unplayed.GamesPlayed != 0 || unplayed.GamesWon != 0 {
		t.Fatalf("ListDeckStats()[unplayed] = %+v, want everything zero", unplayed)
	}
}

// TestListDeckStats_OnlyReturnsOwnDecks guards against leaking another
// user's decks through the join: user2's list must contain exactly their own deck.
func TestListDeckStats_OnlyReturnsOwnDecks(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g, _ := setupListDeckStatsFixture(t, pool)

	otherUsersStats, err := g.stats.ListDeckStats(context.Background(), g.user2.ID)
	if err != nil {
		t.Fatalf("ListDeckStats() error = %v, want nil", err)
	}
	if len(otherUsersStats) != 1 || otherUsersStats[0].DeckID != g.deck2ID {
		t.Fatalf("ListDeckStats(user2) = %+v, want only deck2ID (%s)", otherUsersStats, g.deck2ID)
	}
}

func TestGetPlaygroupStats_AggregatesFinishedGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	playgroupsSvc := playgroups.NewService(pool)
	// The playgroup's creator doesn't take part in the game; only the group's ID is used.
	founder := createFounder(t, pool)
	playgroup, err := playgroupsSvc.CreatePlaygroup(ctx, founder.ID, playgroups.CreatePlaygroupRequest{Name: "Liga"})
	if err != nil {
		t.Fatalf("CreatePlaygroup() error = %v", err)
	}

	g := setupTwoPlayerGame(t, pool, founder.ID, playgroup.ID)
	mustRecordCombatDamage(t, g.actions, g.gameID, g.user1.ID, g.player1ID, g.player2ID, 40)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)

	res, err := g.stats.GetPlaygroupStats(ctx, playgroup.ID, founder.ID)
	if err != nil {
		t.Fatalf("GetPlaygroupStats() error = %v, want nil", err)
	}
	if res.GamesPlayed != 1 {
		t.Fatalf("GetPlaygroupStats().GamesPlayed = %d, want 1", res.GamesPlayed)
	}
	assertPlaygroupMemberStats(t, res.Members, g.user1.ID)
}

func TestGetPlaygroupStats_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	playgroupsSvc := playgroups.NewService(pool)
	founder := createFounder(t, pool)
	playgroup, err := playgroupsSvc.CreatePlaygroup(ctx, founder.ID, playgroups.CreatePlaygroupRequest{Name: "Liga"})
	if err != nil {
		t.Fatalf("CreatePlaygroup() error = %v", err)
	}
	usersSvc := testutil.NewUsersService(pool)
	outsider, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "playgroup-stats-outsider", Email: "playgroup-stats-outsider@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}

	// Regression test: before the fix, any authenticated user with the
	// playgroup's UUID could see its aggregated member statistics.
	svc := statistics.NewService(pool, playgroupsSvc)
	_, err = svc.GetPlaygroupStats(ctx, playgroup.ID, outsider.ID)
	if !errors.Is(err, statistics.ErrPlaygroupNotFound) {
		t.Fatalf("GetPlaygroupStats() for a non-member: error = %v, want ErrPlaygroupNotFound", err)
	}
}

func TestListOpponentStats_NoSharedGames_ReturnsEmpty(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)

	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "lone-user", Email: "lone-user@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registering user: %v", err)
	}

	res, err := statistics.NewService(pool, playgroups.NewService(pool)).ListOpponentStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("ListOpponentStats() error = %v, want nil", err)
	}
	if len(res) != 0 {
		t.Fatalf("ListOpponentStats() for a user with no games = %+v, want empty", res)
	}
}

// TestListOpponentStats_AggregatesAcrossGames plays 2 finished games between
// the same 2 users (player1 eliminates player2 both times) and checks the
// head-to-head record accumulates correctly for both sides.
func TestListOpponentStats_AggregatesAcrossGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)

	g1 := setupTwoPlayerGame(t, pool, "irrelevant", "")
	mustRecordElimination(t, g1.actions, g1.gameID, g1.user1.ID, g1.player1ID, g1.player2ID)
	mustFinishGame(t, g1.games, g1.gameID, g1.user1.ID)

	// Second game, same 2 users (setupTwoPlayerGame always creates fresh users,
	// so this one is built by hand reusing g1's).
	game2 := mustCreateGame(t, g1.games, "irrelevant", "")
	player1ID := mustJoinReturningPlayerID(t, g1.games, game2.ID, g1.user1.ID, g1.deck1ID)
	player2ID := mustJoinReturningPlayerID(t, g1.games, game2.ID, g1.user2.ID, g1.deck2ID)
	mustStartGame(t, g1.games, game2.ID, g1.user1.ID)
	mustRecordElimination(t, g1.actions, game2.ID, g1.user1.ID, player1ID, player2ID)
	mustFinishGame(t, g1.games, game2.ID, g1.user1.ID)

	assertOpponentStats(t, g1.stats, g1.user1.ID, g1.user2.ID, 2, 2, 0)
	assertOpponentStats(t, g1.stats, g1.user2.ID, g1.user1.ID, 2, 0, 2)
}

func assertOpponentStats(
	t *testing.T, svc statistics.Service, viewerID, opponentID string,
	wantGames, wantYouEliminated, wantEliminatedBy int32,
) {
	t.Helper()
	res, err := svc.ListOpponentStats(context.Background(), viewerID)
	if err != nil {
		t.Fatalf("ListOpponentStats(%s) error = %v, want nil", viewerID, err)
	}
	if len(res) != 1 || res[0].UserID != opponentID {
		t.Fatalf("ListOpponentStats(%s) = %+v, want exactly opponent %s", viewerID, res, opponentID)
	}
	got := res[0]
	if got.GamesTogether != wantGames || got.TimesYouEliminatedThem != wantYouEliminated ||
		got.TimesEliminatedByOpponent != wantEliminatedBy {
		t.Fatalf(
			"ListOpponentStats(%s)[0] = %+v, want games_together=%d you_eliminated=%d eliminated_by=%d",
			viewerID, got, wantGames, wantYouEliminated, wantEliminatedBy,
		)
	}
}

func TestListPlaygroupGameCounts_IncludesGroupsWithZeroFinishedGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	playgroupsSvc := playgroups.NewService(pool)
	founder := createFounder(t, pool)
	played, err := playgroupsSvc.CreatePlaygroup(ctx, founder.ID, playgroups.CreatePlaygroupRequest{Name: "Played"})
	if err != nil {
		t.Fatalf("CreatePlaygroup(played) error = %v", err)
	}
	empty, err := playgroupsSvc.CreatePlaygroup(ctx, founder.ID, playgroups.CreatePlaygroupRequest{Name: "Empty"})
	if err != nil {
		t.Fatalf("CreatePlaygroup(empty) error = %v", err)
	}

	g := setupTwoPlayerGame(t, pool, founder.ID, played.ID)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)
	_, err = playgroupsSvc.AddMember(ctx, played.ID, founder.ID, playgroups.AddMemberRequest{UserID: g.user1.ID})
	if err != nil {
		t.Fatalf("AddMember(played) error = %v", err)
	}
	_, err = playgroupsSvc.AddMember(ctx, empty.ID, founder.ID, playgroups.AddMemberRequest{UserID: g.user1.ID})
	if err != nil {
		t.Fatalf("AddMember(empty) error = %v", err)
	}

	res, err := g.stats.ListPlaygroupGameCounts(ctx, g.user1.ID)
	if err != nil {
		t.Fatalf("ListPlaygroupGameCounts() error = %v, want nil", err)
	}
	countsByGroup := make(map[string]int32, len(res))
	for _, r := range res {
		countsByGroup[r.PlaygroupID] = r.GamesPlayed
	}
	if countsByGroup[played.ID] != 1 {
		t.Fatalf("ListPlaygroupGameCounts()[played] = %d, want 1", countsByGroup[played.ID])
	}
	if got, ok := countsByGroup[empty.ID]; !ok || got != 0 {
		t.Fatalf("ListPlaygroupGameCounts()[empty] = (%d, present=%v), want (0, true)", got, ok)
	}
}

func TestListFinishedGames_ReturnsPlayersWithWinFlag(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)

	g := setupTwoPlayerGame(t, pool, "irrelevant", "")
	mustRecordElimination(t, g.actions, g.gameID, g.user1.ID, g.player1ID, g.player2ID)
	mustFinishGame(t, g.games, g.gameID, g.user1.ID)

	page, err := g.stats.ListFinishedGames(context.Background(), common.PageRequest{Limit: 10}, g.user1.ID)
	if err != nil {
		t.Fatalf("ListFinishedGames() error = %v, want nil", err)
	}
	if len(page.Items) != 1 || page.NextCursor != nil {
		t.Fatalf(
			"ListFinishedGames() = %d items, next_cursor=%v, want 1 item and nil cursor",
			len(page.Items), page.NextCursor,
		)
	}

	item := page.Items[0]
	if item.ID != g.gameID || len(item.Players) != 2 {
		t.Fatalf("ListFinishedGames()[0] = %+v, want id=%s and 2 players", item, g.gameID)
	}
	assertFinishedGamePlayers(t, item.Players, g.user1.ID, g.user2.ID)
}

func assertFinishedGamePlayers(
	t *testing.T, players []statistics.FinishedGamePlayerResponse, winnerUserID, loserUserID string,
) {
	t.Helper()
	wonByUser := make(map[string]bool, len(players))
	for _, p := range players {
		if p.Username == "" || p.DeckName == "" {
			t.Fatalf("player %+v missing username/deck_name enrichment", p)
		}
		wonByUser[p.UserID] = p.Won
	}
	if !wonByUser[winnerUserID] || wonByUser[loserUserID] {
		t.Fatalf("ListFinishedGames() won flags = %+v, want %s=true %s=false", wonByUser, winnerUserID, loserUserID)
	}
}

func assertPlaygroupMemberStats(t *testing.T, members []statistics.PlaygroupMemberStats, winnerUserID string) {
	t.Helper()
	if len(members) != 2 {
		t.Fatalf("Members = %+v, want 2 entries", members)
	}
	for _, m := range members {
		wantWon := int32(0)
		if m.UserID == winnerUserID {
			wantWon = 1
		}
		if m.GamesPlayed != 1 || m.GamesWon != wantWon {
			t.Fatalf("unexpected member %+v (want games_played=1 games_won=%d)", m, wantWon)
		}
	}
}
