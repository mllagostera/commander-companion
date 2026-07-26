package decks

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
)

// ErrDeckNotFound indica que el deck no existe o no pertenece al usuario que lo pide.
var ErrDeckNotFound = errors.New("deck not found")

// MoxfieldClient es lo que decks necesita de un cliente de Moxfield (permite mockearlo en tests).
type MoxfieldClient interface {
	GetDeck(ctx context.Context, publicID string) (*moxfield.Deck, error)
}

// Service define la lógica de negocio del módulo decks.
type Service interface {
	CreateDeck(ctx context.Context, userID string, req CreateDeckRequest) (*DeckResponse, error)
	GetDeck(ctx context.Context, userID, id string) (*DeckResponse, error)
	ListDecks(ctx context.Context, userID string) ([]DeckResponse, error)
	DeleteDeck(ctx context.Context, userID, id string) error
	ImportFromMoxfield(ctx context.Context, userID string, req ImportMoxfieldRequest) (*DeckResponse, error)
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
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
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

// ListDecks devuelve los decks del usuario indicado.
func (s *service) ListDecks(ctx context.Context, userID string) ([]DeckResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
	}

	decks, err := s.repo.ListDecks(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing decks: %w", err)
	}

	result := make([]DeckResponse, 0, len(decks))
	for i := range decks {
		result = append(result, *toDeckResponse(&decks[i]))
	}
	return result, nil
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
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
	}

	publicID, err := moxfield.ExtractPublicID(req.URL)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	moxDeck, err := s.moxfield.GetDeck(ctx, publicID)
	if err != nil {
		if errors.Is(err, moxfield.ErrDeckNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "moxfield deck not found")
		}
		return nil, fmt.Errorf("fetching moxfield deck: %w", err)
	}
	if moxDeck.Commander == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "moxfield deck has no commander (not a Commander-format deck?)")
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
