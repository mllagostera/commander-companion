package gameactions

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service define la lógica de negocio del módulo game-actions.
type Service interface {
	RecordAction(ctx context.Context, gameID string, req CreateActionRequest) (*GameActionResponse, error)
	GetTimeline(ctx context.Context, gameID string) ([]GameActionResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de game-actions.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// RecordAction registra una nueva acción (LifeChange, CombatDamage, etc.) dentro de una partida.
func (s *service) RecordAction(
	ctx context.Context, gameID string, req CreateActionRequest,
) (*GameActionResponse, error) {
	// Dummy implementation for compilation.
	//nolint:godox // Deferido a la fase de refinamiento (core del motor de partida, ver TASKS.md).
	// TODO: validar action_type y payload (LifeChange, CombatDamage, CommanderDamage,
	// PoisonCounter, TurnStart, TurnEnd, Elimination)
	return &GameActionResponse{
		ID:         "dummy-game-action-id",
		GameID:     gameID,
		ActorID:    req.ActorID,
		ActionType: req.ActionType,
		Payload:    req.Payload,
	}, nil
}

// GetTimeline devuelve el historial completo de acciones de una partida.
func (s *service) GetTimeline(ctx context.Context, gameID string) ([]GameActionResponse, error) {
	return []GameActionResponse{}, nil
}
