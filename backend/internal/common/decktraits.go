package common

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// wubrg is the canonical order of a color identity, the same order Moxfield
// and Scryfall print it in. decks.color_identity and
// game_players.deck_color_identity are always stored in it.
const wubrg = "WUBRG"

// Color modes for DeckTraitFilter.ColorMode (the `color_mode` query param).
const (
	ColorModeExact    = "exact"
	ColorModeIncludes = "includes"
	ColorModeWithin   = "within"
)

var (
	// ErrInvalidColorIdentity indicates a color identity with something other than W, U, B, R or G.
	ErrInvalidColorIdentity = InvalidInput("color identity only takes the letters W, U, B, R and G")
	// ErrInvalidBracket indicates a bracket outside Wizards' 1-5 scale.
	ErrInvalidBracket = InvalidInput("bracket must be between 1 and 5")
	// ErrInvalidColorsFilter indicates a `colors` query param that is neither WUBRG letters nor "C".
	ErrInvalidColorsFilter = InvalidInput("colors must be WUBRG letters, or C for colorless")
	// ErrInvalidColorMode indicates a `color_mode` query param other than exact, includes or within.
	ErrInvalidColorMode = InvalidInput("color_mode must be exact, includes or within")
)

// NormalizeColorIdentity uppercases the letters, drops repeats and sorts them
// in WUBRG order. A nil input stays nil (unknown); an empty one stays empty
// (colorless).
func NormalizeColorIdentity(letters []string) ([]string, error) {
	if letters == nil {
		return nil, nil
	}
	seen := make(map[byte]bool, len(wubrg))
	for _, letter := range letters {
		upper := strings.ToUpper(strings.TrimSpace(letter))
		if len(upper) != 1 || !strings.Contains(wubrg, upper) {
			return nil, ErrInvalidColorIdentity
		}
		seen[upper[0]] = true
	}
	normalized := make([]string, 0, len(seen))
	for i := range len(wubrg) {
		if seen[wubrg[i]] {
			normalized = append(normalized, string(wubrg[i]))
		}
	}
	return normalized, nil
}

// ValidBracket reports whether bracket is on Wizards' 1-5 scale.
func ValidBracket(bracket int) bool {
	return bracket >= 1 && bracket <= 5
}

// DeckTraitFilter narrows a list of decks, or of games by the caller's own
// seat, to some brackets and/or a color identity. A nil field doesn't filter.
type DeckTraitFilter struct {
	// Brackets keeps any of these brackets.
	Brackets []int16
	// Colors is in WUBRG order; empty (not nil) means colorless.
	Colors []string
	// ColorMode is one of the ColorMode* constants; only meaningful with Colors.
	ColorMode string
}

// ParseDeckTraitFilter reads the `bracket`, `colors` and `color_mode` query
// params (see BracketFilterParam, ColorsFilterParam and ColorModeParam in
// openapi.yaml).
func ParseDeckTraitFilter(c *fiber.Ctx) (DeckTraitFilter, error) {
	brackets, err := parseBracketsFilter(strings.TrimSpace(c.Query("bracket")))
	if err != nil {
		return DeckTraitFilter{}, err
	}
	colors, err := parseColorsFilter(strings.TrimSpace(c.Query("colors")))
	if err != nil {
		return DeckTraitFilter{}, err
	}
	colorMode, err := parseColorMode(strings.TrimSpace(c.Query("color_mode")))
	if err != nil {
		return DeckTraitFilter{}, err
	}
	return DeckTraitFilter{Brackets: brackets, Colors: colors, ColorMode: colorMode}, nil
}

// parseBracketsFilter reads a comma-separated bracket list; "" doesn't filter.
func parseBracketsFilter(raw string) ([]int16, error) {
	if raw == "" {
		return nil, nil
	}
	var brackets []int16
	for part := range strings.SplitSeq(raw, ",") {
		bracket, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || !ValidBracket(bracket) {
			return nil, ErrInvalidBracket
		}
		//nolint:gosec // bounded to [1, 5] by ValidBracket right above
		brackets = append(brackets, int16(bracket))
	}
	return brackets, nil
}

// parseColorsFilter reads WUBRG letters, or "C" for colorless; "" doesn't filter.
func parseColorsFilter(raw string) ([]string, error) {
	switch {
	case raw == "":
		return nil, nil
	case strings.EqualFold(raw, "C"):
		return []string{}, nil
	}
	colors, err := NormalizeColorIdentity(strings.Split(raw, ""))
	if err != nil {
		return nil, ErrInvalidColorsFilter
	}
	return colors, nil
}

// parseColorMode defaults to exact.
func parseColorMode(raw string) (string, error) {
	switch raw {
	case "":
		return ColorModeExact, nil
	case ColorModeExact, ColorModeIncludes, ColorModeWithin:
		return raw, nil
	}
	return "", ErrInvalidColorMode
}
