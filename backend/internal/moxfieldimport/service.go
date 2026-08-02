// Package moxfieldimport imports in the background all of a Moxfield user's
// public decks, given the username linked to their profile (see
// internal/users: UpdateMoxfieldUsername). It's the bulk, async counterpart of
// POST /decks/import/moxfield (a single deck, synchronous, see internal/decks).
//
// MoxfieldClient.ListDecksByUsername (internal/moxfield/client.go) is the only
// place in the codebase that calls Moxfield's undocumented, reverse-engineered
// deck-search endpoint -- the single-deck import path (internal/decks) never
// uses it, only GetDeck's documented-by-observation /v3/decks/all/{publicId}.
// It hasn't been verified against the real API from this sandbox (network
// policy blocks api2.moxfield.com); confirm end to end from an environment
// with network access. StartImport resolves the deck list synchronously
// before creating the job: if Moxfield fails, the client sees a clean error
// right at request time instead of a job that starts and fails only later
// when queried. The rest (409 for duplicate import, progress, completion) is
// real and tested with a mocked MoxfieldClient.
//
// Background mechanism: a simple goroutine launched from StartImport, not a
// real queue (broker/worker pool) — the project is a single-process monolith
// (see ADR-0010) and this is the first time something async decoupled from a
// request's lifecycle is needed. Explicit limitation, same as already
// documented by the in-memory auth rate limiter (cmd/api/main.go): this only
// works for today's single-instance deployment. A job started in one
// process is invisible to another, and a restart mid-import leaves it
// "in_progress" forever, with no retry. Accepted, not solved, in this pass.
package moxfieldimport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const (
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	statusFailed     = "failed"

	// interDeckDelay spaces out the calls to Moxfield of the bulk import: no
	// parallel fan-out, so as not to risk the Cloudflare block that
	// moxfield.Client's User-Agent already avoids (see internal/moxfield/client.go).
	interDeckDelay = 500 * time.Millisecond

	// activeJobConstraint is the partial unique index that guarantees a single
	// pending/in_progress job per user (migration 00010_moxfield_import_jobs.sql).
	activeJobConstraint = "moxfield_import_jobs_active_user_idx"
)

var (
	// ErrMoxfieldUsernameNotSet indicates that the user hasn't linked a
	// Moxfield username to their profile yet (see internal/users: UpdateMoxfieldUsername).
	ErrMoxfieldUsernameNotSet = common.InvalidInput("no moxfield username linked to profile")
	// ErrImportAlreadyInProgress indicates that the user already has a pending/in_progress import.
	ErrImportAlreadyInProgress = common.Conflict("a moxfield import is already in progress")
	// ErrJobNotFound indicates that the job doesn't exist or doesn't belong to the authenticated user.
	ErrJobNotFound = common.NotFound("import job not found")
)

// MoxfieldClient is what moxfieldimport needs from the Moxfield client.
type MoxfieldClient interface {
	ListDecksByUsername(ctx context.Context, username string) ([]string, error)
}

// DeckImporter is what moxfieldimport needs from the decks module: the
// single-deck import that's already real and tested (see internal/decks/service.go).
type DeckImporter interface {
	ImportFromMoxfield(ctx context.Context, userID string, req decks.ImportMoxfieldRequest) (*decks.DeckResponse, error)
}

// UserLookup is what moxfieldimport needs from the users module: reading the
// moxfield_username linked to the profile.
type UserLookup interface {
	GetUser(ctx context.Context, id string) (*users.UserResponse, error)
}

// Service defines the business logic of the bulk Moxfield import.
type Service interface {
	StartImport(ctx context.Context, userID string) (*JobResponse, error)
	GetJobStatus(ctx context.Context, userID, jobID string) (*JobResponse, error)
}

type service struct {
	repo     *Queries
	users    UserLookup
	decks    DeckImporter
	moxfield MoxfieldClient
}

// NewService creates a new bulk Moxfield import service.
func NewService(
	db *pgxpool.Pool, userLookup UserLookup, deckImporter DeckImporter, moxfieldClient MoxfieldClient,
) Service {
	return &service{repo: New(db), users: userLookup, decks: deckImporter, moxfield: moxfieldClient}
}

// StartImport triggers a background import of all public decks of the
// Moxfield username linked to the authenticated user's profile. Returns the
// newly created job; progress is queried with GetJobStatus.
//
// The deck list is resolved SYNCHRONOUSLY, before creating the job: if
// Moxfield can't list them, the client sees a clean error right at request
// time, instead of a job that starts and fails only later when queried.
func (s *service) StartImport(ctx context.Context, userID string) (*JobResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	user, err := s.users.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.MoxfieldUsername == nil || *user.MoxfieldUsername == "" {
		return nil, ErrMoxfieldUsernameNotSet
	}

	publicIDs, err := s.resolveDeckList(ctx, *user.MoxfieldUsername)
	if err != nil {
		return nil, err
	}

	job, err := s.createJob(ctx, uid, *user.MoxfieldUsername, len(publicIDs))
	if err != nil {
		return nil, err
	}

	//nolint:gosec // G118: intentional, not an oversight -- runImport uses
	// context.Background() on purpose because ctx (the request's) stops being
	// valid as soon as the handler returns, see runImport's doc.
	go s.runImport(job.ID, userID, publicIDs)

	return toJobResponse(&job), nil
}

// resolveDeckList translates MoxfieldClient.ListDecksByUsername's error into the
// corresponding domain error, same mapping as internal/decks uses for GetDeck.
func (s *service) resolveDeckList(ctx context.Context, moxfieldUsername string) ([]string, error) {
	publicIDs, err := s.moxfield.ListDecksByUsername(ctx, moxfieldUsername)
	if err != nil {
		if errors.Is(err, moxfield.ErrUpstreamUnavailable) {
			return nil, common.UpstreamUnavailable("moxfield no está disponible, intentalo de nuevo en unos minutos")
		}
		return nil, fmt.Errorf("listing moxfield decks: %w", err)
	}
	return publicIDs, nil
}

// createJob inserts the job (pending) and marks it in_progress with the total deck
// count already known, in the same request that creates it.
func (s *service) createJob(
	ctx context.Context, uid pgtype.UUID, moxfieldUsername string, totalDecks int,
) (MoxfieldImportJob, error) {
	job, err := s.repo.CreateImportJob(ctx, CreateImportJobParams{
		UserID:           uid,
		MoxfieldUsername: moxfieldUsername,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == activeJobConstraint {
			return MoxfieldImportJob{}, ErrImportAlreadyInProgress
		}
		return MoxfieldImportJob{}, fmt.Errorf("creating import job: %w", err)
	}

	job, err = s.repo.SetImportJobInProgress(ctx, SetImportJobInProgressParams{
		ID:         job.ID,
		TotalDecks: pgtype.Int4{Int32: deckCount(totalDecks), Valid: true},
	})
	if err != nil {
		return MoxfieldImportJob{}, fmt.Errorf("marking import job in_progress: %w", err)
	}
	return job, nil
}

// deckCount converts a len() to int32 in an explicitly safe way (clamped to
// math.MaxInt32) instead of needing a nolint:gosec — a real Moxfield user
// never gets close to that many decks.
func deckCount(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	//nolint:gosec // guarded above, gosec's G115 doesn't recognize the bounds check just above
	return int32(n)
}

// GetJobStatus returns the status of a job, restricted to it belonging to the
// authenticated user (404 if not, same "don't reveal" criteria as the rest of the project).
func (s *service) GetJobStatus(ctx context.Context, userID, jobID string) (*JobResponse, error) {
	jid, err := common.ParseUUID(jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	job, err := s.repo.GetImportJob(ctx, jid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("looking up import job: %w", err)
	}
	if job.UserID.String() != userID {
		return nil, ErrJobNotFound
	}

	return toJobResponse(&job), nil
}

// runImport runs in its own goroutine, decoupled from the request that triggered
// it: it uses context.Background() (Fiber invalidates the request's context when
// the handler finishes, it can't be reused here) and recovers any panic — unlike
// an HTTP handler, already covered by Fiber's recover.New(), nothing protects a
// loose goroutine, and an unrecovered panic brings down the whole process. The
// deck list is already resolved (see StartImport): all that's left here is importing them.
func (s *service) runImport(jobID pgtype.UUID, userID string, publicIDs []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("moxfieldimport: panic recuperado en el job %s: %v", jobID, r)
		}
	}()

	ctx := context.Background()
	var failed int
	for i, publicID := range publicIDs {
		if i > 0 {
			time.Sleep(interDeckDelay)
		}

		_, importErr := s.decks.ImportFromMoxfield(ctx, userID, decks.ImportMoxfieldRequest{URL: publicID})
		delta := RecordImportJobDeckResultParams{ID: jobID}
		if importErr != nil {
			failed++
			delta.FailedDelta = 1
		} else {
			delta.ImportedDelta = 1
		}
		if _, updateErr := s.repo.RecordImportJobDeckResult(ctx, delta); updateErr != nil {
			log.Printf("moxfieldimport: actualizando progreso del job %s: %v", jobID, updateErr)
		}
	}

	status := statusCompleted
	if len(publicIDs) > 0 && failed == len(publicIDs) {
		status = statusFailed
	}
	s.finishJob(ctx, jobID, status, "")
}

func (s *service) finishJob(ctx context.Context, jobID pgtype.UUID, status, errMsg string) {
	params := FinishImportJobParams{ID: jobID, Status: status}
	if errMsg != "" {
		params.ErrorMessage = pgtype.Text{String: errMsg, Valid: true}
	}
	if _, err := s.repo.FinishImportJob(ctx, params); err != nil {
		log.Printf("moxfieldimport: finalizando el job %s: %v", jobID, err)
	}
}

func toJobResponse(job *MoxfieldImportJob) *JobResponse {
	res := &JobResponse{
		ID:               job.ID.String(),
		MoxfieldUsername: job.MoxfieldUsername,
		Status:           job.Status,
		ImportedCount:    job.ImportedCount,
		FailedCount:      job.FailedCount,
	}
	if job.TotalDecks.Valid {
		total := job.TotalDecks.Int32
		res.TotalDecks = &total
	}
	if job.ErrorMessage.Valid {
		res.ErrorMessage = &job.ErrorMessage.String
	}
	return res
}
