package games

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	minPlayersToStart = 2

	statusPending  = "pending"
	statusActive   = "active"
	statusFinished = "finished"
)

var (
	// ErrGameNotFound indicates that the game doesn't exist.
	ErrGameNotFound = common.NotFound("game not found")
	// ErrDeckNotFound indicates that the deck doesn't exist or doesn't belong to the joining player.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrNotAMember indicates that the user doesn't have a seat in the game.
	ErrNotAMember = common.NotFound("not a member of this game")
	// ErrInvalidPlaygroupID indicates that the received playgroup_id isn't a valid UUID.
	ErrInvalidPlaygroupID = common.InvalidInput("invalid playgroup_id")
	// ErrInvalidDeckID indicates that the received deck_id isn't a valid UUID.
	ErrInvalidDeckID = common.InvalidInput("invalid deck_id")
	// ErrGameClosedToPlayers indicates that the game no longer accepts new players.
	ErrGameClosedToPlayers = common.Conflict("game is not accepting new players")
	// ErrAlreadyJoined indicates that the user is already in the game.
	ErrAlreadyJoined = common.Conflict("already joined this game")
	// ErrGameAlreadyStarted indicates that the game already started or finished.
	ErrGameAlreadyStarted = common.Conflict("game already started or finished")
	// ErrCannotLeaveStartedGame indicates that an already-started game can't be left.
	ErrCannotLeaveStartedGame = common.Conflict("cannot leave a game that already started")
	// ErrNotEnoughPlayers indicates that the game doesn't reach the minimum player count to start.
	ErrNotEnoughPlayers = common.Conflict("not enough players to start")
	// ErrGameNotActive indicates that only an active game can be finished.
	ErrGameNotActive = common.Conflict("only an active game can be finished")
	// ErrProxyJoinRequiresPlaygroup indicates an attempted proxy-join (user_id
	// different from the caller) on a game without playgroup_id — there's no group to
	// validate shared membership against (see ADR-0013).
	ErrProxyJoinRequiresPlaygroup = common.InvalidInput("joining another user requires a game linked to a playgroup")
	// ErrProxyJoinNotAuthorized indicates that the caller or the target user don't
	// share the game's playgroup. It doesn't distinguish which of the two failed,
	// same criteria as the rest of the module with resources belonging to others.
	ErrProxyJoinNotAuthorized = common.Forbidden("cannot join this game on behalf of that user")
	// ErrPlaygroupNotFound indicates that the playgroup_id doesn't exist or the
	// authenticated user isn't a member — it doesn't distinguish which of the two
	// cases it is (same criteria as playgroups.ErrPlaygroupNotFound).
	ErrPlaygroupNotFound = common.NotFound("playgroup not found")
)

// StatisticsRecalculator is what games needs from the statistics module to
// trigger the recalculation when a game finishes (allows mocking it in tests).
type StatisticsRecalculator interface {
	RecalculateForGame(ctx context.Context, gameID string) error
}

// Broadcaster is what games needs to announce live, via WebSocket, that a
// game finished (allows mocking it in tests and keeps this package from depending on
// internal/websocket, same pattern as StatisticsRecalculator). See ADR-0005
// (docs/decisions/0005-websocket-protocol.md).
type Broadcaster interface {
	BroadcastGameFinished(gameID string)
}

// PlaygroupMembership is what games needs from playgroups to authorize a
// proxy-join (see ADR-0013): confirming a user is a member of a group, without
// this package depending on internal/playgroups directly (same pattern as
// StatisticsRecalculator/Broadcaster).
type PlaygroupMembership interface {
	IsMember(ctx context.Context, playgroupID, userID string) (bool, error)
}

// Service defines the business logic of the games module.
type Service interface {
	CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error)
	// GetGame returns a game by ID, if userID may access it (see authorizeGameAccess).
	GetGame(ctx context.Context, id, userID string) (*GameResponse, error)
	// ListGames returns a page of the game history of the given user only —
	// never the full cross-tenant history (see ListGamesPage in query.sql).
	ListGames(ctx context.Context, page common.PageRequest, userID string) (*GameListResponse, error)
	// ListGamesForPlaygroup returns the complete game history of a group,
	// if the given user is a member.
	ListGamesForPlaygroup(ctx context.Context, playgroupID, userID string) (*GameListResponse, error)
	JoinGame(ctx context.Context, gameID, userID string, req JoinGameRequest) (*GamePlayerResponse, error)
	LeaveGame(ctx context.Context, gameID, userID string) error
	StartGame(ctx context.Context, gameID, userID string) (*GameResponse, error)
	FinishGame(ctx context.Context, gameID, userID string) (*GameResponse, error)
	// CanAccessGame reports whether userID may access gameID. Implements
	// websocket.MembershipChecker so the live game channel enforces the same
	// authorization boundary as GetGame (see ADR-0013).
	CanAccessGame(ctx context.Context, gameID, userID string) (bool, error)
}

type service struct {
	repo        *Queries
	stats       StatisticsRecalculator
	broadcaster Broadcaster
	membership  PlaygroupMembership
}

// NewService creates a new games service.
func NewService(
	db *pgxpool.Pool, stats StatisticsRecalculator, broadcaster Broadcaster, membership PlaygroupMembership,
) Service {
	return &service{repo: New(db), stats: stats, broadcaster: broadcaster, membership: membership}
}

// CreateGame creates a new game in pending state.
func (s *service) CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error) {
	var playgroupID pgtype.UUID
	if req.PlaygroupID != "" {
		pid, err := common.ParseUUID(req.PlaygroupID)
		if err != nil {
			return nil, ErrInvalidPlaygroupID
		}
		playgroupID = pid
	}

	game, err := s.repo.CreateGame(ctx, CreateGameParams{PlaygroupID: playgroupID, Status: statusPending})
	if err != nil {
		return nil, fmt.Errorf("creating game: %w", err)
	}
	return toGameResponse(&game, nil), nil
}

// GetGame returns a game by its ID, including the current state of its
// players, if userID may access it (see authorizeGameAccess).
func (s *service) GetGame(ctx context.Context, id, userID string) (*GameResponse, error) {
	game, err := s.getGame(ctx, id)
	if err != nil {
		return nil, err
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
	}

	if err := s.authorizeGameAccess(ctx, game, userID, players); err != nil {
		return nil, err
	}

	return toGameResponse(game, players), nil
}

// authorizeGameAccess requires the caller to either be a member of the
// game's playgroup (if it has one — playgroup membership is the established
// trust boundary for this module, see ADR-0013) or to hold a seat in the
// game itself (for ad-hoc games with no playgroup). Denies with
// ErrGameNotFound, not a 403, to not reveal whether a game the caller can't
// access exists — same "don't reveal" pattern used across the rest of the module.
func (s *service) authorizeGameAccess(ctx context.Context, game *Game, userID string, players []GamePlayer) error {
	allowed, err := s.isGameMember(ctx, game, userID, players)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrGameNotFound
	}
	return nil
}

// isGameMember is the boolean core of authorizeGameAccess, factored out so
// CanAccessGame (which wants a plain bool, not an error to translate) doesn't
// have to distinguish "not authorized" from a real lookup failure by string
// matching or discarding a non-nil error.
func (s *service) isGameMember(ctx context.Context, game *Game, userID string, players []GamePlayer) (bool, error) {
	if game.PlaygroupID.Valid {
		isMember, err := s.membership.IsMember(ctx, game.PlaygroupID.String(), userID)
		if err != nil {
			return false, fmt.Errorf("checking playgroup membership: %w", err)
		}
		if isMember {
			return true, nil
		}
	}
	for i := range players {
		if players[i].UserID.String() == userID {
			return true, nil
		}
	}
	return false, nil
}

// CanAccessGame reports whether userID may access gameID (see
// authorizeGameAccess). A game that doesn't exist and one the caller can't
// access both return false, nil: from a caller like the WebSocket handler's
// perspective, "doesn't exist" and "not yours" are the same "may not connect".
func (s *service) CanAccessGame(ctx context.Context, gameID, userID string) (bool, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		if errors.Is(err, ErrGameNotFound) {
			return false, nil
		}
		return false, err
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return false, fmt.Errorf("listing game players: %w", err)
	}

	return s.isGameMember(ctx, game, userID, players)
}

// ListGames returns a page of userID's game history, from most recent to
// oldest — scoped to games where they have a seat (see ListGamesPage in
// query.sql), never the full cross-tenant history. See
// internal/common/pagination.go for the cursor scheme.
func (s *service) ListGames(ctx context.Context, page common.PageRequest, userID string) (*GameListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// One row more than the limit is requested: if it comes back, there's a
	// next page. Avoids a separate COUNT(*) just to know whether to keep paginating.
	params := ListGamesPageParams{UserID: uid, PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, cursorErr := decodeCursor(page.Cursor)
		if cursorErr != nil {
			return nil, cursorErr
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListGamesPage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing games: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}

	items := make([]GameResponse, 0, len(rows))
	for i := range rows {
		items = append(items, *toGameResponse(&rows[i], nil))
	}
	return &GameListResponse{Items: items, NextCursor: nextCursor}, nil
}

// ListGamesForPlaygroup returns the complete game history of a group, from
// most recent to oldest, without pagination (see the comment on
// ListGamesForPlaygroup in query.sql). Requires userID to be a member of the group,
// same "don't reveal" criteria as playgroups.GetPlaygroup.
func (s *service) ListGamesForPlaygroup(ctx context.Context, playgroupID, userID string) (*GameListResponse, error) {
	isMember, err := s.membership.IsMember(ctx, playgroupID, userID)
	if err != nil {
		return nil, fmt.Errorf("checking playgroup membership: %w", err)
	}
	if !isMember {
		return nil, ErrPlaygroupNotFound
	}

	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}

	rows, err := s.repo.ListGamesForPlaygroup(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup games: %w", err)
	}

	items := make([]GameResponse, 0, len(rows))
	for i := range rows {
		players, err := s.repo.ListGamePlayers(ctx, rows[i].ID)
		if err != nil {
			return nil, fmt.Errorf("listing game players: %w", err)
		}
		items = append(items, *toGameResponse(&rows[i], players))
	}
	return &GameListResponse{Items: items, NextCursor: nil}, nil
}

// decodeCursor translates the request's opaque cursor into the query's parameters.
func decodeCursor(encoded string) (pgtype.Timestamp, pgtype.UUID, error) {
	cursor, err := common.DecodeCursor(encoded)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, err
	}
	cursorID, err := common.ParseUUID(cursor.ID)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, common.ErrInvalidCursor
	}
	return pgtype.Timestamp{Time: cursor.CreatedAt, Valid: true}, cursorID, nil
}

// JoinGame adds a player to a game in pending state, with one of their decks.
// Without req.UserID (or if it matches the caller), the player is the caller
// themselves. With a different req.UserID, it's a proxy-join (see ADR-0013): the
// caller adds another user on their behalf, authorized only if both share the
// game's playgroup — and it's recorded in added_by, authorizing the caller to
// record actions for that player (internal/game-actions/service.go).
func (s *service) JoinGame(
	ctx context.Context, gameID, userID string, req JoinGameRequest,
) (*GamePlayerResponse, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game.Status != statusPending {
		return nil, ErrGameClosedToPlayers
	}

	targetUserID, addedBy, err := s.resolveJoinTarget(ctx, game, userID, req)
	if err != nil {
		return nil, err
	}

	uid, err := common.ParseUUID(targetUserID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	deckID, err := s.resolveOwnedDeckID(ctx, targetUserID, req.DeckID)
	if err != nil {
		return nil, err
	}

	if joinErr := s.ensureNotAlreadyJoined(ctx, game.ID, targetUserID); joinErr != nil {
		return nil, joinErr
	}

	player, err := s.repo.AddGamePlayer(
		ctx, AddGamePlayerParams{GameID: game.ID, UserID: uid, DeckID: deckID, AddedBy: addedBy},
	)
	if err != nil {
		return nil, fmt.Errorf("joining game: %w", err)
	}
	return toGamePlayerResponse(&player), nil
}

// resolveJoinTarget decides who the join seats: the caller themselves (self-join,
// without req.UserID or equal to the caller) or, if req.UserID is given and differs, a
// proxy-join authorized by shared playgroup membership (see ADR-0013) — in
// that case it also returns the added_by to persist.
func (s *service) resolveJoinTarget(
	ctx context.Context, game *Game, callerID string, req JoinGameRequest,
) (targetUserID string, addedBy pgtype.UUID, err error) {
	if req.UserID == "" || req.UserID == callerID {
		return callerID, pgtype.UUID{}, nil
	}

	if authErr := s.authorizeProxyJoin(ctx, game, callerID, req.UserID); authErr != nil {
		return "", pgtype.UUID{}, authErr
	}
	callerUID, err := common.ParseUUID(callerID)
	if err != nil {
		return "", pgtype.UUID{}, common.ErrInvalidUser
	}
	return req.UserID, callerUID, nil
}

// authorizeProxyJoin validates that caller and target share the game's
// playgroup — the only trust relationship that enables adding another user.
func (s *service) authorizeProxyJoin(ctx context.Context, game *Game, callerID, targetID string) error {
	if !game.PlaygroupID.Valid {
		return ErrProxyJoinRequiresPlaygroup
	}
	playgroupID := game.PlaygroupID.String()

	callerIsMember, err := s.membership.IsMember(ctx, playgroupID, callerID)
	if err != nil {
		return fmt.Errorf("checking caller membership: %w", err)
	}
	if !callerIsMember {
		return ErrProxyJoinNotAuthorized
	}

	targetIsMember, err := s.membership.IsMember(ctx, playgroupID, targetID)
	if err != nil {
		return fmt.Errorf("checking target membership: %w", err)
	}
	if !targetIsMember {
		return ErrProxyJoinNotAuthorized
	}
	return nil
}

// resolveOwnedDeckID validates that deckID belongs to the given user and returns its parsed UUID.
func (s *service) resolveOwnedDeckID(ctx context.Context, userID, deckID string) (pgtype.UUID, error) {
	did, err := common.ParseUUID(deckID)
	if err != nil {
		return pgtype.UUID{}, ErrInvalidDeckID
	}

	deck, err := s.repo.GetDeckByID(ctx, did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, ErrDeckNotFound
		}
		return pgtype.UUID{}, fmt.Errorf("looking up deck: %w", err)
	}
	if deck.UserID.String() != userID {
		// "doesn't exist" isn't distinguished from "not yours": avoids revealing that the deck exists.
		return pgtype.UUID{}, ErrDeckNotFound
	}
	return did, nil
}

// ensureNotAlreadyJoined returns ErrAlreadyJoined if the user already has a seat in the game.
func (s *service) ensureNotAlreadyJoined(ctx context.Context, gameID pgtype.UUID, userID string) error {
	players, err := s.repo.ListGamePlayers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("listing game players: %w", err)
	}
	for i := range players {
		if players[i].UserID.String() == userID {
			return ErrAlreadyJoined
		}
	}
	return nil
}

// LeaveGame removes the authenticated user from a game in pending state.
func (s *service) LeaveGame(ctx context.Context, gameID, userID string) error {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return err
	}
	if game.Status != statusPending {
		return ErrCannotLeaveStartedGame
	}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return common.ErrInvalidUser
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return fmt.Errorf("listing game players: %w", err)
	}
	member := false
	for i := range players {
		if players[i].UserID.String() == userID {
			member = true
			break
		}
	}
	if !member {
		return ErrNotAMember
	}

	if err := s.repo.RemoveGamePlayer(ctx, RemoveGamePlayerParams{GameID: game.ID, UserID: uid}); err != nil {
		return fmt.Errorf("leaving game: %w", err)
	}
	return nil
}

// StartGame starts the given game, if the caller may access it and it has
// enough players.
func (s *service) StartGame(ctx context.Context, gameID, userID string) (*GameResponse, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return nil, err
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
	}
	if authErr := s.authorizeGameAccess(ctx, game, userID, players); authErr != nil {
		return nil, authErr
	}

	if game.Status != statusPending {
		return nil, ErrGameAlreadyStarted
	}
	if len(players) < minPlayersToStart {
		return nil, ErrNotEnoughPlayers
	}

	started, err := s.repo.StartGame(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("starting game: %w", err)
	}
	return toGameResponse(&started, players), nil
}

// FinishGame finishes an active game, if the caller may access it.
func (s *service) FinishGame(ctx context.Context, gameID, userID string) (*GameResponse, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return nil, err
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
	}
	if authErr := s.authorizeGameAccess(ctx, game, userID, players); authErr != nil {
		return nil, authErr
	}

	if game.Status != statusActive {
		return nil, ErrGameNotActive
	}

	finished, err := s.repo.FinishGame(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("finishing game: %w", err)
	}

	if err := s.stats.RecalculateForGame(ctx, gameID); err != nil {
		return nil, fmt.Errorf("recalculating statistics: %w", err)
	}

	s.broadcaster.BroadcastGameFinished(gameID)

	return toGameResponse(&finished, nil), nil
}

func (s *service) getGame(ctx context.Context, id string) (*Game, error) {
	gid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrGameNotFound
	}

	game, err := s.repo.GetGame(ctx, gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGameNotFound
		}
		return nil, fmt.Errorf("looking up game: %w", err)
	}
	return &game, nil
}

func toGameResponse(game *Game, players []GamePlayer) *GameResponse {
	res := &GameResponse{
		ID:     game.ID.String(),
		Status: game.Status,
	}
	if game.PlaygroupID.Valid {
		pid := game.PlaygroupID.String()
		res.PlaygroupID = &pid
	}
	if game.StartedAt.Valid {
		t := game.StartedAt.Time.Format(time.RFC3339)
		res.StartedAt = &t
	}
	if game.FinishedAt.Valid {
		t := game.FinishedAt.Time.Format(time.RFC3339)
		res.FinishedAt = &t
	}
	if game.CurrentTurnPlayerID.Valid {
		tid := game.CurrentTurnPlayerID.String()
		res.CurrentTurnPlayerID = &tid
	}
	if players != nil {
		res.Players = make([]GamePlayerResponse, 0, len(players))
		for i := range players {
			res.Players = append(res.Players, *toGamePlayerResponse(&players[i]))
		}
	}
	return res
}

func toGamePlayerResponse(p *GamePlayer) *GamePlayerResponse {
	return &GamePlayerResponse{
		ID:                 p.ID.String(),
		GameID:             p.GameID.String(),
		UserID:             p.UserID.String(),
		DeckID:             p.DeckID.String(),
		LifeTotal:          p.LifeTotal.Int32,
		PoisonCounters:     p.PoisonCounters.Int32,
		EnergyCounters:     p.EnergyCounters.Int32,
		ExperienceCounters: p.ExperienceCounters.Int32,
		IsEliminated:       p.IsEliminated.Bool,
	}
}
