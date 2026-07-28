// Package moxfieldimport importa en background todos los decks públicos de un
// usuario de Moxfield, dado el username vinculado a su perfil (ver
// internal/users: UpdateMoxfieldUsername). Es la contraparte masiva y asíncrona de
// POST /decks/import/moxfield (un deck puntual, síncrono, ver internal/decks).
//
// Alcance de esta pasada: el scaffold completo (tabla de jobs, goroutine, endpoints)
// está implementado, PERO MoxfieldClient.ListDecksByUsername queda como un stub que
// siempre devuelve un error. No hay forma de verificar ese endpoint de Moxfield
// desde este entorno (la política de red del sandbox bloquea api2.moxfield.com);
// hace falta confirmarlo en uno con acceso real antes de que esta feature funcione
// de punta a punta. StartImport resuelve la lista de decks de forma síncrona antes
// de crear el job, así que hoy responde 501 en el momento — nunca llega a crear un
// job ni a lanzar la goroutine. El resto (409 de import duplicado, progreso,
// finalización) es real y está probado con un MoxfieldClient mockeado.
//
// Mecanismo de background: una goroutine simple lanzada desde StartImport, no una
// cola de verdad (broker/worker pool) — el proyecto es un monolito de un solo
// proceso (ver ADR-0010) y esta es la primera vez que hace falta algo async
// desacoplado del ciclo de vida de un request. Limitación explícita, igual que ya
// documenta el rate limiter de auth en memoria (cmd/api/main.go): esto solo
// funciona para el despliegue de una sola instancia de hoy. Un job iniciado en un
// proceso es invisible para otro, y un restart a mitad de import lo deja
// "in_progress" para siempre, sin reintento. Aceptado, no resuelto, en esta pasada.
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

	// interDeckDelay espacia las llamadas a Moxfield del import masivo: nada de
	// fan-out paralelo, para no arriesgar el bloqueo de Cloudflare que ya evita el
	// User-Agent de moxfield.Client (ver internal/moxfield/client.go).
	interDeckDelay = 500 * time.Millisecond

	// activeJobConstraint es el índice único parcial que garantiza un solo job
	// pending/in_progress por usuario (migración 00010_moxfield_import_jobs.sql).
	activeJobConstraint = "moxfield_import_jobs_active_user_idx"
)

var (
	// ErrMoxfieldUsernameNotSet indica que el usuario no vinculó un username de
	// Moxfield a su perfil todavía (ver internal/users: UpdateMoxfieldUsername).
	ErrMoxfieldUsernameNotSet = common.InvalidInput("no moxfield username linked to profile")
	// ErrImportAlreadyInProgress indica que el usuario ya tiene un import pending/in_progress.
	ErrImportAlreadyInProgress = common.Conflict("a moxfield import is already in progress")
	// ErrJobNotFound indica que el job no existe o no es del usuario autenticado.
	ErrJobNotFound = common.NotFound("import job not found")
	// ErrListDecksNotImplemented es lo que MoxfieldClient.ListDecksByUsername debe
	// devolver hasta que se verifique el endpoint real de Moxfield (ver
	// docs/roadmap/TASKS.md, Stage 8, y el doc de este paquete).
	ErrListDecksNotImplemented = common.NotImplemented("listing a moxfield user's decks is not implemented yet")
)

// MoxfieldClient es lo que moxfieldimport necesita del cliente de Moxfield.
// ListDecksByUsername es un STUB en esta pasada — ver el doc del paquete.
type MoxfieldClient interface {
	ListDecksByUsername(ctx context.Context, username string) ([]string, error)
}

// DeckImporter es lo que moxfieldimport necesita del módulo decks: el import
// puntual de un deck ya existente y probado (ver internal/decks/service.go).
type DeckImporter interface {
	ImportFromMoxfield(ctx context.Context, userID string, req decks.ImportMoxfieldRequest) (*decks.DeckResponse, error)
}

// UserLookup es lo que moxfieldimport necesita del módulo users: leer el
// moxfield_username vinculado al perfil.
type UserLookup interface {
	GetUser(ctx context.Context, id string) (*users.UserResponse, error)
}

// Service define la lógica de negocio del import masivo de Moxfield.
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

// NewService crea un nuevo servicio de import masivo de Moxfield.
func NewService(
	db *pgxpool.Pool, userLookup UserLookup, deckImporter DeckImporter, moxfieldClient MoxfieldClient,
) Service {
	return &service{repo: New(db), users: userLookup, decks: deckImporter, moxfield: moxfieldClient}
}

// StartImport dispara un import en background de todos los decks públicos del
// username de Moxfield vinculado al perfil del usuario autenticado. Devuelve el job
// recién creado; el progreso se consulta con GetJobStatus.
//
// La lista de decks se resuelve de forma SÍNCRONA, antes de crear el job: si
// Moxfield no puede listarlos (hoy, siempre — MoxfieldClient.ListDecksByUsername es
// un stub, ver el doc del paquete), el cliente ve un 501 limpio en el momento de la
// request, en vez de un job que arranca y falla recién después al consultarlo.
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

	go s.runImport(job.ID, userID, publicIDs)

	return toJobResponse(&job), nil
}

// resolveDeckList traduce el stub/error de MoxfieldClient.ListDecksByUsername al
// error de dominio correspondiente (501 mientras siga sin implementar).
func (s *service) resolveDeckList(ctx context.Context, moxfieldUsername string) ([]string, error) {
	publicIDs, err := s.moxfield.ListDecksByUsername(ctx, moxfieldUsername)
	if err != nil {
		if errors.Is(err, moxfield.ErrListDecksByUsernameNotImplemented) {
			return nil, ErrListDecksNotImplemented
		}
		return nil, fmt.Errorf("listing moxfield decks: %w", err)
	}
	return publicIDs, nil
}

// createJob inserta el job (pending) y lo marca in_progress con el total de decks ya
// conocido, en la misma request que lo crea.
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

// deckCount convierte un len() a int32 de forma explícitamente segura (clampeado a
// math.MaxInt32) en vez de necesitar un nolint:gosec — un usuario real de Moxfield
// nunca se acerca a esa cantidad de decks.
func deckCount(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	//nolint:gosec // guarded above, gosec's G115 no distingue el bounds check previo
	return int32(n)
}

// GetJobStatus devuelve el estado de un job, acotado a que sea del usuario
// autenticado (404 si no, mismo criterio de "no revelar" que el resto del proyecto).
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

// runImport corre en su propia goroutine, desacoplada del request que la disparó:
// usa context.Background() (Fiber invalida el contexto del request al terminar el
// handler, no se puede reusar acá) y recupera cualquier panic — a diferencia de un
// handler HTTP, ya cubierto por recover.New() de Fiber, nada protege a una
// goroutine suelta, y un panic sin recuperar tumba todo el proceso. La lista de
// decks ya viene resuelta (ver StartImport): acá solo queda importarlos.
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
