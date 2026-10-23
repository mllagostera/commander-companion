package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxTimelineActions caps the actions get_game returns. A real game stays far
// below it; the cap only keeps a pathological one from flooding the context.
const maxTimelineActions = 500

func (t *tools) registerGames(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_game",
		Description: "One game by ID (IDs come from query_games or get_overview): its seats " +
			"with final life and poison, and its full action timeline (life changes, combat " +
			"and commander damage, turns, eliminations) in order. For deck names of a finished " +
			"game use query_games.",
		Annotations: readOnly("Game timeline"),
	}, t.getGame)
}

// GetGameInput is get_game's arguments.
type GetGameInput struct {
	GameID string `json:"game_id" jsonschema:"the game's ID"`
}

// GameDetail is get_game's answer.
type GameDetail struct {
	ID          string      `json:"id"`
	Status      string      `json:"status" jsonschema:"pending, active or finished"`
	PlaygroupID *string     `json:"playgroup_id,omitempty"`
	StartedAt   *string     `json:"started_at,omitempty"`
	FinishedAt  *string     `json:"finished_at,omitempty"`
	Seats       []GameSeat  `json:"seats"`
	Actions     []GameEvent `json:"actions" jsonschema:"oldest first; undone actions are left out"`
	Truncated   bool        `json:"truncated"`
}

// GameSeat is one seat of a game.
type GameSeat struct {
	SeatID         string `json:"seat_id" jsonschema:"what actions refer to as actor/target"`
	UserID         string `json:"user_id"`
	Username       string `json:"username,omitempty" jsonschema:"known for playgroup games and the user's own seat"`
	DeckID         string `json:"deck_id"`
	LifeTotal      int    `json:"life_total"`
	PoisonCounters int    `json:"poison_counters"`
	IsEliminated   bool   `json:"is_eliminated"`
}

// GameEvent is one action of the timeline.
type GameEvent struct {
	At      string         `json:"at"`
	Type    string         `json:"type"`
	Actor   string         `json:"actor" jsonschema:"username, or seat ID when unknown"`
	Target  string         `json:"target,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type apiGame struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	PlaygroupID *string `json:"playgroup_id"`
	StartedAt   *string `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
	Players     []struct {
		ID             string `json:"id"`
		UserID         string `json:"user_id"`
		DeckID         string `json:"deck_id"`
		LifeTotal      int    `json:"life_total"`
		PoisonCounters int    `json:"poison_counters"`
		IsEliminated   bool   `json:"is_eliminated"`
	} `json:"players"`
}

type apiAction struct {
	ActorID    string         `json:"actor_id"`
	TargetID   *string        `json:"target_id"`
	ActionType string         `json:"action_type"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  string         `json:"created_at"`
}

func (t *tools) getGame(
	ctx context.Context, _ *mcp.CallToolRequest, in GetGameInput,
) (*mcp.CallToolResult, GameDetail, error) {
	id := strings.TrimSpace(in.GameID)
	if !isUUID(id) {
		return nil, GameDetail{}, fmt.Errorf("%w: game_id must be a game's ID; find it with query_games", ErrInvalidInput)
	}
	var g apiGame
	if err := t.c.Get(ctx, withID(pathGame, id), nil, &g); err != nil {
		return nil, GameDetail{}, err
	}
	var actions []apiAction
	if err := t.c.Get(ctx, withID(pathGameTimeline, id), nil, &actions); err != nil {
		return nil, GameDetail{}, err
	}
	usernames, err := t.usernamesFor(ctx, g.PlaygroupID)
	if err != nil {
		return nil, GameDetail{}, err
	}

	out := GameDetail{
		ID: g.ID, Status: g.Status, PlaygroupID: g.PlaygroupID, StartedAt: g.StartedAt, FinishedAt: g.FinishedAt,
		Seats: make([]GameSeat, 0, len(g.Players)), Actions: []GameEvent{},
	}
	seatName := make(map[string]string, len(g.Players))
	for i := range g.Players {
		p := &g.Players[i]
		name := usernames[p.UserID]
		seatName[p.ID] = cmp.Or(name, p.ID)
		out.Seats = append(out.Seats, GameSeat{
			SeatID: p.ID, UserID: p.UserID, Username: name, DeckID: p.DeckID,
			LifeTotal: p.LifeTotal, PoisonCounters: p.PoisonCounters, IsEliminated: p.IsEliminated,
		})
	}
	for i := range actions {
		if i == maxTimelineActions {
			out.Truncated = true
			break
		}
		a := &actions[i]
		ev := GameEvent{
			At: a.CreatedAt, Type: a.ActionType, Actor: cmp.Or(seatName[a.ActorID], a.ActorID), Payload: a.Payload,
		}
		if a.TargetID != nil {
			ev.Target = cmp.Or(seatName[*a.TargetID], *a.TargetID)
		}
		out.Actions = append(out.Actions, ev)
	}
	return nil, out, nil
}

// usernamesFor maps user IDs to usernames: the playgroup's roster when the
// game has one, plus the caller in every case.
func (t *tools) usernamesFor(ctx context.Context, playgroupID *string) (map[string]string, error) {
	myID, me, err := t.c.Me(ctx)
	if err != nil {
		return nil, err
	}
	names := map[string]string{myID: me}
	if playgroupID == nil {
		return names, nil
	}
	var g apiPlaygroup
	if err := t.c.Get(ctx, withID(pathPlaygroup, *playgroupID), nil, &g); err != nil {
		return nil, err
	}
	for _, m := range g.Members {
		names[m.UserID] = m.Username
	}
	return names, nil
}
