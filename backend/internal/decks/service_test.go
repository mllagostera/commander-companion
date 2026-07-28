package decks_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
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
	usersSvc := testutil.NewUsersService(pool)
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

	listA, err := svc.ListDecks(context.Background(), userA.ID, firstPage())
	if err != nil {
		t.Fatalf("ListDecks(A) error = %v", err)
	}
	if len(listA.Items) != 1 || listA.Items[0].Name != "A1" {
		t.Fatalf("ListDecks(A) = %+v, want solo el deck de A", listA.Items)
	}
	if listA.NextCursor != nil {
		t.Fatalf("ListDecks(A) next_cursor = %q, want nil (única página)", *listA.NextCursor)
	}
}

// firstPage pide la primera página con el tamaño por defecto.
func firstPage() common.PageRequest {
	return common.PageRequest{Limit: common.DefaultPageLimit}
}

func TestListDecks_PaginatesWithCursor(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "list-paged@example.com")
	svc := newDecksSvc(pool, nil)

	const total = 5
	for i := range total {
		req := decks.CreateDeckRequest{Name: fmt.Sprintf("Deck %d", i), Commander: "C"}
		if _, err := svc.CreateDeck(context.Background(), owner.ID, req); err != nil {
			t.Fatalf("CreateDeck(%d) error = %v", i, err)
		}
	}

	// Se recorre la lista entera de a 2 y se verifica que no falte ni se repita
	// ningún deck: es lo que garantiza la paginación keyset.
	seen := collectAllPages(t, svc, owner.ID, 2, total)
	if len(seen) != total {
		t.Fatalf("ListDecks() paginado devolvió %d decks distintos, want %d", len(seen), total)
	}
}

// collectAllPages recorre /decks siguiendo next_cursor y devuelve los IDs vistos,
// fallando si alguno se repite entre páginas o si la paginación no termina.
func collectAllPages(t *testing.T, svc decks.Service, userID string, limit, maxPages int) map[string]bool {
	t.Helper()

	seen := make(map[string]bool, maxPages*limit)
	//nolint:gosec // limit es una constante del test, no entrada externa
	page := common.PageRequest{Limit: int32(limit)}

	for pages := 0; pages <= maxPages; pages++ {
		res, err := svc.ListDecks(context.Background(), userID, page)
		if err != nil {
			t.Fatalf("ListDecks() página %d: error = %v", pages, err)
		}
		if len(res.Items) > limit {
			t.Fatalf("ListDecks() página %d devolvió %d decks, want <= %d", pages, len(res.Items), limit)
		}
		markSeen(t, seen, res.Items)

		if res.NextCursor == nil {
			return seen
		}
		page.Cursor = *res.NextCursor
	}

	t.Fatalf("la paginación no termina: más de %d páginas", maxPages)
	return nil
}

func markSeen(t *testing.T, seen map[string]bool, items []decks.DeckResponse) {
	t.Helper()
	for i := range items {
		if seen[items[i].ID] {
			t.Fatalf("ListDecks() repitió el deck %s entre páginas", items[i].ID)
		}
		seen[items[i].ID] = true
	}
}

func TestListDecks_InvalidCursor(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "list-badcursor@example.com")
	svc := newDecksSvc(pool, nil)

	page := common.PageRequest{Limit: common.DefaultPageLimit, Cursor: "no-es-un-cursor"}
	_, err := svc.ListDecks(context.Background(), owner.ID, page)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("ListDecks() con cursor inválido: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
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

func TestImportFromMoxfield_MoxfieldUnavailable_ReturnsServiceUnavailable(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-unavailable@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{err: fmt.Errorf("%w: 503", moxfield.ErrUpstreamUnavailable)})

	_, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{URL: resyncPublicID})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusServiceUnavailable {
		t.Fatalf("ImportFromMoxfield() moxfield caído: code = %d, want %d", fiberErr.Code, fiber.StatusServiceUnavailable)
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

// importDeck importa un deck de Moxfield con el contenido cargado en el mock y
// devuelve el servicio (con el mismo mock) para poder re-sincronizarlo después.
func importDeck(
	t *testing.T, pool *pgxpool.Pool, userID string, mox *mockMoxfieldClient,
) (decks.Service, *decks.DeckResponse) {
	t.Helper()
	svc := newDecksSvc(pool, mox)
	req := decks.ImportMoxfieldRequest{URL: mox.deck.PublicID}
	deck, err := svc.ImportFromMoxfield(context.Background(), userID, req)
	if err != nil {
		t.Fatalf("ImportFromMoxfield() error = %v", err)
	}
	return svc, deck
}

// Constantes del escenario de resync: el mismo deck de Moxfield antes y después
// de cambiar en el origen.
const (
	resyncPublicID = "abc123"
	newName        = "Nombre nuevo"
)

func TestResyncFromMoxfield_AppliesRemoteChanges(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "resync-changed@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: resyncPublicID, Name: "Nombre viejo", Commander: "Atraxa"}}
	svc, imported := importDeck(t, pool, owner.ID, mox)

	// El deck cambió en Moxfield desde el import.
	mox.deck = &moxfield.Deck{PublicID: resyncPublicID, Name: newName, Commander: "Kenrith"}

	state, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, resyncPublicID)
	if err != nil {
		t.Fatalf("ResyncFromMoxfield() error = %v, want nil", err)
	}
	if !state.Changed {
		t.Fatal("ResyncFromMoxfield() changed = false, want true")
	}
	if state.Deck.Name != newName || state.Deck.Commander != "Kenrith" {
		t.Fatalf("ResyncFromMoxfield() deck = %+v, want el nombre/comandante nuevos", state.Deck)
	}
	if state.Deck.ID != imported.ID {
		t.Fatalf("ResyncFromMoxfield() creó un deck nuevo (%q) en vez de actualizar %q", state.Deck.ID, imported.ID)
	}
	if state.LastSyncedAt == nil {
		t.Fatal("ResyncFromMoxfield() last_synced_at = nil, want el momento del sync")
	}

	// El cambio quedó persistido, no solo en la respuesta.
	stored, err := svc.GetDeck(context.Background(), owner.ID, imported.ID)
	if err != nil {
		t.Fatalf("GetDeck() tras resync: error = %v", err)
	}
	if stored.Name != newName {
		t.Fatalf("GetDeck() tras resync: name = %q, want %q", stored.Name, newName)
	}
}

func TestResyncFromMoxfield_NoRemoteChanges(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "resync-unchanged@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: "same1", Name: "Igual", Commander: "Atraxa"}}
	svc, _ := importDeck(t, pool, owner.ID, mox)

	state, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, "same1")
	if err != nil {
		t.Fatalf("ResyncFromMoxfield() error = %v, want nil", err)
	}
	if state.Changed {
		t.Fatal("ResyncFromMoxfield() changed = true, want false (Moxfield no cambió)")
	}
	// Aunque no haya cambios, el sync queda registrado: es lo que reporta después
	// GetMoxfieldSyncState como last_synced_at.
	if state.LastSyncedAt == nil {
		t.Fatal("ResyncFromMoxfield() last_synced_at = nil, want el momento del sync")
	}
}

// Re-sincronizar un deck que el usuario no importó es un 404, con el mismo criterio
// que el resto del módulo: no se distingue "no existe" de "no es tuyo".
func TestResyncFromMoxfield_NotImported_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "resync-missing@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{
		deck: &moxfield.Deck{PublicID: "nope", Name: "N", Commander: "C"},
	})

	_, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, "nope")
	if !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("ResyncFromMoxfield() de un deck no importado: error = %v, want ErrDeckNotFound", err)
	}
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("ResyncFromMoxfield() de un deck no importado: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestResyncFromMoxfield_OtherUsersDeck_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "resync-owner@example.com")
	intruder := createTestUser(t, pool, "resync-intruder@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: "ajeno", Name: "N", Commander: "C"}}
	svc, _ := importDeck(t, pool, owner.ID, mox)

	_, err := svc.ResyncFromMoxfield(context.Background(), intruder.ID, "ajeno")
	if !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("ResyncFromMoxfield() sobre un deck ajeno: error = %v, want ErrDeckNotFound", err)
	}
}

// Un deck importado que nunca se re-sincronizó no tiene last_synced_at.
func TestGetMoxfieldSyncState_NeverResynced(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "syncstate-never@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: "fresh", Name: "Fresco", Commander: "C"}}
	svc, imported := importDeck(t, pool, owner.ID, mox)

	state, err := svc.GetMoxfieldSyncState(context.Background(), owner.ID, "fresh")
	if err != nil {
		t.Fatalf("GetMoxfieldSyncState() error = %v, want nil", err)
	}
	if state.LastSyncedAt != nil {
		t.Fatalf("GetMoxfieldSyncState() last_synced_at = %v, want nil (nunca se re-sincronizó)", state.LastSyncedAt)
	}
	if state.Deck.ID != imported.ID {
		t.Fatalf("GetMoxfieldSyncState() deck id = %q, want %q", state.Deck.ID, imported.ID)
	}
}

func TestGetMoxfieldSyncState_AfterResync(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "syncstate-after@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: "synced1", Name: "N", Commander: "C"}}
	svc, _ := importDeck(t, pool, owner.ID, mox)

	if _, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, "synced1"); err != nil {
		t.Fatalf("ResyncFromMoxfield() error = %v", err)
	}

	state, err := svc.GetMoxfieldSyncState(context.Background(), owner.ID, "synced1")
	if err != nil {
		t.Fatalf("GetMoxfieldSyncState() error = %v, want nil", err)
	}
	if state.LastSyncedAt == nil {
		t.Fatal("GetMoxfieldSyncState() last_synced_at = nil, want el momento del último sync")
	}
	if state.Changed {
		t.Fatal("GetMoxfieldSyncState() changed = true, want false (no llama a Moxfield)")
	}
}
