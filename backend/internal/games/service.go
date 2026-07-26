package games

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
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

// ErrGameNotFound indica que la partida no existe.
var ErrGameNotFound = errors.New("game not found")

// StatisticsRecalculator es lo que games necesita del módulo de estadísticas para
// disparar el recálculo al finalizar una partida (permite mockearlo en tests).
type StatisticsRecalculator interface {
	RecalculateForGame(ctx context.Context, gameID string) error
}

// Service define la lógica de negocio del módulo games.
type Service interface {
	CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error)
	GetGame(ctx context.Context, id string) (*GameResponse, error)
	ListGames(ctx context.Context) ([]GameResponse, error)
	JoinGame(ctx context.Context, gameID, userID string, req JoinGameRequest) (*GamePlayerResponse, error)
	LeaveGame(ctx context.Context, gameID, userID string) error
	StartGame(ctx context.Context, gameID string) (*GameResponse, error)
	FinishGame(ctx context.Context, gameID string) (*GameResponse, error)
}

type service struct {
	repo  *Queries
	stats StatisticsRecalculator
}

// NewService crea un nuevo servicio de games.
func NewService(db *pgxpool.Pool, stats StatisticsRecalculator) Service {
	return &service{repo: New(db), stats: stats}
}

// CreateGame crea una nueva partida en estado pending.
func (s *service) CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error) {
	var playgroupID pgtype.UUID
	if req.PlaygroupID != "" {
		pid, err := common.ParseUUID(req.PlaygroupID)
		if err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest, "invalid playgroup_id")
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

// ListGames devuelve el historial de partidas.
func (s *service) ListGames(ctx context.Context) ([]GameResponse, error) {
	list, err := s.repo.ListGames(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing games: %w", err)
	}

	result := make([]GameResponse, 0, len(list))
	for i := range list {
		result = append(result, *toGameResponse(&list[i], nil))
	}
	return result, nil
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
		return nil, fiber.NewError(fiber.StatusConflict, "game is not accepting new players")
	}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
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
		return pgtype.UUID{}, fiber.NewError(fiber.StatusBadRequest, "invalid deck_id")
	}

	deck, err := s.repo.GetDeckByID(ctx, did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, fiber.NewError(fiber.StatusNotFound, "deck not found")
		}
		return pgtype.UUID{}, fmt.Errorf("looking up deck: %w", err)
	}
	if deck.UserID.String() != userID {
		// No se distingue "no existe" de "no es tuyo": evita revelar que el deck existe.
		return pgtype.UUID{}, fiber.NewError(fiber.StatusNotFound, "deck not found")
	}
	return did, nil
}

// ensureNotAlreadyJoined devuelve un 409 si el usuario ya tiene un asiento en la partida.
func (s *service) ensureNotAlreadyJoined(ctx context.Context, gameID pgtype.UUID, userID string) error {
	players, err := s.repo.ListGamePlayers(ctx, gameID)
	if err != nil {
		return fmt.Errorf("listing game players: %w", err)
	}
	for i := range players {
		if players[i].UserID.String() == userID {
			return fiber.NewError(fiber.StatusConflict, "already joined this game")
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
		return fiber.NewError(fiber.StatusConflict, "cannot leave a game that already started")
	}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid user")
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
		return fiber.NewError(fiber.StatusNotFound, "not a member of this game")
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
		return nil, fiber.NewError(fiber.StatusConflict, "game already started or finished")
	}

	players, err := s.repo.ListGamePlayers(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("listing game players: %w", err)
	}
	if len(players) < minPlayersToStart {
		return nil, fiber.NewError(fiber.StatusConflict, "not enough players to start")
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
		return nil, fiber.NewError(fiber.StatusConflict, "only an active game can be finished")
	}

	finished, err := s.repo.FinishGame(ctx, game.ID)
	if err != nil {
		return nil, fmt.Errorf("finishing game: %w", err)
	}

	if err := s.stats.RecalculateForGame(ctx, gameID); err != nil {
		return nil, fmt.Errorf("recalculating statistics: %w", err)
	}

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
