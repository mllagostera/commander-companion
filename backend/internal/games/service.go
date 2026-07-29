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
	// ErrGameNotFound indica que la partida no existe.
	ErrGameNotFound = common.NotFound("game not found")
	// ErrDeckNotFound indica que el deck no existe o no pertenece al jugador que se une.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrNotAMember indica que el usuario no tiene asiento en la partida.
	ErrNotAMember = common.NotFound("not a member of this game")
	// ErrInvalidPlaygroupID indica que el playgroup_id recibido no es un UUID válido.
	ErrInvalidPlaygroupID = common.InvalidInput("invalid playgroup_id")
	// ErrInvalidDeckID indica que el deck_id recibido no es un UUID válido.
	ErrInvalidDeckID = common.InvalidInput("invalid deck_id")
	// ErrGameClosedToPlayers indica que la partida ya no admite nuevos jugadores.
	ErrGameClosedToPlayers = common.Conflict("game is not accepting new players")
	// ErrAlreadyJoined indica que el usuario ya está en la partida.
	ErrAlreadyJoined = common.Conflict("already joined this game")
	// ErrGameAlreadyStarted indica que la partida ya arrancó o terminó.
	ErrGameAlreadyStarted = common.Conflict("game already started or finished")
	// ErrCannotLeaveStartedGame indica que no se puede abandonar una partida ya iniciada.
	ErrCannotLeaveStartedGame = common.Conflict("cannot leave a game that already started")
	// ErrNotEnoughPlayers indica que la partida no llega al mínimo de jugadores para arrancar.
	ErrNotEnoughPlayers = common.Conflict("not enough players to start")
	// ErrGameNotActive indica que solo una partida activa puede finalizarse.
	ErrGameNotActive = common.Conflict("only an active game can be finished")
	// ErrPlaygroupNotFound indica que el playgroup_id no existe o el usuario autenticado
	// no es miembro — no se distingue cuál de los dos casos es (mismo criterio que
	// playgroups.ErrPlaygroupNotFound).
	ErrPlaygroupNotFound = common.NotFound("playgroup not found")
)

// StatisticsRecalculator es lo que games necesita del módulo de estadísticas para
// disparar el recálculo al finalizar una partida (permite mockearlo en tests).
type StatisticsRecalculator interface {
	RecalculateForGame(ctx context.Context, gameID string) error
}

// Broadcaster es lo que games necesita para avisar en vivo, por WebSocket, que una
// partida terminó (permite mockearlo en tests y evita que este paquete dependa de
// internal/websocket, mismo patrón que StatisticsRecalculator). Ver ADR-0005
// (docs/decisions/0005-websocket-protocol.md).
type Broadcaster interface {
	BroadcastGameFinished(gameID string)
}

// Service define la lógica de negocio del módulo games.
type Service interface {
	CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error)
	GetGame(ctx context.Context, id string) (*GameResponse, error)
	ListGames(ctx context.Context, page common.PageRequest) (*GameListResponse, error)
	// ListGamesForPlaygroup devuelve el historial completo de partidas de un grupo,
	// si el usuario indicado es miembro.
	ListGamesForPlaygroup(ctx context.Context, playgroupID, userID string) (*GameListResponse, error)
	JoinGame(ctx context.Context, gameID, userID string, req JoinGameRequest) (*GamePlayerResponse, error)
	LeaveGame(ctx context.Context, gameID, userID string) error
	StartGame(ctx context.Context, gameID string) (*GameResponse, error)
	FinishGame(ctx context.Context, gameID string) (*GameResponse, error)
}

type service struct {
	repo        *Queries
	stats       StatisticsRecalculator
	broadcaster Broadcaster
}

// NewService crea un nuevo servicio de games.
func NewService(db *pgxpool.Pool, stats StatisticsRecalculator, broadcaster Broadcaster) Service {
	return &service{repo: New(db), stats: stats, broadcaster: broadcaster}
}

// CreateGame crea una nueva partida en estado pending.
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

// GetGame devuelve una partida por su ID, incluyendo el estado actual de sus jugadores.
func (s *service) GetGame(ctx context.Context, id string) (*GameResponse, error) {
	game, err := s.getGame(ctx, id)
	if err != nil {
		return nil, err
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
	}

	return toGameResponse(game, players), nil
}

// ListGames devuelve una página del historial de partidas, de la más reciente a la
// más vieja. Ver internal/common/pagination.go para el esquema de cursor.
func (s *service) ListGames(ctx context.Context, page common.PageRequest) (*GameListResponse, error) {
	// Se pide una fila de más que el límite: si vuelve, es que hay página
	// siguiente. Evita un COUNT(*) aparte solo para saber si seguir paginando.
	params := ListGamesPageParams{PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, err := decodeCursor(page.Cursor)
		if err != nil {
			return nil, err
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

// ListGamesForPlaygroup devuelve el historial completo de partidas de un grupo, de
// la más reciente a la más vieja, sin paginar (ver el comentario de
// ListGamesForPlaygroup en query.sql). Requiere que userID sea miembro del grupo,
// mismo criterio de "no revelar" que playgroups.GetPlaygroup.
func (s *service) ListGamesForPlaygroup(ctx context.Context, playgroupID, userID string) (*GameListResponse, error) {
	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}

	isMember, err := s.repo.IsPlaygroupMember(ctx, IsPlaygroupMemberParams{PlaygroupID: pid, UserID: uid})
	if err != nil {
		return nil, fmt.Errorf("checking playgroup membership: %w", err)
	}
	if !isMember {
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

// decodeCursor traduce el cursor opaco de la request a los parámetros de la query.
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

// JoinGame añade al usuario autenticado, con uno de sus decks, a una partida en estado pending.
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

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	deckID, err := s.resolveOwnedDeckID(ctx, userID, req.DeckID)
	if err != nil {
		return nil, err
	}

	err = s.ensureNotAlreadyJoined(ctx, game.ID, userID)
	if err != nil {
		return nil, err
	}

	player, err := s.repo.AddGamePlayer(ctx, AddGamePlayerParams{GameID: game.ID, UserID: uid, DeckID: deckID})
	if err != nil {
		return nil, fmt.Errorf("joining game: %w", err)
	}
	return toGamePlayerResponse(&player), nil
}

// resolveOwnedDeckID valida que deckID pertenezca al usuario indicado y devuelve su UUID parseado.
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
		// No se distingue "no existe" de "no es tuyo": evita revelar que el deck existe.
		return pgtype.UUID{}, ErrDeckNotFound
	}
	return did, nil
}

// ensureNotAlreadyJoined devuelve ErrAlreadyJoined si el usuario ya tiene un asiento en la partida.
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

// LeaveGame remueve al usuario autenticado de una partida en estado pending.
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

// StartGame inicia la partida indicada, si tiene suficientes jugadores.
func (s *service) StartGame(ctx context.Context, gameID string) (*GameResponse, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game.Status != statusPending {
		return nil, ErrGameAlreadyStarted
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
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

// FinishGame finaliza una partida activa.
func (s *service) FinishGame(ctx context.Context, gameID string) (*GameResponse, error) {
	game, err := s.getGame(ctx, gameID)
	if err != nil {
		return nil, err
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
