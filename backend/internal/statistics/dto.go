package statistics

// UserStatsResponse is the DTO for a user's global statistics.
type UserStatsResponse struct {
	UserID               string `json:"user_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	TotalDamageDealt     int32  `json:"total_damage_dealt"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
	TotalEliminations    int32  `json:"total_eliminations"`
}

// DeckStatsResponse is the DTO for a deck's statistics.
type DeckStatsResponse struct {
	DeckID               string `json:"deck_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	HighestLifeTotal     int32  `json:"highest_life_total_achieved"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
}

// PlaygroupStatsResponse is the DTO for a playgroup's aggregated statistics,
// calculated live (there's no precalculated per-playgroup summary table).
type PlaygroupStatsResponse struct {
	PlaygroupID string                 `json:"playgroup_id"`
	GamesPlayed int32                  `json:"games_played"`
	Members     []PlaygroupMemberStats `json:"members"`
}

// PlaygroupMemberStats summarizes the games played/won by a member within the group.
type PlaygroupMemberStats struct {
	UserID      string `json:"user_id"`
	GamesPlayed int32  `json:"games_played"`
	GamesWon    int32  `json:"games_won"`
}

// OpponentStatsResponse is the DTO for the head-to-head record against one
// other user, across every finished game they've shared a seat in.
type OpponentStatsResponse struct {
	UserID                    string `json:"user_id"`
	Username                  string `json:"username"`
	GamesTogether             int32  `json:"games_together"`
	TimesYouEliminatedThem    int32  `json:"times_you_eliminated_them"`
	TimesEliminatedByOpponent int32  `json:"times_eliminated_by_opponent"`
}

// PlaygroupGameCountResponse is the DTO for how many finished games the
// authenticated user has played within one of their playgroups.
type PlaygroupGameCountResponse struct {
	PlaygroupID   string `json:"playgroup_id"`
	PlaygroupName string `json:"playgroup_name"`
	GamesPlayed   int32  `json:"games_played"`
}

// FinishedGameResponse is one entry of the finished-games history
// (GET /statistics/games): a finished game enriched with every seat's
// username/deck, so the client doesn't need a lookup per player.
type FinishedGameResponse struct {
	ID            string                       `json:"id"`
	PlaygroupID   *string                      `json:"playgroup_id,omitempty"`
	PlaygroupName *string                      `json:"playgroup_name,omitempty"`
	StartedAt     *string                      `json:"started_at,omitempty"`
	FinishedAt    *string                      `json:"finished_at,omitempty"`
	Players       []FinishedGamePlayerResponse `json:"players"`
	// TurnCount is how many turns were played in total (every TurnStart action
	// belongs to one player's turn, see ListGameActionSummaryForGames). Zero for a
	// game whose actions weren't tracked turn-by-turn.
	TurnCount int32 `json:"turn_count"`
	// BiggestHit is the single largest CombatDamage/CommanderDamage hit dealt in the
	// game, if any were recorded.
	BiggestHit *BiggestHitResponse `json:"biggest_hit,omitempty"`
}

// BiggestHitResponse is the single largest hit dealt within a FinishedGameResponse.
type BiggestHitResponse struct {
	Amount   int32  `json:"amount"`
	Username string `json:"username"`
}

// FinishedGamePlayerResponse is one seat within a FinishedGameResponse.
type FinishedGamePlayerResponse struct {
	UserID        string  `json:"user_id"`
	Username      string  `json:"username"`
	DeckID        string  `json:"deck_id"`
	DeckName      string  `json:"deck_name"`
	DeckCommander string  `json:"deck_commander"`
	DeckImageURL  *string `json:"deck_image_url,omitempty"`
	Won           bool    `json:"won"`
}

// DashboardResponse is everything the web dashboard renders, in one payload.
//
// Deliberately shaped after that screen rather than after the domain: the
// client used to assemble it from six different endpoints, paginating through
// every deck and pulling every group's full game history just to show four
// recent games -- 30 requests and 539 KB for a 400-game account, of which it
// displayed almost nothing (measured 2026-09-02, see DECISIONS-LOG.md). The
// counts and slices below are therefore fixed server-side: they're the
// dashboard's layout, not a general-purpose query surface. Anything that needs
// the full lists still has /decks, /playgroups and /statistics/games.
type DashboardResponse struct {
	Stats UserStatsResponse `json:"stats"`
	// TotalDecks/TotalPlaygroups are the real totals, not len(Decks)/len(Playgroups):
	// the dashboard prints them in its "N games · N decks · N groups" line while
	// only listing the first few.
	TotalDecks      int32 `json:"total_decks"`
	TotalPlaygroups int32 `json:"total_playgroups"`
	// BestDeck is null until at least one game has been played with some deck.
	BestDeck    *DashboardDeckResponse       `json:"best_deck"`
	Decks       []DashboardDeckResponse      `json:"decks"`
	Playgroups  []DashboardPlaygroupResponse `json:"playgroups"`
	RecentGames []DashboardGameResponse      `json:"recent_games"`
	// Streak is how many finished games in a row ended the same way, counting
	// back from the most recent; StreakWon says which way. StreakWon is null
	// exactly when Streak is 0 (no finished games yet).
	Streak    int32 `json:"streak"`
	StreakWon *bool `json:"streak_won"`
}

// DashboardDeckResponse is one deck plus its record, flattened: the dashboard
// shows the two together everywhere, and keeping them apart only forced the
// client to join two lists by id.
type DashboardDeckResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Commander   string  `json:"commander"`
	ImageURL    *string `json:"image_url,omitempty"`
	GamesPlayed int32   `json:"games_played"`
	GamesWon    int32   `json:"games_won"`
}

// DashboardPlaygroupResponse is one group card. GamesPlayed is the group's
// finished games (what the card claims), not only the caller's; MemberCount is
// the full roster even though Members carries just enough for the avatar strip.
type DashboardPlaygroupResponse struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	MemberCount int32                     `json:"member_count"`
	GamesPlayed int32                     `json:"games_played"`
	Members     []DashboardMemberResponse `json:"members"`
}

// DashboardMemberResponse is one avatar in a group card's strip.
type DashboardMemberResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// DashboardGameResponse is one row of the "recent games" list, already resolved
// from the caller's point of view: their own result, their own deck, and the
// other seats by name.
type DashboardGameResponse struct {
	ID            string  `json:"id"`
	PlaygroupName *string `json:"playgroup_name,omitempty"`
	StartedAt     *string `json:"started_at,omitempty"`
	FinishedAt    *string `json:"finished_at,omitempty"`
	Won           bool    `json:"won"`
	// Deck is null when the caller's seat has no deck resolvable any more (the
	// deck was deleted); the client renders a placeholder for it.
	Deck      *DashboardDeckRef `json:"deck"`
	Opponents []string          `json:"opponents"`
}

// DashboardDeckRef is the deck a seat was played with — identity and art only,
// no record (the game's own result is what the row shows).
type DashboardDeckRef struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Commander string  `json:"commander"`
	ImageURL  *string `json:"image_url,omitempty"`
}

// FinishedGameListResponse is a page of the finished-games history.
// NextCursor is the cursor to pass as the `cursor` query param to request the
// next page, or null if this was the last one. The cursor is opaque: the
// client returns it as-is.
type FinishedGameListResponse struct {
	Items      []FinishedGameResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}
