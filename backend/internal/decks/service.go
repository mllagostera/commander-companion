package decks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
)

var (
	// ErrDeckNotFound indica que el deck no existe o no pertenece al usuario que lo pide.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrMoxfieldDeckNotFound indica que Moxfield no tiene ningún deck público con ese ID.
	ErrMoxfieldDeckNotFound = common.NotFound("moxfield deck not found")
	// ErrNoCommander indica que el deck de Moxfield no declara comandante, así que no
	// es un deck de formato Commander y no tiene sentido importarlo.
	ErrNoCommander = common.InvalidInput("moxfield deck has no commander (not a Commander-format deck?)")
)

// MoxfieldClient es lo que decks necesita de un cliente de Moxfield (permite mockearlo en tests).
type MoxfieldClient interface {
	GetDeck(ctx context.Context, publicID string) (*moxfield.Deck, error)
}

// Service define la lógica de negocio del módulo decks.
type Service interface {
	CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error)
	GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error)
	ListDecks(ctx context.Context, userID string, page common.PageRequest) (*DeckListResponse, error)
	DeleteDeck(ctx context.Context, userID, id string) error
	ImportFromMoxfield(ctx context.Context, userID string, req ImportMoxfieldRequest) (*DeckResponse, error)
	// ResyncFromMoxfield vuelve a consultar Moxfield para un deck YA importado y
	// actualiza nombre y comandante si cambiaron (ver internal/sync).
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*MoxfieldSyncState, error)
	// GetMoxfieldSyncState devuelve el estado de sincronización guardado de un deck
	// importado, sin llamar a Moxfield.
	GetMoxfieldSyncState(ctx context.Context, userID, moxfieldID string) (*MoxfieldSyncState, error)
}

type service struct {
	repo     *Queries
	moxfield MoxfieldClient
}

// NewService crea un nuevo servicio de decks.
func NewService(db *pgxpool.Pool, moxfieldClient MoxfieldClient) Service {
	return &service{repo: New(db), moxfield: moxfieldClient}
}

// CreateDeck crea un nuevo deck para el usuario indicado.
func (s *service) CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	var moxfieldID pgtype.Text
	if req.MoxfieldID != "" {
		moxfieldID = pgtype.Text{String: req.MoxfieldID, Valid: true}
	}

	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:     uid,
		Name:       req.Name,
		Commander:  req.Commander,
		MoxfieldID: moxfieldID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating deck: %w", err)
	}

	return toDeckResponse(&deck), nil
}

// GetDeck devuelve un deck por su ID, si pertenece al usuario indicado.
func (s *service) GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error) {
	deck, err := s.getOwnedDeck(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toDeckResponse(deck), nil
}

// ListDecks devuelve una página de los decks del usuario indicado, del más
// reciente al más viejo. Ver internal/common/pagination.go para el esquema de
// cursor.
func (s *service) ListDecks(
	ctx context.Context, userID string, page common.PageRequest,
) (*DeckListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// Se pide una fila de más que el límite: si vuelve, es que hay página
	// siguiente. Evita un COUNT(*) aparte solo para saber si seguir paginando.
	params := ListDecksPageParams{UserID: uid, PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, decodeErr := decodeCursor(page.Cursor)
		if decodeErr != nil {
			return nil, decodeErr
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListDecksPage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing decks: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}

	items := make([]DeckResponse, 0, len(rows))
	for i := range rows {
		items = append(items, *toDeckResponse(&rows[i]))
	}
	return &DeckListResponse{Items: items, NextCursor: nextCursor}, nil
}

// decodeCursor traduce el cursor opaco de la request a los parámetros de la query.
func decodeCursor(encoded string) (pgtype.Timestamp, pgtype.UUID, error) {
	cursor, err := common.DecodeCursor(encoded)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, err
	}
	cursorID, err := common.ParseUUID(cursor.ID)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, common.ErrInvalidCursor
	}
	return pgtype.Timestamp{Time: cursor.CreatedAt, Valid: true}, cursorID, nil
}

// DeleteDeck elimina un deck, si pertenece al usuario indicado.
func (s *service) DeleteDeck(ctx context.Context, userID, id string) error {
	if _, err := s.getOwnedDeck(ctx, userID, id); err != nil {
		return err
	}

	deckID, err := common.ParseUUID(id)
	if err != nil {
		return ErrDeckNotFound
	}
	if err := s.repo.DeleteDeck(ctx, deckID); err != nil {
		return fmt.Errorf("deleting deck: %w", err)
	}
	return nil
}

// ImportFromMoxfield importa un deck público de Moxfield como un deck nuevo del usuario.
func (s *service) ImportFromMoxfield(
	ctx context.Context, userID string, req ImportMoxfieldRequest,
) (*DeckResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	publicID, err := moxfield.ExtractPublicID(req.URL)
	if err != nil {
		return nil, common.InvalidInput(err.Error())
	}

	moxDeck, err := s.moxfield.GetDeck(ctx, publicID)
	if err != nil {
		if errors.Is(err, moxfield.ErrDeckNotFound) {
			return nil, ErrMoxfieldDeckNotFound
		}
		return nil, fmt.Errorf("fetching moxfield deck: %w", err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:     uid,
		Name:       moxDeck.Name,
		Commander:  moxDeck.Commander,
		MoxfieldID: pgtype.Text{String: moxDeck.PublicID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("saving imported deck: %w", err)
	}

	return toDeckResponse(&deck), nil
}

// ResyncFromMoxfield vuelve a consultar Moxfield para un deck ya importado por el
// usuario y actualiza nombre y comandante con lo que Moxfield devuelve hoy. A
// diferencia de ImportFromMoxfield (que crea un deck nuevo), acá el deck tiene que
// existir: si el usuario no tiene ninguno con ese moxfield_id es un 404, con el
// mismo criterio que el resto del módulo (no se distingue "no existe" de "no es tuyo").
//
// El UPDATE se ejecuta aunque no haya cambios, porque es lo que mueve updated_at:
// esa columna es el "último sync exitoso" que después reporta GetMoxfieldSyncState.
func (s *service) ResyncFromMoxfield(
	ctx context.Context, userID, moxfieldID string,
) (*MoxfieldSyncState, error) {
	deck, err := s.getDeckByMoxfieldID(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}

	moxDeck, err := s.moxfield.GetDeck(ctx, deck.MoxfieldID.String)
	if err != nil {
		if errors.Is(err, moxfield.ErrDeckNotFound) {
			return nil, ErrMoxfieldDeckNotFound
		}
		return nil, fmt.Errorf("fetching moxfield deck: %w", err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	changed := moxDeck.Name != deck.Name || moxDeck.Commander != deck.Commander

	updated, err := s.repo.UpdateDeckFromMoxfield(ctx, UpdateDeckFromMoxfieldParams{
		ID:        deck.ID,
		Name:      moxDeck.Name,
		Commander: moxDeck.Commander,
	})
	if err != nil {
		return nil, fmt.Errorf("updating deck from moxfield: %w", err)
	}

	return &MoxfieldSyncState{
		Deck:         toDeckResponse(&updated),
		LastSyncedAt: lastSyncedAt(&updated),
		Changed:      changed,
	}, nil
}

// GetMoxfieldSyncState devuelve el estado guardado de un deck importado sin llamar
// a Moxfield: solo dice cómo quedó el deck y cuándo fue su último sync.
func (s *service) GetMoxfieldSyncState(
	ctx context.Context, userID, moxfieldID string,
) (*MoxfieldSyncState, error) {
	deck, err := s.getDeckByMoxfieldID(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}
	return &MoxfieldSyncState{Deck: toDeckResponse(deck), LastSyncedAt: lastSyncedAt(deck)}, nil
}

// getDeckByMoxfieldID resuelve el deck del usuario asociado a un ID (o URL) de
// Moxfield.
func (s *service) getDeckByMoxfieldID(ctx context.Context, userID, moxfieldID string) (*Deck, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	publicID, err := moxfield.ExtractPublicID(moxfieldID)
	if err != nil {
		return nil, common.InvalidInput(err.Error())
	}

	deck, err := s.repo.GetDeckByMoxfieldID(ctx, GetDeckByMoxfieldIDParams{
		UserID:     uid,
		MoxfieldID: pgtype.Text{String: publicID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeckNotFound
		}
		return nil, fmt.Errorf("looking up deck by moxfield id: %w", err)
	}
	return &deck, nil
}

// lastSyncedAt devuelve el momento del último sync con Moxfield, o nil si el deck
// se importó y nunca se re-sincronizó (updated_at sigue NULL desde el INSERT).
func lastSyncedAt(deck *Deck) *time.Time {
	if !deck.UpdatedAt.Valid {
		return nil
	}
	t := deck.UpdatedAt.Time
	return &t
}

func (s *service) getOwnedDeck(ctx context.Context, userID, id string) (*Deck, error) {
	deckID, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrDeckNotFound
	}

	deck, err := s.repo.GetDeck(ctx, deckID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeckNotFound
		}
		return nil, fmt.Errorf("looking up deck: %w", err)
	}

	if deck.UserID.String() != userID {
		// No se distingue "no existe" de "no es tuyo": evita revelar que el deck existe.
		return nil, ErrDeckNotFound
	}

	return &deck, nil
}

func toDeckResponse(deck *Deck) *DeckResponse {
	return &DeckResponse{
		ID:         deck.ID.String(),
		UserID:     deck.UserID.String(),
		Name:       deck.Name,
		Commander:  deck.Commander,
		MoxfieldID: deck.MoxfieldID.String,
	}
}
