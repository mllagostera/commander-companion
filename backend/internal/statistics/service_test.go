package statistics_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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

// noopMoxfieldClient satisface decks.MoxfieldClient sin golpear la API real;
// estos tests nunca importan decks de Moxfield, solo necesitan crear decks propios.
type noopMoxfieldClient struct{}

func (noopMoxfieldClient) GetDeck(_ context.Context, _ string) (*moxfield.Deck, error) {
	return nil, nil //nolint:nilnil // stub nunca invocado en estos tests
}

// noopBroadcaster satisface tanto games.Broadcaster como gameactions.Broadcaster sin
// retransmitir nada de verdad: estos tests no ejercitan internal/websocket, solo
// necesitan que la dependencia esté presente para poder construir los servicios.
type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastGameFinished(_ string)                              {}
func (noopBroadcaster) BroadcastAction(_ string, _ *gameactions.GameActionResponse) {}

func truncateStatsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games"/"playgroups" limpian por CASCADE game_players/game_actions/playgroup_members;
	// "users" limpia decks y los resúmenes de estadísticas (FK a users/decks).
	testutil.Truncate(t, pool, "games", "playgroups", "users")
}

// twoPlayerGame agrupa todos los servicios y IDs de una partida de 2 jugadores lista
// para jugarse, para no repetir el setup en cada test.
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

func setupTwoPlayerGame(t *testing.T, pool *pgxpool.Pool, playgroupID string) *twoPlayerGame {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	statsSvc := statistics.NewService(pool)
	gamesSvc := games.NewService(pool, statsSvc, noopBroadcaster{})
	actionsSvc := gameactions.NewService(pool, noopBroadcaster{})

	user1, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "p1-" + t.Name(), Email: "p1-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario 1: %v", err)
	}
	user2, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "p2-" + t.Name(), Email: "p2-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario 2: %v", err)
	}

	deck1, err := decksSvc.CreateDeck(ctx, user1.ID, decks.CreateDeckRequest{Name: "D1", Commander: "C1"})
	if err != nil {
		t.Fatalf("creando deck 1: %v", err)
	}
	deck2, err := decksSvc.CreateDeck(ctx, user2.ID, decks.CreateDeckRequest{Name: "D2", Commander: "C2"})
	if err != nil {
		t.Fatalf("creando deck 2: %v", err)
	}

	game := mustCreateGame(t, gamesSvc, playgroupID)
	p1, err := gamesSvc.JoinGame(ctx, game.ID, user1.ID, games.JoinGameRequest{DeckID: deck1.ID})
	if err != nil {
		t.Fatalf("JoinGame(1) error = %v", err)
	}
	p2, err := gamesSvc.JoinGame(ctx, game.ID, user2.ID, games.JoinGameRequest{DeckID: deck2.ID})
	if err != nil {
		t.Fatalf("JoinGame(2) error = %v", err)
	}
	mustStartGame(t, gamesSvc, game.ID)

	return &twoPlayerGame{
		games: gamesSvc, actions: actionsSvc, stats: statsSvc,
		gameID: game.ID,
		user1:  user1, deck1ID: deck1.ID, player1ID: p1.ID,
		user2: user2, deck2ID: deck2.ID, player2ID: p2.ID,
	}
}

func mustCreateGame(t *testing.T, svc games.Service, playgroupID string) *games.GameResponse {
	t.Helper()
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
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

func mustStartGame(t *testing.T, svc games.Service, gameID string) {
	t.Helper()
	_, err := svc.StartGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}
}

func mustFinishGame(t *testing.T, svc games.Service, gameID string) {
	t.Helper()
	_, err := svc.FinishGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("FinishGame() error = %v, want nil", err)
	}
}

func mustRecordCombatDamage(t *testing.T, svc gameactions.Service, gameID, actorID, targetID string, amount float64) {
	t.Helper()
	_, err := svc.RecordAction(context.Background(), gameID, gameactions.CreateActionRequest{
		ActorID: actorID, TargetID: targetID, ActionType: "CombatDamage",
		Payload: map[string]interface{}{"amount": amount},
	})
	if err != nil {
		t.Fatalf("RecordAction(CombatDamage) error = %v", err)
	}
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
		t.Fatalf("registrando fundador del playgroup: %v", err)
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
		t.Fatalf("registrando usuario: %v", err)
	}

	res := mustGetUserStats(t, statistics.NewService(pool), user.ID)
	if res.GamesPlayed != 0 || res.GamesWon != 0 {
		t.Fatalf("GetUserStats() de un usuario sin partidas = %+v, want todo en cero", res)
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
		t.Fatalf("registrando usuario: %v", err)
	}
	intruder, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "deck-intruder", Email: "deck-intruder@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}
	deck, err := decksSvc.CreateDeck(ctx, owner.ID, decks.CreateDeckRequest{Name: "D", Commander: "C"})
	if err != nil {
		t.Fatalf("creando deck: %v", err)
	}

	svc := statistics.NewService(pool)
	_, err = svc.GetDeckStats(ctx, intruder.ID, deck.ID)
	if !errors.Is(err, statistics.ErrDeckNotFound) {
		t.Fatalf("GetDeckStats() de un deck ajeno: error = %v, want ErrDeckNotFound", err)
	}
}

func TestRecalculateForGame_WinnerGetsCreditForWinAndDamage(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupTwoPlayerGame(t, pool, "")

	// player1 elimina a player2 a puro CombatDamage; player1 sobrevive con vida llena.
	mustRecordCombatDamage(t, g.actions, g.gameID, g.player1ID, g.player2ID, 40)
	mustFinishGame(t, g.games, g.gameID)

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
			"GetDeckStats(loser).HighestLifeTotal = %d, want 40 (nunca ganó vida de más)",
			loserDeckStats.HighestLifeTotal,
		)
	}
}

func TestRecalculateForGame_NoSurvivors_NoWinnerCredited(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	g := setupTwoPlayerGame(t, pool, "")

	// Partida cortada manualmente sin que nadie quede eliminado: 2 sobrevivientes, sin ganador.
	mustFinishGame(t, g.games, g.gameID)

	stats1 := mustGetUserStats(t, g.stats, g.user1.ID)
	stats2 := mustGetUserStats(t, g.stats, g.user2.ID)
	if stats1.GamesPlayed != 1 || stats1.GamesWon != 0 || stats2.GamesPlayed != 1 || stats2.GamesWon != 0 {
		t.Fatalf("sin ganador claro nadie debería sumar games_won: p1=%+v p2=%+v", stats1, stats2)
	}
}

func TestRecalculateForGame_AccumulatesAcrossGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	g1 := setupTwoPlayerGame(t, pool, "")
	mustFinishGame(t, g1.games, g1.gameID)

	// setupTwoPlayerGame crea usuarios nuevos en cada llamada; para probar la
	// acumulación real hace falta jugar 2 veces con el MISMO usuario, así que la
	// segunda partida se arma a mano reutilizando g1.user1/g1.deck1ID.
	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	statsSvc := statistics.NewService(pool)
	gamesSvc := games.NewService(pool, statsSvc, noopBroadcaster{})

	opponent, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "opponent-" + t.Name(), Email: "opponent-" + t.Name() + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando oponente: %v", err)
	}
	opponentDeck, err := decksSvc.CreateDeck(ctx, opponent.ID, decks.CreateDeckRequest{Name: "OD", Commander: "OC"})
	if err != nil {
		t.Fatalf("creando deck del oponente: %v", err)
	}

	game2 := mustCreateGame(t, gamesSvc, "")
	mustJoin(t, gamesSvc, game2.ID, g1.user1.ID, g1.deck1ID)
	mustJoin(t, gamesSvc, game2.ID, opponent.ID, opponentDeck.ID)
	mustStartGame(t, gamesSvc, game2.ID)
	mustFinishGame(t, gamesSvc, game2.ID)

	stats := mustGetUserStats(t, statsSvc, g1.user1.ID)
	if stats.GamesPlayed != 2 {
		t.Fatalf("GetUserStats() tras 2 partidas del mismo usuario: games_played = %d, want 2", stats.GamesPlayed)
	}
}

func TestGetPlaygroupStats_AggregatesFinishedGames(t *testing.T) {
	pool := testutil.DB(t)
	truncateStatsTables(t, pool)
	ctx := context.Background()

	playgroupsSvc := playgroups.NewService(pool)
	// El creador del playgroup no participa de la partida; solo se usa el ID del grupo.
	founder := createFounder(t, pool)
	playgroup, err := playgroupsSvc.CreatePlaygroup(ctx, founder.ID, playgroups.CreatePlaygroupRequest{Name: "Liga"})
	if err != nil {
		t.Fatalf("CreatePlaygroup() error = %v", err)
	}

	g := setupTwoPlayerGame(t, pool, playgroup.ID)
	mustRecordCombatDamage(t, g.actions, g.gameID, g.player1ID, g.player2ID, 40)
	mustFinishGame(t, g.games, g.gameID)

	res, err := g.stats.GetPlaygroupStats(ctx, playgroup.ID)
	if err != nil {
		t.Fatalf("GetPlaygroupStats() error = %v, want nil", err)
	}
	if res.GamesPlayed != 1 {
		t.Fatalf("GetPlaygroupStats().GamesPlayed = %d, want 1", res.GamesPlayed)
	}
	assertPlaygroupMemberStats(t, res.Members, g.user1.ID)
}

func assertPlaygroupMemberStats(t *testing.T, members []statistics.PlaygroupMemberStats, winnerUserID string) {
	t.Helper()
	if len(members) != 2 {
		t.Fatalf("Members = %+v, want 2 entradas", members)
	}
	for _, m := range members {
		wantWon := int32(0)
		if m.UserID == winnerUserID {
			wantWon = 1
		}
		if m.GamesPlayed != 1 || m.GamesWon != wantWon {
			t.Fatalf("miembro %+v inesperado (want games_played=1 games_won=%d)", m, wantWon)
		}
	}
}
