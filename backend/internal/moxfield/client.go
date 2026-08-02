// Package moxfield is a client for Moxfield's public (unofficial) API: it
// imports decks by their URL or public ID (GetDeck) and lists a user's public
// decks by username (ListDecksByUsername, used only by internal/moxfieldimport's
// background bulk import -- the single-deck import path never calls it).
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
	// apiRootURL is Moxfield's public (unofficial) API host. Client.baseURL
	// defaults to it and gets overridden in tests to point at an httptest.Server.
	apiRootURL = "https://api2.moxfield.com"
	// deckPath + "/{publicId}" fetches a single public deck (GetDeck).
	deckPath = "/v3/decks/all"
	// searchPath is Moxfield's public deck-search endpoint, reused by
	// ListDecksByUsername filtered to a single author. Undocumented and
	// reverse-engineered from a third-party client (github.com/Aleqsd/moxfield-api),
	// not from Moxfield's own docs -- there are none.
	searchPath = "/v2/decks/search-sfw"
	// searchPageSize is how many decks ListDecksByUsername asks for per page;
	// Moxfield's search-sfw endpoint paginates.
	searchPageSize = 100
	requestTimeout = 10 * time.Second
	// Moxfield is behind Cloudflare and blocks clients without a
	// User-Agent that looks like a real browser.
	userAgent = "Mozilla/5.0 (compatible; CommanderCompanion/1.0; +https://github.com/mllagostera/commander-companion)"
	// imageURLTemplate builds the art crop of the deck's "main" card (the same
	// one Moxfield uses as its own og:image), from that card's short id.
	imageURLTemplate = "https://assets.moxfield.net/cards/card-%s-art_crop.jpg"
	// imageURLFaceTemplate is the equivalent for the id of an individual FACE of a
	// two-faced card (transform/MDFC). Careful: "card-{faceId}" (without "face-") also
	// returns 200 but it's a collision with some other card that has that same short id
	// — the face id namespace is distinct from the card id namespace, and
	// assets.moxfield.net serves them under separate prefixes.
	imageURLFaceTemplate = "https://assets.moxfield.net/cards/card-face-%s-art_crop.jpg"

	// maxAttempts: 1 initial attempt + 2 retries on transient errors
	// (network/timeout, 5xx, 429). A 404 is never retried, it isn't transient.
	maxAttempts       = 3
	initialRetryDelay = 200 * time.Millisecond
)

var (
	// ErrDeckNotFound indicates that Moxfield has no deck with that public ID.
	ErrDeckNotFound = errors.New("moxfield deck not found")
	// ErrUnexpectedStatus indicates that Moxfield responded with an unexpected status (neither 200 nor 404).
	ErrUnexpectedStatus = errors.New("moxfield returned an unexpected status")
	// ErrMissingIdentifier indicates that no Moxfield URL/ID was passed.
	ErrMissingIdentifier = errors.New("moxfield url or id is required")
	// ErrIDNotFoundInURL indicates that the given URL doesn't have the expected shape (.../decks/{id}).
	ErrIDNotFoundInURL = errors.New("could not find a deck id in the moxfield url")
	// ErrUpstreamUnavailable indicates that Moxfield kept failing (network/timeout, 5xx,
	// 429) after GetDeck or ListDecksByUsername exhausted their retries.
	ErrUpstreamUnavailable = errors.New("moxfield unavailable after retries")
)

// Deck holds the Moxfield deck data relevant to importing.
type Deck struct {
	PublicID  string
	Name      string
	Commander string
	// ImageURL is the art crop of the deck's main card (usually the
	// commander), or "" if Moxfield didn't report one.
	ImageURL string
}

// Client is an HTTP client for Moxfield's public API.
type Client struct {
	httpClient *http.Client
	// baseURL is apiBaseURL in production; tests point it to an
	// httptest.Server to simulate Moxfield responses without real network access.
	baseURL string
}

// NewClient creates a new Moxfield client.
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: requestTimeout}, baseURL: apiRootURL}
}

type deckResponse struct {
	Name     string     `json:"name"`
	PublicID string     `json:"publicId"`
	Boards   deckBoards `json:"boards"`
	// Main is the card Moxfield highlights as the deck's cover (usually the
	// commander); its Id is what builds the art crop URL shown as og:image.
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
	// CardFaces is present only on two-faced cards (transform/modal DFC).
	// Moxfield doesn't cache any art crop asset under the card's combined id
	// (main.id) for these: only under each individual face's id.
	CardFaces []cardInfo `json:"card_faces,omitempty"`
}

// GetDeck queries a public Moxfield deck by its ID. It retries on
// transient errors (network/timeout, 5xx, 429) with exponential backoff; a 404
// is never retried. If retries are exhausted, it returns an error that
// wraps ErrUpstreamUnavailable.
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

// getDeckOnce makes a single attempt to fetch the deck. retryAfter comes
// out nonzero only when Moxfield responds 429 with the
// Retry-After header, so that GetDeck respects it instead of the fixed backoff.
func (c *Client) getDeckOnce(ctx context.Context, publicID string) (*Deck, time.Duration, error) {
	reqURL := fmt.Sprintf("%s%s/%s", c.baseURL, deckPath, url.PathEscape(publicID))

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

// retryAfterDuration parses the Retry-After header (only the seconds form,
// the only one Moxfield uses in practice) and returns 0 if it's missing or invalid.
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
	// Two-faced card: the art crop lives under the face's id (usually the
	// front) with the "card-face-" prefix, never under the combined id that Moxfield
	// reports as main.id nor under the plain "card-" prefix (that short face id
	// collides with the full-card id namespace, serving the image of
	// some other card entirely).
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
	sort.Strings(names) // deterministic order: a map's iterator in Go isn't
	return strings.Join(names, " & ")
}

// ExtractPublicID gets a deck's public ID from a Moxfield URL
// (https://moxfield.com/decks/{id}) or returns it as-is if it's already
// an ID (doesn't contain "://").
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

// searchResponse is the paginated shape of Moxfield's public deck-search
// endpoint (searchPath). Only the fields ListDecksByUsername needs are mapped.
type searchResponse struct {
	Data       []searchDeckSummary `json:"data"`
	TotalPages int                 `json:"totalPages"`
}

type searchDeckSummary struct {
	PublicID string `json:"publicId"`
}

// ListDecksByUsername returns the public IDs of all of a Moxfield user's
// public decks, given their username (not their internal user ID), newest
// updated first. It pages through searchPath until Moxfield reports no more
// pages or a page comes back empty.
func (c *Client) ListDecksByUsername(ctx context.Context, username string) ([]string, error) {
	var publicIDs []string

	for page := 1; ; page++ {
		result, err := c.searchDecksPageWithRetry(ctx, username, page)
		if err != nil {
			return nil, err
		}
		for _, d := range result.Data {
			if d.PublicID != "" {
				publicIDs = append(publicIDs, d.PublicID)
			}
		}
		if len(result.Data) == 0 || page >= result.TotalPages {
			return publicIDs, nil
		}
	}
}

// searchDecksPageWithRetry fetches one page of searchPath, retrying on
// transient errors (network/timeout, 5xx, 429) with the same
// backoff/Retry-After handling as GetDeck.
func (c *Client) searchDecksPageWithRetry(ctx context.Context, username string, page int) (*searchResponse, error) {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, retryAfter, err := c.searchDecksPageOnce(ctx, username, page)
		if err == nil {
			return result, nil
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

// searchDecksPageOnce makes a single attempt at one page of searchPath,
// filtered to a single author, sorted newest-updated first.
func (c *Client) searchDecksPageOnce(
	ctx context.Context, username string, page int,
) (*searchResponse, time.Duration, error) {
	query := url.Values{
		"authorUserNames": {username},
		"pageNumber":      {strconv.Itoa(page)},
		"pageSize":        {strconv.Itoa(searchPageSize)},
		"sortType":        {"Updated"},
		"sortDirection":   {"Descending"},
		"includePinned":   {"true"},
		"showIllegal":     {"true"},
	}
	reqURL := c.baseURL + searchPath + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("building moxfield search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://www.moxfield.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("calling moxfield: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := retryAfterDuration(resp.Header.Get("Retry-After"))
		return nil, retryAfter, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, 0, fmt.Errorf("decoding moxfield search response: %w", err)
	}

	return &parsed, 0, nil
}
