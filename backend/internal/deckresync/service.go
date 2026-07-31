// Package deckresync resynchronizes in the background ALL the decks a user already
// has imported and that have moxfield_id set. It's the "update what I already have"
// counterpart to the bulk import by username in internal/moxfieldimport (which brings
// in NEW decks) — they share the same async job pattern (own goroutine, progress
// table, start/status endpoints) but are distinct concepts, the table and the
// package aren't reused.
//
// It reuses decks.Service.ResyncFromMoxfield (already real and tested, a fetch +
// UPDATE per deck) in a loop spaced out by interDeckDelay, same reason as
// moxfieldimport: no parallel fan-out, so as not to risk the Cloudflare block that
// moxfield.Client's User-Agent already avoids.
//
// Same background mechanism and same explicit limitation as moxfieldimport: a
// simple goroutine (not a real queue), valid only for today's single-instance
// deployment — a job started in one process is invisible to another, and a
// restart mid-resync leaves it "in_progress" forever, with no retry.
package deckresync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
)

const (
	statusCompleted = "completed"
	statusFailed    = "failed"

	// interDeckDelay spaces out the calls to Moxfield of the bulk resync, same value
	// and same reason as moxfieldimport.interDeckDelay.
	interDeckDelay = 500 * time.Millisecond

	// activeJobConstraint is the partial unique index that guarantees a single
	// pending/in_progress job per user (migration 00013_deck_resync_jobs.sql).
	activeJobConstraint = "deck_resync_jobs_active_user_idx"

	// listPageSize is the page size used to enumerate ALL of the user's decks
	// before creating the job (see resolveDeckList) — the maximum allowed by
	// decks.ListDecks, to minimize the number of pages.
	listPageSize = common.MaxPageLimit
)

// ErrNoDecksToResync indicates that the user has no deck imported from
// Moxfield (with moxfield_id set) to resynchronize.
var ErrNoDecksToResync = common.InvalidInput("no decks with a linked moxfield id to resync")

// ErrResyncAlreadyInProgress indicates that the user already has a pending/in_progress resync.
var ErrResyncAlreadyInProgress = common.Conflict("a deck resync is already in progress")

// ErrJobNotFound indicates that the job doesn't exist or doesn't belong to the authenticated user.
var ErrJobNotFound = common.NotFound("resync job not found")

// DeckLister is what deckresync needs from the decks module: enumerate the user's
// decks (to find the ones with moxfield_id) and resynchronize a specific one
// (same real logic already used by POST /sync/moxfield).
type DeckLister interface {
	ListDecks(ctx context.Context, userID string, page common.PageRequest) (*decks.DeckListResponse, error)
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
}

// Service defines the business logic of the bulk deck resync.
type Service interface {
	StartResyncAll(ctx context.Context, userID string) (*JobResponse, error)
	GetJobStatus(ctx context.Context, userID, jobID string) (*JobResponse, error)
}

type service struct {
	repo  *Queries
	decks DeckLister
}

// NewService creates a new bulk deck resync service.
func NewService(db *pgxpool.Pool, deckLister DeckLister) Service {
	return &service{repo: New(db), decks: deckLister}
}

// StartResyncAll triggers in the background the resync of all the authenticated
// user's decks that have moxfield_id. The deck list is resolved SYNCHRONOUSLY
// before creating the job (same criteria as moxfieldimport.StartImport):
// if there's none, the client sees a clean 400 right away, without creating an empty job.
func (s *service) StartResyncAll(ctx context.Context, userID string) (*JobResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	moxfieldIDs, err := s.resolveDeckList(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(moxfieldIDs) == 0 {
		return nil, ErrNoDecksToResync
	}

	job, err := s.createJob(ctx, uid, len(moxfieldIDs))
	if err != nil {
		return nil, err
	}

	//nolint:gosec // G118: intentional -- runResync uses context.Background() on
	// purpose because ctx (the request's) stops being valid as soon as the handler
	// returns, see runResync's doc.
	go s.runResync(job.ID, userID, moxfieldIDs)

	return toJobResponse(&job), nil
}

// resolveDeckList paginates over ALL of the user's decks (ListDecks is already
// scoped to userID) and returns the moxfield_id of the ones that have it set.
func (s *service) resolveDeckList(ctx context.Context, userID string) ([]string, error) {
	var moxfieldIDs []string
	cursor := ""
	for {
		page, err := s.decks.ListDecks(ctx, userID, common.PageRequest{Cursor: cursor, Limit: listPageSize})
		if err != nil {
			return nil, fmt.Errorf("listing decks: %w", err)
		}
		for _, d := range page.Items {
			if d.MoxfieldID != "" {
				moxfieldIDs = append(moxfieldIDs, d.MoxfieldID)
			}
		}
		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
	}
	return moxfieldIDs, nil
}

func (s *service) createJob(ctx context.Context, uid pgtype.UUID, totalDecks int) (DeckResyncJob, error) {
	job, err := s.repo.CreateResyncJob(ctx, CreateResyncJobParams{
		UserID:     uid,
		TotalDecks: int32(totalDecks), //nolint:gosec // bounded by MaxPageLimit*pages, never gets close to overflow
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == activeJobConstraint {
			return DeckResyncJob{}, ErrResyncAlreadyInProgress
		}
		return DeckResyncJob{}, fmt.Errorf("creating resync job: %w", err)
	}
	return job, nil
}

// GetJobStatus returns the status of a job, restricted to it belonging to the
// authenticated user (404 if not, same "don't reveal" criteria as the rest of the project).
func (s *service) GetJobStatus(ctx context.Context, userID, jobID string) (*JobResponse, error) {
	jid, err := common.ParseUUID(jobID)
	if err != nil {
		return nil, ErrJobNotFound
	}

	job, err := s.repo.GetResyncJob(ctx, jid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("looking up resync job: %w", err)
	}
	if job.UserID.String() != userID {
		return nil, ErrJobNotFound
	}

	return toJobResponse(&job), nil
}

// runResync runs in its own goroutine, decoupled from the request that triggered
// it — see the package doc. "Updated" (updated_count) counts any deck processed
// without error, whether or not it has actual changes in Moxfield (same lax
// criteria as moxfieldimport.imported_count = "processed OK", not "brought something new").
func (s *service) runResync(jobID pgtype.UUID, userID string, moxfieldIDs []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("deckresync: panic recuperado en el job %s: %v", jobID, r)
		}
	}()

	ctx := context.Background()
	var failed int
	for i, moxfieldID := range moxfieldIDs {
		if i > 0 {
			time.Sleep(interDeckDelay)
		}

		_, resyncErr := s.decks.ResyncFromMoxfield(ctx, userID, moxfieldID)
		delta := RecordResyncJobDeckResultParams{ID: jobID}
		if resyncErr != nil {
			failed++
			delta.FailedDelta = 1
		} else {
			delta.UpdatedDelta = 1
		}
		if _, updateErr := s.repo.RecordResyncJobDeckResult(ctx, delta); updateErr != nil {
			log.Printf("deckresync: actualizando progreso del job %s: %v", jobID, updateErr)
		}
	}

	status := statusCompleted
	if len(moxfieldIDs) > 0 && failed == len(moxfieldIDs) {
		status = statusFailed
	}
	s.finishJob(ctx, jobID, status, "")
}

func (s *service) finishJob(ctx context.Context, jobID pgtype.UUID, status, errMsg string) {
	params := FinishResyncJobParams{ID: jobID, Status: status}
	if errMsg != "" {
		params.ErrorMessage = pgtype.Text{String: errMsg, Valid: true}
	}
	if _, err := s.repo.FinishResyncJob(ctx, params); err != nil {
		log.Printf("deckresync: finalizando el job %s: %v", jobID, err)
	}
}

func toJobResponse(job *DeckResyncJob) *JobResponse {
	res := &JobResponse{
		ID:           job.ID.String(),
		Status:       job.Status,
		TotalDecks:   job.TotalDecks,
		UpdatedCount: job.UpdatedCount,
		FailedCount:  job.FailedCount,
	}
	if job.ErrorMessage.Valid {
		res.ErrorMessage = &job.ErrorMessage.String
	}
	return res
}
