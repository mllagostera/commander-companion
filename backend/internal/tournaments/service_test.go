package tournaments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/tournaments"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const (
	testPassword  = "correct-horse-battery-staple"
	testGuestName = "Sara"
)

// errFakeMoxfieldNotUsed is a static sentinel (not an inline errors.New/fmt.Errorf
// at the return site) so fakeMoxfieldClient.GetDeck satisfies err113.
var errFakeMoxfieldNotUsed = errors.New("not used in tournaments tests")

func truncateTournamentTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// "tournaments" cascades to tournament_participants/rounds/tables/table_seats;
	// "decks" and "users" clean up any remainder.
	testutil.Truncate(t, pool, "tournaments", "decks", "users")
}

// fakeMoxfieldClient is never actually exercised by these tests (decks are
// created directly via decks.Service.CreateDeck, not imported from Moxfield),
// just needed to satisfy decks.NewService's constructor.
type fakeMoxfieldClient struct{}

func (fakeMoxfieldClient) GetDeck(context.Context, string) (*moxfield.Deck, error) {
	return nil, errFakeMoxfieldNotUsed
}

func newTestServices(pool *pgxpool.Pool) (tSvc tournaments.Service, decksSvc decks.Service, usersSvc users.Service) {
	decksSvc = decks.NewService(pool, fakeMoxfieldClient{})
	usersSvc = testutil.NewUsersService(pool)
	tSvc = tournaments.NewService(pool, decksSvc)
	return tSvc, decksSvc, usersSvc
}

func createTestUser(t *testing.T, usersSvc users.Service, email string) *users.UserResponse {
	t.Helper()
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email, Email: email, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	return user
}

func createTestDeck(t *testing.T, decksSvc decks.Service, userID, name string) *decks.DeckResponse {
	t.Helper()
	deck, err := decksSvc.CreateDeck(context.Background(), userID, decks.CreateDeckRequest{
		Name: name, Commander: name + " Commander",
	})
	if err != nil {
		t.Fatalf("creando mazo de test: %v", err)
	}
	return deck
}

// joinAsUser registers a fresh user with their own deck and joins the
// tournament via its join code -- the common setup for most tests below.
func joinAsUser(
	t *testing.T, tSvc tournaments.Service, decksSvc decks.Service, usersSvc users.Service, joinCode, emailPrefix string,
) (*users.UserResponse, *tournaments.ParticipantResponse) {
	t.Helper()
	user := createTestUser(t, usersSvc, emailPrefix+"@example.com")
	deck := createTestDeck(t, decksSvc, user.ID, emailPrefix+" Deck")
	p, err := tSvc.JoinTournament(context.Background(), user.ID, tournaments.JoinTournamentRequest{
		JoinCode: joinCode, DeckID: deck.ID,
	})
	if err != nil {
		t.Fatalf("JoinTournament(%s) error = %v", emailPrefix, err)
	}
	return user, p
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

func TestCreateTournament_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, _, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "organizer@example.com")

	res, err := tSvc.CreateTournament(
		context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "Viernes EDH"},
	)
	if err != nil {
		t.Fatalf("CreateTournament() error = %v, want nil", err)
	}
	if res.Name != "Viernes EDH" {
		t.Fatalf("Name = %q, want %q", res.Name, "Viernes EDH")
	}
	if res.Status != "registration" {
		t.Fatalf("Status = %q, want %q", res.Status, "registration")
	}
	if res.Format != "commander" {
		t.Fatalf("Format = %q, want %q", res.Format, "commander")
	}
	if len(res.JoinCode) != 6 {
		t.Fatalf("JoinCode = %q, want length 6", res.JoinCode)
	}
	if res.OrganizerID != organizer.ID {
		t.Fatalf("OrganizerID = %q, want %q", res.OrganizerID, organizer.ID)
	}
}

func TestCreateTournament_NoName_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, _, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "noname@example.com")

	_, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "   "})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("CreateTournament() sin nombre: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestJoinTournament_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "join-organizer@example.com")
	tour, err := tSvc.CreateTournament(
		context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "Join test"},
	)
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}

	_, p := joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "joiner")
	if p.CommanderName != "joiner Deck Commander" {
		t.Fatalf("CommanderName = %q, want %q", p.CommanderName, "joiner Deck Commander")
	}
	if p.UserID == nil {
		t.Fatalf("UserID = nil, want set")
	}
	if p.GuestName != nil {
		t.Fatalf("GuestName = %v, want nil (app user, not a guest)", p.GuestName)
	}
}

func TestJoinTournament_InvalidCode_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	user := createTestUser(t, usersSvc, "badcode@example.com")
	deck := createTestDeck(t, decksSvc, user.ID, "Deck")

	_, err := tSvc.JoinTournament(context.Background(), user.ID, tournaments.JoinTournamentRequest{
		JoinCode: "NOPE00", DeckID: deck.ID,
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("JoinTournament() con código inválido: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestJoinTournament_OtherUsersDeck_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "deckowner-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}

	deckOwner := createTestUser(t, usersSvc, "deckowner@example.com")
	othersDeck := createTestDeck(t, decksSvc, deckOwner.ID, "Not yours")
	joiner := createTestUser(t, usersSvc, "not-the-owner@example.com")

	_, err = tSvc.JoinTournament(context.Background(), joiner.ID, tournaments.JoinTournamentRequest{
		JoinCode: tour.JoinCode, DeckID: othersDeck.ID,
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("JoinTournament() con mazo ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestJoinTournament_AlreadyJoined_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "dup-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	user, _ := joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "dup")

	secondDeck := createTestDeck(t, decksSvc, user.ID, "Second Deck")
	_, err = tSvc.JoinTournament(context.Background(), user.ID, tournaments.JoinTournamentRequest{
		JoinCode: tour.JoinCode, DeckID: secondDeck.ID,
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("JoinTournament() ya registrado: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestAddGuestParticipant_Success(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, _, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "guest-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}

	p, err := tSvc.AddGuestParticipant(context.Background(), organizer.ID, tour.ID, tournaments.AddGuestParticipantRequest{
		GuestName: testGuestName, CommanderName: "Meren of Clan Nel Toth",
	})
	if err != nil {
		t.Fatalf("AddGuestParticipant() error = %v", err)
	}
	if p.GuestName == nil || *p.GuestName != testGuestName {
		t.Fatalf("GuestName = %v, want %q", p.GuestName, testGuestName)
	}
	if p.CommanderName != "Meren of Clan Nel Toth" {
		t.Fatalf("CommanderName = %q, want %q", p.CommanderName, "Meren of Clan Nel Toth")
	}
	if p.UserID != nil {
		t.Fatalf("UserID = %v, want nil (guest, no account)", p.UserID)
	}
}

func TestAddGuestParticipant_NotOrganizer_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, _, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "real-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	stranger := createTestUser(t, usersSvc, "stranger@example.com")

	_, err = tSvc.AddGuestParticipant(context.Background(), stranger.ID, tour.ID, tournaments.AddGuestParticipantRequest{
		GuestName: testGuestName, CommanderName: "Meren",
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("AddGuestParticipant() de un no-organizador: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestStartTournament_InvalidParticipantCount_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "toofew-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	// Only 2 participants: below the 3-player minimum for any table.
	joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "toofew-a")
	joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "toofew-b")

	_, err = tSvc.StartTournament(context.Background(), organizer.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("StartTournament() con 2 participantes: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestStartTournament_FiveParticipants_ReturnsBadRequest(t *testing.T) {
	// 5 has no valid split into tables of 3-4 (see pairing.go: tableSizes) --
	// the one gap for n >= 3.
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "five-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 5 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "five-"+string(rune('a'+i)))
	}

	_, err = tSvc.StartTournament(context.Background(), organizer.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("StartTournament() con 5 participantes: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestStartTournament_Success_SeatsRoundOne(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "start-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 4 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "start-"+string(rune('a'+i)))
	}

	detail, err := tSvc.StartTournament(context.Background(), organizer.ID, tour.ID)
	if err != nil {
		t.Fatalf("StartTournament() error = %v, want nil", err)
	}
	assertRoundOneSeated(t, detail)
}

func assertRoundOneSeated(t *testing.T, detail *tournaments.TournamentDetailResponse) {
	t.Helper()
	if detail.Tournament.Status != "in_progress" {
		t.Fatalf("Status = %q, want %q", detail.Tournament.Status, "in_progress")
	}
	if detail.Tournament.RoundCount == nil || *detail.Tournament.RoundCount != 3 {
		t.Fatalf("RoundCount = %v, want 3 (n=4 falls in the <=8 bracket)", detail.Tournament.RoundCount)
	}
	if detail.Tournament.CurrentRound != 1 {
		t.Fatalf("CurrentRound = %d, want 1", detail.Tournament.CurrentRound)
	}
	if len(detail.Rounds) != 1 || len(detail.Rounds[0].Tables) != 1 || len(detail.Rounds[0].Tables[0].Seats) != 4 {
		t.Fatalf("Rounds = %+v, want exactly 1 round with 1 table of 4 seats", detail.Rounds)
	}
}

func TestStartTournament_NotOrganizer_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "start-real-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 3 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "start-ns-"+string(rune('a'+i)))
	}
	stranger := createTestUser(t, usersSvc, "start-stranger@example.com")

	_, err = tSvc.StartTournament(context.Background(), stranger.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("StartTournament() de un no-organizador: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

// TestFullTournamentLifecycle_ThreeParticipants_ReachesFinished plays out an
// entire 3-participant tournament (one 3-seat table every round, round_count=3
// since n<=8) end to end: join, start, record a result and advance the round
// three times, and asserts the tournament reaches 'finished' with the right
// cumulative points.
func TestFullTournamentLifecycle_ThreeParticipants_ReachesFinished(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "lifecycle-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 3 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "lifecycle-"+string(rune('a'+i)))
	}

	detail, err := tSvc.StartTournament(context.Background(), organizer.ID, tour.ID)
	if err != nil {
		t.Fatalf("StartTournament() error = %v", err)
	}
	if *detail.Tournament.RoundCount != 3 {
		t.Fatalf("RoundCount = %d, want 3", *detail.Tournament.RoundCount)
	}

	for round := 1; round <= 3; round++ {
		detail = recordSingleTableResultAndAdvance(t, tSvc, organizer.ID, tour.ID, detail, round)
	}

	if detail.Tournament.Status != "finished" {
		t.Fatalf("final Status = %q, want %q", detail.Tournament.Status, "finished")
	}
	if detail.Tournament.FinishedAt == nil {
		t.Fatalf("FinishedAt = nil, want set")
	}

	// Every participant took 1st once (across 3 rounds) since the seat order
	// never changes seat-to-participant mapping between rounds' table[0] in this
	// test loop -- the first participant listed in Seats always finishes 1st,
	// so exactly one of the three should end with 3*2=6 points (always 1st) and
	// the others with points from 2nd/3rd. Instead of asserting exact per-participant
	// totals (which depend on the pairing/seat order internals), assert the
	// invariant that matters: total points awarded across all participants is
	// 3 rounds * (2+1+0) = 9.
	assertTotalPointsAwarded(t, tSvc, organizer.ID, tour.ID, 9)
}

// recordSingleTableResultAndAdvance records the current round's single table
// (a 3-participant tournament always has exactly one) with seats finishing in
// listed order, then advances to the next round.
func recordSingleTableResultAndAdvance(
	t *testing.T, tSvc tournaments.Service, organizerID, tournamentID string,
	detail *tournaments.TournamentDetailResponse, round int,
) *tournaments.TournamentDetailResponse {
	t.Helper()
	table := detail.Rounds[len(detail.Rounds)-1].Tables[0]
	if len(table.Seats) != 3 {
		t.Fatalf("round %d table has %d seats, want 3", round, len(table.Seats))
	}

	results := make([]tournaments.SeatResult, len(table.Seats))
	for i, seat := range table.Seats {
		results[i] = tournaments.SeatResult{ParticipantID: seat.ParticipantID, FinishPosition: int32(i + 1)}
	}
	if _, err := tSvc.RecordTableResult(
		context.Background(), organizerID, tournamentID, table.ID, tournaments.RecordTableResultRequest{Results: results},
	); err != nil {
		t.Fatalf("RecordTableResult() round %d: error = %v", round, err)
	}

	next, err := tSvc.AdvanceRound(context.Background(), organizerID, tournamentID)
	if err != nil {
		t.Fatalf("AdvanceRound() round %d: error = %v", round, err)
	}
	return next
}

func assertTotalPointsAwarded(t *testing.T, tSvc tournaments.Service, organizerID, tournamentID string, want int32) {
	t.Helper()
	final, err := tSvc.GetTournament(context.Background(), organizerID, tournamentID)
	if err != nil {
		t.Fatalf("GetTournament() error = %v", err)
	}
	var total int32
	for _, p := range final.Participants {
		total += p.Points
	}
	if total != want {
		t.Fatalf("sum of participants' points = %d, want %d", total, want)
	}
}

func TestAdvanceRound_IncompleteRound_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "incomplete-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 3 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "incomplete-"+string(rune('a'+i)))
	}
	if _, err = tSvc.StartTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("StartTournament() error = %v", err)
	}

	_, err = tSvc.AdvanceRound(context.Background(), organizer.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("AdvanceRound() sin resultados: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
}

func TestRecordTableResult_InvalidPermutation_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "badresult-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 3 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "badresult-"+string(rune('a'+i)))
	}
	detail, err := tSvc.StartTournament(context.Background(), organizer.ID, tour.ID)
	if err != nil {
		t.Fatalf("StartTournament() error = %v", err)
	}
	table := detail.Rounds[0].Tables[0]

	// Duplicate finish_position (two seats both claim 1st).
	results := []tournaments.SeatResult{
		{ParticipantID: table.Seats[0].ParticipantID, FinishPosition: 1},
		{ParticipantID: table.Seats[1].ParticipantID, FinishPosition: 1},
		{ParticipantID: table.Seats[2].ParticipantID, FinishPosition: 3},
	}
	_, err = tSvc.RecordTableResult(
		context.Background(), organizer.ID, tour.ID, table.ID, tournaments.RecordTableResultRequest{Results: results},
	)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("RecordTableResult() con posiciones duplicadas: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestGetTournament_VisibleToParticipant_NotToStranger(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "visibility-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	participant, _ := joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "visibility-p")
	stranger := createTestUser(t, usersSvc, "visibility-stranger@example.com")

	if _, err = tSvc.GetTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("GetTournament() del organizador: error = %v, want nil", err)
	}
	if _, err = tSvc.GetTournament(context.Background(), participant.ID, tour.ID); err != nil {
		t.Fatalf("GetTournament() de un participante: error = %v, want nil", err)
	}
	_, err = tSvc.GetTournament(context.Background(), stranger.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetTournament() de un desconocido: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestListTournaments_OrganizedAndParticipating(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "list-organizer@example.com")
	organized, err := tSvc.CreateTournament(
		context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "Organized"},
	)
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}

	other := createTestUser(t, usersSvc, "list-other-organizer@example.com")
	joined, err := tSvc.CreateTournament(
		context.Background(), other.ID, tournaments.CreateTournamentRequest{Name: "Joined"},
	)
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	deck := createTestDeck(t, decksSvc, organizer.ID, "Deck")
	if _, err = tSvc.JoinTournament(context.Background(), organizer.ID, tournaments.JoinTournamentRequest{
		JoinCode: joined.JoinCode, DeckID: deck.ID,
	}); err != nil {
		t.Fatalf("JoinTournament() error = %v", err)
	}

	notInvolved := createTestUser(t, usersSvc, "list-not-involved@example.com")
	unrelatedReq := tournaments.CreateTournamentRequest{Name: "Unrelated"}
	if _, err = tSvc.CreateTournament(context.Background(), notInvolved.ID, unrelatedReq); err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}

	page, err := tSvc.ListTournaments(
		context.Background(), organizer.ID, common.PageRequest{Limit: common.DefaultPageLimit},
	)
	if err != nil {
		t.Fatalf("ListTournaments() error = %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("ListTournaments() = %d tournaments, want 2 (organized + joined, not the unrelated one)", len(page.Items))
	}
	names := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		names = append(names, item.Name)
	}
	if !containsAll(names, organized.Name, joined.Name) {
		t.Fatalf("ListTournaments() names = %v, want to include %q and %q", names, organized.Name, joined.Name)
	}
}

func TestDeleteTournament_Success_TakesRosterWithIt(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "delete-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	// Both kinds of participant: the roster is what the deletion has to clear
	// before the tournaments row can go (no ON DELETE CASCADE in the schema).
	joiner, _ := joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "delete-joiner")
	if _, err = tSvc.AddGuestParticipant(context.Background(), organizer.ID, tour.ID,
		tournaments.AddGuestParticipantRequest{GuestName: testGuestName, CommanderName: "Meren"},
	); err != nil {
		t.Fatalf("AddGuestParticipant() error = %v", err)
	}

	if err = tSvc.DeleteTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("DeleteTournament() error = %v, want nil", err)
	}

	_, err = tSvc.GetTournament(context.Background(), organizer.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetTournament() tras borrar: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
	page, err := tSvc.ListTournaments(context.Background(), joiner.ID, common.PageRequest{Limit: common.DefaultPageLimit})
	if err != nil {
		t.Fatalf("ListTournaments() error = %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("ListTournaments() del participante = %d torneos, want 0 (el torneo se borró)", len(page.Items))
	}
}

func TestDeleteTournament_NotOrganizer_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "delete-real-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	// A registered participant, not just a stranger: being in the tournament
	// lets you see it, never delete it.
	participant, _ := joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "delete-participant")

	err = tSvc.DeleteTournament(context.Background(), participant.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("DeleteTournament() de un participante: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
	if _, err = tSvc.GetTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("GetTournament() tras el borrado rechazado: error = %v, want nil (sigue existiendo)", err)
	}
}

func TestDeleteTournament_AlreadyStarted_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateTournamentTables(t, pool)
	tSvc, decksSvc, usersSvc := newTestServices(pool)
	organizer := createTestUser(t, usersSvc, "delete-started-organizer@example.com")
	tour, err := tSvc.CreateTournament(context.Background(), organizer.ID, tournaments.CreateTournamentRequest{Name: "T"})
	if err != nil {
		t.Fatalf("CreateTournament() error = %v", err)
	}
	for i := range 3 {
		joinAsUser(t, tSvc, decksSvc, usersSvc, tour.JoinCode, "delete-started-"+string(rune('a'+i)))
	}
	if _, err = tSvc.StartTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("StartTournament() error = %v", err)
	}

	err = tSvc.DeleteTournament(context.Background(), organizer.ID, tour.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("DeleteTournament() ya iniciado: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}
	if _, err = tSvc.GetTournament(context.Background(), organizer.ID, tour.ID); err != nil {
		t.Fatalf("GetTournament() tras el borrado rechazado: error = %v, want nil (sigue existiendo)", err)
	}
}

func containsAll(haystack []string, needles ...string) bool {
	for _, needle := range needles {
		found := false
		for _, h := range haystack {
			if h == needle {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
