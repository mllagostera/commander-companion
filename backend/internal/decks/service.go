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
	// ErrDeckNotFound indicates that the deck doesn't exist or doesn't belong to the user requesting it.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrMoxfieldDeckNotFound indicates that Moxfield doesn't have any public deck with that ID.
	ErrMoxfieldDeckNotFound = common.NotFound("moxfield deck not found")
	// ErrNoCommander indicates that the Moxfield deck doesn't declare a commander, so it's
	// not a Commander-format deck and there's no point importing it.
	ErrNoCommander = common.InvalidInput("moxfield deck has no commander (not a Commander-format deck?)")
)

// MoxfieldClient is what decks needs from a Moxfield client (allows mocking it in tests).
type MoxfieldClient interface {
	GetDeck(ctx context.Context, publicID string) (*moxfield.Deck, error)
}

// Service defines the business logic of the decks module.
type Service interface {
	CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error)
	GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error)
	ListDecks(ctx context.Context, userID string, page common.PageRequest) (*DeckListResponse, error)
	DeleteDeck(ctx context.Context, userID, id string) error
	ImportFromMoxfield(ctx context.Context, userID string, req ImportMoxfieldRequest) (*DeckResponse, error)
	// ResyncFromMoxfield queries Moxfield again for an ALREADY imported deck and
	// updates the name and commander if they changed (see internal/sync).
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*MoxfieldSyncState, error)
	// GetMoxfieldSyncState returns the stored sync state of an imported deck,
	// without calling Moxfield.
	GetMoxfieldSyncState(ctx context.Context, userID, moxfieldID string) (*MoxfieldSyncState, error)
}

type service struct {
	repo     *Queries
	moxfield MoxfieldClient
}

// NewService creates a new decks service.
func NewService(db *pgxpool.Pool, moxfieldClient MoxfieldClient) Service {
	return &service{repo: New(db), moxfield: moxfieldClient}
}

// CreateDeck creates a new deck for the given user.
func (s *service) CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	var moxfieldID pgtype.Text
	if req.MoxfieldID != "" {
		moxfieldID = pgtype.Text{String: req.MoxfieldID, Valid: true}
	}
	var imageURL pgtype.Text
	if req.ImageURL != "" {
		imageURL = pgtype.Text{String: req.ImageURL, Valid: true}
	}

	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:     uid,
		Name:       req.Name,
		Commander:  req.Commander,
		MoxfieldID: moxfieldID,
		ImageUrl:   imageURL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating deck: %w", err)
	}

	return toDeckResponse(&deck), nil
}

// GetDeck returns a deck by its ID, if it belongs to the given user.
func (s *service) GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error) {
	deck, err := s.getOwnedDeck(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toDeckResponse(deck), nil
}

// ListDecks returns a page of the given user's decks, from the most
// recent to the oldest. See internal/common/pagination.go for the cursor
// scheme.
func (s *service) ListDecks(
	ctx context.Context, userID string, page common.PageRequest,
) (*DeckListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// One row more than the limit is requested: if it comes back, there's a next
	// page. This avoids a separate COUNT(*) just to know whether to keep paginating.
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

// decodeCursor translates the request's opaque cursor into the query parameters.
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

// DeleteDeck deletes a deck, if it belongs to the given user.
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

// ImportFromMoxfield imports a public Moxfield deck as a new deck for the user.
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
		if errors.Is(err, moxfield.ErrUpstreamUnavailable) {
			return nil, common.UpstreamUnavailable("moxfield no está disponible, intentalo de nuevo en unos minutos")
		}
		return nil, fmt.Errorf("fetching moxfield deck: %w", err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	var imageURL pgtype.Text
	if moxDeck.ImageURL != "" {
		imageURL = pgtype.Text{String: moxDeck.ImageURL, Valid: true}
	}

	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:     uid,
		Name:       moxDeck.Name,
		Commander:  moxDeck.Commander,
		MoxfieldID: pgtype.Text{String: moxDeck.PublicID, Valid: true},
		ImageUrl:   imageURL,
	})
	if err != nil {
		return nil, fmt.Errorf("saving imported deck: %w", err)
	}

	return toDeckResponse(&deck), nil
}

// ResyncFromMoxfield queries Moxfield again for a deck already imported by the
// user and updates the name and commander with what Moxfield returns today. Unlike
// ImportFromMoxfield (which creates a new deck), here the deck must already
// exist: if the user has none with that moxfield_id it's a 404, following the
// same criteria as the rest of the module ("doesn't exist" is not distinguished from "isn't yours").
//
// The UPDATE runs even when there are no changes, because that's what moves updated_at:
// that column is the "last successful sync" that GetMoxfieldSyncState reports afterward.
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
		if errors.Is(err, moxfield.ErrUpstreamUnavailable) {
			return nil, common.UpstreamUnavailable("moxfield no está disponible, intentalo de nuevo en unos minutos")
		}
		return nil, fmt.Errorf("fetching moxfield deck: %w", err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	changed := moxDeck.Name != deck.Name || moxDeck.Commander != deck.Commander || moxDeck.ImageURL != deck.ImageUrl.String

	var imageURL pgtype.Text
	if moxDeck.ImageURL != "" {
		imageURL = pgtype.Text{String: moxDeck.ImageURL, Valid: true}
	}

	updated, err := s.repo.UpdateDeckFromMoxfield(ctx, UpdateDeckFromMoxfieldParams{
		ID:        deck.ID,
		Name:      moxDeck.Name,
		Commander: moxDeck.Commander,
		ImageUrl:  imageURL,
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

// GetMoxfieldSyncState returns the stored state of an imported deck without calling
// Moxfield: it only reports how the deck ended up and when its last sync happened.
func (s *service) GetMoxfieldSyncState(
	ctx context.Context, userID, moxfieldID string,
) (*MoxfieldSyncState, error) {
	deck, err := s.getDeckByMoxfieldID(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}
	return &MoxfieldSyncState{Deck: toDeckResponse(deck), LastSyncedAt: lastSyncedAt(deck)}, nil
}

// getDeckByMoxfieldID resolves the user's deck associated with a Moxfield ID (or
// URL).
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

// lastSyncedAt returns the moment of the last sync with Moxfield, or nil if the deck
// was imported and never re-synced (updated_at is still NULL from the INSERT).
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
		// "doesn't exist" is not distinguished from "isn't yours": avoids revealing that the deck exists.
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
		ImageURL:   deck.ImageUrl.String,
	}
}
