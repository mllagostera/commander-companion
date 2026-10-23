package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultGamesLimit = 25
	maxGamesLimit     = 200
)

func (t *tools) registerStatistics(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_overview",
		Description: "The user's overall record, best and most played decks, playgroups, " +
			"last finished games and current win/loss streak, in one call. Call it first.",
		Annotations: readOnly("Overview"),
	}, t.getOverview)

	mcp.AddTool(s, &mcp.Tool{
		Name: "query_games",
		Description: "The user's finished games, most recent first, with every seat's " +
			"player, deck, commander, bracket and color identity as they were when played. " +
			"Filter by date, opponent, or by the user's own deck bracket/colors.",
		Annotations: readOnly("Query finished games"),
	}, t.queryGames)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_breakdown",
		Description: "The user's games and wins grouped by their own deck's bracket or " +
			"color identity at the time of each game.",
		Annotations: readOnly("Breakdown by bracket or colors"),
	}, t.getBreakdown)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_head_to_head",
		Description: "The user's record against every opponent they have shared a finished " +
			"game with: games together and eliminations in each direction.",
		Annotations: readOnly("Head-to-head record"),
	}, t.getHeadToHead)
}

// --- get_overview ---------------------------------------------------------

type noInput struct{}

// Overview is get_overview's answer.
type Overview struct {
	Me              string             `json:"me" jsonschema:"the logged-in username"`
	Stats           UserStats          `json:"stats"`
	TotalDecks      int                `json:"total_decks"`
	TotalPlaygroups int                `json:"total_playgroups"`
	BestDeck        *DeckRecord        `json:"best_deck,omitempty" jsonschema:"highest win rate among played decks"`
	TopDecks        []DeckRecord       `json:"top_decks" jsonschema:"most played first; not every deck, see list_decks"`
	Playgroups      []PlaygroupSummary `json:"playgroups"`
	RecentGames     []RecentGame       `json:"recent_games"`
	Streak          int                `json:"streak" jsonschema:"finished games in a row that ended the same way"`
	StreakWon       *bool              `json:"streak_won,omitempty" jsonschema:"true: winning streak; false: losing"`
}

type apiDashboard struct {
	Stats           UserStats     `json:"stats"`
	TotalDecks      int           `json:"total_decks"`
	TotalPlaygroups int           `json:"total_playgroups"`
	BestDeck        *DeckRecord   `json:"best_deck"`
	Decks           []DeckRecord  `json:"decks"`
	Playgroups      []apiDashPG   `json:"playgroups"`
	RecentGames     []apiDashGame `json:"recent_games"`
	Streak          int           `json:"streak"`
	StreakWon       *bool         `json:"streak_won"`
}

type apiDashPG struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	GamesPlayed int    `json:"games_played"`
	Members     []struct {
		Username string `json:"username"`
	} `json:"members"`
}

type apiDashGame struct {
	ID            string  `json:"id"`
	PlaygroupName *string `json:"playgroup_name"`
	FinishedAt    *string `json:"finished_at"`
	Won           bool    `json:"won"`
	Deck          *struct {
		Name      string `json:"name"`
		Commander string `json:"commander"`
	} `json:"deck"`
	Opponents []string `json:"opponents"`
}

func (t *tools) getOverview(
	ctx context.Context, _ *mcp.CallToolRequest, _ noInput,
) (*mcp.CallToolResult, Overview, error) {
	_, me, err := t.c.Me(ctx)
	if err != nil {
		return nil, Overview{}, err
	}
	var d apiDashboard
	if err := t.c.Get(ctx, pathDashboard, nil, &d); err != nil {
		return nil, Overview{}, err
	}
	out := Overview{
		Me:              me,
		Stats:           d.Stats,
		TotalDecks:      d.TotalDecks,
		TotalPlaygroups: d.TotalPlaygroups,
		BestDeck:        d.BestDeck,
		TopDecks:        d.Decks,
		Streak:          d.Streak,
		StreakWon:       d.StreakWon,
	}
	out.Stats.WinRatePct = winRate(d.Stats.GamesWon, d.Stats.GamesPlayed)
	if out.BestDeck != nil {
		out.BestDeck.WinRatePct = winRate(out.BestDeck.GamesWon, out.BestDeck.GamesPlayed)
	}
	for i := range out.TopDecks {
		out.TopDecks[i].WinRatePct = winRate(out.TopDecks[i].GamesWon, out.TopDecks[i].GamesPlayed)
	}
	for i := range d.Playgroups {
		pg := &d.Playgroups[i]
		summary := PlaygroupSummary{ID: pg.ID, Name: pg.Name, GamesPlayed: &pg.GamesPlayed, Members: []string{}}
		for _, m := range pg.Members {
			summary.Members = append(summary.Members, m.Username)
		}
		out.Playgroups = append(out.Playgroups, summary)
	}
	for i := range d.RecentGames {
		g := &d.RecentGames[i]
		rg := RecentGame{
			ID: g.ID, PlaygroupName: g.PlaygroupName, FinishedAt: g.FinishedAt, Won: g.Won, Opponents: g.Opponents,
		}
		if g.Deck != nil {
			rg.DeckName, rg.Commander = g.Deck.Name, g.Deck.Commander
		}
		out.RecentGames = append(out.RecentGames, rg)
	}
	return nil, out, nil
}

// --- query_games ----------------------------------------------------------

// QueryGamesInput is query_games' arguments.
type QueryGamesInput struct {
	Limit     int    `json:"limit,omitempty" jsonschema:"how many games to return; default 25, max 200"`
	Since     string `json:"since,omitempty" jsonschema:"only games finished on or after this date (YYYY-MM-DD)"`
	Opponent  string `json:"opponent,omitempty" jsonschema:"only games where a player's username contains this"`
	Bracket   []int  `json:"bracket,omitempty" jsonschema:"only games where the user's deck had these brackets (1-5)"`
	Colors    string `json:"colors,omitempty" jsonschema:"the user's deck color identity: WUBRG letters (e.g. UB) or C"`
	ColorMode string `json:"color_mode,omitempty" jsonschema:"how colors match: exact (default), includes or within"`
}

// GamesOutput is query_games' answer.
type GamesOutput struct {
	Count     int            `json:"count"`
	Games     []FinishedGame `json:"games"`
	Truncated bool           `json:"truncated" jsonschema:"more games match than were returned"`
}

func (t *tools) queryGames(
	ctx context.Context, _ *mcp.CallToolRequest, in *QueryGamesInput,
) (*mcp.CallToolResult, GamesOutput, error) {
	limit, since, err := in.bounds()
	if err != nil {
		return nil, GamesOutput{}, err
	}
	myID, _, err := t.c.Me(ctx)
	if err != nil {
		return nil, GamesOutput{}, err
	}

	sel := &gameSelector{limit: limit, since: since, opponent: strings.ToLower(strings.TrimSpace(in.Opponent)), myID: myID}
	query := deckFilterQuery(in.Bracket, in.Colors, in.ColorMode)
	games, truncated, err := fetchPages(ctx, t.c, pathFinishedGames, query, sel.visit)
	if err != nil {
		return nil, GamesOutput{}, err
	}
	for i := range games {
		games[i].Won = seatOf(&games[i], myID).Won
	}
	if games == nil {
		games = []FinishedGame{}
	}
	return nil, GamesOutput{Count: len(games), Games: games, Truncated: truncated && !sel.reachedSince}, nil
}

// gameSelector is query_games' filter over the newest-first history.
type gameSelector struct {
	limit        int
	since        time.Time
	opponent     string
	myID         string
	kept         int
	reachedSince bool
}

func (s *gameSelector) visit(g *FinishedGame) (keep, stop bool) {
	if !s.since.IsZero() && finishedBefore(g, s.since) {
		s.reachedSince = true
		return false, true
	}
	if s.opponent != "" && !hasPlayer(g, s.opponent, s.myID) {
		return false, false
	}
	s.kept++
	return true, s.kept >= s.limit
}

// bounds is the limit to return and the date to stop at (zero for none).
func (in *QueryGamesInput) bounds() (limit int, since time.Time, err error) {
	limit = in.Limit
	if limit <= 0 {
		limit = defaultGamesLimit
	}
	limit = min(limit, maxGamesLimit)
	if in.Since != "" {
		since, err = time.Parse(time.DateOnly, in.Since)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("%w: since must be YYYY-MM-DD, got %q", ErrInvalidInput, in.Since)
		}
	}
	return limit, since, nil
}

func finishedBefore(g *FinishedGame, since time.Time) bool {
	if g.FinishedAt == nil {
		return false
	}
	at, err := time.Parse(time.RFC3339, *g.FinishedAt)
	return err == nil && at.Before(since)
}

func hasPlayer(g *FinishedGame, usernamePart, exceptID string) bool {
	for i := range g.Players {
		p := &g.Players[i]
		if p.UserID != exceptID && strings.Contains(strings.ToLower(p.Username), usernamePart) {
			return true
		}
	}
	return false
}

// seatOf returns the given user's seat, or an empty one.
func seatOf(g *FinishedGame, userID string) FinishedSeat {
	for i := range g.Players {
		if g.Players[i].UserID == userID {
			return g.Players[i]
		}
	}
	return FinishedSeat{}
}

// --- get_breakdown --------------------------------------------------------

// BreakdownInput is get_breakdown's arguments.
type BreakdownInput struct {
	GroupBy string `json:"group_by" jsonschema:"bracket or color_identity"`
}

// BreakdownOutput is get_breakdown's answer.
type BreakdownOutput struct {
	GroupBy string           `json:"group_by"`
	Groups  []BreakdownGroup `json:"groups" jsonschema:"most played first"`
}

// BreakdownGroup is one bracket or color identity.
type BreakdownGroup struct {
	Label       string  `json:"label" jsonschema:"e.g. 'bracket 3', 'UB', 'colorless' or 'unknown'"`
	GamesPlayed int     `json:"games_played"`
	GamesWon    int     `json:"games_won"`
	WinRatePct  float64 `json:"win_rate_pct"`
}

func (t *tools) getBreakdown(
	ctx context.Context, _ *mcp.CallToolRequest, in BreakdownInput,
) (*mcp.CallToolResult, BreakdownOutput, error) {
	if in.GroupBy != "bracket" && in.GroupBy != "color_identity" {
		return nil, BreakdownOutput{}, fmt.Errorf("%w: group_by must be bracket or color_identity", ErrInvalidInput)
	}
	var res struct {
		Items []struct {
			Bracket       *int      `json:"bracket"`
			ColorIdentity *[]string `json:"color_identity"`
			GamesPlayed   int       `json:"games_played"`
			GamesWon      int       `json:"games_won"`
		} `json:"items"`
	}
	if err := t.c.Get(ctx, pathBreakdown, url.Values{"group_by": {in.GroupBy}}, &res); err != nil {
		return nil, BreakdownOutput{}, err
	}
	out := BreakdownOutput{GroupBy: in.GroupBy, Groups: []BreakdownGroup{}}
	for _, item := range res.Items {
		label := bracketLabel(item.Bracket)
		if in.GroupBy == "color_identity" {
			label = colorLabel(item.ColorIdentity)
		}
		out.Groups = append(out.Groups, BreakdownGroup{
			Label:       label,
			GamesPlayed: item.GamesPlayed,
			GamesWon:    item.GamesWon,
			WinRatePct:  winRate(item.GamesWon, item.GamesPlayed),
		})
	}
	return nil, out, nil
}

// --- get_head_to_head -----------------------------------------------------

// HeadToHeadInput is get_head_to_head's arguments.
type HeadToHeadInput struct {
	Opponent string `json:"opponent,omitempty" jsonschema:"only opponents whose username contains this"`
}

// HeadToHeadOutput is get_head_to_head's answer.
type HeadToHeadOutput struct {
	Opponents []Opponent `json:"opponents" jsonschema:"most games together first"`
}

// Opponent is the user's record against one player.
type Opponent struct {
	UserID                    string `json:"user_id"`
	Username                  string `json:"username"`
	GamesTogether             int    `json:"games_together"`
	TimesYouEliminatedThem    int    `json:"times_you_eliminated_them"`
	TimesEliminatedByOpponent int    `json:"times_eliminated_by_opponent"`
}

func (t *tools) getHeadToHead(
	ctx context.Context, _ *mcp.CallToolRequest, in HeadToHeadInput,
) (*mcp.CallToolResult, HeadToHeadOutput, error) {
	var all []Opponent
	if err := t.c.Get(ctx, pathOpponents, nil, &all); err != nil {
		return nil, HeadToHeadOutput{}, err
	}
	needle := strings.ToLower(strings.TrimSpace(in.Opponent))
	out := HeadToHeadOutput{Opponents: []Opponent{}}
	for _, o := range all {
		if needle == "" || strings.Contains(strings.ToLower(o.Username), needle) {
			out.Opponents = append(out.Opponents, o)
		}
	}
	slices.SortStableFunc(out.Opponents, func(a, b Opponent) int {
		return cmp.Compare(b.GamesTogether, a.GamesTogether)
	})
	return nil, out, nil
}
