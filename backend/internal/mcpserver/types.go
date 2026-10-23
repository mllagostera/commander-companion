package mcpserver

import (
	"math"
	"strconv"
	"strings"
)

// The types below are what the tools return. They mirror the API's JSON names
// so most of them decode straight from a response, but they carry only what a
// model can use: the JSON decoder drops everything else (image URLs, the
// Moxfield ID, override flags), which is most of what keeps answers small.

// UserStats is the caller's overall record.
type UserStats struct {
	GamesPlayed               int     `json:"games_played"`
	GamesWon                  int     `json:"games_won"`
	WinRatePct                float64 `json:"win_rate_pct"`
	TotalDamageDealt          int     `json:"total_damage_dealt"`
	TotalCommanderDamageDealt int     `json:"total_commander_damage_dealt"`
	TotalEliminations         int     `json:"total_eliminations"`
	TimedTurns                int     `json:"timed_turns"`
	LongestTurnMs             int64   `json:"longest_turn_ms"`
	AverageTurnMs             int64   `json:"average_turn_ms"`
}

// DeckRecord is a deck and its record.
type DeckRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Commander     string    `json:"commander"`
	Bracket       *int      `json:"bracket,omitempty" jsonschema:"Commander bracket 1-5; absent when unknown"`
	ColorIdentity *[]string `json:"color_identity,omitempty" jsonschema:"WUBRG letters; [] colorless, absent unknown"`
	GamesPlayed   int       `json:"games_played"`
	GamesWon      int       `json:"games_won"`
	WinRatePct    float64   `json:"win_rate_pct"`
}

// apiDeck is the slice of the API's Deck schema the tools use.
type apiDeck struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Commander     string    `json:"commander"`
	Bracket       *int      `json:"bracket"`
	ColorIdentity *[]string `json:"color_identity"`
}

type apiDeckStats struct {
	DeckID                    string `json:"deck_id"`
	GamesPlayed               int    `json:"games_played"`
	GamesWon                  int    `json:"games_won"`
	HighestLifeTotalAchieved  int    `json:"highest_life_total_achieved"`
	TotalCommanderDamageDealt int    `json:"total_commander_damage_dealt"`
}

// RecentGame is one finished game seen from the caller's seat.
type RecentGame struct {
	ID            string   `json:"id"`
	PlaygroupName *string  `json:"playgroup_name,omitempty"`
	FinishedAt    *string  `json:"finished_at,omitempty"`
	Won           bool     `json:"won"`
	DeckName      string   `json:"deck_name,omitempty"`
	Commander     string   `json:"commander,omitempty"`
	Opponents     []string `json:"opponents"`
}

// PlaygroupSummary is a playgroup as listed.
type PlaygroupSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Members       []string `json:"members" jsonschema:"member usernames"`
	GamesPlayed   *int     `json:"games_played,omitempty" jsonschema:"finished games of the whole group"`
	MyGamesPlayed *int     `json:"my_games_played,omitempty" jsonschema:"finished games the caller played in this group"`
}

// FinishedGame is one finished game with every seat.
type FinishedGame struct {
	ID            string         `json:"id"`
	PlaygroupName *string        `json:"playgroup_name,omitempty"`
	StartedAt     *string        `json:"started_at,omitempty"`
	FinishedAt    *string        `json:"finished_at,omitempty"`
	Won           bool           `json:"won" jsonschema:"whether the caller won this game"`
	Players       []FinishedSeat `json:"players"`
	LongestTurn   *struct {
		DurationMs int64  `json:"duration_ms"`
		Username   string `json:"username"`
	} `json:"longest_turn,omitempty"`
}

// FinishedSeat is one seat of a finished game, with the deck as it was then.
type FinishedSeat struct {
	UserID        string    `json:"user_id"`
	Username      string    `json:"username"`
	DeckID        string    `json:"deck_id"`
	DeckName      string    `json:"deck_name"`
	Commander     string    `json:"deck_commander"`
	Bracket       *int      `json:"deck_bracket,omitempty"`
	ColorIdentity *[]string `json:"deck_color_identity,omitempty"`
	Won           bool      `json:"won"`
}

// unknown is how an answer spells a value the API doesn't know.
const unknown = "unknown"

// winRate is games won over games played, as a percentage with one decimal.
func winRate(won, played int) float64 {
	if played == 0 {
		return 0
	}
	const percent, oneDecimal = 100, 10
	return math.Round(float64(won)/float64(played)*percent*oneDecimal) / oneDecimal
}

// colorLabel renders a color identity for a human: "WUB", "colorless", or
// "unknown" when the API didn't know it.
func colorLabel(colors *[]string) string {
	switch {
	case colors == nil:
		return unknown
	case len(*colors) == 0:
		return "colorless"
	default:
		return strings.Join(*colors, "")
	}
}

func bracketLabel(bracket *int) string {
	if bracket == nil {
		return unknown
	}
	return "bracket " + strconv.Itoa(*bracket)
}
