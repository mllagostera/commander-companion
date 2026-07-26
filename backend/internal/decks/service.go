package decks

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service define la lógica de negocio del módulo decks.
type Service interface {
	CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error)
	GetDeck(ctx context.Context, id string) (*DeckResponse, error)
	ListDecks(ctx context.Context, userID string) ([]DeckResponse, error)
	DeleteDeck(ctx context.Context, id string) error
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de decks.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// CreateDeck crea un nuevo deck para el usuario indicado.
func (s *service) CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error) {
	// Dummy implementation for compilation
	return &DeckResponse{
		ID:        "dummy-deck-id",
		UserID:    userID,
		Name:      req.Name,
		Commander: req.Commander,
	}, nil
}

// GetDeck devuelve un deck por su ID.
func (s *service) GetDeck(ctx context.Context, id string) (*DeckResponse, error) {
	return &DeckResponse{ID: id}, nil
}

// ListDecks devuelve los decks del usuario indicado.
func (s *service) ListDecks(ctx context.Context, userID string) ([]DeckResponse, error) {
	return []DeckResponse{}, nil
}

// DeleteDeck elimina un deck por su ID.
func (s *service) DeleteDeck(ctx context.Context, id string) error {
	return nil
}
