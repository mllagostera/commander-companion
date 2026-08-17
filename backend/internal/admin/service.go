// Package admin implements the admin dashboard's backend: user listing/detail,
// account activation/deactivation, and a global stats overview. Every endpoint
// requires is_admin = true (see auth.RequireAdmin) — see ADR-0018 for the design.
package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// ErrUserNotFound indicates that no user exists with the given ID.
var ErrUserNotFound = common.NotFound("user not found")

// ErrCannotDeactivateSelf indicates that an admin tried to deactivate their own
// account — a self-lockout guard, since there's no other admin-promotion path yet
// (see ADR-0018) to undo it.
var ErrCannotDeactivateSelf = common.InvalidInput("cannot deactivate your own account")

// Service is the interface for admin dashboard logic.
type Service interface {
	// ListUsers returns a page of users ordered most-recently-created first,
	// optionally filtered by a username/email substring.
	ListUsers(ctx context.Context, page common.PageRequest, search string) (*UserListResponse, error)
	// GetUserDetail returns a single user's admin-facing profile.
	GetUserDetail(ctx context.Context, id string) (*UserDetailResponse, error)
	// UpdateUserStatus activates/deactivates a user's account. callerID is the
	// authenticated admin making the request; ErrCannotDeactivateSelf if
	// callerID == id and isActive is false.
	UpdateUserStatus(ctx context.Context, callerID, id string, isActive bool) (*UserDetailResponse, error)
	// GetOverviewStats returns the global counts shown on the admin home page.
	GetOverviewStats(ctx context.Context) (*OverviewStatsResponse, error)
}

type service struct {
	repo *Queries
}

// NewService creates a new admin service.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// ListUsers returns a page of users, from most recently created to oldest. See
// internal/common/pagination.go for the cursor scheme (same keyset-over-(created_at,
// id) approach as playgroups.ListPlaygroupsPage).
func (s *service) ListUsers(
	ctx context.Context, page common.PageRequest, search string,
) (*UserListResponse, error) {
	params := ListUsersPageParams{PageLimit: page.Limit + 1}
	if search != "" {
		params.Search = pgtype.Text{String: search, Valid: true}
	}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, err := decodeCursor(page.Cursor)
		if err != nil {
			return nil, err
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListUsersPage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing users page: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}

	items := make([]UserSummaryResponse, 0, len(rows))
	for i := range rows {
		items = append(items, toUserSummaryResponse(&rows[i]))
	}
	return &UserListResponse{Items: items, NextCursor: nextCursor}, nil
}

// GetUserDetail returns a single user's admin-facing profile, including deck and
// games-played counts.
func (s *service) GetUserDetail(ctx context.Context, id string) (*UserDetailResponse, error) {
	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	row, err := s.repo.GetUserDetail(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("looking up user detail: %w", err)
	}

	return toUserDetailResponse(&row), nil
}

// UpdateUserStatus activates/deactivates a user's account (see ADR-0018). A
// deactivated account can't log in or refresh its session (internal/users,
// internal/auth), but keeps every other row (decks, games, statistics) intact.
func (s *service) UpdateUserStatus(
	ctx context.Context, callerID, id string, isActive bool,
) (*UserDetailResponse, error) {
	if !isActive && callerID == id {
		return nil, ErrCannotDeactivateSelf
	}

	uid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if _, err := s.repo.UpdateUserActiveStatus(ctx, UpdateUserActiveStatusParams{
		ID:       uid,
		IsActive: isActive,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("updating user active status: %w", err)
	}

	return s.GetUserDetail(ctx, id)
}

// GetOverviewStats returns the global counts shown on the admin home page,
// live-computed on every call (see query.sql: no summary table).
func (s *service) GetOverviewStats(ctx context.Context) (*OverviewStatsResponse, error) {
	row, err := s.repo.GetAdminOverviewStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting admin overview stats: %w", err)
	}

	return &OverviewStatsResponse{
		TotalUsers:         row.TotalUsers,
		ActiveUsers:        row.ActiveUsers,
		VerifiedUsers:      row.VerifiedUsers,
		TotalDecks:         row.TotalDecks,
		TotalPlaygroups:    row.TotalPlaygroups,
		TotalFinishedGames: row.TotalFinishedGames,
		TotalTournaments:   row.TotalTournaments,
	}, nil
}

// decodeCursor turns the opaque page cursor into the DB-typed pair ListUsersPage
// needs, same shape as playgroups.decodeCursor.
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

func toUserSummaryResponse(user *User) UserSummaryResponse {
	var createdAt time.Time
	if user.CreatedAt.Valid {
		createdAt = user.CreatedAt.Time
	}
	return UserSummaryResponse{
		ID:            user.ID.String(),
		Username:      user.Username,
		Email:         user.Email,
		CreatedAt:     createdAt,
		EmailVerified: user.EmailVerified,
		IsAdmin:       user.IsAdmin,
		IsActive:      user.IsActive,
	}
}

func toUserDetailResponse(row *GetUserDetailRow) *UserDetailResponse {
	var createdAt time.Time
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}

	res := &UserDetailResponse{
		ID:               row.ID.String(),
		Username:         row.Username,
		Email:            row.Email,
		CreatedAt:        createdAt,
		HasPassword:      row.PasswordHash.Valid,
		EmailVerified:    row.EmailVerified,
		IsAdmin:          row.IsAdmin,
		IsActive:         row.IsActive,
		DeckCount:        row.DeckCount,
		GamesPlayedCount: row.GamesPlayedCount,
	}
	if row.MoxfieldUsername.Valid {
		res.MoxfieldUsername = &row.MoxfieldUsername.String
	}
	return res
}
