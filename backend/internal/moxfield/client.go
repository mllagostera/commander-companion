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
	"strconv"
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
	// imageURLFaceTemplate es el equivalente para el id de una CARA individual de una
	// carta de dos caras (transform/MDFC). Ojo: "card-{faceId}" (sin "face-") también
	// devuelve 200 pero es la colisión de otra carta cualquiera con ese mismo id corto
	// — el namespace de ids de cara es distinto del namespace de ids de carta, y
	// assets.moxfield.net los sirve bajo prefijos separados.
	imageURLFaceTemplate = "https://assets.moxfield.net/cards/card-face-%s-art_crop.jpg"

	// maxAttempts: 1 intento inicial + 2 reintentos ante errores transitorios
	// (red/timeout, 5xx, 429). Un 404 nunca se reintenta, no es transitorio.
	maxAttempts       = 3
	initialRetryDelay = 200 * time.Millisecond
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
	// ErrUpstreamUnavailable indica que Moxfield siguió fallando (red/timeout, 5xx,
	// 429) después de agotar los reintentos de GetDeck.
	ErrUpstreamUnavailable = errors.New("moxfield unavailable after retries")
	// ErrListDecksByUsernameNotImplemented indica que ListDecksByUsername es un
	// stub: no hay evidencia verificada de qué endpoint de Moxfield lista los decks
	// públicos de un usuario (este sandbox bloquea la red hacia api2.moxfield.com,
	// así que no se pudo investigar como sí se hizo para GetDeck — ver
	// docs/roadmap/TASKS.md, Stage 8). Confirmar el endpoint real (path, forma de
	// paginación, si necesita el mismo User-Agent/Referer que GetDeck) en un
	// entorno con acceso de red antes de implementarlo de verdad.
	ErrListDecksByUsernameNotImplemented = errors.New("listing a moxfield user's decks is not implemented yet")
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
	// baseURL es apiBaseURL en producción; los tests lo apuntan a un
	// httptest.Server para simular respuestas de Moxfield sin red real.
	baseURL string
}

// NewClient crea un nuevo cliente de Moxfield.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: requestTimeout}, baseURL: apiBaseURL}
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
	// CardFaces está presente solo en cartas de dos caras (transform/modal DFC).
	// Moxfield no cachea ningún asset de art crop bajo el id combinado de la
	// carta (main.id) para estas: solo bajo el id de cada cara individual.
	CardFaces []cardInfo `json:"card_faces,omitempty"`
}

// GetDeck consulta un deck público de Moxfield por su ID. Reintenta ante
// errores transitorios (red/timeout, 5xx, 429) con backoff exponencial; un 404
// nunca se reintenta. Si se agotan los reintentos, devuelve un error que
// envuelve ErrUpstreamUnavailable.
func (c *Client) GetDeck(ctx context.Context, publicID string) (*Deck, error) {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		deck, retryAfter, err := c.getDeckOnce(ctx, publicID)
		if err == nil {
			return deck, nil
		}
		if errors.Is(err, ErrDeckNotFound) {
			return nil, err
		}

		lastErr = err
		if attempt == maxAttempts {
			break
		}

		wait := delay
		if retryAfter > 0 {
			wait = retryAfter
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		delay *= 2
	}

	return nil, fmt.Errorf("%w: %w", ErrUpstreamUnavailable, lastErr)
}

// getDeckOnce hace un único intento de traer el deck. retryAfter viene
// distinto de cero solo cuando Moxfield responde 429 con el header
// Retry-After, para que GetDeck lo respete en vez del backoff fijo.
func (c *Client) getDeckOnce(ctx context.Context, publicID string) (*Deck, time.Duration, error) {
	reqURL := fmt.Sprintf("%s/%s", c.baseURL, url.PathEscape(publicID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("building moxfield request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.moxfield.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("calling moxfield: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, 0, ErrDeckNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := retryAfterDuration(resp.Header.Get("Retry-After"))
		return nil, retryAfter, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var parsed deckResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, 0, fmt.Errorf("decoding moxfield response: %w", err)
	}

	return &Deck{
		PublicID:  parsed.PublicID,
		Name:      parsed.Name,
		Commander: commanderNames(parsed.Boards.Commanders.Cards),
		ImageURL:  mainImageURL(parsed.Main),
	}, 0, nil
}

// retryAfterDuration parsea el header Retry-After (solo la forma en segundos,
// la única que Moxfield usa en la práctica) y devuelve 0 si falta o es inválido.
func retryAfterDuration(header string) time.Duration {
	if header == "" {
		return 0
	}
	seconds, err := strconv.Atoi(header)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func mainImageURL(main *cardInfo) string {
	if main == nil {
		return ""
	}
	// Carta de dos caras: el art crop vive bajo el id de la cara (normalmente el
	// front) y con el prefijo "card-face-", nunca bajo el id combinado que Moxfield
	// reporta como main.id ni bajo el prefijo "card-" plano (ese id corto de cara
	// colisiona con el namespace de ids de carta completa, sirviendo la imagen de
	// otra carta cualquiera).
	if len(main.CardFaces) > 0 && main.CardFaces[0].ID != "" {
		return fmt.Sprintf(imageURLFaceTemplate, main.CardFaces[0].ID)
	}
	if main.ID == "" {
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

// ListDecksByUsername debería devolver los IDs públicos de todos los decks
// públicos de un usuario de Moxfield (dado su username, no su ID). STUB: ver
// ErrListDecksByUsernameNotImplemented para el motivo.
func (c *Client) ListDecksByUsername(_ context.Context, _ string) ([]string, error) {
	return nil, ErrListDecksByUsernameNotImplemented
}
