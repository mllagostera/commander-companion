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

const (
	// dateLayout is the calendar-day format used in DailyActivityPoint.Date.
	dateLayout = "2006-01-02"
	// maxActivityDaysBack caps how far back GetDailyActivity will look, so an
	// arbitrary `days` query param can't force an unbounded scan/response.
	maxActivityDaysBack = 90
	hoursPerDay         = 24
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
	// GetDailyActivity returns one point per calendar day for the last daysBack
	// days (today inclusive), days with no games filled to zero.
	GetDailyActivity(ctx context.Context, daysBack int) ([]DailyActivityPoint, error)
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
		OnlineUsers:        row.OnlineUsers,
		ActiveGames:        row.ActiveGames,
	}, nil
}

// GetDailyActivity returns the last daysBack calendar days (today inclusive), one
// point each, for the admin dashboard's activity chart. The underlying query
// (see query.sql: GetDailyActivity) only returns rows for days that actually had
// a game, so this fills every day in the requested range to zero first and
// overlays the real counts on top — the chart always gets a continuous series,
// never a shorter one with holes on quiet days.
func (s *service) GetDailyActivity(ctx context.Context, daysBack int) ([]DailyActivityPoint, error) {
	if daysBack < 1 {
		daysBack = 1
	}
	if daysBack > maxActivityDaysBack {
		daysBack = maxActivityDaysBack
	}

	//nolint:gosec // bounded to [1, maxActivityDaysBack] right above
	rows, err := s.repo.GetDailyActivity(ctx, int32(daysBack))
	if err != nil {
		return nil, fmt.Errorf("getting daily activity: %w", err)
	}

	byDate := make(map[string]GetDailyActivityRow, len(rows))
	for i := range rows {
		byDate[rows[i].Day.Time.Format(dateLayout)] = rows[i]
	}

	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	points := make([]DailyActivityPoint, daysBack)
	for i := range daysBack {
		day := today.AddDate(0, 0, -(daysBack - 1 - i))
		date := day.Format(dateLayout)
		point := DailyActivityPoint{Date: date}
		if row, ok := byDate[date]; ok {
			point.GamesStarted = row.GamesStarted
			point.ActiveUsers = row.ActiveUsers
		}
		points[i] = point
	}

	return points, nil
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
