package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/usuario/commander-companion-backend/internal/mcpserver"
)

const (
	myID        = "11111111-1111-1111-1111-111111111111"
	rivalID     = "22222222-2222-2222-2222-222222222222"
	deckAtraxa  = "aaaaaaaa-0000-0000-0000-000000000001"
	deckYuriko  = "aaaaaaaa-0000-0000-0000-000000000002"
	deckYarok   = "aaaaaaaa-0000-0000-0000-000000000003"
	groupID     = "bbbbbbbb-0000-0000-0000-000000000001"
	gameID      = "cccccccc-0000-0000-0000-000000000001"
	seatMine    = "dddddddd-0000-0000-0000-000000000001"
	seatRival   = "dddddddd-0000-0000-0000-000000000002"
	tournamentA = "eeeeeeee-0000-0000-0000-000000000001"

	me      = "vansid"
	rival   = "rival"
	deckArg = "deck"
)

// fillIDs replaces the $NAME placeholders of a JSON fixture with the IDs above.
func fillIDs(fixture string) string {
	return strings.NewReplacer(
		"$ME", myID, "$RIVAL", rivalID, "$ATRAXA", deckAtraxa, "$YURIKO", deckYuriko, "$YAROK", deckYarok,
		"$GROUP", groupID, "$GAME", gameID, "$SEAT_MINE", seatMine, "$SEAT_RIVAL", seatRival, "$TOURNAMENT", tournamentA,
	).Replace(fixture)
}

// fakeAPI stands in for the Go API: canned JSON per "METHOD /path", bearer
// tokens it hands out itself, and counters for the auth calls.
type fakeAPI struct {
	t      *testing.T
	mu     sync.Mutex
	routes map[string]http.HandlerFunc

	access     string
	refresh    string
	issued     int
	logins     int
	refreshes  int
	rejectNext bool // answer the next authenticated call with 401
	refreshBad bool // answer /auth/refresh with 401
	loginBad   bool // answer /auth/login with 401
	requests   []string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	return &fakeAPI{t: t, routes: map[string]http.HandlerFunc{}}
}

// json serves body, a JSON fixture with $NAME placeholders, on route.
func (f *fakeAPI) json(route, body string) {
	f.routes[route] = func(w http.ResponseWriter, _ *http.Request) { writeRaw(f.t, w, http.StatusOK, body) }
}

func writeRaw(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(fillIDs(body))); err != nil {
		t.Errorf("writing fake response: %v", err)
	}
}

func writeError(t *testing.T, w http.ResponseWriter, status int, message string) {
	t.Helper()
	writeRaw(t, w, status, fmt.Sprintf(`{"code": %d, "message": %q}`, status, message))
}

func (f *fakeAPI) issueTokens(w http.ResponseWriter) {
	f.issued++
	f.access, f.refresh = fmt.Sprintf("access-%d", f.issued), fmt.Sprintf("refresh-%d", f.issued)
	writeRaw(f.t, w, http.StatusOK, fmt.Sprintf(
		`{"access_token": %q, "refresh_token": %q, "user": {"id": "$ME", "username": "vansid"}}`, f.access, f.refresh))
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	f.requests = append(f.requests, r.Method+" "+path+"?"+r.URL.RawQuery)

	switch path {
	case "/auth/login":
		f.logins++
		if f.loginBad {
			writeError(f.t, w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		f.issueTokens(w)
		return
	case "/auth/refresh":
		f.refreshes++
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if f.refreshBad || body.RefreshToken != f.refresh {
			writeError(f.t, w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		f.issueTokens(w)
		return
	case "/auth/logout":
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+f.access || f.rejectNext {
		f.rejectNext = false
		writeError(f.t, w, http.StatusUnauthorized, "token expired")
		return
	}
	h, ok := f.routes[r.Method+" "+path]
	if !ok {
		writeError(f.t, w, http.StatusNotFound, "not found")
		return
	}
	h(w, r)
}

func (f *fakeAPI) requested(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			out = append(out, r)
		}
	}
	return out
}

// connect starts the MCP server against api and returns a connected client.
func connect(t *testing.T, api *fakeAPI) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)

	client, err := mcpserver.NewClient(&mcpserver.ClientConfig{
		BaseURL: srv.URL + "/api/v1", Email: "me@example.com", Password: "secret", HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, connErr := mcpserver.NewServer(client, "test").Connect(ctx, serverT, nil); connErr != nil {
		t.Fatalf("server connect: %v", connErr)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil).
		Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// call runs a tool that must succeed and decodes its structured answer.
func call(t *testing.T, s *mcp.ClientSession, tool string, args map[string]any, out any) {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("%s returned a tool error: %s", tool, text(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("%s: marshaling structured content: %v", tool, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("%s: decoding %s: %v", tool, raw, err)
	}
}

// callErr runs a tool that must fail and returns the message the model sees.
func callErr(t *testing.T, s *mcp.ClientSession, tool string, args map[string]any) string {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if !res.IsError {
		t.Fatalf("%s succeeded, want a tool error", tool)
	}
	return text(res)
}

func text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func TestTools_AllRegisteredAndReadOnly(t *testing.T) {
	s := connect(t, newFakeAPI(t))
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{
		"get_overview": true, "query_games": true, "get_breakdown": true, "get_head_to_head": true,
		"list_decks": true, "get_deck": true, "list_playgroups": true, "get_playgroup": true,
		"get_game": true, "list_tournaments": true, "get_tournament": true,
	}
	if len(res.Tools) != len(want) {
		t.Errorf("got %d tools, want %d", len(res.Tools), len(want))
	}
	for _, tool := range res.Tools {
		if !want[tool.Name] {
			t.Errorf("unexpected tool %q", tool.Name)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("tool %q is not annotated read-only; phase 1 is read-only (ADR-0030)", tool.Name)
		}
		if tool.OutputSchema == nil {
			t.Errorf("tool %q has no output schema", tool.Name)
		}
	}
}

func TestGetOverview_ComputesWinRatesAndNamesTheUser(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /statistics/dashboard", `{
		"stats": {"games_played": 8, "games_won": 3},
		"total_decks": 2, "total_playgroups": 1,
		"best_deck": {"id": "$ATRAXA", "name": "Atraxa", "commander": "Atraxa", "games_played": 2, "games_won": 1,
			"image_url": "https://cards.example/atraxa.jpg"},
		"decks": [],
		"playgroups": [{"id": "$GROUP", "name": "Martes", "member_count": 2, "games_played": 8,
			"members": [{"user_id": "$ME", "username": "vansid"}]}],
		"recent_games": [{"id": "$GAME", "won": true, "opponents": ["rival"],
			"deck": {"id": "$ATRAXA", "name": "Atraxa", "commander": "Atraxa"}}],
		"streak": 1, "streak_won": true
	}`)
	s := connect(t, api)

	var out struct {
		Me    string `json:"me"`
		Stats struct {
			WinRatePct float64 `json:"win_rate_pct"`
		} `json:"stats"`
		BestDeck map[string]any `json:"best_deck"`
		Recent   []struct {
			DeckName  string   `json:"deck_name"`
			Opponents []string `json:"opponents"`
		} `json:"recent_games"`
		Playgroups []struct {
			Members []string `json:"members"`
		} `json:"playgroups"`
	}
	call(t, s, "get_overview", nil, &out)

	if out.Me != me {
		t.Errorf("me = %q, want vansid", out.Me)
	}
	if out.Stats.WinRatePct != 37.5 {
		t.Errorf("win_rate_pct = %v, want 37.5", out.Stats.WinRatePct)
	}
	if out.BestDeck["win_rate_pct"] != 50.0 {
		t.Errorf("best_deck win rate = %v, want 50", out.BestDeck["win_rate_pct"])
	}
	if _, leaked := out.BestDeck["image_url"]; leaked {
		t.Error("image_url reached the model; answers should only carry what it can use")
	}
	if len(out.Recent) != 1 || out.Recent[0].DeckName != "Atraxa" || out.Recent[0].Opponents[0] != rival {
		t.Errorf("recent_games = %+v", out.Recent)
	}
	if len(out.Playgroups) != 1 || out.Playgroups[0].Members[0] != me {
		t.Errorf("playgroups = %+v", out.Playgroups)
	}
}

func decksFixture(api *fakeAPI) {
	api.json("GET /decks", `{"next_cursor": null, "items": [
		{"id": "$ATRAXA", "name": "Atraxa Superfriends", "commander": "Atraxa, Praetors' Voice",
			"bracket": 4, "color_identity": ["W", "U", "B", "G"]},
		{"id": "$YURIKO", "name": "Ninjas", "commander": "Yuriko, the Tiger's Shadow",
			"bracket": 2, "color_identity": ["U", "B"]},
		{"id": "$YAROK", "name": "Lands", "commander": "Yarok, the Desecrated", "bracket": null, "color_identity": []}
	]}`)
	api.json("GET /statistics/decks", `[
		{"deck_id": "$YURIKO", "games_played": 5, "games_won": 2},
		{"deck_id": "$ATRAXA", "games_played": 1, "games_won": 1}
	]`)
}

type listedDeck struct {
	Name          string    `json:"name"`
	GamesPlayed   int       `json:"games_played"`
	WinRatePct    float64   `json:"win_rate_pct"`
	Bracket       *int      `json:"bracket"`
	ColorIdentity *[]string `json:"color_identity"`
}

func TestListDecks_JoinsStatsAndSortsByGamesPlayed(t *testing.T) {
	api := newFakeAPI(t)
	decksFixture(api)
	s := connect(t, api)

	var out struct {
		Count int          `json:"count"`
		Decks []listedDeck `json:"decks"`
	}
	call(t, s, "list_decks", nil, &out)

	if out.Count != 3 || out.Decks[0].Name != "Ninjas" || out.Decks[0].WinRatePct != 40 {
		t.Fatalf("decks = %+v, want Ninjas (most played, 40%%) first", out.Decks)
	}
	lands := out.Decks[2]
	if lands.Bracket != nil {
		t.Errorf("unknown bracket should be absent, got %v", *lands.Bracket)
	}
	if lands.ColorIdentity == nil || len(*lands.ColorIdentity) != 0 {
		t.Errorf("colorless deck must keep [] (absent means unknown), got %v", lands.ColorIdentity)
	}
}

func TestListDecks_ForwardsFiltersAsTheAPIsQueryParams(t *testing.T) {
	api := newFakeAPI(t)
	decksFixture(api)
	s := connect(t, api)

	var out map[string]any
	call(t, s, "list_decks", map[string]any{"bracket": []int{2, 3}, "colors": "UB", "color_mode": "includes"}, &out)

	got := api.requested("GET /decks?")
	if len(got) != 1 {
		t.Fatalf("GET /decks called %d times, want 1: %v", len(got), got)
	}
	for _, param := range []string{"bracket=2%2C3", "colors=UB", "color_mode=includes", "limit=100"} {
		if !strings.Contains(got[0], param) {
			t.Errorf("%s missing from %s", param, got[0])
		}
	}
}

// finishedGame is a two-seat finished game: me (with myDeck) against rival.
func finishedGame(id, myDeck string, iWon bool, finishedAt string) string {
	return fmt.Sprintf(`{"id": %q, "playgroup_name": "Martes", "finished_at": %q, "players": [
		{"user_id": "$ME", "username": "vansid", "deck_id": %q, "deck_name": "Mine",
			"deck_commander": "Someone", "won": %t},
		{"user_id": "$RIVAL", "username": "rival", "deck_id": "$YAROK", "deck_name": "Theirs",
			"deck_commander": "Yarok, the Desecrated", "won": %t}
	]}`, id, finishedAt, myDeck, iWon, !iWon)
}

const (
	otherGame   = "cccccccc-0000-0000-0000-000000000002"
	ancientGame = "cccccccc-0000-0000-0000-000000000003"
)

func TestGetDeck_ResolvesByCommanderAndReportsAmbiguity(t *testing.T) {
	api := newFakeAPI(t)
	decksFixture(api)
	api.json("GET /statistics/deck/"+deckYuriko,
		`{"deck_id": "$YURIKO", "games_played": 5, "games_won": 2, "highest_life_total_achieved": 52}`)
	api.json("GET /statistics/games", `{"next_cursor": null, "items": [`+
		finishedGame(gameID, deckYuriko, true, "2026-10-20T21:00:00Z")+","+
		finishedGame(otherGame, deckAtraxa, false, "2026-10-19T21:00:00Z")+`]}`)
	s := connect(t, api)

	var out struct {
		ID          string `json:"id"`
		HighestLife int    `json:"highest_life_total_achieved"`
		RecentGames []struct {
			ID  string `json:"id"`
			Won bool   `json:"won"`
		} `json:"recent_games"`
	}
	call(t, s, "get_deck", map[string]any{deckArg: "yuriko"}, &out)
	if out.ID != deckYuriko || out.HighestLife != 52 {
		t.Errorf("got %+v, want the Yuriko deck", out)
	}
	if len(out.RecentGames) != 1 || out.RecentGames[0].ID != gameID || !out.RecentGames[0].Won {
		t.Errorf("recent_games = %+v, want only the game played with this deck", out.RecentGames)
	}

	// "the" is in both Yuriko's and Yarok's commander names.
	msg := callErr(t, s, "get_deck", map[string]any{deckArg: "the"})
	if !strings.Contains(msg, "matches 2 decks") || !strings.Contains(msg, deckYarok) {
		t.Errorf("ambiguity message should list the candidates with their ids, got %q", msg)
	}
	if msg := callErr(t, s, "get_deck", map[string]any{deckArg: "Krenko"}); !strings.Contains(msg, "list_decks") {
		t.Errorf("a miss should point at list_decks, got %q", msg)
	}
}

func TestQueryGames_PagesUntilSinceAndFiltersByOpponent(t *testing.T) {
	api := newFakeAPI(t)
	api.routes["GET /statistics/games"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "" {
			writeRaw(t, w, http.StatusOK, `{"next_cursor": "page-2", "items": [`+
				finishedGame(gameID, deckYuriko, true, "2026-10-20T21:00:00Z")+`]}`)
			return
		}
		writeRaw(t, w, http.StatusOK, `{"next_cursor": "page-3", "items": [`+
			finishedGame(otherGame, deckYuriko, false, "2026-10-10T21:00:00Z")+","+
			finishedGame(ancientGame, deckYuriko, false, "2026-01-01T21:00:00Z")+`]}`)
	}
	s := connect(t, api)

	var out struct {
		Count     int  `json:"count"`
		Truncated bool `json:"truncated"`
		Games     []struct {
			Won bool `json:"won"`
		} `json:"games"`
	}
	call(t, s, "query_games", map[string]any{"since": "2026-10-01", "opponent": "RIV"}, &out)
	if out.Count != 2 || !out.Games[0].Won || out.Games[1].Won {
		t.Errorf("got %+v, want the two games since October, the newest won", out)
	}
	if out.Truncated {
		t.Error("stopping at the since date is not truncation")
	}
	if pages := api.requested("GET /statistics/games"); len(pages) != 2 {
		t.Errorf("paged %d times, want 2 (page 3 is past the since date): %v", len(pages), pages)
	}

	call(t, s, "query_games", map[string]any{"opponent": "nobody"}, &out)
	if out.Count != 0 {
		t.Errorf("opponent filter kept %d games, want 0", out.Count)
	}

	msg := callErr(t, s, "query_games", map[string]any{"since": "last week"})
	if !strings.Contains(msg, "YYYY-MM-DD") {
		t.Errorf("bad since should explain the format, got %q", msg)
	}
}

func TestClient_RenewsAnExpiredTokenWithTheRefreshToken(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /statistics/opponents", `[]`)
	s := connect(t, api)

	var out map[string]any
	call(t, s, "get_head_to_head", nil, &out)
	api.mu.Lock()
	api.rejectNext = true
	api.mu.Unlock()
	call(t, s, "get_head_to_head", nil, &out)

	api.mu.Lock()
	defer api.mu.Unlock()
	if api.logins != 1 || api.refreshes != 1 {
		t.Errorf("logins=%d refreshes=%d, want one login and one refresh", api.logins, api.refreshes)
	}
}

func TestClient_LogsInAgainWhenTheRefreshTokenIsRejected(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /statistics/opponents", `[]`)
	s := connect(t, api)

	var out map[string]any
	call(t, s, "get_head_to_head", nil, &out)
	api.mu.Lock()
	api.rejectNext, api.refreshBad = true, true
	api.mu.Unlock()
	call(t, s, "get_head_to_head", nil, &out)

	api.mu.Lock()
	defer api.mu.Unlock()
	if api.logins != 2 || api.refreshes != 1 {
		t.Errorf("logins=%d refreshes=%d, want a second login after the failed refresh", api.logins, api.refreshes)
	}
}

func TestClient_DoesNotRetryARejectedLogin(t *testing.T) {
	api := newFakeAPI(t)
	api.loginBad = true
	s := connect(t, api)

	for range 3 {
		if msg := callErr(t, s, "get_head_to_head", nil); !strings.Contains(msg, "invalid credentials") {
			t.Errorf("the model should see why the login failed, got %q", msg)
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.logins != 1 {
		t.Errorf("login attempted %d times, want 1: failed logins count against a per-account limit", api.logins)
	}
}

const martesRoster = `[{"user_id": "$ME", "username": "vansid"}, {"user_id": "$RIVAL", "username": "rival"}]`

func TestGetPlaygroup_ByNameWithStandingsBestFirst(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /playgroups", `[{"id": "$GROUP", "name": "Martes", "members": `+martesRoster+`}]`)
	api.json("GET /statistics/playgroup/"+groupID, `{"games_played": 10, "members": [
		{"user_id": "$ME", "games_played": 10, "games_won": 3},
		{"user_id": "$RIVAL", "games_played": 10, "games_won": 7}
	]}`)
	s := connect(t, api)

	var out struct {
		GamesPlayed int `json:"games_played"`
		Standings   []struct {
			Username   string  `json:"username"`
			WinRatePct float64 `json:"win_rate_pct"`
		} `json:"standings"`
	}
	call(t, s, "get_playgroup", map[string]any{"playgroup": "mart"}, &out)
	if out.GamesPlayed != 10 || out.Standings[0].Username != rival || out.Standings[0].WinRatePct != 70 {
		t.Errorf("got %+v, want rival (70%%) first", out)
	}
}

func TestGetGame_NamesSeatsInTheTimeline(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /games/"+gameID, `{"id": "$GAME", "status": "finished", "playgroup_id": "$GROUP", "players": [
		{"id": "$SEAT_MINE", "user_id": "$ME", "deck_id": "$YURIKO", "life_total": 12},
		{"id": "$SEAT_RIVAL", "user_id": "$RIVAL", "deck_id": "$YAROK", "is_eliminated": true}
	]}`)
	api.json("GET /games/"+gameID+"/timeline", `[{"actor_id": "$SEAT_MINE", "target_id": "$SEAT_RIVAL",
		"action_type": "CommanderDamage", "payload": {"amount": 21}, "created_at": "2026-10-20T21:00:00Z"}]`)
	api.json("GET /playgroups/"+groupID, `{"id": "$GROUP", "name": "Martes", "members": `+martesRoster+`}`)
	s := connect(t, api)

	var out struct {
		Actions []struct {
			Actor   string         `json:"actor"`
			Target  string         `json:"target"`
			Payload map[string]any `json:"payload"`
		} `json:"actions"`
	}
	call(t, s, "get_game", map[string]any{"game_id": gameID}, &out)
	if len(out.Actions) != 1 || out.Actions[0].Actor != me || out.Actions[0].Target != rival ||
		out.Actions[0].Payload["amount"] != 21.0 {
		t.Errorf("actions = %+v, want vansid -> rival, 21", out.Actions)
	}

	msg := callErr(t, s, "get_game", map[string]any{"game_id": "yesterday's game"})
	if !strings.Contains(msg, "query_games") {
		t.Errorf("a non-ID should point at query_games, got %q", msg)
	}
}

func TestGetTournament_ByJoinCodeWhenTheDetailIsHidden(t *testing.T) {
	api := newFakeAPI(t)
	api.json("GET /tournaments/lookup", `{"participant": null, "current_table": null, "tournament": {
		"id": "$TOURNAMENT", "name": "Liga", "status": "in_progress", "current_round": 2,
		"join_code": "ABC123", "organizer_id": "$RIVAL", "created_at": "2026-10-01T10:00:00Z"}}`)
	s := connect(t, api)

	var out struct {
		Tournament struct {
			Name          string `json:"name"`
			OrganizedByMe bool   `json:"organized_by_me"`
		} `json:"tournament"`
		Note string `json:"note"`
	}
	call(t, s, "get_tournament", map[string]any{"tournament": "ABC123"}, &out)
	if out.Tournament.Name != "Liga" || out.Tournament.OrganizedByMe || out.Note == "" {
		t.Errorf("got %+v, want the lookup's summary and a note about the hidden detail", out)
	}
	if got := api.requested("GET /tournaments/lookup?code=ABC123"); len(got) != 1 {
		t.Errorf("lookup not called with the code: %v", api.requested("GET /tournaments"))
	}
}

func TestPromptsAndResource(t *testing.T) {
	s := connect(t, newFakeAPI(t))
	ctx := context.Background()

	params := &mcp.GetPromptParams{Name: "deck-review", Arguments: map[string]string{deckArg: "Ninjas"}}
	prompt, err := s.GetPrompt(ctx, params)
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	tc, ok := prompt.Messages[0].Content.(*mcp.TextContent)
	if !ok || !strings.Contains(tc.Text, `"Ninjas"`) {
		t.Errorf("deck-review prompt should name the deck, got %+v", prompt.Messages[0].Content)
	}

	res, err := s.ReadResource(ctx, &mcp.ReadResourceParams{URI: "commander://reference/brackets"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if !strings.Contains(res.Contents[0].Text, "sole survivor") {
		t.Error("the reference should define what a win is")
	}
}

func TestNewClient_RejectsMissingCredentialsAndBadURL(t *testing.T) {
	if _, err := mcpserver.NewClient(&mcpserver.ClientConfig{BaseURL: "http://localhost:8080/api/v1"}); err == nil {
		t.Error("missing credentials accepted")
	}
	cfg := &mcpserver.ClientConfig{BaseURL: "localhost:8080", Email: "a@b.c", Password: "x"}
	if _, err := mcpserver.NewClient(cfg); err == nil {
		t.Error("a base URL without scheme accepted")
	}
}
