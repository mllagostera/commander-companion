package gameactions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/decks"
	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

// newGamesSvc crea un games.Service con el recalculador de estadísticas real
// (sobre el mismo pool), así FinishGame ejercita el flujo completo en los tests.
func newGamesSvc(pool *pgxpool.Pool) games.Service {
	return games.NewService(pool, statistics.NewService(pool))
}

// Vocabulario de action_type que entiende el motor (ver game-actions/service.go);
// como constantes acá porque el paquete real no las exporta.
const (
	actionTypeLifeChange    = "LifeChange"
	actionTypeCombatDamage  = "CombatDamage"
	actionTypePoisonCounter = "PoisonCounter"
	actionTypeTurnStart     = "TurnStart"
	actionTypeElimination   = "Elimination"

	payloadAmountKey = "amount"

	testPassword = "test-password-123"
)

// noopMoxfieldClient satisface decks.MoxfieldClient sin golpear la API real;
// estos tests nunca importan decks de Moxfield, solo necesitan crear decks propios.
type noopMoxfieldClient struct {
	deck *moxfield.Deck
	err  error
}

func (m noopMoxfieldClient) GetDeck(_ context.Context, _ string) (*moxfield.Deck, error) {
	return m.deck, m.err
}

func truncateGameActionsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games" limpia game_players/game_actions por CASCADE; "users" limpia decks
	// y cualquier resto de game_players/refresh_tokens.
	testutil.Truncate(t, pool, "games", "users")
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func amountPayload(amount float64) map[string]interface{} {
	return map[string]interface{}{payloadAmountKey: amount}
}

func mustRecordAction(
	t *testing.T, svc gameactions.Service, gameID string, req gameactions.CreateActionRequest,
) *gameactions.GameActionResponse {
	t.Helper()
	action, err := svc.RecordAction(context.Background(), gameID, req)
	if err != nil {
		t.Fatalf("RecordAction(%s) error = %v, want nil", req.ActionType, err)
	}
	return action
}

// setupActiveGame crea 2 usuarios con sus decks, una partida, los une a ambos y
// la inicia, dejando todo listo para ejercitar RecordAction/GetTimeline.
func setupActiveGame(t *testing.T, pool *pgxpool.Pool) (
	gamesSvc games.Service, actionsSvc gameactions.Service, gameID, player1ID, player2ID string,
) {
	t.Helper()
	ctx := context.Background()

	usersSvc := users.NewService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc = newGamesSvc(pool)
	actionsSvc = gameactions.NewService(pool)

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

	if _, err := gamesSvc.StartGame(ctx, game.ID); err != nil {
		t.Fatalf("StartGame() error = %v", err)
	}

	return gamesSvc, actionsSvc, game.ID, p1.ID, p2.ID
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
	gamesSvc, actionsSvc, gameID, player1ID, _ := setupActiveGame(t, pool)

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		ActionType: actionTypeLifeChange,
		Payload:    amountPayload(-3),
	})

	game, err := gamesSvc.GetGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if got := playerByID(t, game.Players, player1ID).LifeTotal; got != 37 {
		t.Fatalf("LifeTotal tras LifeChange(-3) sin target = %d, want 37", got)
	}
}

func TestRecordAction_CombatDamage_ReducesTargetLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	gamesSvc, actionsSvc, gameID, player1ID, player2ID := setupActiveGame(t, pool)

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		TargetID:   player2ID,
		ActionType: actionTypeCombatDamage,
		Payload:    amountPayload(15),
	})

	game, err := gamesSvc.GetGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, player2ID)
	if target.LifeTotal != 25 {
		t.Fatalf("LifeTotal del target tras CombatDamage(15) = %d, want 25", target.LifeTotal)
	}
	if playerByID(t, game.Players, player1ID).LifeTotal != 40 {
		t.Fatalf("LifeTotal del actor no debería cambiar por daño a otro jugador")
	}
}

func TestRecordAction_CombatDamage_AutoEliminatesAtZeroLife(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	gamesSvc, actionsSvc, gameID, player1ID, player2ID := setupActiveGame(t, pool)

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		TargetID:   player2ID,
		ActionType: actionTypeCombatDamage,
		Payload:    amountPayload(40),
	})

	game, err := gamesSvc.GetGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, player2ID)
	if target.LifeTotal != 0 {
		t.Fatalf("LifeTotal = %d, want 0", target.LifeTotal)
	}
	if !target.IsEliminated {
		t.Fatalf("jugador con life_total 0 debería quedar eliminado automáticamente")
	}
}

func TestRecordAction_PoisonCounter_AutoEliminatesAtThreshold(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	gamesSvc, actionsSvc, gameID, player1ID, player2ID := setupActiveGame(t, pool)
	ctx := context.Background()

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID: player1ID, TargetID: player2ID, ActionType: actionTypePoisonCounter, Payload: amountPayload(9),
	})
	game, err := gamesSvc.GetGame(ctx, gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if target := playerByID(t, game.Players, player2ID); target.IsEliminated {
		t.Fatalf("con 9 contadores de veneno no debería estar eliminado todavía")
	}

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID: player1ID, TargetID: player2ID, ActionType: actionTypePoisonCounter, Payload: amountPayload(1),
	})
	game, err = gamesSvc.GetGame(ctx, gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	target := playerByID(t, game.Players, player2ID)
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
	gamesSvc, actionsSvc, gameID, player1ID, _ := setupActiveGame(t, pool)

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		ActionType: actionTypeElimination,
	})

	game, err := gamesSvc.GetGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if !playerByID(t, game.Players, player1ID).IsEliminated {
		t.Fatalf("Elimination sin target debería marcar al propio actor como eliminado")
	}
}

func TestRecordAction_InvalidActionType_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	_, actionsSvc, gameID, player1ID, _ := setupActiveGame(t, pool)

	_, err := actionsSvc.RecordAction(context.Background(), gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		ActionType: "NotARealActionType",
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction() con action_type inválido: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_MissingAmount_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	_, actionsSvc, gameID, player1ID, _ := setupActiveGame(t, pool)

	_, err := actionsSvc.RecordAction(context.Background(), gameID, gameactions.CreateActionRequest{
		ActorID:    player1ID,
		ActionType: actionTypeLifeChange,
		// Sin payload.amount.
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordAction() sin payload.amount: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestRecordAction_GameNotActive_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	ctx := context.Background()

	usersSvc := users.NewService(pool)
	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	gamesSvc := newGamesSvc(pool)
	actionsSvc := gameactions.NewService(pool)

	user, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "pending-actor", Email: "pending-actor@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}
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
	// Partida deliberadamente dejada en pending (no se llama a StartGame).

	_, err = actionsSvc.RecordAction(ctx, game.ID, gameactions.CreateActionRequest{
		ActorID: player.ID, ActionType: actionTypeLifeChange, Payload: amountPayload(1),
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("RecordAction() en partida pending: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestRecordAction_UnknownGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)

	actionsSvc := gameactions.NewService(pool)
	req := gameactions.CreateActionRequest{
		ActorID:    "00000000-0000-0000-0000-000000000000",
		ActionType: actionTypeTurnStart,
	}
	_, err := actionsSvc.RecordAction(context.Background(), "00000000-0000-0000-0000-000000000000", req)
	if !errors.Is(err, gameactions.ErrGameNotFound) {
		t.Fatalf("RecordAction() en partida inexistente: error = %v, want ErrGameNotFound", err)
	}
}

func TestRecordAction_ActorNotInGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	_, actionsSvc, gameID, _, _ := setupActiveGame(t, pool)

	_, err := actionsSvc.RecordAction(context.Background(), gameID, gameactions.CreateActionRequest{
		ActorID:    "00000000-0000-0000-0000-000000000000",
		ActionType: actionTypeTurnStart,
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("RecordAction() con actor ajeno a la partida: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestGetTimeline_ReturnsActionsInOrder(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	_, actionsSvc, gameID, player1ID, player2ID := setupActiveGame(t, pool)

	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID: player1ID, ActionType: actionTypeTurnStart,
	})
	mustRecordAction(t, actionsSvc, gameID, gameactions.CreateActionRequest{
		ActorID: player1ID, TargetID: player2ID, ActionType: actionTypeCombatDamage, Payload: amountPayload(5),
	})

	timeline, err := actionsSvc.GetTimeline(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetTimeline() error = %v, want nil", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("GetTimeline() len = %d, want 2: %+v", len(timeline), timeline)
	}
	if timeline[0].ActionType != actionTypeTurnStart || timeline[1].ActionType != actionTypeCombatDamage {
		t.Fatalf("GetTimeline() orden inesperado: %+v", timeline)
	}
	if timeline[1].TargetID == nil || *timeline[1].TargetID != player2ID {
		t.Fatalf("GetTimeline()[1].TargetID = %v, want %q", timeline[1].TargetID, player2ID)
	}
	if amount, _ := timeline[1].Payload[payloadAmountKey].(float64); amount != 5 {
		t.Fatalf("GetTimeline()[1].Payload = %+v, want amount=5", timeline[1].Payload)
	}
}

func TestGetTimeline_UnknownGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)

	actionsSvc := gameactions.NewService(pool)
	_, err := actionsSvc.GetTimeline(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, gameactions.ErrGameNotFound) {
		t.Fatalf("GetTimeline() en partida inexistente: error = %v, want ErrGameNotFound", err)
	}
}
