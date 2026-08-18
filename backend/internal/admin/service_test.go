package admin_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/admin"
	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

func newAdminSvc(t *testing.T) (admin.Service, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.DB(t)
	// "games"/"playgroups" clean up game_players via CASCADE; "users" cleans up
	// decks/refresh_tokens (same set statistics.service_test.go uses).
	testutil.Truncate(t, pool, "games", "playgroups", "users")
	return admin.NewService(pool), pool
}

// createPlaygroup inserts a bare playgroup row directly (no member rows needed:
// the queries under test here don't join through playgroup_members).
func createPlaygroup(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		"INSERT INTO playgroups (name) VALUES ($1) RETURNING id", name).Scan(&id)
	if err != nil {
		t.Fatalf("insertando playgroup de test: %v", err)
	}
	return id
}

// createGame inserts a game row directly with an explicit started_at, so tests can
// place it on a specific calendar day for GetDailyActivity. deck_id is nullable on
// game_players (see migrations/00001_initial_schema.sql), so no deck is needed.
func createGame(t *testing.T, pool *pgxpool.Pool, playgroupID, status string, startedAt time.Time) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		"INSERT INTO games (playgroup_id, status, started_at) VALUES ($1, $2, $3) RETURNING id",
		playgroupID, status, startedAt).Scan(&id)
	if err != nil {
		t.Fatalf("insertando partida de test: %v", err)
	}
	return id
}

func addGamePlayer(t *testing.T, pool *pgxpool.Pool, gameID, userID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO game_players (game_id, user_id) VALUES ($1, $2)", gameID, userID); err != nil {
		t.Fatalf("insertando game_player de test: %v", err)
	}
}

// insertActiveRefreshToken gives userID an unexpired, unrevoked refresh token, so it
// counts toward GetOverviewStats' online_users. The hash doesn't need to be real (no
// login flow is exercised here), only unique per row.
func insertActiveRefreshToken(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + interval '1 day')`,
		userID, "test-hash-"+userID); err != nil {
		t.Fatalf("insertando refresh token de test: %v", err)
	}
}

func registerUser(t *testing.T, pool *pgxpool.Pool, username, email string) *users.UserResponse {
	t.Helper()
	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: username,
		Email:    email,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	return user
}

func TestListUsers_PaginatesMostRecentFirst(t *testing.T) {
	svc, pool := newAdminSvc(t)

	registerUser(t, pool, "page-a", "page-a@example.com")
	registerUser(t, pool, "page-b", "page-b@example.com")
	registerUser(t, pool, "page-c", "page-c@example.com")

	page, err := svc.ListUsers(context.Background(), common.PageRequest{Limit: 2}, "")
	if err != nil {
		t.Fatalf("ListUsers() error = %v, want nil", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("ListUsers() primera página = %d items, want 2", len(page.Items))
	}
	if page.NextCursor == nil {
		t.Fatal("ListUsers() primera página: next_cursor = nil, want un cursor (hay una tercera fila)")
	}
	if page.Items[0].Username != "page-c" {
		t.Fatalf("ListUsers() primer item = %q, want %q (más reciente primero)", page.Items[0].Username, "page-c")
	}

	next, err := svc.ListUsers(context.Background(), common.PageRequest{Limit: 2, Cursor: *page.NextCursor}, "")
	if err != nil {
		t.Fatalf("ListUsers() segunda página: error = %v, want nil", err)
	}
	if len(next.Items) != 1 {
		t.Fatalf("ListUsers() segunda página = %d items, want 1", len(next.Items))
	}
	if next.NextCursor != nil {
		t.Fatalf("ListUsers() segunda página: next_cursor = %v, want nil (última página)", *next.NextCursor)
	}
	if next.Items[0].Username != "page-a" {
		t.Fatalf("ListUsers() segunda página item = %q, want %q", next.Items[0].Username, "page-a")
	}
}

func TestListUsers_SearchMatchesUsernameOrEmail(t *testing.T) {
	svc, pool := newAdminSvc(t)

	registerUser(t, pool, "findme", "someone@example.com")
	registerUser(t, pool, "other", "findme-by-email@example.com")
	registerUser(t, pool, "unrelated", "unrelated@example.com")

	page, err := svc.ListUsers(context.Background(), common.PageRequest{Limit: 10}, "findme")
	if err != nil {
		t.Fatalf("ListUsers() error = %v, want nil", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("ListUsers() con search=findme = %d items, want 2", len(page.Items))
	}
}

func TestGetUserDetail_Success(t *testing.T) {
	svc, pool := newAdminSvc(t)

	created := registerUser(t, pool, "detail-user", "detail-user@example.com")

	detail, err := svc.GetUserDetail(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetUserDetail() error = %v, want nil", err)
	}
	if detail.ID != created.ID {
		t.Fatalf("GetUserDetail() id = %q, want %q", detail.ID, created.ID)
	}
	if detail.DeckCount != 0 || detail.GamesPlayedCount != 0 {
		t.Fatalf("GetUserDetail() de una cuenta nueva: deck_count=%d games_played_count=%d, want 0/0",
			detail.DeckCount, detail.GamesPlayedCount)
	}
	if detail.IsAdmin {
		t.Fatal("GetUserDetail() IsAdmin = true, want false (cuenta recién creada)")
	}
	if !detail.IsActive {
		t.Fatal("GetUserDetail() IsActive = false, want true (cuenta recién creada)")
	}
}

func TestGetUserDetail_CountsDecks(t *testing.T) {
	svc, pool := newAdminSvc(t)

	created := registerUser(t, pool, "deck-owner", "deck-owner@example.com")
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO decks (user_id, name, commander) VALUES ($1, $2, $3)",
		created.ID, "Test Deck", "Test Commander"); err != nil {
		t.Fatalf("insertando mazo de test: %v", err)
	}

	detail, err := svc.GetUserDetail(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetUserDetail() error = %v, want nil", err)
	}
	if detail.DeckCount != 1 {
		t.Fatalf("GetUserDetail() deck_count = %d, want 1", detail.DeckCount)
	}
}

func TestGetUserDetail_UnknownUser_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.GetUserDetail(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Fatalf("GetUserDetail() con usuario inexistente: error = %v, want ErrUserNotFound", err)
	}
}

func TestUpdateUserStatus_Deactivate_Success(t *testing.T) {
	svc, pool := newAdminSvc(t)

	caller := registerUser(t, pool, "the-admin", "the-admin@example.com")
	target := registerUser(t, pool, "moderated", "moderated@example.com")

	updated, err := svc.UpdateUserStatus(context.Background(), caller.ID, target.ID, false)
	if err != nil {
		t.Fatalf("UpdateUserStatus() error = %v, want nil", err)
	}
	if updated.IsActive {
		t.Fatal("UpdateUserStatus(false) IsActive = true, want false")
	}

	// Reactivating is symmetric.
	reactivated, err := svc.UpdateUserStatus(context.Background(), caller.ID, target.ID, true)
	if err != nil {
		t.Fatalf("UpdateUserStatus() reactivando: error = %v, want nil", err)
	}
	if !reactivated.IsActive {
		t.Fatal("UpdateUserStatus(true) IsActive = false, want true")
	}
}

func TestUpdateUserStatus_CannotDeactivateSelf(t *testing.T) {
	svc, pool := newAdminSvc(t)

	caller := registerUser(t, pool, "self-admin", "self-admin@example.com")

	_, err := svc.UpdateUserStatus(context.Background(), caller.ID, caller.ID, false)
	if !errors.Is(err, admin.ErrCannotDeactivateSelf) {
		t.Fatalf("UpdateUserStatus() autodesactivación: error = %v, want ErrCannotDeactivateSelf", err)
	}
}

func TestUpdateUserStatus_SelfActivateIsAllowed(t *testing.T) {
	svc, pool := newAdminSvc(t)

	caller := registerUser(t, pool, "self-reactivate", "self-reactivate@example.com")

	// The self-lockout guard only blocks deactivating your own account, not
	// activating it (a no-op here, since it's already active) — see ADR-0018.
	updated, err := svc.UpdateUserStatus(context.Background(), caller.ID, caller.ID, true)
	if err != nil {
		t.Fatalf("UpdateUserStatus(self, true) error = %v, want nil", err)
	}
	if !updated.IsActive {
		t.Fatal("UpdateUserStatus(self, true) IsActive = false, want true")
	}
}

func TestUpdateUserStatus_UnknownUser_ReturnsNotFound(t *testing.T) {
	svc, pool := newAdminSvc(t)

	caller := registerUser(t, pool, "caller-only", "caller-only@example.com")

	_, err := svc.UpdateUserStatus(context.Background(), caller.ID, "00000000-0000-0000-0000-000000000000", false)
	if !errors.Is(err, admin.ErrUserNotFound) {
		t.Fatalf("UpdateUserStatus() con usuario inexistente: error = %v, want ErrUserNotFound", err)
	}
}

func TestGetOverviewStats_CountsUsers(t *testing.T) {
	svc, pool := newAdminSvc(t)

	registerUser(t, pool, "stats-a", "stats-a@example.com")
	registerUser(t, pool, "stats-b", "stats-b@example.com")

	stats, err := svc.GetOverviewStats(context.Background())
	if err != nil {
		t.Fatalf("GetOverviewStats() error = %v, want nil", err)
	}
	if stats.TotalUsers != 2 {
		t.Fatalf("GetOverviewStats() total_users = %d, want 2", stats.TotalUsers)
	}
	if stats.ActiveUsers != 2 {
		t.Fatalf("GetOverviewStats() active_users = %d, want 2", stats.ActiveUsers)
	}
}

func TestGetOverviewStats_CountsOnlineUsersAndActiveGames(t *testing.T) {
	svc, pool := newAdminSvc(t)

	online := registerUser(t, pool, "online-user", "online-user@example.com")
	registerUser(t, pool, "offline-user", "offline-user@example.com")
	insertActiveRefreshToken(t, pool, online.ID)

	playgroupID := createPlaygroup(t, pool, "activity-pg")
	createGame(t, pool, playgroupID, "active", time.Now())
	createGame(t, pool, playgroupID, "active", time.Now())
	createGame(t, pool, playgroupID, "finished", time.Now())

	stats, err := svc.GetOverviewStats(context.Background())
	if err != nil {
		t.Fatalf("GetOverviewStats() error = %v, want nil", err)
	}
	if stats.OnlineUsers != 1 {
		t.Fatalf("GetOverviewStats() online_users = %d, want 1", stats.OnlineUsers)
	}
	if stats.ActiveGames != 2 {
		t.Fatalf("GetOverviewStats() active_games = %d, want 2 (the finished game must not count)", stats.ActiveGames)
	}
}

func TestGetOverviewStats_ExpiredOrRevokedTokenDoesNotCountAsOnline(t *testing.T) {
	svc, pool := newAdminSvc(t)

	expired := registerUser(t, pool, "expired-session", "expired-session@example.com")
	revoked := registerUser(t, pool, "revoked-session", "revoked-session@example.com")

	if _, err := pool.Exec(context.Background(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() - interval '1 day')`,
		expired.ID, "expired-hash"); err != nil {
		t.Fatalf("insertando refresh token vencido de test: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at, revoked_at)
		 VALUES ($1, $2, now() + interval '1 day', now())`,
		revoked.ID, "revoked-hash"); err != nil {
		t.Fatalf("insertando refresh token revocado de test: %v", err)
	}

	stats, err := svc.GetOverviewStats(context.Background())
	if err != nil {
		t.Fatalf("GetOverviewStats() error = %v, want nil", err)
	}
	if stats.OnlineUsers != 0 {
		t.Fatalf("GetOverviewStats() online_users = %d, want 0 (both tokens are expired/revoked)", stats.OnlineUsers)
	}
}

func TestGetDailyActivity_FillsGapsAndCountsRealDays(t *testing.T) {
	svc, pool := newAdminSvc(t)

	userA := registerUser(t, pool, "activity-a", "activity-a@example.com")
	userB := registerUser(t, pool, "activity-b", "activity-b@example.com")
	playgroupID := createPlaygroup(t, pool, "daily-activity-pg")

	today := time.Now().UTC().Truncate(24 * time.Hour)
	twoDaysAgo := today.AddDate(0, 0, -2)

	// Today: two games, two distinct players across them -> active_users = 2.
	gameToday1 := createGame(t, pool, playgroupID, "finished", today.Add(2*time.Hour))
	addGamePlayer(t, pool, gameToday1, userA.ID)
	gameToday2 := createGame(t, pool, playgroupID, "active", today.Add(3*time.Hour))
	addGamePlayer(t, pool, gameToday2, userB.ID)

	// Two days ago: one game, one player.
	gameOld := createGame(t, pool, playgroupID, "finished", twoDaysAgo.Add(time.Hour))
	addGamePlayer(t, pool, gameOld, userA.ID)

	points, err := svc.GetDailyActivity(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetDailyActivity() error = %v, want nil", err)
	}
	if len(points) != 5 {
		t.Fatalf("GetDailyActivity(5) devolvió %d puntos, want 5", len(points))
	}
	if points[len(points)-1].Date != today.Format("2006-01-02") {
		t.Fatalf("último punto = %q, want hoy (%q)", points[len(points)-1].Date, today.Format("2006-01-02"))
	}

	byDate := make(map[string]admin.DailyActivityPoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}

	assertActivityPoint(t, byDate, today, "hoy", 2, 2)
	assertActivityPoint(t, byDate, twoDaysAgo, "hace 2 días", 1, 1)
	// A day with no games at all must still appear, filled to zero.
	assertActivityPoint(t, byDate, today.AddDate(0, 0, -1), "ayer (sin partidas)", 0, 0)
}

func assertActivityPoint(
	t *testing.T, byDate map[string]admin.DailyActivityPoint, day time.Time, label string, wantGames, wantUsers int64,
) {
	t.Helper()
	point := byDate[day.Format("2006-01-02")]
	if point.GamesStarted != wantGames || point.ActiveUsers != wantUsers {
		t.Fatalf("punto de %s = %+v, want games_started=%d active_users=%d", label, point, wantGames, wantUsers)
	}
}

func TestGetDailyActivity_ClampsDaysBack(t *testing.T) {
	svc, _ := newAdminSvc(t)

	tooFew, err := svc.GetDailyActivity(context.Background(), 0)
	if err != nil {
		t.Fatalf("GetDailyActivity(0) error = %v, want nil", err)
	}
	if len(tooFew) != 1 {
		t.Fatalf("GetDailyActivity(0) devolvió %d puntos, want 1 (clamped to the minimum)", len(tooFew))
	}

	tooMany, err := svc.GetDailyActivity(context.Background(), 10000)
	if err != nil {
		t.Fatalf("GetDailyActivity(10000) error = %v, want nil", err)
	}
	if len(tooMany) != 90 {
		t.Fatalf("GetDailyActivity(10000) devolvió %d puntos, want 90 (clamped to the maximum)", len(tooMany))
	}
}
