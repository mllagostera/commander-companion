package sync

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service define la lógica de negocio del módulo sync.
type Service interface {
	TriggerSync(ctx context.Context, moxfieldID string) (*SyncResponse, error)
	GetSyncStatus(ctx context.Context, jobID string) (*SyncResponse, error)
}

type service struct {
	db *pgxpool.Pool
}

// NewService crea un nuevo servicio de sync.
func NewService(db *pgxpool.Pool) Service {
	return &service{db: db}
}

// TriggerSync encola una sincronización de deck con Moxfield.
func (s *service) TriggerSync(ctx context.Context, moxfieldID string) (*SyncResponse, error) {
	//nolint:godox // Deferido a la fase de refinamiento (Stage 8: Importación Moxfield).
	// TODO: Send to background worker / queue
	return &SyncResponse{Status: "queued", JobID: "job-" + moxfieldID}, nil
}

// GetSyncStatus devuelve el estado de un job de sincronización.
func (s *service) GetSyncStatus(ctx context.Context, jobID string) (*SyncResponse, error) {
	return &SyncResponse{Status: "in_progress", JobID: jobID}, nil
}
