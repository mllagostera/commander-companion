package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *tools) registerTournaments(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_tournaments",
		Description: "Tournaments the user organizes or plays in, newest first.",
		Annotations: readOnly("List tournaments"),
	}, t.listTournaments)

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_tournament",
		Description: "One Swiss-format tournament, by ID or join code: standings, every " +
			"round's tables and results, and the user's own seat in the current round.",
		Annotations: readOnly("Tournament detail"),
	}, t.getTournament)
}

// TournamentSummary is a tournament as listed.
type TournamentSummary struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status" jsonschema:"registration, in_progress or finished"`
	CurrentRound  int     `json:"current_round"`
	RoundCount    *int    `json:"round_count,omitempty" jsonschema:"set once the tournament starts"`
	JoinCode      string  `json:"join_code"`
	CreatedAt     string  `json:"created_at"`
	FinishedAt    *string `json:"finished_at,omitempty"`
	OrganizedByMe bool    `json:"organized_by_me"`
}

type apiTournament struct {
	TournamentSummary
	OrganizerID string `json:"organizer_id"`
}

func (t *tools) summary(ctx context.Context, tr *apiTournament) (TournamentSummary, error) {
	myID, _, err := t.c.Me(ctx)
	if err != nil {
		return TournamentSummary{}, err
	}
	s := tr.TournamentSummary
	s.OrganizedByMe = tr.OrganizerID == myID
	return s, nil
}

// --- list_tournaments -----------------------------------------------------

// ListTournamentsOutput is list_tournaments' answer.
type ListTournamentsOutput struct {
	Tournaments []TournamentSummary `json:"tournaments"`
	Truncated   bool                `json:"truncated"`
}

func (t *tools) listTournaments(
	ctx context.Context, _ *mcp.CallToolRequest, _ noInput,
) (*mcp.CallToolResult, ListTournamentsOutput, error) {
	all, truncated, err := fetchPages(ctx, t.c, pathTournaments, nil, keepAll[apiTournament])
	if err != nil {
		return nil, ListTournamentsOutput{}, err
	}
	out := ListTournamentsOutput{Tournaments: make([]TournamentSummary, 0, len(all)), Truncated: truncated}
	for i := range all {
		s, err := t.summary(ctx, &all[i])
		if err != nil {
			return nil, ListTournamentsOutput{}, err
		}
		out.Tournaments = append(out.Tournaments, s)
	}
	return nil, out, nil
}

// --- get_tournament -------------------------------------------------------

// GetTournamentInput is get_tournament's arguments.
type GetTournamentInput struct {
	Tournament string `json:"tournament" jsonschema:"tournament ID or join code"`
}

// TournamentDetail is get_tournament's answer.
type TournamentDetail struct {
	Tournament TournamentSummary `json:"tournament"`
	Standings  []TournamentEntry `json:"standings,omitempty" jsonschema:"most points first"`
	Rounds     []TournamentRound `json:"rounds,omitempty"`
	Me         *TournamentEntry  `json:"me,omitempty" jsonschema:"the user's own participant record"`
	MyTable    *TournamentTable  `json:"my_table,omitempty" jsonschema:"the user's table in the current round"`
	Note       string            `json:"note,omitempty"`
}

// TournamentEntry is one participant.
type TournamentEntry struct {
	Name      string `json:"name" jsonschema:"username, or the guest's name"`
	Commander string `json:"commander"`
	Points    int    `json:"points"`
}

// TournamentRound is one round's tables.
type TournamentRound struct {
	Number int               `json:"round_number"`
	Status string            `json:"status"`
	Tables []TournamentTable `json:"tables"`
}

// TournamentTable is one table and its seats.
type TournamentTable struct {
	Number int              `json:"table_number"`
	Seats  []TournamentSeat `json:"seats"`
}

// TournamentSeat is one seat at a table.
type TournamentSeat struct {
	Name           string `json:"name"`
	Commander      string `json:"commander"`
	FinishPosition *int   `json:"finish_position,omitempty" jsonschema:"absent until the result is recorded"`
	Points         int    `json:"points_awarded"`
}

type apiParticipant struct {
	Username      *string `json:"username"`
	GuestName     *string `json:"guest_name"`
	CommanderName string  `json:"commander_name"`
	Points        int     `json:"points"`
}

type apiTable struct {
	TableNumber int `json:"table_number"`
	Seats       []struct {
		Username       *string `json:"username"`
		GuestName      *string `json:"guest_name"`
		CommanderName  string  `json:"commander_name"`
		FinishPosition *int    `json:"finish_position"`
		PointsAwarded  int     `json:"points_awarded"`
	} `json:"seats"`
}

func (t *tools) getTournament(
	ctx context.Context, _ *mcp.CallToolRequest, in GetTournamentInput,
) (*mcp.CallToolResult, TournamentDetail, error) {
	ref := strings.TrimSpace(in.Tournament)
	if ref == "" {
		return nil, TournamentDetail{}, fmt.Errorf("%w: tournament is required", ErrInvalidInput)
	}
	out := TournamentDetail{}
	id := ref
	if !isUUID(ref) {
		if err := t.lookup(ctx, ref, &out); err != nil {
			return nil, TournamentDetail{}, err
		}
		id = out.Tournament.ID
	}
	if err := t.fillDetail(ctx, id, &out); err != nil {
		// The detail is for the organizer and participants. Someone who only
		// has the join code still gets the lookup's summary.
		if out.Tournament.ID != "" && (isStatus(err, http.StatusNotFound) || isStatus(err, http.StatusForbidden)) {
			out.Note = "standings and rounds are only visible to the organizer and participants"
			return nil, out, nil
		}
		return nil, TournamentDetail{}, err
	}
	return nil, out, nil
}

// lookup resolves a join code, filling the summary and the caller's own seat.
func (t *tools) lookup(ctx context.Context, code string, out *TournamentDetail) error {
	var res struct {
		Tournament   apiTournament   `json:"tournament"`
		Participant  *apiParticipant `json:"participant"`
		CurrentTable *apiTable       `json:"current_table"`
	}
	if err := t.c.Get(ctx, pathTournamentLookup, url.Values{"code": {code}}, &res); err != nil {
		return err
	}
	s, err := t.summary(ctx, &res.Tournament)
	if err != nil {
		return err
	}
	out.Tournament = s
	if res.Participant != nil {
		e := entry(res.Participant)
		out.Me = &e
	}
	if res.CurrentTable != nil {
		tbl := table(res.CurrentTable)
		out.MyTable = &tbl
	}
	return nil
}

func (t *tools) fillDetail(ctx context.Context, id string, out *TournamentDetail) error {
	var d struct {
		Tournament   apiTournament    `json:"tournament"`
		Participants []apiParticipant `json:"participants"`
		Rounds       []struct {
			RoundNumber int        `json:"round_number"`
			Status      string     `json:"status"`
			Tables      []apiTable `json:"tables"`
		} `json:"rounds"`
	}
	if err := t.c.Get(ctx, withID(pathTournament, id), nil, &d); err != nil {
		return err
	}
	s, err := t.summary(ctx, &d.Tournament)
	if err != nil {
		return err
	}
	out.Tournament = s
	for i := range d.Participants {
		out.Standings = append(out.Standings, entry(&d.Participants[i]))
	}
	for _, r := range d.Rounds {
		round := TournamentRound{Number: r.RoundNumber, Status: r.Status, Tables: []TournamentTable{}}
		for i := range r.Tables {
			round.Tables = append(round.Tables, table(&r.Tables[i]))
		}
		out.Rounds = append(out.Rounds, round)
	}
	return nil
}

func entry(p *apiParticipant) TournamentEntry {
	return TournamentEntry{Name: displayName(p.Username, p.GuestName), Commander: p.CommanderName, Points: p.Points}
}

func table(tbl *apiTable) TournamentTable {
	out := TournamentTable{Number: tbl.TableNumber, Seats: []TournamentSeat{}}
	for _, s := range tbl.Seats {
		out.Seats = append(out.Seats, TournamentSeat{
			Name:           displayName(s.Username, s.GuestName),
			Commander:      s.CommanderName,
			FinishPosition: s.FinishPosition,
			Points:         s.PointsAwarded,
		})
	}
	return out
}

func displayName(username, guest *string) string {
	if username != nil {
		return *username
	}
	if guest != nil {
		return cmp.Or(*guest, "guest") + " (guest)"
	}
	return unknown
}
