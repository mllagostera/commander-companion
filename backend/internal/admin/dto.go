package admin

import "time"

// UserSummaryResponse is the DTO of one row in the admin user listing —
// deliberately lighter than UserDetailResponse (no moxfield_username, no counts),
// same "list is for browsing, detail is for everything else" split already used by
// playgroups.ListPlaygroupsPage vs. GetPlaygroup.
type UserSummaryResponse struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	EmailVerified bool      `json:"email_verified"`
	IsAdmin       bool      `json:"is_admin"`
	IsActive      bool      `json:"is_active"`
}

// UserListResponse is a page of users (see Service.ListUsers). NextCursor is the
// opaque cursor for the next page, null on the last page — same shape as
// playgroups.PlaygroupListResponse/games.GameListResponse.
type UserListResponse struct {
	Items      []UserSummaryResponse `json:"items"`
	NextCursor *string               `json:"next_cursor"`
}

// UserDetailResponse is a single user's admin-facing profile.
type UserDetailResponse struct {
	ID               string    `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	CreatedAt        time.Time `json:"created_at"`
	MoxfieldUsername *string   `json:"moxfield_username,omitempty"`
	HasPassword      bool      `json:"has_password"`
	EmailVerified    bool      `json:"email_verified"`
	IsAdmin          bool      `json:"is_admin"`
	IsActive         bool      `json:"is_active"`
	DeckCount        int64     `json:"deck_count"`
	GamesPlayedCount int64     `json:"games_played_count"`
}

// UpdateUserStatusRequest is the payload of PATCH /admin/users/{id}/status.
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// OverviewStatsResponse is the DTO of GET /admin/stats/overview.
type OverviewStatsResponse struct {
	TotalUsers         int64 `json:"total_users"`
	ActiveUsers        int64 `json:"active_users"`
	VerifiedUsers      int64 `json:"verified_users"`
	TotalDecks         int64 `json:"total_decks"`
	TotalPlaygroups    int64 `json:"total_playgroups"`
	TotalFinishedGames int64 `json:"total_finished_games"`
	TotalTournaments   int64 `json:"total_tournaments"`
	// OnlineUsers approximates "currently online" as "has at least one unexpired,
	// unrevoked refresh token" — there's no real-time presence tracking, so this
	// reads as "has an active session right now", not "has the app open this
	// instant". See ADR-0018's addendum.
	OnlineUsers int64 `json:"online_users"`
	// ActiveGames is the count of games in the 'active' state (started, not yet finished).
	ActiveGames int64 `json:"active_games"`
}

// DailyActivityPoint is one day of Service.GetDailyActivity's historical series.
type DailyActivityPoint struct {
	// Date is the calendar day, formatted "2006-01-02" (UTC).
	Date         string `json:"date"`
	GamesStarted int64  `json:"games_started"`
	ActiveUsers  int64  `json:"active_users"`
}
