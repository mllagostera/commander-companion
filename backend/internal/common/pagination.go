package common

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	// DefaultPageLimit is the page size when the client doesn't ask for one.
	DefaultPageLimit = 20
	// MaxPageLimit caps what a client can request; anything above it is
	// silently clamped instead of erroring, so as not to break clients that ask for more.
	MaxPageLimit = 100

	cursorSeparator = "|"
)

// ErrInvalidCursor indicates that the received cursor wasn't issued by this API
// (malformed, truncated, or tampered with).
var ErrInvalidCursor = InvalidInput("invalid cursor")

// ErrInvalidLimit indicates that the limit parameter isn't a positive integer.
var ErrInvalidLimit = InvalidInput("limit must be a positive integer")

// Cursor is the position of the last row of a page. Pagination is keyset
// based on (created_at, id) DESC: unlike OFFSET, it doesn't skip or repeat
// rows when new records are inserted while paginating, and the cost doesn't
// grow with page depth. The id is included alongside created_at because
// created_at isn't unique (two rows created in the same microsecond
// are tie-broken by id).
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// PageRequest holds the already-validated pagination parameters of a request.
// An empty Cursor means "first page".
type PageRequest struct {
	Cursor string
	Limit  int32
}

// EncodeCursor serializes a cursor to an opaque string for the client. The
// internal format (base64url of "<created_at>|<id>") is an implementation
// detail: clients must treat it as opaque and return it as-is.
func EncodeCursor(c Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + cursorSeparator + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor reverses EncodeCursor. Returns ErrInvalidCursor for any
// input that didn't come out of this API.
func DecodeCursor(encoded string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	createdAt, id, found := strings.Cut(string(decoded), cursorSeparator)
	if !found || id == "" {
		return Cursor{}, ErrInvalidCursor
	}

	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{CreatedAt: parsed, ID: id}, nil
}

// ParsePageRequest reads the `cursor` and `limit` query params of a request.
func ParsePageRequest(c *fiber.Ctx) (PageRequest, error) {
	limit, err := parsePageLimit(c.Query("limit"))
	if err != nil {
		return PageRequest{}, err
	}
	return PageRequest{Cursor: c.Query("cursor"), Limit: limit}, nil
}

// parsePageLimit validates the `limit` query param: empty uses the default and any
// value above the maximum is clamped.
func parsePageLimit(raw string) (int32, error) {
	if raw == "" {
		return DefaultPageLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, ErrInvalidLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit, nil
	}
	//nolint:gosec // bounded to [1, MaxPageLimit] right above
	return int32(limit), nil
}
