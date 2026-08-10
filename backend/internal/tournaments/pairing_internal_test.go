package tournaments

import (
	"fmt"
	"strings"
	"testing"
)

func TestTableSizes_ValidForAllNExceptTheKnownGaps(t *testing.T) {
	// 3a+4b=n has no non-negative integer solution only at n=1, 2, 5 (the
	// Frobenius/Chicken-McNugget bound for coprime 3 and 4 is 3*4-3-4=5) -- every
	// other n from 3 up must produce a valid split.
	for n := 1; n <= 50; n++ {
		sizes, ok := tableSizes(n)
		if n < minTableSize || n == 5 {
			if ok {
				t.Fatalf("tableSizes(%d) ok = true, want false (no valid 3-4 split exists)", n)
			}
			continue
		}
		if !ok {
			t.Fatalf("tableSizes(%d) ok = false, want true", n)
		}

		sum := 0
		for _, size := range sizes {
			if size < minTableSize || size > maxTableSize {
				t.Fatalf("tableSizes(%d) produced size %d outside [%d,%d]", n, size, minTableSize, maxTableSize)
			}
			sum += size
		}
		if sum != n {
			t.Fatalf("tableSizes(%d) sizes %v sum to %d, want %d", n, sizes, sum, n)
		}
	}
}

func TestTableSizes_MaximizesFourTables(t *testing.T) {
	tests := []struct {
		n          int
		wantFours  int
		wantThrees int
	}{
		{n: 4, wantFours: 1, wantThrees: 0},
		{n: 6, wantFours: 0, wantThrees: 2}, // 4+2 -> 3+3, not left as an invalid 2-table
		{n: 7, wantFours: 1, wantThrees: 1},
		{n: 8, wantFours: 2, wantThrees: 0},
		{n: 9, wantFours: 0, wantThrees: 3}, // 4+4+1 -> 3+3+3
		{n: 13, wantFours: 1, wantThrees: 3},
	}
	for _, tt := range tests {
		sizes, ok := tableSizes(tt.n)
		if !ok {
			t.Fatalf("tableSizes(%d) ok = false", tt.n)
		}
		fours, threes := 0, 0
		for _, size := range sizes {
			if size == 4 {
				fours++
			} else {
				threes++
			}
		}
		if fours != tt.wantFours || threes != tt.wantThrees {
			t.Fatalf("tableSizes(%d) = %d fours + %d threes, want %d fours + %d threes",
				tt.n, fours, threes, tt.wantFours, tt.wantThrees)
		}
	}
}

func TestRoundCount(t *testing.T) {
	tests := []struct{ n, want int }{
		{n: 8, want: 3}, {n: 9, want: 4}, {n: 16, want: 4}, {n: 17, want: 5},
		{n: 32, want: 5}, {n: 33, want: 6}, {n: 64, want: 6}, {n: 65, want: 7},
	}
	for _, tt := range tests {
		if got := roundCount(tt.n); got != tt.want {
			t.Fatalf("roundCount(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func makeRanked(n int) []participant {
	ranked := make([]participant, n)
	for i := range ranked {
		ranked[i] = participant{id: fmt.Sprintf("p%d", i)}
	}
	return ranked
}

// allPlayedPairs mirrors buildAlreadyPlayedSet, but works directly off
// pairRound's own [][]string output instead of the sqlc row type, so pairing
// logic can be tested without touching the database.
func allPlayedPairs(tables [][]string) map[[2]string]bool {
	played := make(map[[2]string]bool)
	for _, table := range tables {
		for i := range table {
			for j := i + 1; j < len(table); j++ {
				played[pairKey(table[i], table[j])] = true
			}
		}
	}
	return played
}

func assertValidTables(t *testing.T, tables [][]string, sizes []int, participantCount int) {
	t.Helper()
	if len(tables) != len(sizes) {
		t.Fatalf("pairRound() produced %d tables, want %d", len(tables), len(sizes))
	}
	seen := make(map[string]bool, participantCount)
	for i, table := range tables {
		if len(table) != sizes[i] {
			t.Fatalf("table %d has %d seats, want %d", i, len(table), sizes[i])
		}
		for _, id := range table {
			if seen[id] {
				t.Fatalf("participant %q seated more than once", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != participantCount {
		t.Fatalf("pairRound() seated %d distinct participants, want %d", len(seen), participantCount)
	}
}

func TestPairRound_FirstRoundSeatsEveryoneExactlyOnce(t *testing.T) {
	ranked := makeRanked(9)
	sizes, ok := tableSizes(9)
	if !ok {
		t.Fatalf("tableSizes(9) ok = false")
	}
	tables := pairRound(ranked, sizes, nil)
	assertValidTables(t, tables, sizes, len(ranked))
}

// TestPairRound_AvoidsRepeatsWhenAFullyRepeatFreeRegroupingExists uses 9
// participants (three 3-tables): round 1 groups them, verified by hand, as
// {p0,p1,p2}, {p3,p4,p5}, {p6,p7,p8}. A fully repeat-free round 2 IS
// mathematically achievable here -- one participant from each original trio
// per new table, the same shape as a 3x3 Latin square -- so this asserts the
// greedy heuristic actually finds it, not just "does its best".
func TestPairRound_AvoidsRepeatsWhenAFullyRepeatFreeRegroupingExists(t *testing.T) {
	ranked := makeRanked(9)
	sizes, ok := tableSizes(9)
	if !ok {
		t.Fatalf("tableSizes(9) ok = false")
	}

	round1 := pairRound(ranked, sizes, nil)
	played := allPlayedPairs(round1)

	round2 := pairRound(ranked, sizes, played)
	assertValidTables(t, round2, sizes, len(ranked))
	for pair := range allPlayedPairs(round2) {
		if played[pair] {
			t.Fatalf("pairRound() repeated pair %v from round 1, even though a fully repeat-free regrouping exists", pair)
		}
	}
}

// TestPairRound_StillProducesValidTablesWhenRepeatsAreUnavoidable uses 8
// participants in two 4-tables: with only two original groups, any new 4-seat
// table can have at most one "safe" pick from each (two safe seats, short of
// four), so some repeat is mathematically unavoidable in round 2. This
// asserts the fallback still produces a complete, valid pairing instead of
// erroring out or leaving someone unseated.
func TestPairRound_StillProducesValidTablesWhenRepeatsAreUnavoidable(t *testing.T) {
	ranked := makeRanked(8)
	sizes, ok := tableSizes(8)
	if !ok {
		t.Fatalf("tableSizes(8) ok = false")
	}

	round1 := pairRound(ranked, sizes, nil)
	round2 := pairRound(ranked, sizes, allPlayedPairs(round1))
	assertValidTables(t, round2, sizes, len(ranked))
}

func TestShuffleParticipants_IsAPermutation(t *testing.T) {
	original := makeRanked(20)
	shuffled := make([]participant, len(original))
	copy(shuffled, original)
	shuffleParticipants(shuffled)

	if len(shuffled) != len(original) {
		t.Fatalf("shuffleParticipants() changed the slice length to %d, want %d", len(shuffled), len(original))
	}
	seen := make(map[string]bool, len(original))
	for _, p := range shuffled {
		seen[p.id] = true
	}
	if len(seen) != len(original) {
		t.Fatalf("shuffleParticipants() lost or duplicated a participant: %d distinct ids, want %d", len(seen), len(original))
	}
}

func TestGenerateJoinCode(t *testing.T) {
	code, err := generateJoinCode()
	if err != nil {
		t.Fatalf("generateJoinCode() error = %v", err)
	}
	if len(code) != joinCodeLength {
		t.Fatalf("generateJoinCode() = %q, want length %d", code, joinCodeLength)
	}
	for _, c := range code {
		if !strings.ContainsRune(joinCodeCharset, c) {
			t.Fatalf("generateJoinCode() = %q, contains %q which is outside the charset", code, c)
		}
	}
}
