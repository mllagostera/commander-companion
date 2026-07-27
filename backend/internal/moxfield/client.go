// Package moxfield es un cliente para la API pública (no oficial) de
// Moxfield, usada para importar decks por su URL o ID público.
package moxfield

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	apiBaseURL     = "https://api2.moxfield.com/v3/decks/all"
	requestTimeout = 10 * time.Second
	// Moxfield está detrás de Cloudflare y bloquea clientes sin un
	// User-Agent que parezca un navegador real.
	userAgent = "Mozilla/5.0 (compatible; CommanderCompanion/1.0; +https://github.com/mllagostera/commander-companion)"
	// imageURLTemplate arma el art crop de la carta "principal" del deck (la misma
	// que Moxfield usa como su propio og:image), a partir del id corto de esa carta.
	imageURLTemplate = "https://assets.moxfield.net/cards/card-%s-art_crop.jpg"
)

var (
	// ErrDeckNotFound indica que Moxfield no tiene ningún deck con ese ID público.
	ErrDeckNotFound = errors.New("moxfield deck not found")
	// ErrUnexpectedStatus indica que Moxfield respondió con un status inesperado (ni 200 ni 404).
	ErrUnexpectedStatus = errors.New("moxfield returned an unexpected status")
	// ErrMissingIdentifier indica que no se pasó ninguna URL/ID de Moxfield.
	ErrMissingIdentifier = errors.New("moxfield url or id is required")
	// ErrIDNotFoundInURL indica que la URL dada no tiene la forma esperada (.../decks/{id}).
	ErrIDNotFoundInURL = errors.New("could not find a deck id in the moxfield url")
)

// Deck son los datos de un deck de Moxfield relevantes para la importación.
type Deck struct {
	PublicID  string
	Name      string
	Commander string
	// ImageURL es el art crop de la carta principal del deck (normalmente el
	// comandante), o "" si Moxfield no informó ninguna.
	ImageURL string
}

// Client es un cliente HTTP para la API pública de Moxfield.
type Client struct {
	httpClient *http.Client
}

// NewClient crea un nuevo cliente de Moxfield.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: requestTimeout}}
}

type deckResponse struct {
	Name     string     `json:"name"`
	PublicID string     `json:"publicId"`
	Boards   deckBoards `json:"boards"`
	// Main es la carta que Moxfield destaca como portada del deck (normalmente el
	// comandante); su Id es el que arma la URL del art crop mostrado como og:image.
	Main *cardInfo `json:"main"`
}

type deckBoards struct {
	Commanders board `json:"commanders"`
}

type board struct {
	Cards map[string]boardCard `json:"cards"`
}

type boardCard struct {
	Card cardInfo `json:"card"`
}

type cardInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetDeck consulta un deck público de Moxfield por su ID.
func (c *Client) GetDeck(ctx context.Context, publicID string) (*Deck, error) {
	reqURL := fmt.Sprintf("%s/%s", apiBaseURL, url.PathEscape(publicID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("building moxfield request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.moxfield.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling moxfield: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrDeckNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var parsed deckResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding moxfield response: %w", err)
	}

	return &Deck{
		PublicID:  parsed.PublicID,
		Name:      parsed.Name,
		Commander: commanderNames(parsed.Boards.Commanders.Cards),
		ImageURL:  mainImageURL(parsed.Main),
	}, nil
}

func mainImageURL(main *cardInfo) string {
	if main == nil || main.ID == "" {
		return ""
	}
	return fmt.Sprintf(imageURLTemplate, main.ID)
}

func commanderNames(cards map[string]boardCard) string {
	names := make([]string, 0, len(cards))
	for _, entry := range cards {
		if entry.Card.Name != "" {
			names = append(names, entry.Card.Name)
		}
	}
	sort.Strings(names) // orden determinístico: el iterador de un map en Go no lo es
	return strings.Join(names, " & ")
}

// ExtractPublicID obtiene el ID público de un deck a partir de una URL de
// Moxfield (https://moxfield.com/decks/{id}) o lo devuelve tal cual si ya es
// un ID (no contiene "://").
func ExtractPublicID(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrMissingIdentifier
	}
	if !strings.Contains(trimmed, "://") {
		return trimmed, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parsing moxfield url: %w", err)
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, seg := range segments {
		if seg == "decks" && i+1 < len(segments) {
			return segments[i+1], nil
		}
	}
	return "", ErrIDNotFoundInURL
}
