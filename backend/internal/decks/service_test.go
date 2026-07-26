package decks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

// mockMoxfieldClient permite controlar la respuesta de Moxfield sin golpear la
// API real desde los tests (ver decks.MoxfieldClient, pensada para esto).
type mockMoxfieldClient struct {
	deck *moxfield.Deck
	err  error
}

func (m *mockMoxfieldClient) GetDeck(_ context.Context, _ string) (*moxfield.Deck, error) {
	return m.deck, m.err
}

func newDecksSvc(pool *pgxpool.Pool, mox decks.MoxfieldClient) decks.Service {
	if mox == nil {
		mox = &mockMoxfieldClient{}
	}
	return decks.NewService(pool, mox)
}

func createTestUser(t *testing.T, pool *pgxpool.Pool, email string) *users.UserResponse {
	t.Helper()
	usersSvc := users.NewService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email,
		Email:    email,
		Password: "test-password-123",
	})
	if err != nil {
		t.Fatalf("creando usuario de test: %v", err)
	}
	return user
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(err, &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func TestCreateDeck_Success(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "create-deck@example.com")
	svc := newDecksSvc(pool, nil)

	res, err := svc.CreateDeck(context.Background(), owner.ID, decks.CreateDeckRequest{
		Name:      "Atraxa Superfriends",
		Commander: "Atraxa, Praetors' Voice",
	})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v, want nil", err)
	}
	if res.UserID != owner.ID {
		t.Fatalf("CreateDeck() UserID = %q, want %q", res.UserID, owner.ID)
	}
	if res.Name != "Atraxa Superfriends" || res.Commander != "Atraxa, Praetors' Voice" {
		t.Fatalf("CreateDeck() devolvió datos inesperados: %+v", res)
	}
}

func TestGetDeck_OwnedByAnotherUser_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "owner@example.com")
	intruder := createTestUser(t, pool, "intruder@example.com")
	svc := newDecksSvc(pool, nil)

	created, err := svc.CreateDeck(context.Background(), owner.ID, decks.CreateDeckRequest{
		Name:      "Deck privado",
		Commander: "Some Commander",
	})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v", err)
	}

	// El deck existe (created.ID es válido) pero pertenece a "owner", no a
	// "intruder": no debe distinguirse de "no existe" (evita revelar el deck).
	if _, err := svc.GetDeck(context.Background(), intruder.ID, created.ID); !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("GetDeck() de un deck ajeno: error = %v, want ErrDeckNotFound", err)
	}
}

func TestGetDeck_UnknownID(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "unknown-id@example.com")
	svc := newDecksSvc(pool, nil)

	_, err := svc.GetDeck(context.Background(), owner.ID, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("GetDeck() con id inexistente: error = %v, want ErrDeckNotFound", err)
	}
}

func TestGetDeck_MalformedID(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "malformed-id@example.com")
	svc := newDecksSvc(pool, nil)

	_, err := svc.GetDeck(context.Background(), owner.ID, "not-a-uuid")
	if !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("GetDeck() con id malformado: error = %v, want ErrDeckNotFound", err)
	}
}

func TestListDecks_OnlyReturnsOwnDecks(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	userA := createTestUser(t, pool, "list-a@example.com")
	userB := createTestUser(t, pool, "list-b@example.com")
	svc := newDecksSvc(pool, nil)

	deckA := decks.CreateDeckRequest{Name: "A1", Commander: "C1"}
	if _, err := svc.CreateDeck(context.Background(), userA.ID, deckA); err != nil {
		t.Fatalf("CreateDeck(A) error = %v", err)
	}
	deckB := decks.CreateDeckRequest{Name: "B1", Commander: "C2"}
	if _, err := svc.CreateDeck(context.Background(), userB.ID, deckB); err != nil {
		t.Fatalf("CreateDeck(B) error = %v", err)
	}

	listA, err := svc.ListDecks(context.Background(), userA.ID)
	if err != nil {
		t.Fatalf("ListDecks(A) error = %v", err)
	}
	if len(listA) != 1 || listA[0].Name != "A1" {
		t.Fatalf("ListDecks(A) = %+v, want solo el deck de A", listA)
	}
}

func TestDeleteDeck_OwnedByAnotherUser_DoesNotDelete(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "delete-owner@example.com")
	intruder := createTestUser(t, pool, "delete-intruder@example.com")
	svc := newDecksSvc(pool, nil)

	req := decks.CreateDeckRequest{Name: "No me borres", Commander: "C"}
	created, err := svc.CreateDeck(context.Background(), owner.ID, req)
	if err != nil {
		t.Fatalf("CreateDeck() error = %v", err)
	}

	if err := svc.DeleteDeck(context.Background(), intruder.ID, created.ID); !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("DeleteDeck() por un usuario ajeno: error = %v, want ErrDeckNotFound", err)
	}

	// El deck sigue existiendo para su dueño real.
	if _, err := svc.GetDeck(context.Background(), owner.ID, created.ID); err != nil {
		t.Fatalf("GetDeck() del dueño tras intento de borrado ajeno: error = %v, want nil", err)
	}
}

func TestDeleteDeck_Success(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "delete-success@example.com")
	svc := newDecksSvc(pool, nil)

	req := decks.CreateDeckRequest{Name: "Borrame", Commander: "C"}
	created, err := svc.CreateDeck(context.Background(), owner.ID, req)
	if err != nil {
		t.Fatalf("CreateDeck() error = %v", err)
	}

	if err := svc.DeleteDeck(context.Background(), owner.ID, created.ID); err != nil {
		t.Fatalf("DeleteDeck() error = %v, want nil", err)
	}

	if _, err := svc.GetDeck(context.Background(), owner.ID, created.ID); !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("GetDeck() tras DeleteDeck(): error = %v, want ErrDeckNotFound", err)
	}
}

func TestImportFromMoxfield_Success(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-success@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{deck: &moxfield.Deck{
		PublicID:  "abc123",
		Name:      "Imported Deck",
		Commander: "Korvold, Fae-Cursed King",
	}})

	res, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{
		URL: "https://moxfield.com/decks/abc123",
	})
	if err != nil {
		t.Fatalf("ImportFromMoxfield() error = %v, want nil", err)
	}
	if res.Name != "Imported Deck" || res.Commander != "Korvold, Fae-Cursed King" || res.MoxfieldID != "abc123" {
		t.Fatalf("ImportFromMoxfield() devolvió datos inesperados: %+v", res)
	}
}

func TestImportFromMoxfield_NoCommander_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-no-commander@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{deck: &moxfield.Deck{
		PublicID: "no-commander",
		Name:     "Deck sin comandante",
		// Commander vacío: no es un deck de formato Commander.
	}})

	_, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{URL: "no-commander"})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ImportFromMoxfield() sin comandante: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestImportFromMoxfield_DeckNotFound_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-not-found@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{err: moxfield.ErrDeckNotFound})

	_, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{URL: "does-not-exist"})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("ImportFromMoxfield() deck inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestImportFromMoxfield_MissingURL_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-missing-url@example.com")
	svc := newDecksSvc(pool, nil)

	_, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{URL: ""})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ImportFromMoxfield() sin URL: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}
