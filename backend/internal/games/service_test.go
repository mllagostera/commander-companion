package games_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/statistics"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
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

// noopBroadcaster satisfies games.Broadcaster without actually broadcasting anything:
// these tests don't exercise internal/websocket, they just need the dependency to be
// present in order to build the service.
type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastGameFinished(_ string) {}

// newGamesSvc creates a games.Service with the real statistics recalculator and
// playgroups membership checker (over the same pool), so FinishGame
// and proxy-joins exercise the complete flow in the tests.
func newGamesSvc(pool *pgxpool.Pool) games.Service {
	return games.NewService(pool, statistics.NewService(pool), noopBroadcaster{}, playgroups.NewService(pool))
}

func truncateGamesTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "games" cleans up game_players/game_actions via CASCADE; "users" cleans up decks
	// and any remainder of game_players/refresh_tokens.
	testutil.Truncate(t, pool, "games", "users")
}

func createUserAndDeck(t *testing.T, pool *pgxpool.Pool, email string) (userID, deckID string) {
	t.Helper()
	ctx := context.Background()

	usersSvc := testutil.NewUsersService(pool)
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

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError (services no longer depend on fiber, see
// internal/common/errors.go), so we can keep verifying the status code the
// client sees.
func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
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

func mustStart(t *testing.T, svc games.Service, gameID, userID string) *games.GameResponse {
	t.Helper()
	game, err := svc.StartGame(context.Background(), gameID, userID)
	if err != nil {
		t.Fatalf("StartGame() error = %v, want nil", err)
	}
	return game
}

// setupTwoPlayerGame creates 2 users with their decks, a game, and joins both
// (the game is left pending, not started). Returns the first user's ID
// because some tests need it to exercise leave/join on them.
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

	// "the deck doesn't exist" isn't distinguished from "the deck belongs to another user".
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

// createPlaygroupWithMembers creates a playgroup with creatorID as the first member
// (see playgroups.CreatePlaygroup) and adds the rest of memberIDs.
func createPlaygroupWithMembers(t *testing.T, pool *pgxpool.Pool, creatorID string, memberIDs ...string) string {
	t.Helper()
	ctx := context.Background()
	pgSvc := playgroups.NewService(pool)

	pg, err := pgSvc.CreatePlaygroup(ctx, creatorID, playgroups.CreatePlaygroupRequest{Name: "Grupo de " + t.Name()})
	if err != nil {
		t.Fatalf("creando playgroup: %v", err)
	}
	for _, memberID := range memberIDs {
		if _, err := pgSvc.AddMember(ctx, pg.ID, creatorID, playgroups.AddMemberRequest{UserID: memberID}); err != nil {
			t.Fatalf("agregando miembro %s: %v", memberID, err)
		}
	}
	return pg.ID
}

func TestJoinGame_ProxyJoin_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	caller, _ := createUserAndDeck(t, pool, "proxy-caller@example.com")
	target, targetDeck := createUserAndDeck(t, pool, "proxy-target@example.com")
	playgroupID := createPlaygroupWithMembers(t, pool, caller, target)
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	player, err := svc.JoinGame(
		context.Background(), game.ID, caller, games.JoinGameRequest{DeckID: targetDeck, UserID: target},
	)
	if err != nil {
		t.Fatalf("JoinGame() proxy error = %v, want nil", err)
	}
	if player.UserID != target || player.DeckID != targetDeck {
		t.Fatalf("JoinGame() proxy devolvió datos inesperados: %+v", player)
	}
}

func TestJoinGame_ProxyJoin_WithoutPlaygroup_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	caller, _ := createUserAndDeck(t, pool, "proxy-nogroup-caller@example.com")
	target, targetDeck := createUserAndDeck(t, pool, "proxy-nogroup-target@example.com")
	game := mustCreateGame(t, svc) // sin playgroup_id

	_, err := svc.JoinGame(
		context.Background(), game.ID, caller, games.JoinGameRequest{DeckID: targetDeck, UserID: target},
	)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("JoinGame() proxy sin playgroup: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestJoinGame_ProxyJoin_TargetNotInPlaygroup_ReturnsForbidden(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	caller, _ := createUserAndDeck(t, pool, "proxy-outsider-caller@example.com")
	target, targetDeck := createUserAndDeck(t, pool, "proxy-outsider-target@example.com")
	// The caller's playgroup doesn't include target.
	playgroupID := createPlaygroupWithMembers(t, pool, caller)
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	_, err = svc.JoinGame(context.Background(), game.ID, caller, games.JoinGameRequest{DeckID: targetDeck, UserID: target})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("JoinGame() proxy con target ajeno al grupo: code = %d, want %d", fiberErr.Code, fiber.StatusForbidden)
	}
}

func TestJoinGame_ProxyJoin_CallerNotInPlaygroup_ReturnsForbidden(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	caller, _ := createUserAndDeck(t, pool, "proxy-nonmember-caller@example.com")
	owner, _ := createUserAndDeck(t, pool, "proxy-nonmember-owner@example.com")
	target, targetDeck := createUserAndDeck(t, pool, "proxy-nonmember-target@example.com")
	// The caller isn't a member of the playgroup, even though target is.
	playgroupID := createPlaygroupWithMembers(t, pool, owner, target)
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	_, err = svc.JoinGame(context.Background(), game.ID, caller, games.JoinGameRequest{DeckID: targetDeck, UserID: target})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("JoinGame() proxy con caller ajeno al grupo: code = %d, want %d", fiberErr.Code, fiber.StatusForbidden)
	}
}

func TestJoinGame_ProxyJoin_DeckMustBelongToTarget_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	caller, callerDeck := createUserAndDeck(t, pool, "proxy-deck-caller@example.com")
	target, _ := createUserAndDeck(t, pool, "proxy-deck-target@example.com")
	playgroupID := createPlaygroupWithMembers(t, pool, caller, target)
	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}

	// The deck belongs to the caller, not the target: it shouldn't be enough even if both
	// share a group — the proxy-join's deck has to belong to the seated player.
	_, err = svc.JoinGame(context.Background(), game.ID, caller, games.JoinGameRequest{DeckID: callerDeck, UserID: target})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("JoinGame() proxy con deck del caller: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestJoinGame_GameAlreadyStarted_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "join-started")
	mustStart(t, svc, gameID, user1)
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
	// Two players: after user1 leaves, user2 (still a member) is used to verify
	// the seat was removed — a game with no members left can't be queried by anyone.
	user1, deck1 := createUserAndDeck(t, pool, "leave-success-1@example.com")
	user2, deck2 := createUserAndDeck(t, pool, "leave-success-2@example.com")
	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, user1, deck1)
	mustJoin(t, svc, game.ID, user2, deck2)

	if err := svc.LeaveGame(context.Background(), game.ID, user1); err != nil {
		t.Fatalf("LeaveGame() error = %v, want nil", err)
	}

	got, err := svc.GetGame(context.Background(), game.ID, user2)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if len(got.Players) != 1 {
		t.Fatalf("GetGame() tras LeaveGame() players = %+v, want 1", got.Players)
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
	mustStart(t, svc, gameID, user1)

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

	_, err := svc.StartGame(context.Background(), game.ID, userID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartGame() con 1 jugador: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestStartGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "start-success")

	started := mustStart(t, svc, gameID, user1)
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
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "start-twice")
	mustStart(t, svc, gameID, user1)

	_, err := svc.StartGame(context.Background(), gameID, user1)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartGame() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestFinishGame_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "finish-success")
	mustStart(t, svc, gameID, user1)

	finished, err := svc.FinishGame(context.Background(), gameID, user1)
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
	userID, deckID := createUserAndDeck(t, pool, "finish-pending@example.com")
	game := mustCreateGame(t, svc)
	mustJoin(t, svc, game.ID, userID, deckID)

	_, err := svc.FinishGame(context.Background(), game.ID, userID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("FinishGame() en pending: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestFinishGame_AlreadyFinished_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "finish-twice")
	mustStart(t, svc, gameID, user1)
	if _, err := svc.FinishGame(context.Background(), gameID, user1); err != nil {
		t.Fatalf("primer FinishGame() error = %v", err)
	}

	_, err := svc.FinishGame(context.Background(), gameID, user1)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("FinishGame() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestGetGame_UnknownID_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	_, err := svc.GetGame(context.Background(), "00000000-0000-0000-0000-000000000000", "irrelevant")
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("GetGame() con id inexistente: error = %v, want ErrGameNotFound", err)
	}
}

func TestGetGame_MalformedID_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	_, err := svc.GetGame(context.Background(), "not-a-uuid", "irrelevant")
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("GetGame() con id malformado: error = %v, want ErrGameNotFound", err)
	}
}

func TestGetGame_NotAMemberOrPlaygroupMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "get-outsider")
	outsider, _ := createUserAndDeck(t, pool, "get-outsider-3@example.com")

	// Regression test for the broken access control fix: an authenticated user
	// with no seat in the game and no shared playgroup must not be able to read it.
	_, err := svc.GetGame(context.Background(), gameID, outsider)
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("GetGame() de un usuario ajeno: error = %v, want ErrGameNotFound", err)
	}
}

func TestStartGame_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, _ := setupTwoPlayerGame(t, pool, svc, "start-outsider")
	outsider, _ := createUserAndDeck(t, pool, "start-outsider-3@example.com")

	// Regression test: before the fix, any authenticated user could force any
	// pending game to start regardless of whether they had a seat in it.
	_, err := svc.StartGame(context.Background(), gameID, outsider)
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("StartGame() de un usuario ajeno: error = %v, want ErrGameNotFound", err)
	}
}

func TestFinishGame_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	gameID, user1 := setupTwoPlayerGame(t, pool, svc, "finish-outsider")
	mustStart(t, svc, gameID, user1)
	outsider, _ := createUserAndDeck(t, pool, "finish-outsider-3@example.com")

	// Regression test: before the fix, any authenticated user could force any
	// active game to finish (and trigger statistics recalculation) regardless
	// of whether they had a seat in it.
	_, err := svc.FinishGame(context.Background(), gameID, outsider)
	if !errors.Is(err, games.ErrGameNotFound) {
		t.Fatalf("FinishGame() de un usuario ajeno: error = %v, want ErrGameNotFound", err)
	}
}

func TestListGames_OnlyReturnsGamesTheUserHasASeatIn(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "list-scoped@example.com")
	myGame := mustCreateGame(t, svc)
	mustJoin(t, svc, myGame.ID, userID, deckID)

	// Another user's game, with no relation to userID: must not leak into
	// their history (regression test for the previously unfiltered ListGames).
	otherUserID, otherDeckID := createUserAndDeck(t, pool, "list-scoped-other@example.com")
	otherGame := mustCreateGame(t, svc)
	mustJoin(t, svc, otherGame.ID, otherUserID, otherDeckID)

	res, err := svc.ListGames(context.Background(), common.PageRequest{Limit: 20}, userID)
	if err != nil {
		t.Fatalf("ListGames() error = %v, want nil", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != myGame.ID {
		t.Fatalf("ListGames() = %+v, want solo %q", res.Items, myGame.ID)
	}
}

func TestListGamesForPlaygroup_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	userID, deckID := createUserAndDeck(t, pool, "list-games-playgroup@example.com")
	playgroupID := createPlaygroupWithMembers(t, pool, userID)

	game, err := svc.CreateGame(context.Background(), games.CreateGameRequest{PlaygroupID: playgroupID})
	if err != nil {
		t.Fatalf("CreateGame() error = %v", err)
	}
	mustJoin(t, svc, game.ID, userID, deckID)

	// A game without this playgroup shouldn't appear in its history.
	mustCreateGame(t, svc)

	res, err := svc.ListGamesForPlaygroup(context.Background(), playgroupID, userID)
	if err != nil {
		t.Fatalf("ListGamesForPlaygroup() error = %v, want nil", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != game.ID {
		t.Fatalf("ListGamesForPlaygroup() = %+v, want solo %q", res.Items, game.ID)
	}
	if len(res.Items[0].Players) != 1 {
		t.Fatalf("ListGamesForPlaygroup() players = %+v, want 1", res.Items[0].Players)
	}
}

// Same "don't reveal" criteria as playgroups.GetPlaygroup: someone outside the
// group can't see its history, without distinguishing "doesn't exist" from "you're not a member".
func TestListGamesForPlaygroup_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateGamesTables(t, pool)

	svc := newGamesSvc(pool)
	owner, _ := createUserAndDeck(t, pool, "list-games-owner@example.com")
	outsider, _ := createUserAndDeck(t, pool, "list-games-outsider@example.com")
	playgroupID := createPlaygroupWithMembers(t, pool, owner)

	_, err := svc.ListGamesForPlaygroup(context.Background(), playgroupID, outsider)
	if !errors.Is(err, games.ErrPlaygroupNotFound) {
		t.Fatalf("ListGamesForPlaygroup() de alguien ajeno: error = %v, want ErrPlaygroupNotFound", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("ListGamesForPlaygroup() de alguien ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}
