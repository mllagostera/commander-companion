// Package sync re-sincroniza un deck YA importado con su versión actual en
// Moxfield. Es la contraparte de POST /decks/import/moxfield, que es un import
// puntual: acá el deck ya existe y solo se refresca su contenido.
//
// Decisión de alcance: la sincronización es SÍNCRONA. El stub original prometía un
// modelo de jobs en background ("queued"/"in_progress" + job_id), pero una cola de
// verdad implicaría meter una dependencia dura nueva (broker + worker + tabla de
// jobs) que el resto del proyecto no tiene, y el trabajo real es una sola llamada
// HTTP a Moxfield más un UPDATE: en el orden de cientos de milisegundos, tolerable
// dentro del request. Como consecuencia:
//
//   - POST /sync/moxfield responde 200 con el resultado ya aplicado (antes: 202 con
//     un job_id inventado).
//   - No hay job_id: GET /sync/status se identifica por moxfield_id y reporta el
//     estado guardado del deck (cuándo fue su último sync), no el avance de un job.
//
// Si en algún momento el sync pasa a ser caro (varios decks a la vez, rate limits
// de Moxfield), este es el punto donde meter la cola, sin tocar a los clientes más
// allá del status code.
package sync

import (
	"context"

	"github.com/usuario/commander-companion-backend/internal/decks"
)

const (
	// statusUpdated indica que Moxfield tenía cambios y el deck se actualizó.
	statusUpdated = "updated"
	// statusUnchanged indica que el deck ya coincidía con lo que hay en Moxfield.
	statusUnchanged = "unchanged"
	// statusSynced indica que el deck tiene al menos un sync exitoso registrado.
	statusSynced = "synced"
	// statusNeverSynced indica que el deck se importó y nunca se re-sincronizó.
	statusNeverSynced = "never_synced"
)

// DeckResyncer es lo que sync necesita del módulo decks: el estado y la escritura
// del deck viven allá (es el dueño de la tabla y del cliente de Moxfield), sync es
// solo el transporte de la operación. Como interfaz, permite mockearlo en tests
// (mismo patrón que games.StatisticsRecalculator).
type DeckResyncer interface {
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
	GetMoxfieldSyncState(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
}

// Service define la lógica de negocio del módulo sync.
type Service interface {
	TriggerSync(ctx context.Context, userID, moxfieldID string) (*Response, error)
	GetSyncStatus(ctx context.Context, userID, moxfieldID string) (*Response, error)
}

type service struct {
	decks DeckResyncer
}

// NewService crea un nuevo servicio de sync.
func NewService(deckResyncer DeckResyncer) Service {
	return &service{decks: deckResyncer}
}

// TriggerSync re-consulta Moxfield para el deck del usuario asociado a moxfieldID y
// aplica los cambios. Devuelve decks.ErrDeckNotFound (→ 404) si el usuario no tiene
// importado ningún deck con ese ID.
func (s *service) TriggerSync(ctx context.Context, userID, moxfieldID string) (*Response, error) {
	state, err := s.decks.ResyncFromMoxfield(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}

	status := statusUnchanged
	if state.Changed {
		status = statusUpdated
	}
	return toResponse(status, state), nil
}

// GetSyncStatus devuelve el estado de sincronización guardado del deck, sin llamar
// a Moxfield.
func (s *service) GetSyncStatus(ctx context.Context, userID, moxfieldID string) (*Response, error) {
	state, err := s.decks.GetMoxfieldSyncState(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}

	status := statusSynced
	if state.LastSyncedAt == nil {
		status = statusNeverSynced
	}
	return toResponse(status, state), nil
}
