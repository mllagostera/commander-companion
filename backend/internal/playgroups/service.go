package playgroups

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// ErrPlaygroupNotFound indica que el grupo no existe o el usuario no es miembro de él.
var ErrPlaygroupNotFound = errors.New("playgroup not found")

// Service define la lógica de negocio del módulo playgroups.
type Service interface {
	CreatePlaygroup(ctx context.Context, userID string, req CreatePlaygroupRequest) (*PlaygroupResponse, error)
	GetPlaygroup(ctx context.Context, userID, id string) (*PlaygroupResponse, error)
	ListPlaygroups(ctx context.Context, userID string) ([]PlaygroupResponse, error)
	AddMember(ctx context.Context, playgroupID, requesterID string, req AddMemberRequest) (*PlaygroupMemberResponse, error)
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de playgroups.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// CreatePlaygroup crea un nuevo grupo de juego y añade al creador como primer miembro.
func (s *service) CreatePlaygroup(
	ctx context.Context, userID string, req CreatePlaygroupRequest,
) (*PlaygroupResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "name is required")
	}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
	}

	playgroup, err := s.repo.CreatePlaygroup(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("creating playgroup: %w", err)
	}

	_, err = s.repo.AddPlaygroupMember(ctx, AddPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: uid})
	if err != nil {
		return nil, fmt.Errorf("joining own playgroup: %w", err)
	}

	members, err := s.repo.ListPlaygroupMembers(ctx, playgroup.ID)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup members: %w", err)
	}
	return toPlaygroupResponse(&playgroup, members), nil
}

// GetPlaygroup devuelve el detalle de un grupo, si el usuario indicado es miembro.
func (s *service) GetPlaygroup(ctx context.Context, userID, id string) (*PlaygroupResponse, error) {
	playgroup, err := s.getMemberPlaygroup(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.ListPlaygroupMembers(ctx, playgroup.ID)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup members: %w", err)
	}
	return toPlaygroupResponse(playgroup, members), nil
}

// ListPlaygroups devuelve los grupos de los que el usuario indicado es miembro.
func (s *service) ListPlaygroups(ctx context.Context, userID string) ([]PlaygroupResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
	}

	list, err := s.repo.ListPlaygroupsForUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing playgroups: %w", err)
	}

	result := make([]PlaygroupResponse, 0, len(list))
	for i := range list {
		result = append(result, *toPlaygroupResponse(&list[i], nil))
	}
	return result, nil
}

// AddMember añade un usuario a un grupo de juego. Solo un miembro existente puede
// invitar a otros; el usuario a añadir debe existir y no estar ya en el grupo.
func (s *service) AddMember(
	ctx context.Context, playgroupID, requesterID string, req AddMemberRequest,
) (*PlaygroupMemberResponse, error) {
	playgroup, err := s.getMemberPlaygroup(ctx, requesterID, playgroupID)
	if err != nil {
		return nil, err
	}

	targetUID, err := common.ParseUUID(req.UserID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid user_id")
	}

	_, err = s.repo.GetUserByID(ctx, targetUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fiber.NewError(fiber.StatusNotFound, "user not found")
		}
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	_, err = s.repo.GetPlaygroupMember(ctx, GetPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: targetUID})
	if err == nil {
		return nil, fiber.NewError(fiber.StatusConflict, "user is already a member")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("checking membership: %w", err)
	}

	member, err := s.repo.AddPlaygroupMember(ctx, AddPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: targetUID})
	if err != nil {
		return nil, fmt.Errorf("adding member: %w", err)
	}
	return toMemberResponse(&member), nil
}

// getMemberPlaygroup resuelve un grupo por ID solo si el usuario indicado es miembro; no
// distingue "el grupo no existe" de "no sos miembro", para no revelar grupos ajenos.
func (s *service) getMemberPlaygroup(ctx context.Context, userID, id string) (*Playgroup, error) {
	pid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}

	playgroup, err := s.repo.GetPlaygroup(ctx, pid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaygroupNotFound
		}
		return nil, fmt.Errorf("looking up playgroup: %w", err)
	}

	_, err = s.repo.GetPlaygroupMember(ctx, GetPlaygroupMemberParams{PlaygroupID: pid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaygroupNotFound
		}
		return nil, fmt.Errorf("checking membership: %w", err)
	}

	return &playgroup, nil
}

func toPlaygroupResponse(p *Playgroup, members []PlaygroupMember) *PlaygroupResponse {
	res := &PlaygroupResponse{ID: p.ID.String(), Name: p.Name}
	if members != nil {
		res.Members = make([]PlaygroupMemberResponse, 0, len(members))
		for i := range members {
			res.Members = append(res.Members, *toMemberResponse(&members[i]))
		}
	}
	return res
}

func toMemberResponse(m *PlaygroupMember) *PlaygroupMemberResponse {
	return &PlaygroupMemberResponse{PlaygroupID: m.PlaygroupID.String(), UserID: m.UserID.String()}
}
