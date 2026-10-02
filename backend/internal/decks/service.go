package decks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
)

// deckMoxfieldIDUniqueConstraint is the partial unique index that guarantees a
// user never has two decks with the same moxfield_id (migration
// 00015_decks_unique_moxfield_id_per_user.sql). Found and fixed 2026-08-03:
// neither ImportFromMoxfield nor the bulk import (internal/moxfieldimport)
// checked for an existing row before inserting, so re-running either created
// duplicates of decks the user already had.
const deckMoxfieldIDUniqueConstraint = "decks_user_id_moxfield_id_unique_idx"

var (
	// ErrDeckNotFound indicates that the deck doesn't exist or doesn't belong to the user requesting it.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrMoxfieldDeckNotFound indicates that Moxfield doesn't have any public deck with that ID.
	ErrMoxfieldDeckNotFound = common.NotFound("moxfield deck not found")
	// ErrNoCommander indicates that the Moxfield deck doesn't declare a commander, so it's
	// not a Commander-format deck and there's no point importing it.
	ErrNoCommander = common.InvalidInput("moxfield deck has no commander (not a Commander-format deck?)")
	// ErrDeckAlreadyImported indicates the user already has a deck imported from
	// this same Moxfield public ID (see deckMoxfieldIDUniqueConstraint).
	ErrDeckAlreadyImported = common.Conflict("this moxfield deck is already imported")
	// ErrNameRequired indicates a manual deck was submitted with a blank name.
	ErrNameRequired = common.InvalidInput("name is required")
	// ErrCommanderRequired indicates a manual deck was submitted with a blank commander.
	// Nothing else identifies a Commander deck, and the whole app keys off it
	// (statistics group by deck, DeckArt falls back to its first letter).
	ErrCommanderRequired = common.InvalidInput("commander is required")
	// ErrResetWithoutMoxfield indicates a reset_* on a deck that wasn't imported
	// from Moxfield: there's no Moxfield value to go back to.
	ErrResetWithoutMoxfield = common.InvalidInput("only a deck imported from moxfield can be reset to its moxfield values")
	// ErrSetAndReset indicates a field both set and reset in the same request.
	ErrSetAndReset = common.InvalidInput("a field cannot be both set and reset")
)

// MoxfieldClient is what decks needs from a Moxfield client (allows mocking it in tests).
type MoxfieldClient interface {
	GetDeck(ctx context.Context, publicID string) (*moxfield.Deck, error)
}

// Service defines the business logic of the decks module.
type Service interface {
	CreateDeck(ctx context.Context, userID string, req *CreateDeckRequest) (*DeckResponse, error)
	GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error)
	ListDecks(
		ctx context.Context, userID string, page common.PageRequest, filter common.DeckTraitFilter,
	) (*DeckListResponse, error)
	// UpdateDeck sets or resets a deck's bracket and color identity (see UpdateDeckRequest).
	UpdateDeck(ctx context.Context, userID, id string, req UpdateDeckRequest) (*DeckResponse, error)
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
func (s *service) CreateDeck(ctx context.Context, userID string, req *CreateDeckRequest) (*DeckResponse, error) {
	// Trimmed and required, same as playgroups/tournaments do with their own
	// names: until the web client grew a "new deck" form this endpoint was
	// only ever called by the Moxfield import (which fills both fields from
	// the fetched deck), so blank values were unreachable in practice and
	// went unvalidated.
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	commander := strings.TrimSpace(req.Commander)
	if commander == "" {
		return nil, ErrCommanderRequired
	}

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

	if req.Bracket != nil && !common.ValidBracket(*req.Bracket) {
		return nil, common.ErrInvalidBracket
	}
	colorIdentity, err := common.NormalizeColorIdentity(req.ColorIdentity)
	if err != nil {
		return nil, err
	}

	// Whatever the user typed here is theirs: flagged as overridden so a
	// later Moxfield sync (if the deck carries a moxfield_id) doesn't undo it.
	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:                  uid,
		Name:                    name,
		Commander:               commander,
		MoxfieldID:              moxfieldID,
		ImageUrl:                imageURL,
		Bracket:                 int2FromPtr(req.Bracket),
		ColorIdentity:           colorIdentity,
		BracketOverridden:       req.Bracket != nil,
		ColorIdentityOverridden: colorIdentity != nil,
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
	ctx context.Context, userID string, page common.PageRequest, filter common.DeckTraitFilter,
) (*DeckListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// One row more than the limit is requested: if it comes back, there's a next
	// page. This avoids a separate COUNT(*) just to know whether to keep paginating.
	params := ListDecksPageParams{
		UserID:    uid,
		PageLimit: page.Limit + 1,
		Brackets:  filter.Brackets,
		Colors:    filter.Colors,
		ColorMode: filter.ColorMode,
	}
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
		return nil, mapMoxfieldGetDeckError(err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	var imageURL pgtype.Text
	if moxDeck.ImageURL != "" {
		imageURL = pgtype.Text{String: moxDeck.ImageURL, Valid: true}
	}

	deck, err := s.repo.CreateDeck(ctx, CreateDeckParams{
		UserID:        uid,
		Name:          moxDeck.Name,
		Commander:     moxDeck.Commander,
		MoxfieldID:    pgtype.Text{String: moxDeck.PublicID, Valid: true},
		ImageUrl:      imageURL,
		Bracket:       int2FromPtr(moxDeck.Bracket),
		ColorIdentity: moxDeck.ColorIdentity,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == deckMoxfieldIDUniqueConstraint {
			return nil, ErrDeckAlreadyImported
		}
		return nil, fmt.Errorf("saving imported deck: %w", err)
	}

	return toDeckResponse(&deck), nil
}

// mapMoxfieldGetDeckError translates a MoxfieldClient.GetDeck error into the
// domain error ImportFromMoxfield/ResyncFromMoxfield return, since both hit
// the exact same failure modes (deck not found, upstream unavailable, or
// something unexpected worth wrapping with context).
func mapMoxfieldGetDeckError(err error) error {
	if errors.Is(err, moxfield.ErrDeckNotFound) {
		return ErrMoxfieldDeckNotFound
	}
	if errors.Is(err, moxfield.ErrUpstreamUnavailable) {
		return common.UpstreamUnavailable("moxfield no está disponible, intentalo de nuevo en unos minutos")
	}
	return fmt.Errorf("fetching moxfield deck: %w", err)
}

// ResyncFromMoxfield queries Moxfield again for a deck already imported by the
// user and updates name, commander, art, bracket and color identity with what
// Moxfield returns today -- except a bracket/color identity set by hand. Unlike
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
		return nil, mapMoxfieldGetDeckError(err)
	}
	if moxDeck.Commander == "" {
		return nil, ErrNoCommander
	}

	var imageURL pgtype.Text
	if moxDeck.ImageURL != "" {
		imageURL = pgtype.Text{String: moxDeck.ImageURL, Valid: true}
	}

	updated, err := s.repo.UpdateDeckFromMoxfield(ctx, UpdateDeckFromMoxfieldParams{
		ID:            deck.ID,
		Name:          moxDeck.Name,
		Commander:     moxDeck.Commander,
		ImageUrl:      imageURL,
		Bracket:       int2FromPtr(moxDeck.Bracket),
		ColorIdentity: moxDeck.ColorIdentity,
	})
	if err != nil {
		return nil, fmt.Errorf("updating deck from moxfield: %w", err)
	}
	if err := s.backfillSeats(ctx, &updated); err != nil {
		return nil, err
	}

	return &MoxfieldSyncState{
		Deck:         toDeckResponse(&updated),
		LastSyncedAt: lastSyncedAt(&updated),
		Changed:      deckChanged(deck, &updated),
	}, nil
}

// deckChanged reports whether a resync changed anything the user can see.
func deckChanged(before, after *Deck) bool {
	return after.Name != before.Name || after.Commander != before.Commander ||
		after.ImageUrl != before.ImageUrl || after.Bracket != before.Bracket ||
		!sameColorIdentity(after.ColorIdentity, before.ColorIdentity)
}

// traitUpdate is an UpdateDeckRequest once parsed and validated.
type traitUpdate struct {
	setBracket, setColors     bool
	bracket                   *int
	colorIdentity             []string
	resetBracket, resetColors bool
}

func parseTraitUpdate(req *UpdateDeckRequest) (*traitUpdate, error) {
	setBracket, bracket, err := parseBracketField(req.Bracket)
	if err != nil {
		return nil, err
	}
	setColors, colorIdentity, err := parseColorIdentityField(req.ColorIdentity)
	if err != nil {
		return nil, err
	}
	if (setBracket && req.ResetBracket) || (setColors && req.ResetColorIdentity) {
		return nil, ErrSetAndReset
	}
	return &traitUpdate{
		setBracket: setBracket, bracket: bracket,
		setColors: setColors, colorIdentity: colorIdentity,
		resetBracket: req.ResetBracket, resetColors: req.ResetColorIdentity,
	}, nil
}

// UpdateDeck sets or resets a deck's bracket and color identity. A value set
// here is flagged as overridden so resyncs keep it; a reset drops the flag and
// takes Moxfield's current value, fetched before anything is written so a
// Moxfield outage leaves the deck untouched.
func (s *service) UpdateDeck(ctx context.Context, userID, id string, req UpdateDeckRequest) (*DeckResponse, error) {
	deck, err := s.getOwnedDeck(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	update, err := parseTraitUpdate(&req)
	if err != nil {
		return nil, err
	}
	params, err := s.resolveTraits(ctx, deck, update)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateDeckTraits(ctx, *params)
	if err != nil {
		return nil, fmt.Errorf("updating deck traits: %w", err)
	}
	if err := s.backfillSeats(ctx, &updated); err != nil {
		return nil, err
	}
	return toDeckResponse(&updated), nil
}

// resolveTraits works out the deck's new bracket/color identity and override
// flags: it starts from what's stored, takes Moxfield's values for the resets,
// then the values set by hand.
func (s *service) resolveTraits(ctx context.Context, deck *Deck, update *traitUpdate) (*UpdateDeckTraitsParams, error) {
	params := &UpdateDeckTraitsParams{
		ID:                      deck.ID,
		Bracket:                 deck.Bracket,
		ColorIdentity:           deck.ColorIdentity,
		BracketOverridden:       deck.BracketOverridden,
		ColorIdentityOverridden: deck.ColorIdentityOverridden,
	}

	if update.resetBracket || update.resetColors {
		if !deck.MoxfieldID.Valid {
			return nil, ErrResetWithoutMoxfield
		}
		moxDeck, err := s.moxfield.GetDeck(ctx, deck.MoxfieldID.String)
		if err != nil {
			return nil, mapMoxfieldGetDeckError(err)
		}
		if update.resetBracket {
			params.Bracket, params.BracketOverridden = int2FromPtr(moxDeck.Bracket), false
		}
		if update.resetColors {
			params.ColorIdentity, params.ColorIdentityOverridden = moxDeck.ColorIdentity, false
		}
	}
	if update.setBracket {
		params.Bracket, params.BracketOverridden = int2FromPtr(update.bracket), true
	}
	if update.setColors {
		params.ColorIdentity, params.ColorIdentityOverridden = update.colorIdentity, true
	}
	return params, nil
}

// parseBracketField reads PATCH's `bracket`: absent leaves it alone, null
// clears it, a number sets it.
func parseBracketField(raw json.RawMessage) (set bool, bracket *int, err error) {
	if raw == nil {
		return false, nil, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return true, nil, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil || !common.ValidBracket(value) {
		return false, nil, common.ErrInvalidBracket
	}
	return true, &value, nil
}

// parseColorIdentityField reads PATCH's `color_identity`, with the same
// absent/null/value rules as parseBracketField.
func parseColorIdentityField(raw json.RawMessage) (set bool, identity []string, err error) {
	if raw == nil {
		return false, nil, nil
	}
	if bytes.Equal(raw, []byte("null")) {
		return true, nil, nil
	}
	var letters []string
	if json.Unmarshal(raw, &letters) != nil {
		return false, nil, common.ErrInvalidColorIdentity
	}
	if letters == nil {
		letters = []string{}
	}
	identity, err = common.NormalizeColorIdentity(letters)
	if err != nil {
		return false, nil, err
	}
	return true, identity, nil
}

// backfillSeats fills the deck's now-known bracket/color identity into the
// past seats that recorded none (see BackfillSeatDeckTraits).
func (s *service) backfillSeats(ctx context.Context, deck *Deck) error {
	if !deck.Bracket.Valid && deck.ColorIdentity == nil {
		return nil
	}
	err := s.repo.BackfillSeatDeckTraits(ctx, BackfillSeatDeckTraitsParams{
		DeckID:        deck.ID,
		Bracket:       deck.Bracket,
		ColorIdentity: deck.ColorIdentity,
	})
	if err != nil {
		return fmt.Errorf("backfilling seat deck traits: %w", err)
	}
	return nil
}

// sameColorIdentity compares two stored identities, telling unknown (nil)
// apart from colorless (empty).
func sameColorIdentity(a, b []string) bool {
	return (a == nil) == (b == nil) && slices.Equal(a, b)
}

func int2FromPtr(value *int) pgtype.Int2 {
	if value == nil {
		return pgtype.Int2{}
	}
	//nolint:gosec // brackets are validated to [1, 5] before reaching here
	return pgtype.Int2{Int16: int16(*value), Valid: true}
}

func ptrFromInt2(value pgtype.Int2) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int16)
	return &v
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

		Bracket:                 ptrFromInt2(deck.Bracket),
		ColorIdentity:           deck.ColorIdentity,
		BracketOverridden:       deck.BracketOverridden,
		ColorIdentityOverridden: deck.ColorIdentityOverridden,
	}
}
