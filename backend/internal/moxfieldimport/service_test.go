package moxfieldimport_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/moxfieldimport"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

const (
	jobStatusCompleted = "completed"
	jobStatusFailed    = "failed"
)

var errSimulatedImportFailure = errors.New("simulated import failure")

// fakeMoxfieldClient controla lo que ListDecksByUsername devuelve, sin golpear la
// API real ni depender del stub de internal/moxfield.
type fakeMoxfieldClient struct {
	publicIDs []string
	err       error
}

func (f fakeMoxfieldClient) ListDecksByUsername(_ context.Context, _ string) ([]string, error) {
	return f.publicIDs, f.err
}

// fakeDeckImporter simula decks.Service.ImportFromMoxfield: falla para los
// publicID listados en failFor, y opcionalmente demora cada llamada (para el test
// de import duplicado, que necesita que el job siga in_progress un rato).
type fakeDeckImporter struct {
	failFor map[string]bool
	delay   time.Duration

	mu    sync.Mutex
	calls []string
}

func (f *fakeDeckImporter) ImportFromMoxfield(
	_ context.Context, _ string, req decks.ImportMoxfieldRequest,
) (*decks.DeckResponse, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.calls = append(f.calls, req.URL)
	f.mu.Unlock()
	if f.failFor[req.URL] {
		return nil, errSimulatedImportFailure
	}
	return &decks.DeckResponse{}, nil
}

func newTestSvc(
	pool *pgxpool.Pool, mox moxfieldimport.MoxfieldClient, imp moxfieldimport.DeckImporter,
) moxfieldimport.Service {
	return moxfieldimport.NewService(pool, users.NewService(pool), imp, mox)
}

func truncateImportTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	testutil.Truncate(t, pool, "moxfield_import_jobs", "users")
}

func registerUserWithMoxfieldUsername(
	t *testing.T, pool *pgxpool.Pool, email, moxfieldUsername string,
) *users.UserResponse {
	t.Helper()
	usersSvc := users.NewService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email, Email: email, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	updated, err := usersSvc.UpdateMoxfieldUsername(context.Background(), user.ID, moxfieldUsername)
	if err != nil {
		t.Fatalf("vinculando moxfield_username: %v", err)
	}
	return updated
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

// waitForTerminalStatus hace polling de GetJobStatus hasta que el job llegue a
// completed/failed: el import corre en su propia goroutine, no hay otra forma de
// sincronizar con su finalización desde el test.
func waitForTerminalStatus(t *testing.T, svc moxfieldimport.Service, userID, jobID string) *moxfieldimport.JobResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := svc.GetJobStatus(context.Background(), userID, jobID)
		if err != nil {
			t.Fatalf("GetJobStatus() error = %v", err)
		}
		if job.Status == jobStatusCompleted || job.Status == jobStatusFailed {
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out esperando que el job llegue a un estado terminal")
	return nil
}

func TestStartImport_ListDecksNotImplemented_ReturnsNotImplemented(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "stub-501@example.com", "someone")

	// Cliente real, sin mockear: ListDecksByUsername es un stub que siempre falla
	// (ver internal/moxfield/client.go), es justamente lo que este test verifica.
	svc := newTestSvc(pool, moxfield.NewClient(), &fakeDeckImporter{})

	_, err := svc.StartImport(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotImplemented {
		t.Fatalf("StartImport() con ListDecksByUsername sin implementar: code = %d, want %d",
			fiberErr.Code, fiber.StatusNotImplemented)
	}

	// No debería haber creado ningún job: la lista se resuelve ANTES de crear el job.
	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM moxfield_import_jobs WHERE user_id = $1", user.ID,
	).Scan(&count); err != nil {
		t.Fatalf("contando jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("StartImport() con 501 creó %d jobs, want 0", count)
	}
}

func TestStartImport_NoMoxfieldUsername_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)

	usersSvc := users.NewService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "sin-username", Email: "sin-username@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}

	svc := newTestSvc(pool, fakeMoxfieldClient{}, &fakeDeckImporter{})
	_, err = svc.StartImport(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("StartImport() sin moxfield_username: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestStartImport_Success_CompletesJob(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "success@example.com", "handle1")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-a", "deck-b"}}
	imp := &fakeDeckImporter{}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}
	if job.TotalDecks == nil || *job.TotalDecks != 2 {
		t.Fatalf("StartImport() TotalDecks = %v, want 2", job.TotalDecks)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed", final.Status)
	}
	if final.ImportedCount != 2 || final.FailedCount != 0 {
		t.Fatalf("ImportedCount/FailedCount = %d/%d, want 2/0", final.ImportedCount, final.FailedCount)
	}
}

func TestStartImport_PartialFailure_StillCompletes(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "partial@example.com", "handle2")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-ok", "deck-bad"}}
	imp := &fakeDeckImporter{failFor: map[string]bool{"deck-bad": true}}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed (falla parcial no debería marcar el job entero como failed)", final.Status)
	}
	if final.ImportedCount != 1 || final.FailedCount != 1 {
		t.Fatalf("ImportedCount/FailedCount = %d/%d, want 1/1", final.ImportedCount, final.FailedCount)
	}
}

func TestStartImport_AllDecksFail_StatusFailed(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "allfail@example.com", "handle3")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-bad-1", "deck-bad-2"}}
	imp := &fakeDeckImporter{failFor: map[string]bool{"deck-bad-1": true, "deck-bad-2": true}}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusFailed {
		t.Fatalf("job.Status = %q, want failed (todos los decks fallaron)", final.Status)
	}
	if final.FailedCount != 2 {
		t.Fatalf("FailedCount = %d, want 2", final.FailedCount)
	}
}

func TestStartImport_AlreadyInProgress_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "duplicate@example.com", "handle4")

	// delay generoso para que el primer job siga in_progress cuando llega el
	// segundo StartImport, sin depender de timing ajustado.
	mox := fakeMoxfieldClient{publicIDs: []string{"deck-a", "deck-b"}}
	imp := &fakeDeckImporter{delay: 500 * time.Millisecond}
	svc := newTestSvc(pool, mox, imp)

	first, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() primera vez: error = %v, want nil", err)
	}

	_, err = svc.StartImport(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartImport() con import ya en curso: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}

	// Limpieza: esperar a que el primer job termine para no dejar una goroutine
	// escribiendo contra una base que el próximo test va a truncar.
	waitForTerminalStatus(t, svc, user.ID, first.ID)
}

func TestGetJobStatus_UnknownJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "unknown-job@example.com", "handle5")

	svc := newTestSvc(pool, fakeMoxfieldClient{}, &fakeDeckImporter{})
	_, err := svc.GetJobStatus(context.Background(), user.ID, "00000000-0000-0000-0000-000000000000")
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestGetJobStatus_OtherUsersJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	owner := registerUserWithMoxfieldUsername(t, pool, "owner@example.com", "handle6")
	other := registerUserWithMoxfieldUsername(t, pool, "other@example.com", "handle7")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-a"}}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	job, err := svc.StartImport(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	_, err = svc.GetJobStatus(context.Background(), other.ID, job.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}

	waitForTerminalStatus(t, svc, owner.ID, job.ID)
}
