package playgroups

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service define la lógica de negocio del módulo playgroups.
type Service interface {
	CreatePlaygroup(ctx context.Context, req CreatePlaygroupRequest) (*PlaygroupResponse, error)
	GetPlaygroup(ctx context.Context, id string) (*PlaygroupResponse, error)
	ListPlaygroups(ctx context.Context) ([]PlaygroupResponse, error)
	AddMember(ctx context.Context, playgroupID string, req AddMemberRequest) (*PlaygroupMemberResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de playgroups.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// CreatePlaygroup crea un nuevo grupo de juego.
func (s *service) CreatePlaygroup(ctx context.Context, req CreatePlaygroupRequest) (*PlaygroupResponse, error) {
	// Dummy implementation for compilation
	return &PlaygroupResponse{ID: "dummy-playgroup-id", Name: req.Name}, nil
}

// GetPlaygroup devuelve un grupo de juego por su ID.
func (s *service) GetPlaygroup(ctx context.Context, id string) (*PlaygroupResponse, error) {
	return &PlaygroupResponse{ID: id}, nil
}

// ListPlaygroups devuelve todos los grupos de juego.
func (s *service) ListPlaygroups(ctx context.Context) ([]PlaygroupResponse, error) {
	return []PlaygroupResponse{}, nil
}

// AddMember añade un usuario a un grupo de juego.
func (s *service) AddMember(
	ctx context.Context, playgroupID string, req AddMemberRequest,
) (*PlaygroupMemberResponse, error) {
	return &PlaygroupMemberResponse{PlaygroupID: playgroupID, UserID: req.UserID}, nil
}
