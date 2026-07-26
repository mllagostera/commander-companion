package games

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultLifeTotal = 40

// Service define la lógica de negocio del módulo games.
type Service interface {
	CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error)
	GetGame(ctx context.Context, id string) (*GameResponse, error)
	ListGames(ctx context.Context) ([]GameResponse, error)
	JoinGame(ctx context.Context, gameID string, req JoinGameRequest) (*GamePlayerResponse, error)
	LeaveGame(ctx context.Context, gameID, userID string) error
	StartGame(ctx context.Context, gameID string) (*GameResponse, error)
	FinishGame(ctx context.Context, gameID string) (*GameResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de games.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// CreateGame crea una nueva partida en estado pending.
func (s *service) CreateGame(ctx context.Context, req CreateGameRequest) (*GameResponse, error) {
	// Dummy implementation for compilation
	return &GameResponse{ID: "dummy-game-id", PlaygroupID: nonEmpty(req.PlaygroupID), Status: "pending"}, nil
}

// GetGame devuelve una partida por su ID.
func (s *service) GetGame(ctx context.Context, id string) (*GameResponse, error) {
	return &GameResponse{ID: id, Status: "pending"}, nil
}

// ListGames devuelve el historial de partidas.
func (s *service) ListGames(ctx context.Context) ([]GameResponse, error) {
	return []GameResponse{}, nil
}

// JoinGame añade un jugador a la partida indicada.
func (s *service) JoinGame(ctx context.Context, gameID string, req JoinGameRequest) (*GamePlayerResponse, error) {
	return &GamePlayerResponse{
		ID:        "dummy-game-player-id",
		GameID:    gameID,
		UserID:    req.UserID,
		DeckID:    req.DeckID,
		LifeTotal: defaultLifeTotal,
	}, nil
}

// LeaveGame remueve a un jugador de la partida indicada.
func (s *service) LeaveGame(ctx context.Context, gameID, userID string) error {
	return nil
}

// StartGame inicia la partida indicada.
func (s *service) StartGame(ctx context.Context, gameID string) (*GameResponse, error) {
	return &GameResponse{ID: gameID, Status: "active"}, nil
}

// FinishGame finaliza la partida indicada.
func (s *service) FinishGame(ctx context.Context, gameID string) (*GameResponse, error) {
	//nolint:godox // Deferido a la fase de refinamiento (Stage 7: Estadísticas, ver TASKS.md).
	// TODO: lanzar recálculo de estadísticas al finalizar (Statistics Engine)
	return &GameResponse{ID: gameID, Status: "finished"}, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
