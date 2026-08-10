// Package tournaments runs standalone Swiss-format Commander tournaments: an
// organizer creates one, participants (app users bringing one of their own
// decks, or guests with no account) register via a join code, and once the
// roster is ready the organizer starts it. Each round pairs participants into
// 3-4 player pods (see pairing.go); the organizer records each table's finish
// order manually, which drives scoring (1st=2, 2nd=1, 3rd/4th=0) and the next
// round's pairings.
//
// Deliberately NOT wired into internal/games' live life-tracker engine
// (games/game_players/WebSocket): that would require inventing guest support
// in a schema whose game_players.user_id has no concept of an accountless
// seat, and extending ADR-0013's proxy-join authorization (today an organizer
// can only seat someone if they share a playgroup with them, which a
// tournament organizer and a participant invited by code usually don't). See
// ADR-0016 for the full reasoning — a read-only "which table am I at" lookup
// is enough for v1.
package tournaments

import (
	"crypto/rand"
	"math/big"
)

const (
	minTableSize = 3
	maxTableSize = 4

	// joinCodeLength/joinCodeCharset: excludes ambiguous glyphs (0/O, 1/I) since
	// this is meant to be read aloud or typed by hand at a table.
	joinCodeLength  = 6
	joinCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

	// tableSizes' remainder-patching switch cases (n % maxTableSize); 0 and 1
	// need no named constant since mnd doesn't flag them.
	remainderOneThree  = 3 // one 3-table, rest 4-tables
	remainderTwoThrees = 2 // one 4-table becomes two 3-tables (4+2 -> 3+3)

	// roundCount's staircase: N players -> how many Swiss rounds (see its doc).
	smallFieldMaxPlayers  = 8
	smallFieldRounds      = 3
	mediumFieldMaxPlayers = 16
	mediumFieldRounds     = 4
	largeFieldMaxPlayers  = 32
	largeFieldRounds      = 5
	hugeFieldMaxPlayers   = 64
	hugeFieldRounds       = 6
	massiveFieldRounds    = 7
)

// tableSizes returns the table-size distribution for n seated participants,
// maximizing 4-tables (see the package doc). ok is false if n has no valid
// split into tables of 3-4: n < 3, or n == 5 -- the only gap for n >= 3, since
// 3a+4b=n has no non-negative integer solution exactly at n=5 (the Frobenius/
// Chicken-McNugget bound for coprime 3 and 4 is 3*4-3-4=5).
func tableSizes(n int) (sizes []int, ok bool) {
	if n < minTableSize || n == 5 {
		return nil, false
	}

	fours := n / maxTableSize
	threes := 0
	switch n % maxTableSize {
	case 0:
		// all fours, nothing to patch
	case remainderOneThree:
		threes = 1
	case remainderTwoThrees:
		// one 4-table becomes two 3-tables (4+2 -> 3+3)
		fours--
		threes = 2
	case 1:
		// two 4-tables become three 3-tables (4+4+1 -> 3+3+3)
		fours -= 2
		threes = 3
	}

	sizes = make([]int, 0, fours+threes)
	for range fours {
		sizes = append(sizes, maxTableSize)
	}
	for range threes {
		sizes = append(sizes, minTableSize)
	}
	return sizes, true
}

// roundCount returns how many Swiss rounds to run for n participants: a fixed
// staircase, not derived from tournament theory (see ADR-0016) -- pods carry
// more information per round than 1v1 Swiss, so fewer rounds are needed.
func roundCount(n int) int {
	switch {
	case n <= smallFieldMaxPlayers:
		return smallFieldRounds
	case n <= mediumFieldMaxPlayers:
		return mediumFieldRounds
	case n <= largeFieldMaxPlayers:
		return largeFieldRounds
	case n <= hugeFieldMaxPlayers:
		return hugeFieldRounds
	default:
		return massiveFieldRounds
	}
}

// participant is the minimal shape pairRound needs: identity, current
// standing, and points for reporting -- callers translate their own row
// types into this before pairing.
type participant struct {
	id     string
	points int32
}

// pairRound assigns participants to tables for a round, given the table-size
// distribution (see tableSizes) and every pair that's already shared a table in
// a previous round of this tournament. ranked must already be in seeding order
// (points DESC for round 2+; shuffled for round 1, see shuffleParticipants) and
// len(ranked) must equal the sum of sizes -- both guaranteed by the caller,
// which derives sizes from len(ranked) via tableSizes.
//
// Heuristic, not a provably-optimal Swiss pairer: greedily seats the next
// participant in seeding order at whichever open table currently has the
// fewest repeat opponents against it, falling back to the least-bad table if
// every option would force a repeat. O(n^2) worst case, fine at tournament
// scale (this is not meant to scale to hundreds of players).
func pairRound(ranked []participant, sizes []int, alreadyPlayed map[[2]string]bool) [][]string {
	tables := make([][]string, len(sizes))
	for i := range sizes {
		tables[i] = make([]string, 0, sizes[i])
	}

	for _, p := range ranked {
		bestTable, bestRepeats := -1, -1
		for i, seated := range tables {
			if len(seated) >= sizes[i] {
				continue
			}
			repeats := 0
			for _, other := range seated {
				if alreadyPlayed[pairKey(p.id, other)] {
					repeats++
				}
			}
			if bestTable == -1 || repeats < bestRepeats {
				bestTable, bestRepeats = i, repeats
			}
		}
		tables[bestTable] = append(tables[bestTable], p.id)
	}
	return tables
}

// pairKey normalizes an unordered participant pair into a lookup key for the
// alreadyPlayed set.
func pairKey(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// shuffleParticipants randomizes seeding order in place, for round 1: every
// participant starts at 0 points, so sorting by points (or falling back to
// registration order) would seed the round by signup order instead of fairly.
// Uses crypto/rand (not math/rand) so this isn't a predictable shuffle.
func shuffleParticipants(participants []participant) {
	for i := len(participants) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			// crypto/rand failing means the OS's entropy source is broken --
			// nothing meaningful to recover into, an unshuffled (registration-order)
			// round 1 is a fair-enough degradation rather than failing tournament start.
			return
		}
		j := int(jBig.Int64())
		participants[i], participants[j] = participants[j], participants[i]
	}
}

// generateJoinCode returns a random human-typeable tournament code.
func generateJoinCode() (string, error) {
	code := make([]byte, joinCodeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinCodeCharset))))
		if err != nil {
			return "", err
		}
		code[i] = joinCodeCharset[n.Int64()]
	}
	return string(code), nil
}
