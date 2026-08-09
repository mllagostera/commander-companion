package tournaments

// CreateTournamentRequest is the payload for POST /tournaments.
type CreateTournamentRequest struct {
	Name          string `json:"name"`
	TargetPlayers *int32 `json:"target_players,omitempty"`
}

// JoinTournamentRequest is the payload for POST /tournaments/join: an
// authenticated app user self-registers with one of their own decks.
type JoinTournamentRequest struct {
	JoinCode string `json:"join_code"`
	DeckID   string `json:"deck_id"`
}

// AddGuestParticipantRequest is the payload for POST /tournaments/{id}/participants:
// organizer-only, for participants with no account.
type AddGuestParticipantRequest struct {
	GuestName     string `json:"guest_name"`
	CommanderName string `json:"commander_name"`
}

// SeatResult is one table seat's finish position, part of RecordTableResultRequest.
type SeatResult struct {
	ParticipantID  string `json:"participant_id"`
	FinishPosition int32  `json:"finish_position"`
}

// RecordTableResultRequest is the payload for POST /tournaments/{id}/tables/{tableId}/result.
// Results must cover exactly the table's seats, one entry each, with
// finish_position forming a permutation of 1..len(seats).
type RecordTableResultRequest struct {
	Results []SeatResult `json:"results"`
}

// TournamentResponse is the DTO of a tournament sent to the client.
type TournamentResponse struct {
	ID            string  `json:"id"`
	OrganizerID   string  `json:"organizer_id"`
	Name          string  `json:"name"`
	Format        string  `json:"format"`
	TargetPlayers *int32  `json:"target_players,omitempty"`
	Status        string  `json:"status"`
	RoundCount    *int32  `json:"round_count,omitempty"`
	CurrentRound  int32   `json:"current_round"`
	JoinCode      string  `json:"join_code"`
	CreatedAt     string  `json:"created_at"`
	StartedAt     *string `json:"started_at,omitempty"`
	FinishedAt    *string `json:"finished_at,omitempty"`
}

// TournamentListResponse is a page of tournaments the caller organizes or
// participates in (see internal/common/pagination.go).
type TournamentListResponse struct {
	Items      []TournamentResponse `json:"items"`
	NextCursor *string              `json:"next_cursor"`
}

// ParticipantResponse is the DTO of a tournament participant. Exactly one of
// UserID/GuestName is set, mirroring tournament_participants' identity CHECK.
type ParticipantResponse struct {
	ID            string  `json:"id"`
	UserID        *string `json:"user_id,omitempty"`
	Username      *string `json:"username,omitempty"`
	GuestName     *string `json:"guest_name,omitempty"`
	DeckID        *string `json:"deck_id,omitempty"`
	DeckName      *string `json:"deck_name,omitempty"`
	CommanderName string  `json:"commander_name"`
	Points        int32   `json:"points"`
}

// SeatResponse is one participant's seat at a table, with their result once
// the organizer records it (FinishPosition/PointsAwarded stay nil/0 until then).
type SeatResponse struct {
	ID             string  `json:"id"`
	ParticipantID  string  `json:"participant_id"`
	UserID         *string `json:"user_id,omitempty"`
	Username       *string `json:"username,omitempty"`
	GuestName      *string `json:"guest_name,omitempty"`
	CommanderName  string  `json:"commander_name"`
	FinishPosition *int32  `json:"finish_position,omitempty"`
	PointsAwarded  int32   `json:"points_awarded"`
}

// TableResponse is one table (pod) within a round.
type TableResponse struct {
	ID          string         `json:"id"`
	TableNumber int32          `json:"table_number"`
	Seats       []SeatResponse `json:"seats"`
}

// RoundResponse is one round of a tournament: its tables and each seat's result.
type RoundResponse struct {
	RoundNumber int32           `json:"round_number"`
	Status      string          `json:"status"`
	Tables      []TableResponse `json:"tables"`
}

// TournamentDetailResponse is the full detail of a tournament: the tournament
// itself, its participants (standings order), and every round played so far.
type TournamentDetailResponse struct {
	Tournament   TournamentResponse    `json:"tournament"`
	Participants []ParticipantResponse `json:"participants"`
	Rounds       []RoundResponse       `json:"rounds,omitempty"`
}

// LookupResponse is the response of GET /tournaments/lookup?code= -- the
// "enter the code in the app" experience: the tournament's public summary,
// plus (if the caller is already a participant) their own status.
type LookupResponse struct {
	Tournament TournamentResponse `json:"tournament"`
	// Participant is the caller's own participant record, nil if they haven't joined.
	Participant *ParticipantResponse `json:"participant,omitempty"`
	// CurrentTable is the caller's table assignment for the tournament's
	// current round, only set once the tournament is in_progress and the
	// caller has a seat in it.
	CurrentTable *TableResponse `json:"current_table,omitempty"`
}
