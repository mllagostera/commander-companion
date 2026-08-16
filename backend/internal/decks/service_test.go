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

// Fixture strings shared across tests. Grouped rather than repeated inline:
// past two uses goconst flags the literal.
const (
	testCommander = "Korvold, Fae-Cursed King"

	deckNameFixture      = "Atraxa Superfriends"
	deckCommanderFixture = "Atraxa, Praetors' Voice"
	commanderFixture     = "Atraxa"
)

// mockMoxfieldClient allows controlling the Moxfield response without hitting the
// real API from the tests (see decks.MoxfieldClient, designed for this).
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

// asFiberError translates the domain error returned by the service to its HTTP
// equivalent with common.MapError (the services no longer depend on fiber, see
// internal/common/errors.go), so we can still verify the status code the client
// sees.
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
		Name:      deckNameFixture,
		Commander: deckCommanderFixture,
	})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v, want nil", err)
	}
	if res.UserID != owner.ID {
		t.Fatalf("CreateDeck() UserID = %q, want %q", res.UserID, owner.ID)
	}
	if res.Name != deckNameFixture || res.Commander != deckCommanderFixture {
		t.Fatalf("CreateDeck() devolvió datos inesperados: %+v", res)
	}
}

// Blank name/commander were accepted until the web client grew a "new deck"
// form: the only previous caller was the Moxfield import, which always fills
// both. Whitespace-only counts as blank, and the stored values are trimmed.
func TestCreateDeck_BlankFields(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "blank-deck@example.com")
	svc := newDecksSvc(pool, nil)

	cases := []struct {
		name    string
		req     decks.CreateDeckRequest
		wantErr error
	}{
		{"sin nombre", decks.CreateDeckRequest{Name: "", Commander: commanderFixture}, decks.ErrNameRequired},
		{"nombre en blanco", decks.CreateDeckRequest{Name: "   ", Commander: commanderFixture}, decks.ErrNameRequired},
		{"sin comandante", decks.CreateDeckRequest{Name: "Deck", Commander: ""}, decks.ErrCommanderRequired},
		{"comandante en blanco", decks.CreateDeckRequest{Name: "Deck", Commander: "\t "}, decks.ErrCommanderRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateDeck(context.Background(), owner.ID, tc.req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateDeck() error = %v, want %v", err, tc.wantErr)
			}
			if got := asFiberError(t, err).Code; got != 400 {
				t.Fatalf("CreateDeck() status = %d, want 400", got)
			}
		})
	}

	res, err := svc.CreateDeck(context.Background(), owner.ID, decks.CreateDeckRequest{
		Name:      "  Atraxa Superfriends  ",
		Commander: "  Atraxa, Praetors' Voice  ",
	})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v, want nil", err)
	}
	if res.Name != deckNameFixture || res.Commander != deckCommanderFixture {
		t.Fatalf("CreateDeck() no recortó los espacios: %+v", res)
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

	// The deck exists (created.ID is valid) but belongs to "owner", not to
	// "intruder": it must not be distinguishable from "doesn't exist" (avoids revealing the deck).
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

// firstPage requests the first page with the default size.
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

	// The whole list is walked 2 at a time, verifying that no deck is missing
	// or repeated: that's what keyset pagination guarantees.
	seen := collectAllPages(t, svc, owner.ID, 2, total)
	if len(seen) != total {
		t.Fatalf("ListDecks() paginado devolvió %d decks distintos, want %d", len(seen), total)
	}
}

// collectAllPages walks /decks following next_cursor and returns the seen IDs,
// failing if any repeats across pages or if pagination never ends.
func collectAllPages(t *testing.T, svc decks.Service, userID string, limit, maxPages int) map[string]bool {
	t.Helper()

	seen := make(map[string]bool, maxPages*limit)
	//nolint:gosec // limit is a test constant, not external input
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

	// The deck still exists for its real owner.
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
		Commander: testCommander,
	}})

	res, err := svc.ImportFromMoxfield(context.Background(), owner.ID, decks.ImportMoxfieldRequest{
		URL: "https://moxfield.com/decks/abc123",
	})
	if err != nil {
		t.Fatalf("ImportFromMoxfield() error = %v, want nil", err)
	}
	if res.Name != "Imported Deck" || res.Commander != testCommander || res.MoxfieldID != "abc123" {
		t.Fatalf("ImportFromMoxfield() devolvió datos inesperados: %+v", res)
	}
}

func TestImportFromMoxfield_AlreadyImported_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-duplicate@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{deck: &moxfield.Deck{
		PublicID:  "dup123",
		Name:      "Duplicate Deck",
		Commander: testCommander,
	}})
	req := decks.ImportMoxfieldRequest{URL: "https://moxfield.com/decks/dup123"}

	if _, err := svc.ImportFromMoxfield(context.Background(), owner.ID, req); err != nil {
		t.Fatalf("ImportFromMoxfield() primera vez: error = %v, want nil", err)
	}

	_, err := svc.ImportFromMoxfield(context.Background(), owner.ID, req)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("ImportFromMoxfield() repetido: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM decks WHERE user_id = $1 AND moxfield_id = $2", owner.ID, "dup123",
	).Scan(&count); err != nil {
		t.Fatalf("contando decks: %v", err)
	}
	if count != 1 {
		t.Fatalf("decks con moxfield_id=dup123 = %d, want 1 (no debería haber creado un duplicado)", count)
	}
}

func TestImportFromMoxfield_NoCommander_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "import-no-commander@example.com")
	svc := newDecksSvc(pool, &mockMoxfieldClient{deck: &moxfield.Deck{
		PublicID: "no-commander",
		Name:     "Deck sin comandante",
		// Empty Commander: not a Commander-format deck.
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

// importDeck imports a Moxfield deck with the content loaded in the mock and
// returns the service (with the same mock) so it can be re-synced afterward.
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

// Constants for the resync scenario: the same Moxfield deck before and after
// it changes at the source.
const (
	resyncPublicID = "abc123"
	newName        = "Nombre nuevo"
)

func TestResyncFromMoxfield_AppliesRemoteChanges(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")

	owner := createTestUser(t, pool, "resync-changed@example.com")
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{
		PublicID: resyncPublicID, Name: "Nombre viejo", Commander: commanderFixture,
	}}
	svc, imported := importDeck(t, pool, owner.ID, mox)

	// The deck changed in Moxfield since the import.
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

	// The change was persisted, not just returned in the response.
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
	mox := &mockMoxfieldClient{deck: &moxfield.Deck{PublicID: "same1", Name: "Igual", Commander: commanderFixture}}
	svc, _ := importDeck(t, pool, owner.ID, mox)

	state, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, "same1")
	if err != nil {
		t.Fatalf("ResyncFromMoxfield() error = %v, want nil", err)
	}
	if state.Changed {
		t.Fatal("ResyncFromMoxfield() changed = true, want false (Moxfield no cambió)")
	}
	// Even when there are no changes, the sync is still recorded: that's what
	// GetMoxfieldSyncState reports afterward as last_synced_at.
	if state.LastSyncedAt == nil {
		t.Fatal("ResyncFromMoxfield() last_synced_at = nil, want el momento del sync")
	}
}

// Re-syncing a deck the user didn't import is a 404, following the same criteria
// as the rest of the module: "doesn't exist" is not distinguished from "isn't yours".
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

// A deck that was imported but never re-synced has no last_synced_at.
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
