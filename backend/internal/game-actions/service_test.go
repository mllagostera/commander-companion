package gameactions_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
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

// noopBroadcaster satisfies both games.Broadcaster and gameactions.Broadcaster without
// actually rebroadcasting anything: these tests don't exercise internal/websocket, they
// just need the dependency to be present to be able to build the services.
type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastGameFinished(_ string)                              {}
func (noopBroadcaster) BroadcastAction(_ string, _ *gameactions.GameActionResponse) {}

// newGamesSvc creates a games.Service with the real statistics recalculator and
// playgroup membership checker (over the same pool), so FinishGame
// and the proxy-joins exercise the full flow in the tests.
func newGamesSvc(pool *pgxpool.Pool) games.Service {
	return games.NewService(pool, statistics.NewService(pool), noopBroadcaster{}, playgroups.NewService(pool))
}

// newActionsSvc creates a gameactions.Service with a noop Broadcaster (these tests
// don't exercise internal/websocket).
func newActionsSvc(pool *pgxpool.Pool) gameactions.Service {
	return gameactions.NewService(pool, noopBroadcaster{})
}

// action_type vocabulary understood by the engine (see game-actions/service.go);
// kept as constants here because the real package doesn't export them.
const (
	actionTypeLifeChange      = "LifeChange"
	actionTypeCombatDamage    = "CombatDamage"
	actionTypeCommanderDamage = "CommanderDamage"
	actionTypePoisonCounter   = "PoisonCounter"
	actionTypeTurnStart       = "TurnStart"
	actionTypeTurnEnd         = "TurnEnd"
	actionTypeElimination     = "Elimination"

	payloadAmountKey = "amount"

	testPassword = "test-password-123"
)

// noopMoxfieldClient satisfies decks.MoxfieldClient without hitting the real API;
// these tests never import decks from Moxfield, they only need to create their own decks.
type noopMoxfieldClient struct {
	deck *moxfield.Deck
	err  error
}

func (m noopMoxfieldClient) GetDeck(_ context.Context, _ string) (*moxfield.Deck, error) {
	return m.deck, m.err
}

func truncateGameActionsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games" clears game_players/game_actions via CASCADE; "users" clears decks
	// and any remaining game_players/refresh_tokens.
	testutil.Truncate(t, pool, "games", "users")
}

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError (the services no longer depend on fiber, see
// internal/common/errors.go), so we can still verify the status code the client
// sees.
func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func amountPayload(amount float64) map[string]interface{} {
	return map[string]interface{}{payloadAmountKey: amount}
}

func mustRecordAction(
	t *testing.T, actionsSvc gameactions.Service, gameID, callerID string, req gameactions.CreateActionRequest,
) *gameactions.GameActionResponse {
	t.Helper()
	action, err := actionsSvc.RecordAction(context.Background(), gameID, callerID, req)
	if err != nil {
		t.Fatalf("RecordAction(%s) error = %v, want nil", req.ActionType, err)
	}
	return action
}

func mustRegisterUser(t *testing.T, usersSvc users.Service, prefix string) *users.UserResponse {
	t.Helper()
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: prefix, Email: prefix + "@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario %s: %v", prefix, err)
	}
	return user
}

// activeGame groups the services and IDs of an active 2-player game, ready
// to exercise RecordAction/GetTimeline. player1ID/player2ID are GamePlayer.ID
// (for actor_id/target_id); user1ID/user2ID are the underlying user.ID that
// joined themselves (for the RecordAction caller).
type activeGame struct {
	games              games.Service
	actions            gameactions.Service
	gameID             string
	player1ID, user1ID string
	player2ID, user2ID string
}

func setupActiveGame(t *testing.T, pool *pgxpool.Pool) activeGame {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc := newGamesSvc(pool)
	actionsSvc := newActionsSvc(pool)

	user1 := mustRegisterUser(t, usersSvc, "p1-"+t.Name())
	user2 := mustRegisterUser(t, usersSvc, "p2-"+t.Name())

	deck1, err := decksSvc.CreateDeck(ctx, user1.ID, decks.CreateDeckRequest{Name: "D1", Commander: "C1"})
	if err != nil {
		t.Fatalf("creando deck 1: %v", err)
	}
	deck2, err := decksSvc.CreateDeck(ctx, user2.ID, decks.CreateDeckRequest{Name: "D2", Commander: "C2"})
	if err != nil {
		t.Fatalf("creando deck 2: %v", err)
	}

	game, err := gamesSvc.CreateGame(ctx, games.CreateGameRequest{})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	p1, err := gamesSvc.JoinGame(ctx, game.ID, user1.ID, games.JoinGameRequest{DeckID: deck1.ID})
	if err != nil {
		t.Fatalf("JoinGame(1) error = %v", err)
	}
	p2, err := gamesSvc.JoinGame(ctx, game.ID, user2.ID, games.JoinGameRequest{DeckID: deck2.ID})
	if err != nil {
		t.Fatalf("JoinGame(2) error = %v", err)
	}

	if _, err := gamesSvc.StartGame(ctx, game.ID, user1.ID); err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	return activeGame{
		games: gamesSvc, actions: actionsSvc, gameID: game.ID,
		player1ID: p1.ID, user1ID: user1.ID,
		player2ID: p2.ID, user2ID: user2.ID,
	}
}

// setupActiveGameWithPlayers creates n users+decks, joins them to a game (in order,
// while it's still pending) and starts it. Used by the CommanderDamage tests that
// need more than 2 seats to verify that damage is tracked per attacker-defender
// pair, not aggregated across different attackers. userIDs is parallel to
// playerIDs (userIDs[i] is the owner of playerIDs[i], for the RecordAction caller).
func setupActiveGameWithPlayers(t *testing.T, pool *pgxpool.Pool, n int) (
	gamesSvc games.Service, actionsSvc gameactions.Service, gameID string, playerIDs, userIDs []string,
) {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc = newGamesSvc(pool)
	actionsSvc = newActionsSvc(pool)

	game, err := gamesSvc.CreateGame(ctx, games.CreateGameRequest{})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	playerIDs = make([]string, 0, n)
	userIDs = make([]string, 0, n)
	for i := 0; i < n; i++ {
		suffix := fmt.Sprintf("%s-%d", t.Name(), i)
		user := mustRegisterUser(t, usersSvc, "u-"+suffix)
		deck, err := decksSvc.CreateDeck(ctx, user.ID, decks.CreateDeckRequest{Name: "D", Commander: "C"})
		if err != nil {
			t.Fatalf("creando deck %d: %v", i, err)
		}
		player, err := gamesSvc.JoinGame(ctx, game.ID, user.ID, games.JoinGameRequest{DeckID: deck.ID})
		if err != nil {
			t.Fatalf("JoinGame(%d) error = %v", i, err)
		}
		playerIDs = append(playerIDs, player.ID)
		userIDs = append(userIDs, user.ID)
	}

	if _, err := gamesSvc.StartGame(ctx, game.ID, userIDs[0]); err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	return gamesSvc, actionsSvc, game.ID, playerIDs, userIDs
}

// proxyJoinedGame is a group game where the scorekeeper sat themselves down
// and also proxy-joined teammate (see ADR-0013) — the "a single
// device scores for the whole table" scenario.
type proxyJoinedGame struct {
	games            games.Service
	actions          gameactions.Service
	gameID           string
	scorekeeperID    string
	teammatePlayerID string
}

func setupProxyJoinedGame(t *testing.T, pool *pgxpool.Pool) proxyJoinedGame {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc := newGamesSvc(pool)
	actionsSvc := newActionsSvc(pool)
	pgSvc := playgroups.NewService(pool)

	scorekeeper := mustRegisterUser(t, usersSvc, "scorekeeper-"+t.Name())
	teammate := mustRegisterUser(t, usersSvc, "teammate-"+t.Name())

	teammateDeck, err := decksSvc.CreateDeck(ctx, teammate.ID, decks.CreateDeckRequest{Name: "D", Commander: "C"})
	if err != nil {
		t.Fatalf("creando deck del teammate: %v", err)
	}
	scorekeeperDeck, err := decksSvc.CreateDeck(ctx, scorekeeper.ID, decks.CreateDeckRequest{Name: "D2", Commander: "C2"})
	if err != nil {
		t.Fatalf("creando deck del scorekeeper: %v", err)
	}

	playgroup, err := pgSvc.CreatePlaygroup(ctx, scorekeeper.ID, playgroups.CreatePlaygroupRequest{Name: "Grupo"})
	if err != nil {
		t.Fatalf("creando playgroup: %v", err)
	}
	if _, addErr := pgSvc.AddMember(
		ctx, playgroup.ID, scorekeeper.ID, playgroups.AddMemberRequest{UserID: teammate.ID},
	); addErr != nil {
		t.Fatalf("agregando teammate al playgroup: %v", addErr)
	}

	game, err := gamesSvc.CreateGame(ctx, games.CreateGameRequest{PlaygroupID: playgroup.ID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	// The scorekeeper joins the teammate as a proxy: they end up with added_by = scorekeeper.
	teammatePlayer, err := gamesSvc.JoinGame(
		ctx, game.ID, scorekeeper.ID, games.JoinGameRequest{DeckID: teammateDeck.ID, UserID: teammate.ID},
	)
	if err != nil {
		t.Fatalf("JoinGame() proxy error = %v", err)
	}
	// Second seat (the scorekeeper themselves, self-join) so the game can start (minPlayersToStart = 2).
	if _, joinErr := gamesSvc.JoinGame(
		ctx, game.ID, scorekeeper.ID, games.JoinGameRequest{DeckID: scorekeeperDeck.ID},
	); joinErr != nil {
		t.Fatalf("JoinGame() del scorekeeper error = %v", joinErr)
	}
	if _, startErr := gamesSvc.StartGame(ctx, game.ID, scorekeeper.ID); startErr != nil {
		t.Fatalf("StartGame() error = %v", startErr)
	}

	return proxyJoinedGame{
		games: gamesSvc, actions: actionsSvc, gameID: game.ID,
		scorekeeperID: scorekeeper.ID, teammatePlayerID: teammatePlayer.ID,
	}
}

func playerByID(t *testing.T, players []games.GamePlayerResponse, id string) *games.GamePlayerResponse {
	t.Helper()
	for i := range players {
		if players[i].ID == id {
			return &players[i]
		}
	}
	t.Fatalf("player %s no encontrado en %+v", id, players)
	return nil
}

func TestRecordAction_LifeChange_AppliesToActorWhenNoTarget(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeLifeChange,
		Payload:    amountPayload(-3),
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if got := playerByID(t, game.Players, g.player1ID).LifeTotal; got != 37 {
		t.Fatalf("LifeTotal tras LifeChange(-3) sin target = %d, want 37", got)
	}
}

func TestRecordAction_CombatDamage_ReducesTargetLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		TargetID:   g.player2ID,
		ActionType: actionTypeCombatDamage,
		Payload:    amountPayload(15),
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, g.player2ID)
	if target.LifeTotal != 25 {
		t.Fatalf("LifeTotal del target tras CombatDamage(15) = %d, want 25", target.LifeTotal)
	}
	if playerByID(t, game.Players, g.player1ID).LifeTotal != 40 {
		t.Fatalf("LifeTotal del actor no debería cambiar por daño a otro jugador")
	}
}

func TestRecordAction_CombatDamage_AutoEliminatesAtZeroLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		TargetID:   g.player2ID,
		ActionType: actionTypeCombatDamage,
		Payload:    amountPayload(40),
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, g.player2ID)
	if target.LifeTotal != 0 {
		t.Fatalf("LifeTotal = %d, want 0", target.LifeTotal)
	}
	if !target.IsEliminated {
		t.Fatalf("jugador con life_total 0 debería quedar eliminado automáticamente")
	}
}

func TestRecordAction_CommanderDamage_AccumulatesAndReducesLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	for range 2 {
		mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
			ActorID:    g.player1ID,
			TargetID:   g.player2ID,
			ActionType: actionTypeCommanderDamage,
			Payload:    amountPayload(5),
		})
	}

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, g.player2ID)
	if target.LifeTotal != 30 {
		t.Fatalf("LifeTotal tras 2x CommanderDamage(5) = %d, want 30", target.LifeTotal)
	}
	if target.IsEliminated {
		t.Fatalf("10 de daño de comandante acumulado no debería eliminar (umbral es 21)")
	}
}

func TestRecordAction_CommanderDamage_MissingTarget_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeCommanderDamage,
		Payload:    amountPayload(5),
		// No target_id: CommanderDamage doesn't make sense without an identified defender.
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction(CommanderDamage) sin target_id: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_CommanderDamage_SelfTarget_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		TargetID:   g.player1ID,
		ActionType: actionTypeCommanderDamage,
		Payload:    amountPayload(5),
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction(CommanderDamage) con target == actor: code = %d, want %d",
			fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_CommanderDamage_EliminatesAt21FromSingleSourceWithPositiveLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCommanderDamage, Payload: amountPayload(15),
	})
	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCommanderDamage, Payload: amountPayload(6),
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, g.player2ID)
	if target.LifeTotal != 19 {
		t.Fatalf("LifeTotal tras 21 de comandante = %d, want 19 (vida positiva)", target.LifeTotal)
	}
	if !target.IsEliminated {
		t.Fatalf("21 de daño de comandante de una misma fuente debería eliminar aunque quede vida positiva")
	}
}

func TestRecordAction_CommanderDamage_DistinctAttackersDoNotCombine(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	gamesSvc, actionsSvc, gameID, playerIDs, userIDs := setupActiveGameWithPlayers(t, pool, 3)
	attackerA, attackerB, defender := playerIDs[0], playerIDs[1], playerIDs[2]
	userA, userB := userIDs[0], userIDs[1]

	mustRecordAction(t, actionsSvc, gameID, userA, gameactions.CreateActionRequest{
		ActorID: attackerA, TargetID: defender, ActionType: actionTypeCommanderDamage, Payload: amountPayload(15),
	})
	mustRecordAction(t, actionsSvc, gameID, userB, gameactions.CreateActionRequest{
		ActorID: attackerB, TargetID: defender, ActionType: actionTypeCommanderDamage, Payload: amountPayload(15),
	})

	game, err := gamesSvc.GetGame(context.Background(), gameID, userA)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, defender)
	if target.LifeTotal != 10 {
		t.Fatalf("LifeTotal tras 15+15 de dos atacantes distintos = %d, want 10", target.LifeTotal)
	}
	if target.IsEliminated {
		t.Fatalf("15+15 de DOS atacantes distintos no debería eliminar (ninguno llega a 21 por sí solo)")
	}
}

func TestRecordAction_PoisonCounter_AutoEliminatesAtThreshold(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)
	ctx := context.Background()

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypePoisonCounter, Payload: amountPayload(9),
	})
	game, err := g.games.GetGame(ctx, g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if target := playerByID(t, game.Players, g.player2ID); target.IsEliminated {
		t.Fatalf("con 9 contadores de veneno no debería estar eliminado todavía")
	}

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypePoisonCounter, Payload: amountPayload(1),
	})
	game, err = g.games.GetGame(ctx, g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, g.player2ID)
	if target.PoisonCounters != 10 {
		t.Fatalf("PoisonCounters = %d, want 10", target.PoisonCounters)
	}
	if !target.IsEliminated {
		t.Fatalf("con 10 contadores de veneno debería quedar eliminado automáticamente")
	}
}

func TestRecordAction_Elimination_MarksSelfEliminated(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeElimination,
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if !playerByID(t, game.Players, g.player1ID).IsEliminated {
		t.Fatalf("Elimination sin target debería marcar al propio actor como eliminado")
	}
}

func TestRecordAction_TurnStart_SetsCurrentTurnPlayer(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeTurnStart,
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if game.CurrentTurnPlayerID == nil || *game.CurrentTurnPlayerID != g.player1ID {
		t.Fatalf("CurrentTurnPlayerID = %v, want %q", game.CurrentTurnPlayerID, g.player1ID)
	}
}

func TestRecordAction_TurnEnd_ClearsCurrentTurnPlayer(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeTurnStart,
	})
	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeTurnEnd,
	})

	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if game.CurrentTurnPlayerID != nil {
		t.Fatalf("CurrentTurnPlayerID tras TurnEnd = %v, want nil", *game.CurrentTurnPlayerID)
	}
}

func TestRecordAction_InvalidActionType_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: "NotARealActionType",
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction() con action_type inválido: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_MissingAmount_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    g.player1ID,
		ActionType: actionTypeLifeChange,
		// No payload.amount.
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction() sin payload.amount: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_GameNotActive_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc := newGamesSvc(pool)
	actionsSvc := newActionsSvc(pool)

	user := mustRegisterUser(t, usersSvc, "pending-actor")
	deck, err := decksSvc.CreateDeck(ctx, user.ID, decks.CreateDeckRequest{Name: "D", Commander: "C"})
	if err != nil {
		t.Fatalf("creando deck: %v", err)
	}
	game, err := gamesSvc.CreateGame(ctx, games.CreateGameRequest{})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	player, err := gamesSvc.JoinGame(ctx, game.ID, user.ID, games.JoinGameRequest{DeckID: deck.ID})
	if err != nil {
		t.Fatalf("JoinGame() error = %v", err)
	}
	// Game deliberately left in pending (StartGame is not called).

	_, err = actionsSvc.RecordAction(ctx, game.ID, user.ID, gameactions.CreateActionRequest{
		ActorID: player.ID, ActionType: actionTypeLifeChange, Payload: amountPayload(1),
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("RecordAction() en partida pending: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestRecordAction_UnknownGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)

	actionsSvc := newActionsSvc(pool)
	req := gameactions.CreateActionRequest{
		ActorID:    "00000000-0000-0000-0000-000000000000",
		ActionType: actionTypeTurnStart,
	}
	_, err := actionsSvc.RecordAction(context.Background(), "00000000-0000-0000-0000-000000000000", "irrelevant", req)
	if !errors.Is(err, gameactions.ErrGameNotFound) {
		t.Fatalf("RecordAction() en partida inexistente: error = %v, want ErrGameNotFound", err)
	}
}

func TestRecordAction_ActorNotInGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID:    "00000000-0000-0000-0000-000000000000",
		ActionType: actionTypeTurnStart,
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("RecordAction() con actor ajeno a la partida: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

// The caller was always the actor's owner in the tests above (the only case that
// existed in production until now). These cover the new authorization rule
// (see ADR-0013): owner ok, whoever proxy-joined them ok, anyone else 403 — before
// this change, any authenticated user who knew the actor_id could
// record actions on their behalf.
func TestRecordAction_CallerIsActorOwner_Allowed(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, ActionType: actionTypeLifeChange, Payload: amountPayload(-1),
	})
	if err != nil {
		t.Fatalf("RecordAction() del dueño del actor: error = %v, want nil", err)
	}
}

func TestRecordAction_CallerIsUnrelatedUser_ReturnsForbidden(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	// user2 has their own GamePlayer in this game, but is not the owner of
	// player1ID nor did they proxy-join it: they cannot act on its behalf.
	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.user2ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, ActionType: actionTypeLifeChange, Payload: amountPayload(-1),
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("RecordAction() de un usuario ajeno al actor: code = %d, want %d", fiberErr.Code, fiber.StatusForbidden)
	}
}

func TestRecordAction_CallerProxyJoinedTheActor_Allowed(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupProxyJoinedGame(t, pool)

	// The scorekeeper (not the teammate) records the action for the teammate's seat.
	_, err := g.actions.RecordAction(context.Background(), g.gameID, g.scorekeeperID, gameactions.CreateActionRequest{
		ActorID: g.teammatePlayerID, ActionType: actionTypeLifeChange, Payload: amountPayload(-4),
	})
	if err != nil {
		t.Fatalf("RecordAction() del scorekeeper por el teammate proxy-joineado: error = %v, want nil", err)
	}

	game, err := g.games.GetGame(context.Background(), g.gameID, g.scorekeeperID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if got := playerByID(t, game.Players, g.teammatePlayerID).LifeTotal; got != 36 {
		t.Fatalf("LifeTotal del teammate tras LifeChange(-4) del scorekeeper = %d, want 36", got)
	}
}

func TestGetTimeline_ReturnsActionsInOrder(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, ActionType: actionTypeTurnStart,
	})
	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCombatDamage, Payload: amountPayload(5),
	})

	timeline, err := g.actions.GetTimeline(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetTimeline() error = %v, want nil", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("GetTimeline() len = %d, want 2: %+v", len(timeline), timeline)
	}
	if timeline[0].ActionType != actionTypeTurnStart || timeline[1].ActionType != actionTypeCombatDamage {
		t.Fatalf("GetTimeline() orden inesperado: %+v", timeline)
	}
	if timeline[1].TargetID == nil || *timeline[1].TargetID != g.player2ID {
		t.Fatalf("GetTimeline()[1].TargetID = %v, want %q", timeline[1].TargetID, g.player2ID)
	}
	if amount, _ := timeline[1].Payload[payloadAmountKey].(float64); amount != 5 {
		t.Fatalf("GetTimeline()[1].Payload = %+v, want amount=5", timeline[1].Payload)
	}
}

func TestGetTimeline_UnknownGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)

	actionsSvc := newActionsSvc(pool)
	_, err := actionsSvc.GetTimeline(context.Background(), "00000000-0000-0000-0000-000000000000", "irrelevant")
	if !errors.Is(err, gameactions.ErrGameNotFound) {
		t.Fatalf("GetTimeline() en partida inexistente: error = %v, want ErrGameNotFound", err)
	}
}
