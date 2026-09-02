package tournaments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
)

const (
	statusRegistration = "registration"
	statusInProgress   = "in_progress"

	// maxJoinCodeAttempts bounds the retry loop against a join_code collision
	// (see the joinCodeCharset/joinCodeLength keyspace in pairing.go -- a
	// collision is already unlikely, this is only a safety net).
	maxJoinCodeAttempts = 5

	tournamentsJoinCodeConstraint              = "tournaments_join_code_idx"
	tournamentParticipantsUniqueUserConstraint = "tournament_participants_unique_user_idx"
)

var (
	// ErrTournamentNotFound indicates the tournament doesn't exist, or (for
	// organizer-only actions) doesn't belong to the caller -- same
	// "don't reveal" pattern as playgroups.getMemberPlaygroup.
	ErrTournamentNotFound = common.NotFound("tournament not found")
	// ErrNameRequired indicates an attempt to create a tournament without a name.
	ErrNameRequired = common.InvalidInput("name is required")
	// ErrGuestNameRequired indicates a guest participant with no name.
	ErrGuestNameRequired = common.InvalidInput("guest_name is required")
	// ErrCommanderNameRequired indicates a guest participant with no commander.
	ErrCommanderNameRequired = common.InvalidInput("commander_name is required")
	// ErrAlreadyJoined indicates the authenticated user already registered for this tournament.
	ErrAlreadyJoined = common.Conflict("already registered in this tournament")
	// ErrTournamentNotOpen indicates an attempt to join a tournament that isn't in 'registration'.
	ErrTournamentNotOpen = common.Conflict("tournament is not open for registration")
	// ErrTournamentNotInRegistration indicates an attempt to start an already-started tournament.
	ErrTournamentNotInRegistration = common.Conflict("tournament has already started")
	// ErrTournamentNotInProgress indicates an action that requires an in-progress
	// tournament (recording a result, advancing a round) attempted at the wrong stage.
	ErrTournamentNotInProgress = common.Conflict("tournament is not in progress")
	// ErrTournamentNotDeletable indicates an attempt to delete a tournament that
	// already started: deleting it would take rounds and results away from every
	// participant, not just the organizer. See Service.DeleteTournament.
	ErrTournamentNotDeletable = common.Conflict("only a tournament still open for registration can be deleted")
	// ErrInvalidParticipantCount indicates a roster size with no valid 3-4
	// player table split (see pairing.go: tableSizes).
	ErrInvalidParticipantCount = common.InvalidInput(
		"invalid number of participants for a 3-4 player Swiss bracket (add or remove one)")
	// ErrTableNotFound indicates the table doesn't exist, or doesn't belong to this tournament.
	ErrTableNotFound = common.NotFound("table not found")
	// ErrInvalidResults indicates the submitted results don't cover exactly the
	// table's seats with a valid finish-position permutation.
	ErrInvalidResults = common.InvalidInput(
		"results must cover exactly this table's seats with a valid finish order")
	// ErrRoundNotComplete indicates an attempt to advance a round before every
	// table in it has a recorded result.
	ErrRoundNotComplete = common.Conflict("not every table in the current round has a recorded result yet")

	// errJoinCodeExhausted wraps a failure to generate a unique join code after
	// maxJoinCodeAttempts tries -- not a domain/client error, an internal one.
	errJoinCodeExhausted = errors.New("could not generate a unique tournament join code")
	// errInvalidTableSizeMidTournament indicates a roster size that stopped
	// having a valid 3-4 table split after the tournament already started --
	// see advanceOrFinish's doc for why this shouldn't be reachable in practice.
	errInvalidTableSizeMidTournament = errors.New("table size distribution became invalid mid-tournament")
)

// DeckLookup is what tournaments needs from the decks module: an
// ownership-checked deck lookup, so a participant can only bring one of their
// own decks (same real logic already used elsewhere, see internal/decks.Service.GetDeck).
type DeckLookup interface {
	GetDeck(ctx context.Context, userID, id string) (*decks.DeckResponse, error)
}

// Service defines the business logic of the tournaments module.
type Service interface {
	CreateTournament(ctx context.Context, organizerID string, req CreateTournamentRequest) (*TournamentResponse, error)
	ListTournaments(ctx context.Context, userID string, page common.PageRequest) (*TournamentListResponse, error)
	GetTournament(ctx context.Context, userID, id string) (*TournamentDetailResponse, error)
	// DeleteTournament deletes a tournament and its roster. Organizer-only, and
	// only while it's still in 'registration' -- the "I created it by mistake"
	// escape hatch, not a way to erase rounds already played.
	DeleteTournament(ctx context.Context, organizerID, id string) error
	// JoinTournament is self-service registration: an authenticated app user
	// registers themselves with one of their own decks (ownership-checked via
	// DeckLookup). Guests can't self-register, see AddGuestParticipant.
	JoinTournament(ctx context.Context, userID string, req JoinTournamentRequest) (*ParticipantResponse, error)
	// AddGuestParticipant is organizer-only: registers a participant with no
	// account (name + commander only, no deck to pick from).
	AddGuestParticipant(
		ctx context.Context, organizerID, tournamentID string, req AddGuestParticipantRequest,
	) (*ParticipantResponse, error)
	// StartTournament locks the roster, computes round_count, and seats round 1
	// (see pairing.go). Organizer-only.
	StartTournament(ctx context.Context, organizerID, id string) (*TournamentDetailResponse, error)
	// RecordTableResult sets a table's finish order and updates the seated
	// participants' points. Organizer-only.
	RecordTableResult(
		ctx context.Context, organizerID, tournamentID, tableID string, req RecordTableResultRequest,
	) (*TableResponse, error)
	// AdvanceRound finishes the current round (once every table has a result)
	// and either seats the next round or finishes the tournament. Organizer-only.
	AdvanceRound(ctx context.Context, organizerID, id string) (*TournamentDetailResponse, error)
	// LookupByCode is the "enter the code in the app" endpoint: the
	// tournament's public summary, plus the caller's own participation status
	// and current table assignment if they're registered.
	LookupByCode(ctx context.Context, userID, code string) (*LookupResponse, error)
}

type service struct {
	repo  *Queries
	pool  *pgxpool.Pool
	decks DeckLookup
}

// NewService creates a new tournaments service.
func NewService(db *pgxpool.Pool, deckLookup DeckLookup) Service {
	return &service{repo: New(db), pool: db, decks: deckLookup}
}

// CreateTournament creates a standalone tournament (not tied to a playgroup)
// with the caller as organizer, and a freshly generated join code.
func (s *service) CreateTournament(
	ctx context.Context, organizerID string, req CreateTournamentRequest,
) (*TournamentResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrNameRequired
	}
	oid, err := common.ParseUUID(organizerID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	var targetPlayers pgtype.Int4
	if req.TargetPlayers != nil {
		targetPlayers = pgtype.Int4{Int32: *req.TargetPlayers, Valid: true}
	}

	t, err := s.createWithUniqueCode(ctx, oid, name, targetPlayers)
	if err != nil {
		return nil, err
	}
	return toTournamentResponse(&t), nil
}

// createWithUniqueCode retries CreateTournament against a fresh random code on
// a join_code collision -- unlikely given the keyspace, but not impossible.
func (s *service) createWithUniqueCode(
	ctx context.Context, organizerID pgtype.UUID, name string, targetPlayers pgtype.Int4,
) (Tournament, error) {
	for range maxJoinCodeAttempts {
		code, err := generateJoinCode()
		if err != nil {
			return Tournament{}, fmt.Errorf("generating join code: %w", err)
		}

		t, err := s.repo.CreateTournament(ctx, CreateTournamentParams{
			OrganizerID: organizerID, Name: name, TargetPlayers: targetPlayers, JoinCode: code,
		})
		if err == nil {
			return t, nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == tournamentsJoinCodeConstraint {
			continue
		}
		return Tournament{}, fmt.Errorf("creating tournament: %w", err)
	}
	return Tournament{}, fmt.Errorf("%w: %d attempts", errJoinCodeExhausted, maxJoinCodeAttempts)
}

// ListTournaments returns a page of tournaments the caller organizes or
// participates in, from most recently created to oldest. See
// internal/common/pagination.go for the cursor scheme.
func (s *service) ListTournaments(
	ctx context.Context, userID string, page common.PageRequest,
) (*TournamentListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	// One row more than the limit is requested: if it comes back, there's a
	// next page. Avoids a separate COUNT(*) just to know whether to keep paginating.
	params := ListTournamentsForUserPageParams{UserID: uid, PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, cursorErr := decodeCursor(page.Cursor)
		if cursorErr != nil {
			return nil, cursorErr
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListTournamentsForUserPage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listing tournaments: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}

	items := make([]TournamentResponse, 0, len(rows))
	for i := range rows {
		items = append(items, *toTournamentResponse(&rows[i]))
	}
	return &TournamentListResponse{Items: items, NextCursor: nextCursor}, nil
}

func decodeCursor(encoded string) (pgtype.Timestamp, pgtype.UUID, error) {
	cursor, err := common.DecodeCursor(encoded)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, err
	}
	cursorID, err := common.ParseUUID(cursor.ID)
	if err != nil {
		return pgtype.Timestamp{}, pgtype.UUID{}, common.ErrInvalidCursor
	}
	return pgtype.Timestamp{Time: cursor.CreatedAt, Valid: true}, cursorID, nil
}

// GetTournament returns the full detail of a tournament: standings and, once
// it's left registration, every round played so far. Visible to its
// organizer and to any registered participant; ErrTournamentNotFound otherwise.
func (s *service) GetTournament(ctx context.Context, userID, id string) (*TournamentDetailResponse, error) {
	tid, err := common.ParseUUID(id)
	if err != nil {
		return nil, ErrTournamentNotFound
	}

	t, err := s.repo.GetTournament(ctx, tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("looking up tournament: %w", err)
	}

	visible, err := s.isVisible(ctx, &t, userID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrTournamentNotFound
	}

	return s.buildDetail(ctx, &t)
}

// isVisible reports whether userID may see t's detail: its organizer, or any registered participant.
func (s *service) isVisible(ctx context.Context, t *Tournament, userID string) (bool, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return false, nil //nolint:nilerr // malformed caller id treated as "not visible", not an error
	}
	if t.OrganizerID == uid {
		return true, nil
	}

	_, err = s.repo.GetParticipantByUser(ctx, GetParticipantByUserParams{TournamentID: t.ID, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("checking participation: %w", err)
	}
	return true, nil
}

func (s *service) buildDetail(ctx context.Context, t *Tournament) (*TournamentDetailResponse, error) {
	participantRows, err := s.repo.ListParticipantsForTournament(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("listing participants: %w", err)
	}
	participants := make([]ParticipantResponse, 0, len(participantRows))
	for i := range participantRows {
		participants = append(participants, toParticipantResponseFromListRow(&participantRows[i]))
	}

	detail := &TournamentDetailResponse{Tournament: *toTournamentResponse(t), Participants: participants}
	if t.Status == statusRegistration {
		return detail, nil
	}

	rounds := make([]RoundResponse, 0, t.CurrentRound)
	for roundNumber := int32(1); roundNumber <= t.CurrentRound; roundNumber++ {
		round, err := s.repo.GetRoundByNumber(ctx, GetRoundByNumberParams{TournamentID: t.ID, RoundNumber: roundNumber})
		if err != nil {
			return nil, fmt.Errorf("looking up round %d: %w", roundNumber, err)
		}
		tables, err := s.buildTables(ctx, round.ID)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, RoundResponse{RoundNumber: round.RoundNumber, Status: round.Status, Tables: tables})
	}
	detail.Rounds = rounds
	return detail, nil
}

func (s *service) buildTables(ctx context.Context, roundID pgtype.UUID) ([]TableResponse, error) {
	seatRows, err := s.repo.ListSeatsForRound(ctx, roundID)
	if err != nil {
		return nil, fmt.Errorf("listing seats: %w", err)
	}

	var tables []TableResponse
	for i := range seatRows {
		row := &seatRows[i]
		if len(tables) == 0 || tables[len(tables)-1].ID != row.TableID.String() {
			tables = append(tables, TableResponse{ID: row.TableID.String(), TableNumber: row.TableNumber})
		}
		tables[len(tables)-1].Seats = append(tables[len(tables)-1].Seats, toSeatResponseFromRoundRow(row))
	}
	return tables, nil
}

// DeleteTournament removes a tournament and its whole roster. Gated to
// 'registration' so it can only undo a mistake: once round 1 is seated the
// tournament isn't only the organizer's any more, and the rows a deletion would
// have to take with it (rounds, tables, seats) are games its participants played.
func (s *service) DeleteTournament(ctx context.Context, organizerID, id string) error {
	t, err := s.getOrganizerTournament(ctx, organizerID, id)
	if err != nil {
		return err
	}
	if t.Status != statusRegistration {
		return ErrTournamentNotDeletable
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.repo.WithTx(tx)

	// No ON DELETE CASCADE anywhere in 00016_tournaments.sql, so the roster goes
	// first -- and if the tournament raced into 'in_progress' between the check
	// above and here, the seats' FK onto tournament_participants rejects this and
	// the transaction rolls back, rather than half-deleting a running tournament.
	if err := q.DeleteParticipantsForTournament(ctx, t.ID); err != nil {
		return fmt.Errorf("deleting participants: %w", err)
	}
	if err := q.DeleteTournament(ctx, t.ID); err != nil {
		return fmt.Errorf("deleting tournament: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tournament deletion: %w", err)
	}
	return nil
}

// JoinTournament is the self-service registration path for an app user: the
// caller's OWN decks are what's offered to pick from (never another user's),
// same ownership check as internal/decks.Service.GetDeck.
func (s *service) JoinTournament(
	ctx context.Context, userID string, req JoinTournamentRequest,
) (*ParticipantResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	t, err := s.repo.GetTournamentByJoinCode(ctx, normalizeJoinCode(req.JoinCode))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("looking up tournament by code: %w", err)
	}
	if t.Status != statusRegistration {
		return nil, ErrTournamentNotOpen
	}

	deck, err := s.decks.GetDeck(ctx, userID, req.DeckID)
	if err != nil {
		return nil, err
	}
	did, err := common.ParseUUID(deck.ID)
	if err != nil {
		return nil, fmt.Errorf("parsing deck id: %w", err)
	}

	p, err := s.repo.CreateParticipant(ctx, CreateParticipantParams{
		TournamentID: t.ID,
		UserID:       uid,
		DeckID:       did,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == tournamentParticipantsUniqueUserConstraint {
			return nil, ErrAlreadyJoined
		}
		return nil, fmt.Errorf("joining tournament: %w", err)
	}

	return &ParticipantResponse{
		ID:            p.ID.String(),
		UserID:        strPtr(uid.String()),
		DeckID:        strPtr(did.String()),
		DeckName:      &deck.Name,
		CommanderName: deck.Commander,
		Points:        p.Points,
	}, nil
}

func normalizeJoinCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

// AddGuestParticipant registers a participant with no account. Organizer-only:
// guests can't self-serve since they can't authenticate.
func (s *service) AddGuestParticipant(
	ctx context.Context, organizerID, tournamentID string, req AddGuestParticipantRequest,
) (*ParticipantResponse, error) {
	guestName := strings.TrimSpace(req.GuestName)
	if guestName == "" {
		return nil, ErrGuestNameRequired
	}
	commanderName := strings.TrimSpace(req.CommanderName)
	if commanderName == "" {
		return nil, ErrCommanderNameRequired
	}

	t, err := s.getOrganizerTournament(ctx, organizerID, tournamentID)
	if err != nil {
		return nil, err
	}
	if t.Status != statusRegistration {
		return nil, ErrTournamentNotOpen
	}

	p, err := s.repo.CreateParticipant(ctx, CreateParticipantParams{
		TournamentID:  t.ID,
		GuestName:     pgtype.Text{String: guestName, Valid: true},
		CommanderName: pgtype.Text{String: commanderName, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("adding guest participant: %w", err)
	}

	return &ParticipantResponse{
		ID:            p.ID.String(),
		GuestName:     &guestName,
		CommanderName: commanderName,
		Points:        p.Points,
	}, nil
}

// getOrganizerTournament resolves a tournament by ID only if organizerID
// organizes it -- doesn't distinguish "doesn't exist" from "not yours", same
// "don't reveal" criteria as the rest of the project.
func (s *service) getOrganizerTournament(ctx context.Context, organizerID, tournamentID string) (*Tournament, error) {
	tid, err := common.ParseUUID(tournamentID)
	if err != nil {
		return nil, ErrTournamentNotFound
	}
	oid, err := common.ParseUUID(organizerID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	t, err := s.repo.GetTournament(ctx, tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("looking up tournament: %w", err)
	}
	if t.OrganizerID != oid {
		return nil, ErrTournamentNotFound
	}
	return &t, nil
}

// StartTournament locks the roster (must be a valid 3-4 player table split,
// see pairing.go: tableSizes), computes round_count, and seats round 1
// (randomized -- everyone's at 0 points, see pairing.go: shuffleParticipants).
func (s *service) StartTournament(ctx context.Context, organizerID, id string) (*TournamentDetailResponse, error) {
	t, err := s.getOrganizerTournament(ctx, organizerID, id)
	if err != nil {
		return nil, err
	}
	if t.Status != statusRegistration {
		return nil, ErrTournamentNotInRegistration
	}

	count, err := s.repo.CountParticipants(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("counting participants: %w", err)
	}
	sizes, ok := tableSizes(int(count))
	if !ok {
		return nil, ErrInvalidParticipantCount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.repo.WithTx(tx)

	//nolint:gosec // bounded by the staircase in roundCount, never approaches int32 overflow
	rc := int32(roundCount(int(count)))
	started, err := q.StartTournament(ctx, StartTournamentParams{
		RoundCount: pgtype.Int4{Int32: rc, Valid: true},
		ID:         t.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("starting tournament: %w", err)
	}

	ranked, err := rankedParticipants(ctx, q, t.ID)
	if err != nil {
		return nil, err
	}
	shuffleParticipants(ranked)

	if err := s.createRoundTables(ctx, q, t.ID, 1, ranked, sizes, nil); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing tournament start: %w", err)
	}
	return s.buildDetail(ctx, &started)
}

func rankedParticipants(ctx context.Context, q *Queries, tournamentID pgtype.UUID) ([]participant, error) {
	rows, err := q.ListParticipantsForTournament(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("listing participants: %w", err)
	}
	ranked := make([]participant, len(rows))
	for i := range rows {
		ranked[i] = participant{id: rows[i].ID.String(), points: rows[i].Points}
	}
	return ranked, nil
}

// createRoundTables creates round roundNumber's tables/seats from ranked
// (already in seeding order) and sizes (see pairing.go: tableSizes), avoiding
// repeats from alreadyPlayed where possible (see pairing.go: pairRound). Runs
// against q so callers can wrap it in their own transaction.
func (s *service) createRoundTables(
	ctx context.Context, q *Queries, tournamentID pgtype.UUID, roundNumber int32,
	ranked []participant, sizes []int, alreadyPlayed map[[2]string]bool,
) error {
	round, err := q.CreateRound(ctx, CreateRoundParams{TournamentID: tournamentID, RoundNumber: roundNumber})
	if err != nil {
		return fmt.Errorf("creating round %d: %w", roundNumber, err)
	}

	for i, seatIDs := range pairRound(ranked, sizes, alreadyPlayed) {
		table, tableErr := q.CreateTable(ctx, CreateTableParams{
			TournamentRoundID: round.ID,
			TableNumber:       int32(i + 1),
		})
		if tableErr != nil {
			return fmt.Errorf("creating table %d of round %d: %w", i+1, roundNumber, tableErr)
		}
		for _, pid := range seatIDs {
			participantUUID, parseErr := common.ParseUUID(pid)
			if parseErr != nil {
				return fmt.Errorf("parsing participant id: %w", parseErr)
			}
			if _, seatErr := q.CreateTableSeat(ctx, CreateTableSeatParams{
				TournamentTableID: table.ID, TournamentParticipantID: participantUUID,
			}); seatErr != nil {
				return fmt.Errorf("seating participant at table %d of round %d: %w", i+1, roundNumber, seatErr)
			}
		}
	}
	return nil
}

// RecordTableResult sets a table's finish order (validated as a permutation
// of 1..len(seats) covering exactly its seats) and adds the corresponding
// points (1st=2, 2nd=1, 3rd/4th=0) to each seated participant.
func (s *service) RecordTableResult(
	ctx context.Context, organizerID, tournamentID, tableID string, req RecordTableResultRequest,
) (*TableResponse, error) {
	t, err := s.getOrganizerTournament(ctx, organizerID, tournamentID)
	if err != nil {
		return nil, err
	}
	if t.Status != statusInProgress {
		return nil, ErrTournamentNotInProgress
	}

	table, seats, positions, err := s.resolveTableResult(ctx, t.ID, tableID, req.Results)
	if err != nil {
		return nil, err
	}

	if err := s.applyTableResult(ctx, seats, positions); err != nil {
		return nil, err
	}
	return s.getTableResponse(ctx, table)
}

// resolveTableResult resolves tableID (scoped to tournamentID) and validates
// results against its seats, without mutating anything yet.
func (s *service) resolveTableResult(
	ctx context.Context, tournamentID pgtype.UUID, tableID string, results []SeatResult,
) (*TournamentTable, []TournamentTableSeat, map[string]int32, error) {
	table, err := s.getTournamentTable(ctx, tournamentID, tableID)
	if err != nil {
		return nil, nil, nil, err
	}

	seats, err := s.repo.ListSeatsForTable(ctx, table.ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing table seats: %w", err)
	}
	positions, err := resultsByParticipant(results, seats)
	if err != nil {
		return nil, nil, nil, err
	}
	return table, seats, positions, nil
}

// applyTableResult records every seat's finish position/points and adds them
// to each participant's running total, atomically.
func (s *service) applyTableResult(ctx context.Context, seats []TournamentTableSeat, positions map[string]int32) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.repo.WithTx(tx)

	for _, seat := range seats {
		position := positions[seat.TournamentParticipantID.String()]
		points := pointsForPosition(position)
		if _, seatErr := q.RecordSeatResult(ctx, RecordSeatResultParams{
			ID:             seat.ID,
			FinishPosition: pgtype.Int4{Int32: position, Valid: true},
			PointsAwarded:  points,
		}); seatErr != nil {
			return fmt.Errorf("recording seat result: %w", seatErr)
		}
		if _, pointsErr := q.AddParticipantPoints(ctx, AddParticipantPointsParams{
			ID: seat.TournamentParticipantID, Delta: points,
		}); pointsErr != nil {
			return fmt.Errorf("updating participant points: %w", pointsErr)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing table result: %w", err)
	}
	return nil
}

// getTournamentTable resolves a table by ID only if it belongs to a round of
// tournamentID -- otherwise an organizer of one tournament could record a
// result against a table of another.
func (s *service) getTournamentTable(
	ctx context.Context, tournamentID pgtype.UUID, tableID string,
) (*TournamentTable, error) {
	tid, err := common.ParseUUID(tableID)
	if err != nil {
		return nil, ErrTableNotFound
	}
	table, err := s.repo.GetTable(ctx, tid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTableNotFound
		}
		return nil, fmt.Errorf("looking up table: %w", err)
	}

	round, err := s.repo.GetRound(ctx, table.TournamentRoundID)
	if err != nil {
		return nil, fmt.Errorf("looking up table's round: %w", err)
	}
	if round.TournamentID != tournamentID {
		return nil, ErrTableNotFound
	}
	return &table, nil
}

// resultsByParticipant validates that results covers exactly seats with a
// finish-position permutation of 1..len(seats) (no gaps, no repeats),
// returning the finish position keyed by participant ID.
func resultsByParticipant(results []SeatResult, seats []TournamentTableSeat) (map[string]int32, error) {
	if len(results) != len(seats) {
		return nil, ErrInvalidResults
	}

	seated := make(map[string]bool, len(seats))
	for _, seat := range seats {
		seated[seat.TournamentParticipantID.String()] = true
	}

	positions := make(map[string]int32, len(results))
	seenPositions := make(map[int32]bool, len(results))
	for _, r := range results {
		if !seated[r.ParticipantID] {
			return nil, ErrInvalidResults
		}
		if _, dup := positions[r.ParticipantID]; dup {
			return nil, ErrInvalidResults
		}
		if r.FinishPosition < 1 || int(r.FinishPosition) > len(seats) || seenPositions[r.FinishPosition] {
			return nil, ErrInvalidResults
		}
		seenPositions[r.FinishPosition] = true
		positions[r.ParticipantID] = r.FinishPosition
	}
	return positions, nil
}

const (
	firstPlacePosition  = 1
	secondPlacePosition = 2

	winnerPoints   = 2
	runnerUpPoints = 1
)

// pointsForPosition maps a table finish position to tournament points: 1st=2,
// 2nd=1, 3rd/4th=0.
func pointsForPosition(position int32) int32 {
	switch position {
	case firstPlacePosition:
		return winnerPoints
	case secondPlacePosition:
		return runnerUpPoints
	default:
		return 0
	}
}

func (s *service) getTableResponse(ctx context.Context, table *TournamentTable) (*TableResponse, error) {
	seatRows, err := s.repo.ListEnrichedSeatsForTable(ctx, table.ID)
	if err != nil {
		return nil, fmt.Errorf("listing seats: %w", err)
	}
	resp := &TableResponse{ID: table.ID.String(), TableNumber: table.TableNumber}
	for i := range seatRows {
		resp.Seats = append(resp.Seats, toSeatResponseFromTableRow(&seatRows[i]))
	}
	return resp, nil
}

// AdvanceRound finishes the current round -- gated on every table already
// having a recorded result -- and either seats the next round (from updated
// standings, avoiding repeat opponents where possible) or, if this was the
// last round, finishes the tournament.
func (s *service) AdvanceRound(ctx context.Context, organizerID, id string) (*TournamentDetailResponse, error) {
	t, err := s.getOrganizerTournament(ctx, organizerID, id)
	if err != nil {
		return nil, err
	}
	if t.Status != statusInProgress {
		return nil, ErrTournamentNotInProgress
	}

	round, err := s.repo.GetRoundByNumber(ctx, GetRoundByNumberParams{TournamentID: t.ID, RoundNumber: t.CurrentRound})
	if err != nil {
		return nil, fmt.Errorf("looking up current round: %w", err)
	}
	unrecorded, err := s.repo.CountUnrecordedSeatsInRound(ctx, round.ID)
	if err != nil {
		return nil, fmt.Errorf("counting unrecorded seats: %w", err)
	}
	if unrecorded > 0 {
		return nil, ErrRoundNotComplete
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.repo.WithTx(tx)

	if _, finishErr := q.FinishRound(ctx, round.ID); finishErr != nil {
		return nil, fmt.Errorf("finishing round: %w", finishErr)
	}

	final, err := s.advanceOrFinish(ctx, q, t)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing round advance: %w", err)
	}
	return s.buildDetail(ctx, final)
}

// advanceOrFinish is AdvanceRound's second half, split out to keep both
// branches (finish vs. seat the next round) readable. t.RoundCount is
// guaranteed valid here: it's only set (by StartTournament) together with
// status='in_progress', which the caller already checked.
func (s *service) advanceOrFinish(ctx context.Context, q *Queries, t *Tournament) (*Tournament, error) {
	if t.CurrentRound >= t.RoundCount.Int32 {
		finished, err := q.FinishTournament(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("finishing tournament: %w", err)
		}
		return &finished, nil
	}

	ranked, err := rankedParticipants(ctx, q, t.ID)
	if err != nil {
		return nil, err
	}
	// Already sorted points DESC, created_at ASC by the query -- no shuffle
	// needed here, unlike round 1 (see StartTournament).

	sizes, ok := tableSizes(len(ranked))
	if !ok {
		// Can't happen in practice: the roster is locked at StartTournament and
		// participants are never added or removed once a tournament is in progress.
		return nil, fmt.Errorf("%w: %d participants", errInvalidTableSizeMidTournament, len(ranked))
	}

	pastSeatRows, err := q.ListPastSeatsForTournament(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("listing past seats: %w", err)
	}

	nextRound := t.CurrentRound + 1
	played := buildAlreadyPlayedSet(pastSeatRows)
	if pairErr := s.createRoundTables(ctx, q, t.ID, nextRound, ranked, sizes, played); pairErr != nil {
		return nil, pairErr
	}

	advanced, err := q.AdvanceTournamentRound(ctx, t.ID)
	if err != nil {
		return nil, fmt.Errorf("advancing round: %w", err)
	}
	return &advanced, nil
}

// buildAlreadyPlayedSet turns every past (table, participant) seat into the
// set of participant pairs that have already shared a table, for pairRound's
// repeat-avoidance.
func buildAlreadyPlayedSet(rows []ListPastSeatsForTournamentRow) map[[2]string]bool {
	byTable := make(map[string][]string)
	for _, row := range rows {
		tableID := row.TournamentTableID.String()
		byTable[tableID] = append(byTable[tableID], row.TournamentParticipantID.String())
	}

	played := make(map[[2]string]bool)
	for _, seated := range byTable {
		for i := range seated {
			for j := i + 1; j < len(seated); j++ {
				played[pairKey(seated[i], seated[j])] = true
			}
		}
	}
	return played
}

// LookupByCode is the "enter the code in the app" endpoint: resolves a
// tournament by its join code and, if the caller is already registered,
// their own participation status and current table assignment.
func (s *service) LookupByCode(ctx context.Context, userID, code string) (*LookupResponse, error) {
	t, err := s.repo.GetTournamentByJoinCode(ctx, normalizeJoinCode(code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("looking up tournament by code: %w", err)
	}
	resp := &LookupResponse{Tournament: *toTournamentResponse(&t)}

	uid, err := common.ParseUUID(userID)
	if err != nil {
		return resp, nil //nolint:nilerr // malformed caller id: show the public summary only
	}

	row, found, err := s.findParticipant(ctx, t.ID, uid)
	if err != nil {
		return nil, err
	}
	if !found {
		return resp, nil
	}
	participantResp := toParticipantResponseFromGetRow(&row)
	resp.Participant = &participantResp

	if t.Status != statusInProgress {
		return resp, nil
	}

	table, found, err := s.findCurrentTable(ctx, &t, row.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return resp, nil
	}

	tableResp, err := s.getTableResponse(ctx, table)
	if err != nil {
		return nil, err
	}
	resp.CurrentTable = tableResp
	return resp, nil
}

// findParticipant looks up userID's participant record in tournamentID.
// found is false (with a nil error) if they're simply not registered.
func (s *service) findParticipant(
	ctx context.Context, tournamentID, userID pgtype.UUID,
) (GetParticipantByUserRow, bool, error) {
	row, err := s.repo.GetParticipantByUser(ctx, GetParticipantByUserParams{TournamentID: tournamentID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GetParticipantByUserRow{}, false, nil
		}
		return GetParticipantByUserRow{}, false, fmt.Errorf("checking participation: %w", err)
	}
	return row, true, nil
}

// findCurrentTable looks up participantID's table assignment in t's current
// round. found is false (with a nil error) if they don't have one yet.
func (s *service) findCurrentTable(
	ctx context.Context, t *Tournament, participantID pgtype.UUID,
) (*TournamentTable, bool, error) {
	round, err := s.repo.GetRoundByNumber(ctx, GetRoundByNumberParams{TournamentID: t.ID, RoundNumber: t.CurrentRound})
	if err != nil {
		return nil, false, fmt.Errorf("looking up current round: %w", err)
	}

	table, err := s.repo.GetTableForParticipantInRound(ctx, GetTableForParticipantInRoundParams{
		TournamentRoundID: round.ID, TournamentParticipantID: participantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("looking up current table: %w", err)
	}
	return &table, true, nil
}

func toTournamentResponse(t *Tournament) *TournamentResponse {
	return &TournamentResponse{
		ID:            t.ID.String(),
		OrganizerID:   t.OrganizerID.String(),
		Name:          t.Name,
		Format:        t.Format,
		TargetPlayers: optionalInt32(t.TargetPlayers),
		Status:        t.Status,
		RoundCount:    optionalInt32(t.RoundCount),
		CurrentRound:  t.CurrentRound,
		JoinCode:      t.JoinCode,
		CreatedAt:     t.CreatedAt.Time.Format(time.RFC3339),
		StartedAt:     optionalTimestamp(t.StartedAt),
		FinishedAt:    optionalTimestamp(t.FinishedAt),
	}
}

func toParticipantResponseFromListRow(row *ListParticipantsForTournamentRow) ParticipantResponse {
	return ParticipantResponse{
		ID:            row.ID.String(),
		UserID:        optionalUUIDString(row.UserID),
		Username:      optionalString(row.Username),
		GuestName:     optionalString(row.GuestName),
		DeckID:        optionalUUIDString(row.DeckID),
		DeckName:      optionalString(row.DeckName),
		CommanderName: participantCommander(row.DeckCommander, row.CommanderName),
		Points:        row.Points,
	}
}

func toParticipantResponseFromGetRow(row *GetParticipantByUserRow) ParticipantResponse {
	return ParticipantResponse{
		ID:            row.ID.String(),
		UserID:        optionalUUIDString(row.UserID),
		Username:      optionalString(row.Username),
		GuestName:     optionalString(row.GuestName),
		DeckID:        optionalUUIDString(row.DeckID),
		DeckName:      optionalString(row.DeckName),
		CommanderName: participantCommander(row.DeckCommander, row.CommanderName),
		Points:        row.Points,
	}
}

func toSeatResponseFromRoundRow(row *ListSeatsForRoundRow) SeatResponse {
	return SeatResponse{
		ID:             row.SeatID.String(),
		ParticipantID:  row.TournamentParticipantID.String(),
		UserID:         optionalUUIDString(row.UserID),
		Username:       optionalString(row.Username),
		GuestName:      optionalString(row.GuestName),
		CommanderName:  participantCommander(row.DeckCommander, row.ParticipantCommanderName),
		FinishPosition: optionalInt32(row.FinishPosition),
		PointsAwarded:  row.PointsAwarded,
	}
}

func toSeatResponseFromTableRow(row *ListEnrichedSeatsForTableRow) SeatResponse {
	return SeatResponse{
		ID:             row.SeatID.String(),
		ParticipantID:  row.TournamentParticipantID.String(),
		UserID:         optionalUUIDString(row.UserID),
		Username:       optionalString(row.Username),
		GuestName:      optionalString(row.GuestName),
		CommanderName:  participantCommander(row.DeckCommander, row.ParticipantCommanderName),
		FinishPosition: optionalInt32(row.FinishPosition),
		PointsAwarded:  row.PointsAwarded,
	}
}

// participantCommander prefers the linked deck's commander (kept in sync with
// the deck itself) and falls back to the free-text commander_name (guests, or
// a registered user without a linked deck).
func participantCommander(deckCommander, commanderName pgtype.Text) string {
	if deckCommander.Valid {
		return deckCommander.String
	}
	if commanderName.Valid {
		return commanderName.String
	}
	return ""
}

func optionalString(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func optionalUUIDString(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	return strPtr(u.String())
}

func optionalInt32(i pgtype.Int4) *int32 {
	if !i.Valid {
		return nil
	}
	return &i.Int32
}

func optionalTimestamp(t pgtype.Timestamp) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

func strPtr(s string) *string { return &s }
