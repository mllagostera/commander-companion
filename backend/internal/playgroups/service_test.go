package playgroups_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/playgroups"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const renamedPlaygroupName = "Nombre nuevo"

func truncatePlaygroupsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "playgroups" cleans up playgroup_members via CASCADE; "users" cleans up any remainder.
	testutil.Truncate(t, pool, "playgroups", "users")
}

func createTestUser(t *testing.T, pool *pgxpool.Pool, email string) *users.UserResponse {
	t.Helper()
	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email,
		Email:    email,
		Password: "test-password-123",
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	return user
}

func mustCreatePlaygroup(t *testing.T, svc playgroups.Service, userID, name string) *playgroups.PlaygroupResponse {
	t.Helper()
	res, err := svc.CreatePlaygroup(context.Background(), userID, playgroups.CreatePlaygroupRequest{Name: name})
	if err != nil {
		t.Fatalf("CreatePlaygroup() error = %v, want nil", err)
	}
	return res
}

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError (services no longer depend on fiber, see
// internal/common/errors.go), so we can keep verifying the status code the
// client sees.
func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func TestCreatePlaygroup_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "create-playgroup@example.com")

	res, err := svc.CreatePlaygroup(
		context.Background(), owner.ID, playgroups.CreatePlaygroupRequest{Name: "Mesa de los viernes"},
	)
	if err != nil {
		t.Fatalf("CreatePlaygroup() error = %v, want nil", err)
	}
	if res.Name != "Mesa de los viernes" {
		t.Fatalf("CreatePlaygroup() Name = %q, want %q", res.Name, "Mesa de los viernes")
	}
	if len(res.Members) != 1 || res.Members[0].UserID != owner.ID {
		t.Fatalf("CreatePlaygroup() debería auto-unir al creador como miembro: %+v", res.Members)
	}
}

func TestCreatePlaygroup_EmptyName_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "create-playgroup-empty@example.com")

	_, err := svc.CreatePlaygroup(context.Background(), owner.ID, playgroups.CreatePlaygroupRequest{Name: "   "})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("CreatePlaygroup() con nombre vacío: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestGetPlaygroup_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "get-playgroup-owner@example.com")
	outsider := createTestUser(t, pool, "get-playgroup-outsider@example.com")

	created := mustCreatePlaygroup(t, svc, owner.ID, "Privado")

	// "doesn't exist" isn't distinguished from "you're not a member".
	_, err := svc.GetPlaygroup(context.Background(), outsider.ID, created.ID)
	if !errors.Is(err, playgroups.ErrPlaygroupNotFound) {
		t.Fatalf("GetPlaygroup() de un grupo ajeno: error = %v, want ErrPlaygroupNotFound", err)
	}
}

func TestGetPlaygroup_UnknownID_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "get-playgroup-unknown@example.com")

	_, err := svc.GetPlaygroup(context.Background(), owner.ID, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, playgroups.ErrPlaygroupNotFound) {
		t.Fatalf("GetPlaygroup() con id inexistente: error = %v, want ErrPlaygroupNotFound", err)
	}
}

func TestListPlaygroups_OnlyReturnsMemberships(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	userA := createTestUser(t, pool, "list-a@example.com")
	userB := createTestUser(t, pool, "list-b@example.com")

	mustCreatePlaygroup(t, svc, userA.ID, "Grupo A")
	mustCreatePlaygroup(t, svc, userB.ID, "Grupo B")

	listA, err := svc.ListPlaygroups(context.Background(), userA.ID)
	if err != nil {
		t.Fatalf("ListPlaygroups(A) error = %v", err)
	}
	if len(listA) != 1 || listA[0].Name != "Grupo A" {
		t.Fatalf("ListPlaygroups(A) = %+v, want solo el grupo de A", listA)
	}
}

// The listing needs the members populated so the web can show the member
// count without an extra GetPlaygroup per group.
func TestListPlaygroups_IncludesMembers(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "list-members-owner@example.com")
	other := createTestUser(t, pool, "list-members-other@example.com")

	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "Grupo con miembros")
	_, err := svc.AddMember(
		context.Background(), playgroup.ID, owner.ID, playgroups.AddMemberRequest{UserID: other.ID},
	)
	if err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}

	list, err := svc.ListPlaygroups(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("ListPlaygroups() error = %v", err)
	}
	if len(list) != 1 || len(list[0].Members) != 2 {
		t.Fatalf("ListPlaygroups() members = %+v, want 2", list[0].Members)
	}
}

// TestListPlaygroupsPage_OnlyReturnsMemberships mirrors
// TestListPlaygroups_OnlyReturnsMemberships: the paginated listing must keep
// the same membership scoping as the unpaginated one.
func TestListPlaygroupsPage_OnlyReturnsMemberships(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	userA := createTestUser(t, pool, "page-list-a@example.com")
	userB := createTestUser(t, pool, "page-list-b@example.com")

	mustCreatePlaygroup(t, svc, userA.ID, "Grupo A")
	mustCreatePlaygroup(t, svc, userB.ID, "Grupo B")

	page, err := svc.ListPlaygroupsPage(context.Background(), common.PageRequest{Limit: common.DefaultPageLimit}, userA.ID)
	if err != nil {
		t.Fatalf("ListPlaygroupsPage(A) error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Grupo A" {
		t.Fatalf("ListPlaygroupsPage(A) = %+v, want solo el grupo de A", page.Items)
	}
}

// TestListPlaygroupsPage_DoesNotPopulateMembers documents the one behavioral
// difference from ListPlaygroups: a paginated page is for browsing many
// groups, not showing each one's roster up front.
func TestListPlaygroupsPage_DoesNotPopulateMembers(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "page-no-members@example.com")
	mustCreatePlaygroup(t, svc, owner.ID, "Grupo sin roster")

	page, err := svc.ListPlaygroupsPage(context.Background(), common.PageRequest{Limit: common.DefaultPageLimit}, owner.ID)
	if err != nil {
		t.Fatalf("ListPlaygroupsPage() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Members != nil {
		t.Fatalf("ListPlaygroupsPage() Members = %+v, want nil (not populated on a page)", page.Items[0].Members)
	}
}

// TestListPlaygroupsPage_PaginatesWithCursor mirrors decks.TestListDecks_PaginatesWithCursor:
// the whole list is walked a few at a time, verifying keyset pagination
// doesn't miss or repeat a playgroup across pages.
func TestListPlaygroupsPage_PaginatesWithCursor(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "page-cursor@example.com")

	const total = 5
	for i := range total {
		mustCreatePlaygroup(t, svc, owner.ID, fmt.Sprintf("Grupo %d", i))
	}

	seen := collectAllPlaygroupPages(t, svc, owner.ID, 2, total)
	if len(seen) != total {
		t.Fatalf("ListPlaygroupsPage() paginado devolvió %d grupos distintos, want %d", len(seen), total)
	}
}

// collectAllPlaygroupPages walks /playgroups?cursor=... following next_cursor
// and returns the seen IDs, failing if any repeats across pages or if
// pagination never ends.
func collectAllPlaygroupPages(
	t *testing.T, svc playgroups.Service, userID string, limit, maxPages int,
) map[string]bool {
	t.Helper()

	seen := make(map[string]bool, maxPages*limit)
	//nolint:gosec // limit is a test constant, not external input
	page := common.PageRequest{Limit: int32(limit)}

	for pages := 0; pages <= maxPages; pages++ {
		res, err := svc.ListPlaygroupsPage(context.Background(), page, userID)
		if err != nil {
			t.Fatalf("ListPlaygroupsPage() página %d: error = %v", pages, err)
		}
		if len(res.Items) > limit {
			t.Fatalf("ListPlaygroupsPage() página %d devolvió %d grupos, want <= %d", pages, len(res.Items), limit)
		}
		for _, item := range res.Items {
			if seen[item.ID] {
				t.Fatalf("ListPlaygroupsPage() repitió el grupo %s entre páginas", item.ID)
			}
			seen[item.ID] = true
		}

		if res.NextCursor == nil {
			return seen
		}
		page.Cursor = *res.NextCursor
	}

	t.Fatalf("la paginación no termina: más de %d páginas", maxPages)
	return nil
}

func TestListPlaygroupsPage_InvalidCursor(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	owner := createTestUser(t, pool, "page-badcursor@example.com")
	svc := playgroups.NewService(pool)

	page := common.PageRequest{Limit: common.DefaultPageLimit, Cursor: "no-es-un-cursor"}
	_, err := svc.ListPlaygroupsPage(context.Background(), page, owner.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ListPlaygroupsPage() con cursor inválido: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestAddMember_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "add-member-owner@example.com")
	newMember := createTestUser(t, pool, "add-member-new@example.com")

	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	res, err := svc.AddMember(
		context.Background(), playgroup.ID, owner.ID, playgroups.AddMemberRequest{UserID: newMember.ID},
	)
	if err != nil {
		t.Fatalf("AddMember() error = %v, want nil", err)
	}
	if res.UserID != newMember.ID {
		t.Fatalf("AddMember() UserID = %q, want %q", res.UserID, newMember.ID)
	}

	got, err := svc.GetPlaygroup(context.Background(), newMember.ID, playgroup.ID)
	if err != nil {
		t.Fatalf("GetPlaygroup() del nuevo miembro: error = %v, want nil (ya debería poder verlo)", err)
	}
	if len(got.Members) != 2 {
		t.Fatalf("GetPlaygroup() members = %+v, want 2", got.Members)
	}
}

func TestAddMember_RequesterNotMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "add-member-req-owner@example.com")
	outsider := createTestUser(t, pool, "add-member-req-outsider@example.com")
	target := createTestUser(t, pool, "add-member-req-target@example.com")

	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	_, err := svc.AddMember(
		context.Background(), playgroup.ID, outsider.ID, playgroups.AddMemberRequest{UserID: target.ID},
	)
	if !errors.Is(err, playgroups.ErrPlaygroupNotFound) {
		t.Fatalf("AddMember() por alguien ajeno al grupo: error = %v, want ErrPlaygroupNotFound", err)
	}
}

func TestAddMember_TargetAlreadyMember_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "add-member-dup-owner@example.com")

	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	// The creator themselves is already a member (they were auto-added when creating the group).
	_, err := svc.AddMember(context.Background(), playgroup.ID, owner.ID, playgroups.AddMemberRequest{UserID: owner.ID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("AddMember() de alguien ya miembro: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestUpdatePlaygroup_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "update-playgroup-owner@example.com")
	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "Nombre original")

	updated, err := svc.UpdatePlaygroup(
		context.Background(), playgroup.ID, owner.ID, playgroups.UpdatePlaygroupRequest{Name: renamedPlaygroupName},
	)
	if err != nil {
		t.Fatalf("UpdatePlaygroup() error = %v, want nil", err)
	}
	if updated.Name != renamedPlaygroupName {
		t.Fatalf("UpdatePlaygroup() Name = %q, want %q", updated.Name, renamedPlaygroupName)
	}

	got, err := svc.GetPlaygroup(context.Background(), owner.ID, playgroup.ID)
	if err != nil {
		t.Fatalf("GetPlaygroup() error = %v", err)
	}
	if got.Name != renamedPlaygroupName {
		t.Fatalf("GetPlaygroup() tras renombrar: Name = %q, want %q", got.Name, renamedPlaygroupName)
	}
}

func TestUpdatePlaygroup_EmptyName_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "update-playgroup-empty@example.com")
	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	_, err := svc.UpdatePlaygroup(
		context.Background(), playgroup.ID, owner.ID, playgroups.UpdatePlaygroupRequest{Name: "   "},
	)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("UpdatePlaygroup() con nombre vacío: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

// Same "don't reveal" criteria as AddMember: someone outside the group can't
// edit it, and the response doesn't distinguish "doesn't exist" from "you're not a member".
func TestUpdatePlaygroup_RequesterNotMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "update-playgroup-req-owner@example.com")
	outsider := createTestUser(t, pool, "update-playgroup-req-outsider@example.com")
	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	_, err := svc.UpdatePlaygroup(
		context.Background(), playgroup.ID, outsider.ID, playgroups.UpdatePlaygroupRequest{Name: "Robado"},
	)
	if !errors.Is(err, playgroups.ErrPlaygroupNotFound) {
		t.Fatalf("UpdatePlaygroup() por alguien ajeno al grupo: error = %v, want ErrPlaygroupNotFound", err)
	}
}

func TestAddMember_UnknownTargetUser_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "add-member-unknown-owner@example.com")

	playgroup := mustCreatePlaygroup(t, svc, owner.ID, "G")

	_, err := svc.AddMember(context.Background(), playgroup.ID, owner.ID, playgroups.AddMemberRequest{
		UserID: "00000000-0000-0000-0000-000000000000",
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("AddMember() con usuario inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestCreatePlaygroup_RecordsCreator(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "created-by@example.com")

	res := mustCreatePlaygroup(t, svc, owner.ID, "Mesa")
	if res.CreatedBy == nil || *res.CreatedBy != owner.ID {
		t.Fatalf("CreatePlaygroup() CreatedBy = %v, want %q", res.CreatedBy, owner.ID)
	}
}

func TestDeletePlaygroup_Creator_DeletesGroupAndMembers(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "delete-owner@example.com")
	member := createTestUser(t, pool, "delete-member@example.com")
	pg := mustCreatePlaygroup(t, svc, owner.ID, "Creado por error")
	addReq := playgroups.AddMemberRequest{UserID: member.ID}
	if _, err := svc.AddMember(context.Background(), pg.ID, owner.ID, addReq); err != nil {
		t.Fatalf("AddMember() error = %v, want nil", err)
	}

	if err := svc.DeletePlaygroup(context.Background(), pg.ID, owner.ID); err != nil {
		t.Fatalf("DeletePlaygroup() error = %v, want nil", err)
	}

	_, err := svc.GetPlaygroup(context.Background(), owner.ID, pg.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetPlaygroup() after delete: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
	var remaining int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM playgroup_members WHERE playgroup_id = $1", pg.ID).Scan(&remaining); err != nil {
		t.Fatalf("counting members: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("playgroup_members left after delete = %d, want 0", remaining)
	}
}

func TestDeletePlaygroup_MemberNotCreator_ReturnsForbidden(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "delete-forbidden-owner@example.com")
	member := createTestUser(t, pool, "delete-forbidden-member@example.com")
	pg := mustCreatePlaygroup(t, svc, owner.ID, "Mesa")
	addReq := playgroups.AddMemberRequest{UserID: member.ID}
	if _, err := svc.AddMember(context.Background(), pg.ID, owner.ID, addReq); err != nil {
		t.Fatalf("AddMember() error = %v, want nil", err)
	}

	err := svc.DeletePlaygroup(context.Background(), pg.ID, member.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusForbidden {
		t.Fatalf("DeletePlaygroup() by a non-creator member: code = %d, want %d", fiberErr.Code, fiber.StatusForbidden)
	}
}

func TestDeletePlaygroup_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "delete-notfound-owner@example.com")
	outsider := createTestUser(t, pool, "delete-notfound-outsider@example.com")
	pg := mustCreatePlaygroup(t, svc, owner.ID, "Mesa")

	err := svc.DeletePlaygroup(context.Background(), pg.ID, outsider.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("DeletePlaygroup() by a non-member: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestDeletePlaygroup_WithGames_ReturnsConflictAndKeepsGroup(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "delete-games-owner@example.com")
	pg := mustCreatePlaygroup(t, svc, owner.ID, "Mesa con partidas")
	// A pending game is enough: any game, in any status, blocks the delete.
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO games (playgroup_id, status) VALUES ($1, 'pending')", pg.ID); err != nil {
		t.Fatalf("inserting test game: %v", err)
	}

	err := svc.DeletePlaygroup(context.Background(), pg.ID, owner.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("DeletePlaygroup() with games: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}

	got, err := svc.GetPlaygroup(context.Background(), owner.ID, pg.ID)
	if err != nil {
		t.Fatalf("GetPlaygroup() after a rejected delete: error = %v, want nil", err)
	}
	if len(got.Members) != 1 {
		t.Fatalf("members after a rejected delete = %d, want 1 (the transaction must roll back)", len(got.Members))
	}
}

func mustRotateInvite(t *testing.T, svc playgroups.Service, playgroupID, userID string) string {
	t.Helper()
	res, err := svc.RotateInvite(context.Background(), playgroupID, userID)
	if err != nil {
		t.Fatalf("RotateInvite() error = %v, want nil", err)
	}
	return res.InviteCode
}

func TestRotateInvite_ExposesCodeToMembers(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-rotate@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")
	if group.InviteCode != nil {
		t.Fatalf("new group InviteCode = %q, want nil", *group.InviteCode)
	}

	code := mustRotateInvite(t, svc, group.ID, owner.ID)
	if len(code) != 22 {
		t.Fatalf("invite code %q has length %d, want 22", code, len(code))
	}

	got, err := svc.GetPlaygroup(context.Background(), owner.ID, group.ID)
	if err != nil {
		t.Fatalf("GetPlaygroup() error = %v", err)
	}
	if got.InviteCode == nil || *got.InviteCode != code {
		t.Fatalf("GetPlaygroup() InviteCode = %v, want %q", got.InviteCode, code)
	}
}

func TestRotateInvite_NotAMember_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-owner@example.com")
	outsider := createTestUser(t, pool, "invite-outsider@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")

	_, err := svc.RotateInvite(context.Background(), group.ID, outsider.ID)
	if got := asFiberError(t, err).Code; got != fiber.StatusNotFound {
		t.Fatalf("RotateInvite() by a non-member status = %d, want 404", got)
	}
}

func TestPreviewInvite_DescribesGroupBeforeAndAfterJoining(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-preview-owner@example.com")
	guest := createTestUser(t, pool, "invite-preview-guest@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")
	code := mustRotateInvite(t, svc, group.ID, owner.ID)

	preview, err := svc.PreviewInvite(context.Background(), code, guest.ID)
	if err != nil {
		t.Fatalf("PreviewInvite() error = %v", err)
	}
	if preview.PlaygroupID != group.ID || preview.Name != "Invite group" || preview.MemberCount != 1 || preview.IsMember {
		t.Fatalf("PreviewInvite() = %+v, want the group with 1 member and is_member false", preview)
	}

	if _, err = svc.AcceptInvite(context.Background(), code, guest.ID); err != nil {
		t.Fatalf("AcceptInvite() error = %v", err)
	}

	preview, err = svc.PreviewInvite(context.Background(), code, guest.ID)
	if err != nil {
		t.Fatalf("PreviewInvite() after joining error = %v", err)
	}
	if !preview.IsMember || preview.MemberCount != 2 {
		t.Fatalf("PreviewInvite() after joining = %+v, want is_member true and 2 members", preview)
	}
}

// One link for the whole group: accepting it doesn't consume it.
func TestAcceptInvite_CodeStaysValidForEveryone(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-reuse-owner@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")
	code := mustRotateInvite(t, svc, group.ID, owner.ID)

	for _, email := range []string{"invite-reuse-a@example.com", "invite-reuse-b@example.com"} {
		guest := createTestUser(t, pool, email)
		if _, err := svc.AcceptInvite(context.Background(), code, guest.ID); err != nil {
			t.Fatalf("AcceptInvite() by %s error = %v, want nil", email, err)
		}
	}

	got, err := svc.GetPlaygroup(context.Background(), owner.ID, group.ID)
	if err != nil {
		t.Fatalf("GetPlaygroup() error = %v", err)
	}
	if len(got.Members) != 3 {
		t.Fatalf("GetPlaygroup() members = %+v, want 3", got.Members)
	}
}

func TestAcceptInvite_AlreadyMember_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-dup@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")
	code := mustRotateInvite(t, svc, group.ID, owner.ID)

	_, err := svc.AcceptInvite(context.Background(), code, owner.ID)
	if got := asFiberError(t, err).Code; got != fiber.StatusConflict {
		t.Fatalf("AcceptInvite() by a member status = %d, want 409", got)
	}
}

func TestInvite_RotatedAndRevokedCodesStopWorking(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	owner := createTestUser(t, pool, "invite-revoke-owner@example.com")
	guest := createTestUser(t, pool, "invite-revoke-guest@example.com")
	group := mustCreatePlaygroup(t, svc, owner.ID, "Invite group")

	oldCode := mustRotateInvite(t, svc, group.ID, owner.ID)
	newCode := mustRotateInvite(t, svc, group.ID, owner.ID)
	if oldCode == newCode {
		t.Fatalf("RotateInvite() returned the same code twice: %q", oldCode)
	}
	_, err := svc.AcceptInvite(context.Background(), oldCode, guest.ID)
	if got := asFiberError(t, err).Code; got != fiber.StatusNotFound {
		t.Fatalf("AcceptInvite() with a rotated-out code status = %d, want 404", got)
	}

	if revokeErr := svc.RevokeInvite(context.Background(), group.ID, owner.ID); revokeErr != nil {
		t.Fatalf("RevokeInvite() error = %v", revokeErr)
	}
	_, err = svc.PreviewInvite(context.Background(), newCode, guest.ID)
	if got := asFiberError(t, err).Code; got != fiber.StatusNotFound {
		t.Fatalf("PreviewInvite() with a revoked code status = %d, want 404", got)
	}
	// Idempotent: revoking again is fine.
	if err := svc.RevokeInvite(context.Background(), group.ID, owner.ID); err != nil {
		t.Fatalf("second RevokeInvite() error = %v, want nil", err)
	}
}

func TestPreviewInvite_MalformedCode_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncatePlaygroupsTables(t, pool)

	svc := playgroups.NewService(pool)
	user := createTestUser(t, pool, "invite-malformed@example.com")

	for _, code := range []string{"", "short", "this-is-not-base64-!!!", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		_, err := svc.PreviewInvite(context.Background(), code, user.ID)
		if got := asFiberError(t, err).Code; got != fiber.StatusNotFound {
			t.Fatalf("PreviewInvite(%q) status = %d, want 404", code, got)
		}
	}
}
