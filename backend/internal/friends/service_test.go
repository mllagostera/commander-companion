package friends_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/friends"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

func truncateFriendsTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	testutil.Truncate(t, pool, "friend_requests", "users")
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

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError, same helper as internal/playgroups' tests.
func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

// requireNoErr fails the test immediately if err isn't nil. Pulling this
// check into its own function (instead of repeating "if err != nil { t.Fatalf }"
// inline) keeps golangci-lint's cyclop count on the tests themselves honest --
// most of a service test's branches are the actual assertions, not plumbing.
func requireNoErr(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s error = %v, want nil", what, err)
	}
}

func TestSendFriendRequest_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "send-a@example.com")
	b := createTestUser(t, pool, "send-b@example.com")

	res, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")
	if res.Status != "pending" {
		t.Fatalf("SendFriendRequest() Status = %q, want %q", res.Status, "pending")
	}
	if res.AddresseeID != b.ID || res.AddresseeUsername != b.Username {
		t.Fatalf("SendFriendRequest() addressee = %+v, want id=%s username=%s", res, b.ID, b.Username)
	}

	incoming, err := svc.ListIncomingRequests(context.Background(), b.ID)
	requireNoErr(t, err, "ListIncomingRequests(b)")
	if len(incoming) != 1 || incoming[0].RequesterID != a.ID {
		t.Fatalf("ListIncomingRequests(b) = %+v, want one request from %s", incoming, a.ID)
	}

	outgoing, err := svc.ListOutgoingRequests(context.Background(), a.ID)
	requireNoErr(t, err, "ListOutgoingRequests(a)")
	if len(outgoing) != 1 || outgoing[0].AddresseeID != b.ID {
		t.Fatalf("ListOutgoingRequests(a) = %+v, want one request to %s", outgoing, b.ID)
	}
}

func TestSendFriendRequest_ToSelf_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "self@example.com")

	_, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: a.ID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("SendFriendRequest(self) code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestSendFriendRequest_UnknownAddressee_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "unknown-addressee@example.com")

	_, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{
		AddresseeID: "00000000-0000-0000-0000-000000000000",
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("SendFriendRequest(unknown) code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestSendFriendRequest_DuplicatePending_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "dup-a@example.com")
	b := createTestUser(t, pool, "dup-b@example.com")

	_, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "primer SendFriendRequest()")

	_, err = svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("SendFriendRequest(duplicado) code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestSendFriendRequest_CrossedRequests_AutoAccepts(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "cross-a@example.com")
	b := createTestUser(t, pool, "cross-b@example.com")

	// A pide a B primero.
	_, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest(A->B)")

	// B pide a A antes de responder: se auto-acepta la solicitud existente en vez
	// de crear una segunda fila.
	res, err := svc.SendFriendRequest(context.Background(), b.ID, friends.SendFriendRequestRequest{AddresseeID: a.ID})
	requireNoErr(t, err, "SendFriendRequest(B->A cruzado)")
	if res.Status != "accepted" {
		t.Fatalf("SendFriendRequest(B->A cruzado) Status = %q, want %q", res.Status, "accepted")
	}

	friendsOfA, err := svc.ListFriends(context.Background(), a.ID)
	requireNoErr(t, err, "ListFriends(a)")
	if len(friendsOfA) != 1 || friendsOfA[0].ID != b.ID {
		t.Fatalf("ListFriends(a) = %+v, want one friend %s", friendsOfA, b.ID)
	}

	// Ya no debería quedar ninguna solicitud pendiente para ninguno de los dos.
	incoming, err := svc.ListIncomingRequests(context.Background(), a.ID)
	requireNoErr(t, err, "ListIncomingRequests(a)")
	if len(incoming) != 0 {
		t.Fatalf("ListIncomingRequests(a) = %+v, want empty", incoming)
	}

	outgoing, err := svc.ListOutgoingRequests(context.Background(), b.ID)
	requireNoErr(t, err, "ListOutgoingRequests(b)")
	if len(outgoing) != 0 {
		t.Fatalf("ListOutgoingRequests(b) = %+v, want empty", outgoing)
	}
}

func TestSendFriendRequest_AlreadyFriends_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "already-a@example.com")
	b := createTestUser(t, pool, "already-b@example.com")

	req, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")

	_, err = svc.AcceptFriendRequest(context.Background(), b.ID, req.ID)
	requireNoErr(t, err, "AcceptFriendRequest()")

	_, err = svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("SendFriendRequest(ya amigos) code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestAcceptFriendRequest_ByNonAddressee_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "accept-a@example.com")
	b := createTestUser(t, pool, "accept-b@example.com")
	outsider := createTestUser(t, pool, "accept-outsider@example.com")

	req, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")

	// Ni el propio requester ni un tercero pueden aceptar -- solo el addressee.
	_, err = svc.AcceptFriendRequest(context.Background(), outsider.ID, req.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("AcceptFriendRequest(tercero) code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}

	_, err = svc.AcceptFriendRequest(context.Background(), a.ID, req.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("AcceptFriendRequest(requester) code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestRejectFriendRequest_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "reject-a@example.com")
	b := createTestUser(t, pool, "reject-b@example.com")

	req, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")

	err = svc.RejectFriendRequest(context.Background(), b.ID, req.ID)
	requireNoErr(t, err, "RejectFriendRequest()")

	// Rechazada: ya no aparece como pendiente, y volver a pedir debe funcionar.
	incoming, err := svc.ListIncomingRequests(context.Background(), b.ID)
	requireNoErr(t, err, "ListIncomingRequests(b) tras rechazo")
	if len(incoming) != 0 {
		t.Fatalf("ListIncomingRequests(b) tras rechazo = %+v, want empty", incoming)
	}

	_, err = svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest() tras rechazo previo")
}

func TestCancelFriendRequest_ByNonRequester_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "cancel-a@example.com")
	b := createTestUser(t, pool, "cancel-b@example.com")

	req, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")

	// El addressee no puede cancelar -- solo rechazar. Solo el requester puede cancelar.
	err = svc.CancelFriendRequest(context.Background(), b.ID, req.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("CancelFriendRequest(addressee) code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}

	err = svc.CancelFriendRequest(context.Background(), a.ID, req.ID)
	requireNoErr(t, err, "CancelFriendRequest(requester)")
}

func TestRemoveFriend(t *testing.T) {
	pool := testutil.DB(t)
	truncateFriendsTables(t, pool)

	svc := friends.NewService(pool)
	a := createTestUser(t, pool, "remove-a@example.com")
	b := createTestUser(t, pool, "remove-b@example.com")

	req, err := svc.SendFriendRequest(context.Background(), a.ID, friends.SendFriendRequestRequest{AddresseeID: b.ID})
	requireNoErr(t, err, "SendFriendRequest()")

	_, err = svc.AcceptFriendRequest(context.Background(), b.ID, req.ID)
	requireNoErr(t, err, "AcceptFriendRequest()")

	err = svc.RemoveFriend(context.Background(), a.ID, b.ID)
	requireNoErr(t, err, "RemoveFriend()")

	list, err := svc.ListFriends(context.Background(), a.ID)
	requireNoErr(t, err, "ListFriends(a) tras unfriend")
	if len(list) != 0 {
		t.Fatalf("ListFriends(a) tras unfriend = %+v, want empty", list)
	}

	// Ya no son amigos: eliminar de nuevo es un 404.
	err = svc.RemoveFriend(context.Background(), a.ID, b.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("RemoveFriend(ya eliminados) code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}
