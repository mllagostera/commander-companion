package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *tools) registerPlaygroups(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_playgroups",
		Description: "The user's playgroups with their members and how many games the user played in each.",
		Annotations: readOnly("List playgroups"),
	}, t.listPlaygroups)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_playgroup",
		Description: "One playgroup, by ID or name: its finished games and every member's " +
			"record within it, best first.",
		Annotations: readOnly("Playgroup standings"),
	}, t.getPlaygroup)
}

type apiPlaygroup struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Members []struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
	} `json:"members"`
}

// --- list_playgroups ------------------------------------------------------

// ListPlaygroupsOutput is list_playgroups' answer.
type ListPlaygroupsOutput struct {
	Playgroups []PlaygroupSummary `json:"playgroups"`
}

func (t *tools) listPlaygroups(
	ctx context.Context, _ *mcp.CallToolRequest, _ noInput,
) (*mcp.CallToolResult, ListPlaygroupsOutput, error) {
	groups, err := t.allPlaygroups(ctx)
	if err != nil {
		return nil, ListPlaygroupsOutput{}, err
	}
	var counts []struct {
		PlaygroupID string `json:"playgroup_id"`
		GamesPlayed int    `json:"games_played"`
	}
	if err := t.c.Get(ctx, pathPlaygroupGameCount, nil, &counts); err != nil {
		return nil, ListPlaygroupsOutput{}, err
	}
	mine := make(map[string]int, len(counts))
	for _, c := range counts {
		mine[c.PlaygroupID] = c.GamesPlayed
	}
	out := ListPlaygroupsOutput{Playgroups: make([]PlaygroupSummary, 0, len(groups))}
	for i := range groups {
		played := mine[groups[i].ID]
		summary := PlaygroupSummary{ID: groups[i].ID, Name: groups[i].Name, Members: []string{}, MyGamesPlayed: &played}
		for _, m := range groups[i].Members {
			summary.Members = append(summary.Members, m.Username)
		}
		out.Playgroups = append(out.Playgroups, summary)
	}
	return nil, out, nil
}

// allPlaygroups uses the unpaginated listing, the only one that includes
// each group's members.
func (t *tools) allPlaygroups(ctx context.Context) ([]apiPlaygroup, error) {
	var groups []apiPlaygroup
	if err := t.c.Get(ctx, pathPlaygroups, nil, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// --- get_playgroup --------------------------------------------------------

// GetPlaygroupInput is get_playgroup's arguments.
type GetPlaygroupInput struct {
	Playgroup string `json:"playgroup" jsonschema:"playgroup ID or name (a partial name works if unique)"`
}

// PlaygroupDetail is get_playgroup's answer.
type PlaygroupDetail struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	GamesPlayed int        `json:"games_played" jsonschema:"finished games of the group"`
	Standings   []Standing `json:"standings" jsonschema:"most wins first"`
}

// Standing is one member's record within a playgroup.
type Standing struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	GamesPlayed int     `json:"games_played"`
	GamesWon    int     `json:"games_won"`
	WinRatePct  float64 `json:"win_rate_pct"`
}

func (t *tools) getPlaygroup(
	ctx context.Context, _ *mcp.CallToolRequest, in GetPlaygroupInput,
) (*mcp.CallToolResult, PlaygroupDetail, error) {
	group, err := t.resolvePlaygroup(ctx, in.Playgroup)
	if err != nil {
		return nil, PlaygroupDetail{}, err
	}
	var st struct {
		GamesPlayed int        `json:"games_played"`
		Members     []Standing `json:"members"`
	}
	if err := t.c.Get(ctx, withID(pathPlaygroupStats, group.ID), nil, &st); err != nil {
		return nil, PlaygroupDetail{}, err
	}
	names := make(map[string]string, len(group.Members))
	for _, m := range group.Members {
		names[m.UserID] = m.Username
	}
	out := PlaygroupDetail{ID: group.ID, Name: group.Name, GamesPlayed: st.GamesPlayed, Standings: []Standing{}}
	for _, m := range st.Members {
		m.Username = names[m.UserID]
		m.WinRatePct = winRate(m.GamesWon, m.GamesPlayed)
		out.Standings = append(out.Standings, m)
	}
	slices.SortStableFunc(out.Standings, func(a, b Standing) int {
		return cmp.Or(cmp.Compare(b.GamesWon, a.GamesWon), cmp.Compare(b.WinRatePct, a.WinRatePct))
	})
	return nil, out, nil
}

func (t *tools) resolvePlaygroup(ctx context.Context, ref string) (*apiPlaygroup, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("%w: playgroup is required", ErrInvalidInput)
	}
	if isUUID(ref) {
		var g apiPlaygroup
		if err := t.c.Get(ctx, withID(pathPlaygroup, strings.TrimSpace(ref)), nil, &g); err != nil {
			return nil, err
		}
		return &g, nil
	}
	groups, err := t.allPlaygroups(ctx)
	if err != nil {
		return nil, err
	}
	match, candidates := matchName(ref, groups, func(g *apiPlaygroup) []string { return []string{g.Name} })
	if match != nil {
		return match, nil
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no playgroup is named %q; call list_playgroups", ErrNoMatch, ref)
	}
	names := make([]string, 0, len(candidates))
	for i := range candidates {
		names = append(names, fmt.Sprintf("%s (id %s)", candidates[i].Name, candidates[i].ID))
	}
	return nil, fmt.Errorf("%w: %q matches %d playgroups: %s", ErrAmbiguous, ref, len(names), strings.Join(names, "; "))
}
