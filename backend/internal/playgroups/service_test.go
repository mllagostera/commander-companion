package playgroups_test

import (
	"context"
	"errors"
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
	// "playgroups" limpia playgroup_members por CASCADE; "users" limpia cualquier resto.
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

// asFiberError traduce el error de dominio que devuelve el service a su equivalente
// HTTP con common.MapError (los services ya no dependen de fiber, ver
// internal/common/errors.go), para poder seguir verificando el status code que ve
// el cliente.
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

	// No se distingue "no existe" de "no sos miembro".
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

// El listado necesita los miembros poblados para que la web muestre la cantidad
// de integrantes sin un GetPlaygroup extra por grupo.
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

	// El propio creador ya es miembro (se auto-agregó al crear el grupo).
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

// Mismo criterio de "no revelar" que AddMember: alguien ajeno al grupo no puede
// editarlo, y la respuesta no distingue "no existe" de "no sos miembro".
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
