package games_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
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

// newGamesSvc crea un games.Service con el recalculador de estadísticas real
// (sobre el mismo pool), así FinishGame ejercita el flujo completo en los tests.
func newGamesSvc(pool *pgxpool.Pool) games.Service {
	return games.NewService(pool, statistics.NewService(pool))
}

func truncateGamesTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games" limpia game_players/game_actions por CASCADE; "users" limpia decks
	// y cualquier resto de game_players/refresh_tokens.
	testutil.Truncate(t, pool, "games", "users")
}

func createUserAndDeck(t *testing.T, pool *pgxpool.Pool, email string) (userID, deckID string) {
	t.Helper()
	ctx := context.Background()

	usersSvc := users.NewService(pool)
	user, err := usersSvc.RegisterUser(ctx, users.RegisterRequest{
		Username: "user-" + email,
		Email:    email,
		Password: "test-password-123",
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}

	decksSvc := decks.NewService(pool, noopMoxfieldClient{})
	deck, err := decksSvc.CreateDeck(ctx, user.ID, decks.CreateDeckRequest{
		Name:      "Deck de " + email,
		Commander: "Some Commander",
	})
	if err != nil {
		t.Fatalf("creando deck de test: %v", err)
	}
	return user.ID, deck.ID
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func mustCreateGame(t *testing.T, svc games.Service) *games.GameResponse {
	t.Helper()
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	return game
}

func mustJoin(t *testing.T, svc games.Service, gameID, userID, deckID string) *games.GamePlayerResponse {
	t.Helper()
	player, err := svc.JoinGame(context.Background(), gameID, userID, games.JoinGameRequest{DeckID: deckID})
	if err != nil {
		t.Fatalf("JoinGame(%s) error = %v, want nil", userID, err)
	}
	return player
}

func mustStart(t *testing.T, svc games.Service, gameID string) *games.GameResponse {
	t.Helper()
	game, err := svc.StartGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("StartGame() error = %v, want nil", err)
	}
	return game
}

// setupTwoPlayerGame crea 2 usuarios con sus decks, una partida y une a ambos
// (la partida queda en pending, sin iniciar). Devuelve el ID del primer
// usuario porque algunos tests lo necesitan para ejercitar leave/join sobre él.
func setupTwoPlayerGame(t *testing.T, pool *pgxpool.Pool, svc games.Service, namePrefix string) (gameID, user1 string) {
	t.Helper()
	user1, deck1 := createUserAndDeck(t, pool, namePrefix+"-1@example.com")
	user2, deck2 := createUserAndDeck(t, pool, namePrefix+"-2@example.com")

	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, user1, deck1)
	mustJoin(t, svc, game.ID, user2, deck2)
	return game.ID, user1
}

func TestCreateGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	res := mustCreateGame(t, svc)
	if res.Status != "pending" {
		t.Fatalf("CreateGame() status = %q, want pending", res.Status)
	}
	if res.PlaygroupID != nil {
		t.Fatalf("CreateGame() sin playgroup_id devolvió PlaygroupID = %v, want nil", res.PlaygroupID)
	}
}

func TestCreateGame_InvalidPlaygroupID_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	_, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: "not-a-uuid"})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("CreateGame() con playgroup_id inválido: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestJoinGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "join-success@example.com")
	game := mustCreateGame(t, svc)

	player := mustJoin(t, svc, game.ID, userID, deckID)
	if player.UserID != userID || player.DeckID != deckID {
		t.Fatalf("JoinGame() devolvió datos inesperados: %+v", player)
	}
	if player.LifeTotal != 40 {
		t.Fatalf("JoinGame() LifeTotal = %d, want 40 (default de Commander)", player.LifeTotal)
	}
}

func TestJoinGame_DeckOwnedByAnotherUser_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, _ := createUserAndDeck(t, pool, "join-owner@example.com")
	_, otherDeckID := createUserAndDeck(t, pool, "join-intruder@example.com")
	game := mustCreateGame(t, svc)

	// No se distingue "el deck no existe" de "el deck es de otro usuario".
	_, err := svc.JoinGame(context.Background(), game.ID, userID, games.JoinGameRequest{DeckID: otherDeckID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("JoinGame() con deck ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestJoinGame_AlreadyJoined_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "join-twice@example.com")
	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, userID, deckID)

	_, err := svc.JoinGame(context.Background(), game.ID, userID, games.JoinGameRequest{DeckID: deckID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("JoinGame() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestJoinGame_UnknownGame_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "join-unknown-game@example.com")

	_, err := svc.JoinGame(
		context.Background(), "00000000-0000-0000-0000-000000000000", userID, games.JoinGameRequest{DeckID: deckID},
	)
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("JoinGame() en partida inexistente: error = %v, want ErrGameNotFound", err)
	}
}

func TestJoinGame_GameAlreadyStarted_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "join-started")
	mustStart(t, svc, gameID)
	user3, deck3 := createUserAndDeck(t, pool, "join-started-3@example.com")

	_, err := svc.JoinGame(context.Background(), gameID, user3, games.JoinGameRequest{DeckID: deck3})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("JoinGame() tras start: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestLeaveGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "leave-success@example.com")
	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, userID, deckID)

	if err := svc.LeaveGame(context.Background(), game.ID, userID); err != nil {
		t.Fatalf("LeaveGame() error = %v, want nil", err)
	}

	got, err := svc.GetGame(context.Background(), game.ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if len(got.Players) != 0 {
		t.Fatalf("GetGame() tras LeaveGame() players = %+v, want vacío", got.Players)
	}
}

func TestLeaveGame_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, _ := createUserAndDeck(t, pool, "leave-not-member@example.com")
	game := mustCreateGame(t, svc)

	err := svc.LeaveGame(context.Background(), game.ID, userID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("LeaveGame() sin ser miembro: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestLeaveGame_GameAlreadyStarted_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "leave-started")
	mustStart(t, svc, gameID)

	err := svc.LeaveGame(context.Background(), gameID, user1)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("LeaveGame() tras start: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestStartGame_NotEnoughPlayers_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "start-not-enough@example.com")
	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, userID, deckID)

	_, err := svc.StartGame(context.Background(), game.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartGame() con 1 jugador: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestStartGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "start-success")

	started := mustStart(t, svc, gameID)
	if started.Status != "active" {
		t.Fatalf("StartGame() status = %q, want active", started.Status)
	}
	if started.StartedAt == nil {
		t.Fatalf("StartGame() StartedAt = nil, want seteado")
	}
	if len(started.Players) != 2 {
		t.Fatalf("StartGame() players = %+v, want 2", started.Players)
	}
}

func TestStartGame_AlreadyStarted_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "start-twice")
	mustStart(t, svc, gameID)

	_, err := svc.StartGame(context.Background(), gameID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartGame() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestFinishGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "finish-success")
	mustStart(t, svc, gameID)

	finished, err := svc.FinishGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("FinishGame() error = %v, want nil", err)
	}
	if finished.Status != "finished" {
		t.Fatalf("FinishGame() status = %q, want finished", finished.Status)
	}
	if finished.FinishedAt == nil {
		t.Fatalf("FinishGame() FinishedAt = nil, want seteado")
	}
}

func TestFinishGame_NotActive_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	game := mustCreateGame(t, svc)

	_, err := svc.FinishGame(context.Background(), game.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("FinishGame() en pending: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestFinishGame_AlreadyFinished_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "finish-twice")
	mustStart(t, svc, gameID)
	if _, err := svc.FinishGame(context.Background(), gameID); err != nil {
		t.Fatalf("primer FinishGame() error = %v", err)
	}

	_, err := svc.FinishGame(context.Background(), gameID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("FinishGame() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestGetGame_UnknownID_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	_, err := svc.GetGame(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("GetGame() con id inexistente: error = %v, want ErrGameNotFound", err)
	}
}

func TestGetGame_MalformedID_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	_, err := svc.GetGame(context.Background(), "not-a-uuid")
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("GetGame() con id malformado: error = %v, want ErrGameNotFound", err)
	}
}
