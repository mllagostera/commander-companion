package playgroups

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

var (
	// ErrPlaygroupNotFound indicates that the group doesn't exist or the user isn't a member of it.
	ErrPlaygroupNotFound = common.NotFound("playgroup not found")
	// ErrUserNotFound indicates that the user to add to the group doesn't exist.
	ErrUserNotFound = common.NotFound("user not found")
	// ErrNameRequired indicates an attempt to create a group without a name.
	ErrNameRequired = common.InvalidInput("name is required")
	// ErrInvalidUserID indicates that the received user_id isn't a valid UUID.
	ErrInvalidUserID = common.InvalidInput("invalid user_id")
	// ErrAlreadyMember indicates that the user already belongs to the group.
	ErrAlreadyMember = common.Conflict("user is already a member")
)

// Service defines the business logic of the playgroups module.
type Service interface {
	CreatePlaygroup(ctx context.Context, userID string, req CreatePlaygroupRequest) (*PlaygroupResponse, error)
	GetPlaygroup(ctx context.Context, userID, id string) (*PlaygroupResponse, error)
	ListPlaygroups(ctx context.Context, userID string) ([]PlaygroupResponse, error)
	// ListPlaygroupsPage is the cursor-paginated counterpart of ListPlaygroups (see
	// internal/common/pagination.go), opt-in via the `cursor`/`limit` query params
	// so as not to change ListPlaygroups' response shape for existing clients.
	ListPlaygroupsPage(ctx context.Context, page common.PageRequest, userID string) (*PlaygroupListResponse, error)
	AddMember(ctx context.Context, playgroupID, requesterID string, req AddMemberRequest) (*PlaygroupMemberResponse, error)
	// UpdatePlaygroup renames a group. Only an existing member can do it.
	UpdatePlaygroup(
		ctx context.Context, playgroupID, requesterID string, req UpdatePlaygroupRequest,
	) (*PlaygroupResponse, error)
	// IsMember confirms whether userID belongs to playgroupID. Used by games.JoinGame
	// to authorize a proxy-join (see ADR-0013) — implements games.PlaygroupMembership.
	IsMember(ctx context.Context, playgroupID, userID string) (bool, error)
	// ListMemberDecks returns memberUserID's decks, if requesterID is also a
	// member of playgroupID (see ADR-0013).
	ListMemberDecks(ctx context.Context, playgroupID, requesterID, memberUserID string) ([]DeckResponse, error)
}

type service struct {
	repo *Queries
}

// NewService creates a new playgroups service.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// CreatePlaygroup creates a new playgroup and adds the creator as its first member.
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

// GetPlaygroup returns the detail of a group, if the given user is a member.
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

// ListPlaygroups returns the groups the given user is a member of,
// with their members populated (unlike before: the web listing needs
// to show the member count without an extra round-trip per group).
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

// ListPlaygroupsPage returns a page of the given user's playgroups, from most
// recently created to oldest, WITHOUT members populated (unlike ListPlaygroups --
// a paginated listing is for browsing many groups, not showing each one's roster
// up front; fetch the detail via GetPlaygroup for that). See
// internal/common/pagination.go for the cursor scheme.
func (s *service) ListPlaygroupsPage(
	ctx context.Context, page common.PageRequest, userID string,
) (*PlaygroupListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// One row more than the limit is requested: if it comes back, there's a
	// next page. Avoids a separate COUNT(*) just to know whether to keep paginating.
	params := ListPlaygroupsForUserPageParams{UserID: uid, PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, cursorErr := decodeCursor(page.Cursor)
		if cursorErr != nil {
			return nil, cursorErr
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListPlaygroupsForUserPage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing playgroups page: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}

	items := make([]PlaygroupResponse, 0, len(rows))
	for i := range rows {
		items = append(items, *toPlaygroupResponse(&rows[i], nil))
	}
	return &PlaygroupListResponse{Items: items, NextCursor: nextCursor}, nil
}

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

// AddMember adds a user to a playgroup. Only an existing member can
// invite others; the user to add must exist and not already be in the group.
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

// UpdatePlaygroup renames a playgroup. Same authorization criteria as
// AddMember: only an existing member can edit it, and a group belonging to
// someone else or a nonexistent one respond the same way (ErrPlaygroupNotFound), without distinguishing.
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

// IsMember confirms whether userID belongs to playgroupID. Malformed IDs are treated
// as "not a member" instead of propagating a parse error: the caller (games,
// for a proxy-join) has already validated its own IDs before getting here.
func (s *service) IsMember(ctx context.Context, playgroupID, userID string) (bool, error) {
	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return false, nil //nolint:nilerr // malformed ID treated as "not a member" on purpose, see method doc
	}
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return false, nil //nolint:nilerr // same as above
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

// ListMemberDecks returns memberUserID's decks. Authorization: requesterID
// has to be a member of playgroupID (getMemberPlaygroup) AND memberUserID too
// — without distinguishing "someone else's group" from "that user isn't a member",
// same "don't reveal" criteria as the rest of the module.
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

// getMemberPlaygroup resolves a group by ID only if the given user is a member; it doesn't
// distinguish "the group doesn't exist" from "you're not a member", so as not to reveal other users' groups.
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
