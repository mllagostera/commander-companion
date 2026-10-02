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

// fakeBroadcaster records the games admin announced as deleted.
type fakeBroadcaster struct {
	deleted []string
}

func (f *fakeBroadcaster) BroadcastGameDeleted(gameID string) {
	f.deleted = append(f.deleted, gameID)
}

func newAdminSvc(t *testing.T) (admin.Service, *pgxpool.Pool) {
	t.Helper()
	svc, pool, _ := newAdminSvcWithBroadcaster(t)
	return svc, pool
}

func newAdminSvcWithBroadcaster(t *testing.T) (admin.Service, *pgxpool.Pool, *fakeBroadcaster) {
	t.Helper()
	pool := testutil.DB(t)
	// "games"/"playgroups" clean up game_players via CASCADE; "users" cleans up
	// decks/refresh_tokens (same set statistics.service_test.go uses).
	testutil.Truncate(t, pool, "games", "playgroups", "users")
	broadcaster := &fakeBroadcaster{}
	return admin.NewService(pool, broadcaster), pool, broadcaster
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

// createGameCreatedAt inserts a game row with an explicit created_at, so tests
// can control the order of ListUnfinishedGames (oldest first).
func createGameCreatedAt(t *testing.T, pool *pgxpool.Pool, playgroupID, status string, createdAt time.Time) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		"INSERT INTO games (playgroup_id, status, created_at) VALUES ($1, $2, $3) RETURNING id",
		playgroupID, status, createdAt).Scan(&id)
	if err != nil {
		t.Fatalf("inserting test game: %v", err)
	}
	return id
}

func addGamePlayer(t *testing.T, pool *pgxpool.Pool, gameID, userID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"INSERT INTO game_players (game_id, user_id) VALUES ($1, $2) RETURNING id",
		gameID, userID).Scan(&id); err != nil {
		t.Fatalf("insertando game_player de test: %v", err)
	}
	return id
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting rows (%s): %v", query, err)
	}
	return n
}

// setLastSeen sets userID's last_seen_at to ago before now, standing in for the
// writes users.ActivityTracker makes on authenticated requests.
func setLastSeen(t *testing.T, pool *pgxpool.Pool, userID string, ago time.Duration) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE users SET last_seen_at = now() - make_interval(secs => $2) WHERE id = $1`,
		userID, ago.Seconds()); err != nil {
		t.Fatalf("setting test last_seen_at: %v", err)
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
	setLastSeen(t, pool, online.ID, time.Minute)

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

func TestGetOverviewStats_StaleOrNeverSeenDoesNotCountAsOnline(t *testing.T) {
	svc, pool := newAdminSvc(t)

	stale := registerUser(t, pool, "stale-user", "stale-user@example.com")
	registerUser(t, pool, "never-seen", "never-seen@example.com")
	setLastSeen(t, pool, stale.ID, 10*time.Minute)

	stats, err := svc.GetOverviewStats(context.Background())
	if err != nil {
		t.Fatalf("GetOverviewStats() error = %v, want nil", err)
	}
	if stats.OnlineUsers != 0 {
		t.Fatalf("GetOverviewStats() online_users = %d, want 0 (one seen 10 minutes ago, one never seen)", stats.OnlineUsers)
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

func TestListUnfinishedGames_OldestFirst_ExcludesFinished(t *testing.T) {
	svc, pool := newAdminSvc(t)

	playgroupID := createPlaygroup(t, pool, "order-pg")
	now := time.Now()
	newer := createGameCreatedAt(t, pool, playgroupID, "pending", now.Add(-time.Hour))
	older := createGameCreatedAt(t, pool, playgroupID, "active", now.Add(-48*time.Hour))
	createGameCreatedAt(t, pool, playgroupID, "finished", now.Add(-72*time.Hour))

	res, err := svc.ListUnfinishedGames(context.Background(), common.PageRequest{Limit: 10}, "")
	if err != nil {
		t.Fatalf("ListUnfinishedGames() error = %v, want nil", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("ListUnfinishedGames() returned %d games, want 2 (the finished one must not appear)", len(res.Items))
	}
	if res.Items[0].ID != older || res.Items[1].ID != newer {
		t.Fatalf("ListUnfinishedGames() order = [%s, %s], want oldest first [%s, %s]",
			res.Items[0].ID, res.Items[1].ID, older, newer)
	}
	if res.NextCursor != nil {
		t.Fatalf("ListUnfinishedGames() next_cursor = %v, want nil on the last page", *res.NextCursor)
	}
}

func TestListUnfinishedGames_IncludesPlaygroupAndPlayers(t *testing.T) {
	svc, pool := newAdminSvc(t)

	alice := registerUser(t, pool, "alice", "alice@example.com")
	bob := registerUser(t, pool, "bob", "bob@example.com")
	playgroupID := createPlaygroup(t, pool, "friday-pg")
	seated := createGameCreatedAt(t, pool, playgroupID, "active", time.Now().Add(-2*time.Hour))
	createGameCreatedAt(t, pool, playgroupID, "pending", time.Now().Add(-time.Hour))
	addGamePlayer(t, pool, seated, bob.ID)
	addGamePlayer(t, pool, seated, alice.ID)

	res, err := svc.ListUnfinishedGames(context.Background(), common.PageRequest{Limit: 10}, "")
	if err != nil {
		t.Fatalf("ListUnfinishedGames() error = %v, want nil", err)
	}
	first, empty := res.Items[0], res.Items[1]
	if first.PlaygroupName == nil || *first.PlaygroupName != "friday-pg" {
		t.Fatalf("ListUnfinishedGames() playgroup_name = %v, want friday-pg", first.PlaygroupName)
	}
	if len(first.Players) != 2 || first.Players[0].Username != "alice" || first.Players[1].Username != "bob" {
		t.Fatalf("ListUnfinishedGames() players = %+v, want alice and bob, by username", first.Players)
	}
	if empty.Players == nil || len(empty.Players) != 0 {
		t.Fatalf("ListUnfinishedGames() players of a game with no seats = %v, want an empty list", empty.Players)
	}
}

func TestListUnfinishedGames_FiltersByStatus(t *testing.T) {
	svc, pool := newAdminSvc(t)

	playgroupID := createPlaygroup(t, pool, "filter-pg")
	pending := createGameCreatedAt(t, pool, playgroupID, "pending", time.Now().Add(-time.Hour))
	createGameCreatedAt(t, pool, playgroupID, "active", time.Now().Add(-2*time.Hour))

	res, err := svc.ListUnfinishedGames(context.Background(), common.PageRequest{Limit: 10}, "pending")
	if err != nil {
		t.Fatalf("ListUnfinishedGames(pending) error = %v, want nil", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != pending {
		t.Fatalf("ListUnfinishedGames(pending) = %+v, want only %s", res.Items, pending)
	}
}

func TestListUnfinishedGames_InvalidStatus_ReturnsError(t *testing.T) {
	svc, _ := newAdminSvc(t)

	_, err := svc.ListUnfinishedGames(context.Background(), common.PageRequest{Limit: 10}, "finished")
	if !errors.Is(err, admin.ErrInvalidGameStatus) {
		t.Fatalf("ListUnfinishedGames(finished) error = %v, want ErrInvalidGameStatus", err)
	}
}

func TestListUnfinishedGames_Paginates(t *testing.T) {
	svc, pool := newAdminSvc(t)

	playgroupID := createPlaygroup(t, pool, "page-pg")
	now := time.Now()
	createGameCreatedAt(t, pool, playgroupID, "pending", now.Add(-3*time.Hour))
	createGameCreatedAt(t, pool, playgroupID, "pending", now.Add(-2*time.Hour))
	newest := createGameCreatedAt(t, pool, playgroupID, "active", now.Add(-time.Hour))

	first, err := svc.ListUnfinishedGames(context.Background(), common.PageRequest{Limit: 2}, "")
	if err != nil {
		t.Fatalf("ListUnfinishedGames() page 1 error = %v, want nil", err)
	}
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("ListUnfinishedGames() page 1 = %d items, next_cursor %v; want 2 and a cursor",
			len(first.Items), first.NextCursor)
	}

	second, err := svc.ListUnfinishedGames(context.Background(),
		common.PageRequest{Limit: 2, Cursor: *first.NextCursor}, "")
	if err != nil {
		t.Fatalf("ListUnfinishedGames() page 2 error = %v, want nil", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != newest || second.NextCursor != nil {
		t.Fatalf("ListUnfinishedGames() page 2 = %+v, want only %s and no cursor", second.Items, newest)
	}
}

// seedGameWithDependents creates an active game in playgroupID with two seats and
// a row in every table that points at games (or at its seats), so a delete that
// misses one of them fails on its FK.
func seedGameWithDependents(t *testing.T, pool *pgxpool.Pool, playgroupID string) string {
	t.Helper()
	ctx := context.Background()

	alice := registerUser(t, pool, "del-alice", "del-alice@example.com")
	bob := registerUser(t, pool, "del-bob", "del-bob@example.com")
	gameID := createGame(t, pool, playgroupID, "active", time.Now())
	aliceSeat := addGamePlayer(t, pool, gameID, alice.ID)
	bobSeat := addGamePlayer(t, pool, gameID, bob.ID)
	if _, err := pool.Exec(ctx,
		"UPDATE games SET current_turn_player_id = $2 WHERE id = $1", gameID, aliceSeat); err != nil {
		t.Fatalf("setting test current turn: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO game_actions (game_id, actor_id, target_id, action_type, payload)
		 VALUES ($1, $2, $3, 'CommanderDamage', '{"amount": 5}')`, gameID, aliceSeat, bobSeat); err != nil {
		t.Fatalf("inserting test game action: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO commander_damage (game_id, attacker_id, defender_id, amount) VALUES ($1, $2, $3, 5)",
		gameID, aliceSeat, bobSeat); err != nil {
		t.Fatalf("inserting test commander damage: %v", err)
	}
	return gameID
}

func TestDeleteUnfinishedGame_DeletesGameAndDependentRows(t *testing.T) {
	svc, pool, broadcaster := newAdminSvcWithBroadcaster(t)

	playgroupID := createPlaygroup(t, pool, "delete-pg")
	gameID := seedGameWithDependents(t, pool, playgroupID)

	if err := svc.DeleteUnfinishedGame(context.Background(), gameID); err != nil {
		t.Fatalf("DeleteUnfinishedGame() error = %v, want nil", err)
	}

	for _, table := range []string{"game_actions", "commander_damage", "game_players"} {
		if n := countRows(t, pool, "SELECT count(*) FROM "+table+" WHERE game_id = $1", gameID); n != 0 {
			t.Fatalf("%s rows left for the deleted game = %d, want 0", table, n)
		}
	}
	if n := countRows(t, pool, "SELECT count(*) FROM games WHERE id = $1", gameID); n != 0 {
		t.Fatalf("games rows left = %d, want 0", n)
	}
	if n := countRows(t, pool, "SELECT count(*) FROM playgroups WHERE id = $1", playgroupID); n != 1 {
		t.Fatalf("playgroup rows = %d, want 1 (deleting a game must not touch its group)", n)
	}
	if len(broadcaster.deleted) != 1 || broadcaster.deleted[0] != gameID {
		t.Fatalf("broadcast game_deleted = %v, want [%s]", broadcaster.deleted, gameID)
	}
}

func TestDeleteUnfinishedGame_PendingGame_Succeeds(t *testing.T) {
	svc, pool := newAdminSvc(t)

	gameID := createGameCreatedAt(t, pool, createPlaygroup(t, pool, "pending-pg"), "pending", time.Now())

	if err := svc.DeleteUnfinishedGame(context.Background(), gameID); err != nil {
		t.Fatalf("DeleteUnfinishedGame() error = %v, want nil", err)
	}
	if n := countRows(t, pool, "SELECT count(*) FROM games WHERE id = $1", gameID); n != 0 {
		t.Fatalf("games rows left = %d, want 0", n)
	}
}

func TestDeleteUnfinishedGame_FinishedGame_ReturnsConflictAndKeepsIt(t *testing.T) {
	svc, pool, broadcaster := newAdminSvcWithBroadcaster(t)

	user := registerUser(t, pool, "finished-user", "finished-user@example.com")
	gameID := createGame(t, pool, createPlaygroup(t, pool, "finished-pg"), "finished", time.Now())
	addGamePlayer(t, pool, gameID, user.ID)

	err := svc.DeleteUnfinishedGame(context.Background(), gameID)
	if !errors.Is(err, admin.ErrGameFinished) {
		t.Fatalf("DeleteUnfinishedGame(finished) error = %v, want ErrGameFinished", err)
	}
	if n := countRows(t, pool, "SELECT count(*) FROM game_players WHERE game_id = $1", gameID); n != 1 {
		t.Fatalf("game_players rows = %d, want 1 (a rejected delete must leave the game intact)", n)
	}
	if len(broadcaster.deleted) != 0 {
		t.Fatalf("broadcast game_deleted = %v, want none", broadcaster.deleted)
	}
}

func TestDeleteUnfinishedGame_UnknownOrMalformedID_ReturnsNotFound(t *testing.T) {
	svc, _ := newAdminSvc(t)

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		if err := svc.DeleteUnfinishedGame(context.Background(), id); !errors.Is(err, admin.ErrGameNotFound) {
			t.Fatalf("DeleteUnfinishedGame(%q) error = %v, want ErrGameNotFound", id, err)
		}
	}
}
