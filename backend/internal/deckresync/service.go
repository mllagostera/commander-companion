// Package deckresync resincroniza en background TODOS los decks que un usuario ya
// tiene importados y tienen moxfield_id seteado. Es la contraparte "actualizar lo que
// ya tengo" del import masivo por username de internal/moxfieldimport (que trae decks
// NUEVOS) — comparten el mismo patrón de job asíncrono (goroutine propia, tabla de
// progreso, endpoints de start/status) pero son conceptos distintos, no se
// reutiliza la tabla ni el paquete.
//
// Reutiliza decks.Service.ResyncFromMoxfield (ya real y probado, un fetch + UPDATE
// por deck) en un loop espaciado por interDeckDelay, mismo motivo que
// moxfieldimport: nada de fan-out paralelo, para no arriesgar el bloqueo de
// Cloudflare que ya evita el User-Agent de moxfield.Client.
//
// Mismo mecanismo de background y misma limitación explícita que moxfieldimport: una
// goroutine simple (no una cola de verdad), válida solo para el despliegue de una
// sola instancia de hoy — un job iniciado en un proceso es invisible para otro, y un
// restart a mitad de resync lo deja "in_progress" para siempre, sin reintento.
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

	// interDeckDelay espacia las llamadas a Moxfield del resync masivo, mismo valor
	// y mismo motivo que moxfieldimport.interDeckDelay.
	interDeckDelay = 500 * time.Millisecond

	// activeJobConstraint es el índice único parcial que garantiza un solo job
	// pending/in_progress por usuario (migración 00013_deck_resync_jobs.sql).
	activeJobConstraint = "deck_resync_jobs_active_user_idx"

	// listPageSize es el tamaño de página usado para enumerar TODOS los decks del
	// usuario antes de crear el job (ver resolveDeckList) — el máximo permitido por
	// decks.ListDecks, para minimizar la cantidad de páginas.
	listPageSize = common.MaxPageLimit
)

// ErrNoDecksToResync indica que el usuario no tiene ningún deck importado de
// Moxfield (con moxfield_id seteado) para resincronizar.
var ErrNoDecksToResync = common.InvalidInput("no decks with a linked moxfield id to resync")

// ErrResyncAlreadyInProgress indica que el usuario ya tiene un resync pending/in_progress.
var ErrResyncAlreadyInProgress = common.Conflict("a deck resync is already in progress")

// ErrJobNotFound indica que el job no existe o no es del usuario autenticado.
var ErrJobNotFound = common.NotFound("resync job not found")

// DeckLister es lo que deckresync necesita del módulo decks: enumerar los decks del
// usuario (para encontrar los que tienen moxfield_id) y resincronizar uno puntual
// (misma lógica real que ya usa POST /sync/moxfield).
type DeckLister interface {
	ListDecks(ctx context.Context, userID string, page common.PageRequest) (*decks.DeckListResponse, error)
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
}

// Service define la lógica de negocio del resync masivo de decks.
type Service interface {
	StartResyncAll(ctx context.Context, userID string) (*JobResponse, error)
	GetJobStatus(ctx context.Context, userID, jobID string) (*JobResponse, error)
}

type service struct {
	repo  *Queries
	decks DeckLister
}

// NewService crea un nuevo servicio de resync masivo de decks.
func NewService(db *pgxpool.Pool, deckLister DeckLister) Service {
	return &service{repo: New(db), decks: deckLister}
}

// StartResyncAll dispara en background el resync de todos los decks del usuario
// autenticado que tengan moxfield_id. La lista de decks se resuelve de forma
// SÍNCRONA antes de crear el job (mismo criterio que moxfieldimport.StartImport):
// si no hay ninguno, el cliente ve un 400 limpio en el momento, sin crear un job vacío.
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

	//nolint:gosec // G118: intencional -- runResync usa context.Background() a
	// propósito porque ctx (el del request) deja de ser válido en cuanto el handler
	// retorna, ver el doc de runResync.
	go s.runResync(job.ID, userID, moxfieldIDs)

	return toJobResponse(&job), nil
}

// resolveDeckList pagina sobre TODOS los decks del usuario (ListDecks ya está
// acotado a userID) y devuelve el moxfield_id de los que lo tienen seteado.
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
		TotalDecks: int32(totalDecks), //nolint:gosec // acotado por MaxPageLimit*páginas, nunca se acerca a overflow
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

// GetJobStatus devuelve el estado de un job, acotado a que sea del usuario
// autenticado (404 si no, mismo criterio de "no revelar" que el resto del proyecto).
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

// runResync corre en su propia goroutine, desacoplada del request que la disparó —
// ver el doc del paquete. "Actualizado" (updated_count) cuenta cualquier deck
// procesado sin error, tenga o no cambios reales en Moxfield (mismo criterio laxo
// que moxfieldimport.imported_count = "procesado OK", no "trajo algo nuevo").
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
