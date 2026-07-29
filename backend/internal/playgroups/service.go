package playgroups

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

var (
	// ErrPlaygroupNotFound indica que el grupo no existe o el usuario no es miembro de él.
	ErrPlaygroupNotFound = common.NotFound("playgroup not found")
	// ErrUserNotFound indica que el usuario a añadir al grupo no existe.
	ErrUserNotFound = common.NotFound("user not found")
	// ErrNameRequired indica que se intentó crear un grupo sin nombre.
	ErrNameRequired = common.InvalidInput("name is required")
	// ErrInvalidUserID indica que el user_id recibido no es un UUID válido.
	ErrInvalidUserID = common.InvalidInput("invalid user_id")
	// ErrAlreadyMember indica que el usuario ya pertenece al grupo.
	ErrAlreadyMember = common.Conflict("user is already a member")
)

// Service define la lógica de negocio del módulo playgroups.
type Service interface {
	CreatePlaygroup(ctx context.Context, userID string, req CreatePlaygroupRequest) (*PlaygroupResponse, error)
	GetPlaygroup(ctx context.Context, userID, id string) (*PlaygroupResponse, error)
	ListPlaygroups(ctx context.Context, userID string) ([]PlaygroupResponse, error)
	AddMember(ctx context.Context, playgroupID, requesterID string, req AddMemberRequest) (*PlaygroupMemberResponse, error)
	// UpdatePlaygroup renombra un grupo. Solo un miembro existente puede hacerlo.
	UpdatePlaygroup(
		ctx context.Context, playgroupID, requesterID string, req UpdatePlaygroupRequest,
	) (*PlaygroupResponse, error)
	// IsMember confirma si userID pertenece a playgroupID. Usado por games.JoinGame
	// para autorizar un proxy-join (ver ADR-0013) — implementa games.PlaygroupMembership.
	IsMember(ctx context.Context, playgroupID, userID string) (bool, error)
	// ListMemberDecks devuelve los decks de memberUserID, si requesterID también es
	// miembro de playgroupID (ver ADR-0013).
	ListMemberDecks(ctx context.Context, playgroupID, requesterID, memberUserID string) ([]DeckResponse, error)
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
		return nil, ErrNameRequired
	}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
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

// ListPlaygroups devuelve los grupos de los que el usuario indicado es miembro,
// con sus miembros poblados (a diferencia de antes: el listado de la web necesita
// mostrar la cantidad de miembros sin un round-trip extra por grupo).
func (s *service) ListPlaygroups(ctx context.Context, userID string) ([]PlaygroupResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	list, err := s.repo.ListPlaygroupsForUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing playgroups: %w", err)
	}

	result := make([]PlaygroupResponse, 0, len(list))
	for i := range list {
		members, err := s.repo.ListPlaygroupMembers(ctx, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("listing playgroup members: %w", err)
		}
		result = append(result, *toPlaygroupResponse(&list[i], members))
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
		return nil, ErrInvalidUserID
	}

	targetUser, err := s.repo.GetUserByID(ctx, targetUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	_, err = s.repo.GetPlaygroupMember(ctx, GetPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: targetUID})
	if err == nil {
		return nil, ErrAlreadyMember
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("checking membership: %w", err)
	}

	member, err := s.repo.AddPlaygroupMember(ctx, AddPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: targetUID})
	if err != nil {
		return nil, fmt.Errorf("adding member: %w", err)
	}
	return &PlaygroupMemberResponse{
		PlaygroupID: member.PlaygroupID.String(),
		UserID:      member.UserID.String(),
		Username:    targetUser.Username,
	}, nil
}

// UpdatePlaygroup renombra un grupo de juego. Mismo criterio de autorización que
// AddMember: solo un miembro existente puede editarlo, y un grupo ajeno o
// inexistente responde igual (ErrPlaygroupNotFound), sin distinguir.
func (s *service) UpdatePlaygroup(
	ctx context.Context, playgroupID, requesterID string, req UpdatePlaygroupRequest,
) (*PlaygroupResponse, error) {
	playgroup, err := s.getMemberPlaygroup(ctx, requesterID, playgroupID)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	updated, err := s.repo.UpdatePlaygroupName(ctx, UpdatePlaygroupNameParams{ID: playgroup.ID, Name: name})
	if err != nil {
		return nil, fmt.Errorf("renaming playgroup: %w", err)
	}

	members, err := s.repo.ListPlaygroupMembers(ctx, updated.ID)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup members: %w", err)
	}
	return toPlaygroupResponse(&updated, members), nil
}

// IsMember confirma si userID pertenece a playgroupID. IDs malformados se tratan
// como "no es miembro" en vez de propagar un error de parseo: quien llama (games,
// para un proxy-join) ya validó sus propios IDs antes de llegar acá.
func (s *service) IsMember(ctx context.Context, playgroupID, userID string) (bool, error) {
	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return false, nil //nolint:nilerr // ID malformado tratado como "no es miembro" a propósito, ver doc del método
	}
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return false, nil //nolint:nilerr // idem
	}

	_, err = s.repo.GetPlaygroupMember(ctx, GetPlaygroupMemberParams{PlaygroupID: pid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("checking membership: %w", err)
	}
	return true, nil
}

// ListMemberDecks devuelve los decks de memberUserID. Autorización: requesterID
// tiene que ser miembro de playgroupID (getMemberPlaygroup) Y memberUserID también
// — sin distinguir "grupo ajeno" de "ese usuario no es miembro", mismo criterio de
// no revelar que el resto del módulo.
func (s *service) ListMemberDecks(
	ctx context.Context, playgroupID, requesterID, memberUserID string,
) ([]DeckResponse, error) {
	playgroup, err := s.getMemberPlaygroup(ctx, requesterID, playgroupID)
	if err != nil {
		return nil, err
	}

	memberUID, err := common.ParseUUID(memberUserID)
	if err != nil {
		return nil, ErrPlaygroupNotFound
	}
	if _, memberErr := s.repo.GetPlaygroupMember(
		ctx, GetPlaygroupMemberParams{PlaygroupID: playgroup.ID, UserID: memberUID},
	); memberErr != nil {
		if errors.Is(memberErr, pgx.ErrNoRows) {
			return nil, ErrPlaygroupNotFound
		}
		return nil, fmt.Errorf("checking target membership: %w", memberErr)
	}

	memberDecks, err := s.repo.ListDecksByUserID(ctx, memberUID)
	if err != nil {
		return nil, fmt.Errorf("listing member decks: %w", err)
	}

	result := make([]DeckResponse, 0, len(memberDecks))
	for i := range memberDecks {
		result = append(result, *toDeckResponse(&memberDecks[i]))
	}
	return result, nil
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

func toPlaygroupResponse(p *Playgroup, members []ListPlaygroupMembersRow) *PlaygroupResponse {
	res := &PlaygroupResponse{ID: p.ID.String(), Name: p.Name}
	if members != nil {
		res.Members = make([]PlaygroupMemberResponse, 0, len(members))
		for i := range members {
			res.Members = append(res.Members, PlaygroupMemberResponse{
				PlaygroupID: members[i].PlaygroupID.String(),
				UserID:      members[i].UserID.String(),
				Username:    members[i].Username,
			})
		}
	}
	return res
}

func toDeckResponse(d *Deck) *DeckResponse {
	res := &DeckResponse{ID: d.ID.String(), UserID: d.UserID.String(), Name: d.Name, Commander: d.Commander}
	if d.MoxfieldID.Valid {
		res.MoxfieldID = d.MoxfieldID.String
	}
	if d.ImageUrl.Valid {
		res.ImageURL = d.ImageUrl.String
	}
	return res
}
