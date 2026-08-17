package admin_test

import (
	"context"
	"errors"
	"testing"

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
	testutil.Truncate(t, pool, "users")
	return admin.NewService(pool), pool
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
