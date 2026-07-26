package statistics

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service define la lógica de negocio del módulo statistics.
type Service interface {
	GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error)
	GetDeckStats(ctx context.Context, deckID string) (*DeckStatsResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de statistics.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// GetUserStats devuelve las estadísticas globales precalculadas de un usuario.
func (s *service) GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error) {
	// Dummy implementation until repo is used properly with UUID mapping
	return &UserStatsResponse{UserID: userID}, nil
}

// GetDeckStats devuelve las estadísticas precalculadas de un deck.
func (s *service) GetDeckStats(ctx context.Context, deckID string) (*DeckStatsResponse, error) {
	return &DeckStatsResponse{DeckID: deckID}, nil
}
