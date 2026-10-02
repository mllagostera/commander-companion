package decks_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

// Deck names for the trait fixtures.
const (
	manualDeckName = "Manual"
	deckAzorius    = "azorius"
	deckMonoBlue   = "mono-blue"
	deckEsper      = "esper"
	deckColorless  = "colorless"
)

func intPtr(v int) *int { return &v }

func colors(identity []string) string {
	if identity == nil {
		return "<unknown>"
	}
	return strings.Join(identity, "")
}

func traitsDeck(publicID string, bracket *int, identity []string) *moxfield.Deck {
	return &moxfield.Deck{
		PublicID: publicID, Name: deckNameFixture, Commander: deckCommanderFixture,
		Bracket: bracket, ColorIdentity: identity,
	}
}

func mustUpdateDeck(
	t *testing.T, svc decks.Service, userID, deckID string, req decks.UpdateDeckRequest,
) *decks.DeckResponse {
	t.Helper()
	deck, err := svc.UpdateDeck(context.Background(), userID, deckID, req)
	if err != nil {
		t.Fatalf("UpdateDeck(%+v) error = %v", req, err)
	}
	return deck
}

func TestImportFromMoxfield_StoresBracketAndColorIdentity(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-import@example.com")

	_, deck := importDeck(t, pool, owner.ID, &mockMoxfieldClient{
		deck: traitsDeck("traits1", intPtr(2), []string{"W", "U", "B", "R"}),
	})

	if deck.Bracket == nil || *deck.Bracket != 2 || colors(deck.ColorIdentity) != "WUBR" {
		t.Fatalf("imported deck = bracket %v colors %s, want 2 and WUBR", deck.Bracket, colors(deck.ColorIdentity))
	}
	if deck.BracketOverridden || deck.ColorIdentityOverridden {
		t.Fatal("imported deck is flagged as overridden, want Moxfield-owned values")
	}
}

func TestCreateDeck_WithTraits_IsOverriddenAndNormalized(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-create@example.com")
	svc := newDecksSvc(pool, nil)

	deck, err := svc.CreateDeck(context.Background(), owner.ID, &decks.CreateDeckRequest{
		Name: manualDeckName, Commander: testCommander, Bracket: intPtr(3), ColorIdentity: []string{"g", "B", "R", "b"},
	})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v", err)
	}
	if colors(deck.ColorIdentity) != "BRG" || !deck.BracketOverridden || !deck.ColorIdentityOverridden {
		t.Fatalf("CreateDeck() = %+v, want colors BRG in WUBRG order and both flagged overridden", deck)
	}

	_, err = svc.CreateDeck(context.Background(), owner.ID, &decks.CreateDeckRequest{
		Name: manualDeckName, Commander: testCommander, Bracket: intPtr(6),
	})
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("CreateDeck(bracket 6) code = %d, want 400", fiberErr.Code)
	}
}

func TestResyncFromMoxfield_KeepsOverriddenTraits(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-resync@example.com")

	mox := &mockMoxfieldClient{deck: traitsDeck("traits2", intPtr(2), []string{"U"})}
	svc, deck := importDeck(t, pool, owner.ID, mox)

	// The user pins the bracket by hand; the color identity stays Moxfield's.
	mustUpdateDeck(t, svc, owner.ID, deck.ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("4")})

	mox.deck = traitsDeck("traits2", intPtr(3), []string{"U", "R"})
	state, err := svc.ResyncFromMoxfield(context.Background(), owner.ID, "traits2")
	if err != nil {
		t.Fatalf("ResyncFromMoxfield() error = %v", err)
	}
	if *state.Deck.Bracket != 4 {
		t.Fatalf("bracket after resync = %d, want 4 (set by hand, kept)", *state.Deck.Bracket)
	}
	if colors(state.Deck.ColorIdentity) != "UR" {
		t.Fatalf("colors after resync = %s, want UR (from Moxfield)", colors(state.Deck.ColorIdentity))
	}
	if !state.Changed {
		t.Fatal("Changed = false, want true (the color identity changed)")
	}
}

func TestUpdateDeck_ClearingCountsAsSetByHand(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-clear@example.com")

	svc, deck := importDeck(t, pool, owner.ID, &mockMoxfieldClient{deck: traitsDeck("traits3", intPtr(2), []string{"W"})})
	updated := mustUpdateDeck(t, svc, owner.ID, deck.ID, decks.UpdateDeckRequest{
		Bracket: json.RawMessage("null"), ColorIdentity: json.RawMessage(`[]`),
	})

	if updated.Bracket != nil || updated.ColorIdentity == nil || len(updated.ColorIdentity) != 0 {
		t.Fatalf("after clear = bracket %v colors %s, want unknown bracket and colorless", updated.Bracket,
			colors(updated.ColorIdentity))
	}
	if !updated.BracketOverridden || !updated.ColorIdentityOverridden {
		t.Fatal("cleared values are not flagged as overridden")
	}
}

func TestUpdateDeck_ResetTakesMoxfieldValueAgain(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-reset@example.com")

	mox := &mockMoxfieldClient{deck: traitsDeck("traits5", intPtr(2), []string{"W"})}
	svc, deck := importDeck(t, pool, owner.ID, mox)
	mustUpdateDeck(t, svc, owner.ID, deck.ID, decks.UpdateDeckRequest{
		Bracket: json.RawMessage("1"), ColorIdentity: json.RawMessage(`["G"]`),
	})

	mox.deck = traitsDeck("traits5", intPtr(5), []string{"W", "B"})
	updated := mustUpdateDeck(t, svc, owner.ID, deck.ID, decks.UpdateDeckRequest{ResetBracket: true})

	if *updated.Bracket != 5 || updated.BracketOverridden {
		t.Fatalf("after reset = bracket %d overridden %v, want Moxfield's 5 and not overridden",
			*updated.Bracket, updated.BracketOverridden)
	}
	if colors(updated.ColorIdentity) != "G" || !updated.ColorIdentityOverridden {
		t.Fatal("resetting the bracket touched the color identity")
	}
}

func TestUpdateDeck_InvalidRequests(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-invalid@example.com")
	ctx := context.Background()

	svc, imported := importDeck(t, pool, owner.ID, &mockMoxfieldClient{deck: traitsDeck("traits4", nil, nil)})
	manual, err := svc.CreateDeck(ctx, owner.ID, &decks.CreateDeckRequest{Name: manualDeckName, Commander: testCommander})
	if err != nil {
		t.Fatalf("CreateDeck() error = %v", err)
	}

	cases := map[string]struct {
		deckID string
		req    decks.UpdateDeckRequest
	}{
		"bracket out of range": {imported.ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("0")}},
		"bracket not a number": {imported.ID, decks.UpdateDeckRequest{Bracket: json.RawMessage(`"2"`)}},
		"unknown color":        {imported.ID, decks.UpdateDeckRequest{ColorIdentity: json.RawMessage(`["W","X"]`)}},
		"set and reset":        {imported.ID, decks.UpdateDeckRequest{Bracket: json.RawMessage("2"), ResetBracket: true}},
		"reset manual deck":    {manual.ID, decks.UpdateDeckRequest{ResetColorIdentity: true}},
	}
	for name, tc := range cases {
		_, err := svc.UpdateDeck(ctx, owner.ID, tc.deckID, tc.req)
		if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", name, fiberErr.Code)
		}
	}
}

// seedTraitDecks creates one manual deck per entry, named after its key.
func seedTraitDecks(
	t *testing.T, pool *pgxpool.Pool, userID string, seeds map[string]decks.CreateDeckRequest,
) decks.Service {
	t.Helper()
	svc := newDecksSvc(pool, nil)
	for name, req := range seeds {
		req.Name, req.Commander = name, testCommander
		if _, err := svc.CreateDeck(context.Background(), userID, &req); err != nil {
			t.Fatalf("CreateDeck(%s) error = %v", name, err)
		}
	}
	return svc
}

func TestListDecks_FiltersByBracketAndColors(t *testing.T) {
	pool := testutil.DB(t)
	testutil.Truncate(t, pool, "users")
	owner := createTestUser(t, pool, "traits-filter@example.com")

	svc := seedTraitDecks(t, pool, owner.ID, map[string]decks.CreateDeckRequest{
		deckAzorius:   {Bracket: intPtr(2), ColorIdentity: []string{"W", "U"}},
		deckMonoBlue:  {Bracket: intPtr(3), ColorIdentity: []string{"U"}},
		deckEsper:     {Bracket: intPtr(4), ColorIdentity: []string{"W", "U", "B"}},
		deckColorless: {Bracket: intPtr(2), ColorIdentity: []string{}},
		"unknown":     {},
	})

	cases := map[string]struct {
		filter common.DeckTraitFilter
		want   []string
	}{
		"bracket 2":      {common.DeckTraitFilter{Brackets: []int16{2}}, []string{deckAzorius, deckColorless}},
		"bracket 3 or 4": {common.DeckTraitFilter{Brackets: []int16{3, 4}}, []string{deckEsper, deckMonoBlue}},
		"exact WU": {
			common.DeckTraitFilter{Colors: []string{"W", "U"}, ColorMode: common.ColorModeExact}, []string{deckAzorius},
		},
		"includes U": {
			common.DeckTraitFilter{Colors: []string{"U"}, ColorMode: common.ColorModeIncludes},
			[]string{deckAzorius, deckEsper, deckMonoBlue},
		},
		"within WU": {
			common.DeckTraitFilter{Colors: []string{"W", "U"}, ColorMode: common.ColorModeWithin},
			[]string{deckAzorius, deckColorless, deckMonoBlue},
		},
		"colorless": {
			common.DeckTraitFilter{Colors: []string{}, ColorMode: common.ColorModeExact}, []string{deckColorless},
		},
		"bracket 2 and includes W": {
			common.DeckTraitFilter{Brackets: []int16{2}, Colors: []string{"W"}, ColorMode: common.ColorModeIncludes},
			[]string{deckAzorius},
		},
	}
	for name, tc := range cases {
		res, err := svc.ListDecks(context.Background(), owner.ID, firstPage(), tc.filter)
		if err != nil {
			t.Fatalf("%s: ListDecks() error = %v", name, err)
		}
		got := make([]string, 0, len(res.Items))
		for i := range res.Items {
			got = append(got, res.Items[i].Name)
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: ListDecks() = %v, want %v", name, got, tc.want)
		}
	}
}
