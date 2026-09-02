package statistics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// GetDashboard assembles the whole dashboard payload.
//
// The point of this endpoint is the query count, so the shape of this function
// matters: it runs a fixed 10 queries no matter how large the account is. The
// client's previous version grew with it -- one request per playgroup for that
// group's *entire* game history (itself an N+1 per game inside the backend),
// plus a cursor chain through every deck, 20 at a time.
//
// The queries run sequentially rather than in parallel goroutines: they're
// single-digit milliseconds each against a Postgres in the same datacenter, and
// a fan-out here would trade that for pool contention and partial-failure
// handling -- the exact problem this endpoint exists to remove.
func (s *service) GetDashboard(ctx context.Context, userID string) (*DashboardResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	res := &DashboardResponse{}

	if res.Stats, err = s.dashboardUserStats(ctx, userID, uid); err != nil {
		return nil, err
	}
	if res.TotalDecks, res.TotalPlaygroups, err = s.dashboardTotals(ctx, uid); err != nil {
		return nil, err
	}
	if res.Decks, res.BestDeck, err = s.dashboardDecks(ctx, uid); err != nil {
		return nil, err
	}
	if res.Playgroups, err = s.dashboardPlaygroups(ctx, uid); err != nil {
		return nil, err
	}
	if res.RecentGames, err = s.dashboardRecentGames(ctx, uid); err != nil {
		return nil, err
	}
	if res.Streak, res.StreakWon, err = s.dashboardStreak(ctx, uid); err != nil {
		return nil, err
	}

	return res, nil
}

// dashboardUserStats reads the precalculated summary, treating a user who never
// finished a game as zeros rather than an error -- same rule as GetUserStats.
func (s *service) dashboardUserStats(ctx context.Context, userID string, uid pgtype.UUID) (UserStatsResponse, error) {
	stats, err := s.repo.GetUserStatistics(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserStatsResponse{UserID: userID}, nil
		}
		return UserStatsResponse{}, fmt.Errorf("looking up user statistics: %w", err)
	}
	return *toUserStatsResponse(&stats), nil
}

// dashboardTotals counts the full collections, which the header line reports
// even though only the first few of each are listed.
func (s *service) dashboardTotals(ctx context.Context, uid pgtype.UUID) (decks, playgroups int32, err error) {
	deckCount, err := s.repo.CountDecksForUser(ctx, uid)
	if err != nil {
		return 0, 0, fmt.Errorf("counting decks: %w", err)
	}
	playgroupCount, err := s.repo.CountPlaygroupsForUser(ctx, uid)
	if err != nil {
		return 0, 0, fmt.Errorf("counting playgroups: %w", err)
	}
	//nolint:gosec // a single user's deck/playgroup counts never approach int32 overflow
	return int32(deckCount), int32(playgroupCount), nil
}

// dashboardDecks returns the most-played decks and the best one by win rate.
// They're two queries because they're two different orderings, and the best deck
// is frequently not in the top four by games played.
func (s *service) dashboardDecks(
	ctx context.Context, uid pgtype.UUID,
) (played []DashboardDeckResponse, best *DashboardDeckResponse, err error) {
	rows, err := s.repo.ListDashboardDecksForUser(ctx, ListDashboardDecksForUserParams{
		UserID:    uid,
		DeckLimit: dashboardDeckLimit,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("listing dashboard decks: %w", err)
	}

	decks := make([]DashboardDeckResponse, 0, len(rows))
	for i := range rows {
		decks = append(decks, DashboardDeckResponse{
			ID:          rows[i].DeckID.String(),
			Name:        rows[i].Name,
			Commander:   rows[i].Commander,
			ImageURL:    textToPtr(rows[i].ImageUrl),
			GamesPlayed: rows[i].GamesPlayed,
			GamesWon:    rows[i].GamesWon,
		})
	}

	// No row here means no deck has been played yet, not an error: the client
	// renders an empty state in its place.
	bestRow, err := s.repo.GetDashboardBestDeckForUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decks, nil, nil
		}
		return nil, nil, fmt.Errorf("looking up best deck: %w", err)
	}

	return decks, &DashboardDeckResponse{
		ID:          bestRow.DeckID.String(),
		Name:        bestRow.Name,
		Commander:   bestRow.Commander,
		ImageURL:    textToPtr(bestRow.ImageUrl),
		GamesPlayed: bestRow.GamesPlayed,
		GamesWon:    bestRow.GamesWon,
	}, nil
}

// dashboardPlaygroups returns the group cards with their avatar strips, taking
// the members for every card in one batched query instead of one per group.
func (s *service) dashboardPlaygroups(ctx context.Context, uid pgtype.UUID) ([]DashboardPlaygroupResponse, error) {
	rows, err := s.repo.ListDashboardPlaygroupsForUser(ctx, ListDashboardPlaygroupsForUserParams{
		UserID:         uid,
		PlaygroupLimit: dashboardPlaygroupLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("listing dashboard playgroups: %w", err)
	}

	groups := make([]DashboardPlaygroupResponse, 0, len(rows))
	if len(rows) == 0 {
		return groups, nil
	}

	ids := make([]pgtype.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].PlaygroupID
	}
	memberRows, err := s.repo.ListDashboardPlaygroupMembers(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("listing dashboard playgroup members: %w", err)
	}

	// Capped here rather than in SQL: the query is already scoped to the three
	// groups on screen, so the rows it returns are a handful either way, and a
	// per-group LIMIT would need a window function for no real gain.
	membersByGroup := make(map[pgtype.UUID][]DashboardMemberResponse, len(rows))
	for i := range memberRows {
		gid := memberRows[i].PlaygroupID
		if len(membersByGroup[gid]) >= dashboardAvatarLimit {
			continue
		}
		membersByGroup[gid] = append(membersByGroup[gid], DashboardMemberResponse{
			UserID:   memberRows[i].UserID.String(),
			Username: memberRows[i].Username,
		})
	}

	for i := range rows {
		members := membersByGroup[rows[i].PlaygroupID]
		if members == nil {
			members = []DashboardMemberResponse{}
		}
		groups = append(groups, DashboardPlaygroupResponse{
			ID:          rows[i].PlaygroupID.String(),
			Name:        rows[i].PlaygroupName,
			MemberCount: rows[i].MemberCount,
			GamesPlayed: rows[i].GamesPlayed,
			Members:     members,
		})
	}
	return groups, nil
}

// dashboardRecentGames reuses the finished-games history query (first page,
// trimmed to what the list shows) and resolves each game from the caller's
// point of view: their result, their deck, everyone else by name.
func (s *service) dashboardRecentGames(ctx context.Context, uid pgtype.UUID) ([]DashboardGameResponse, error) {
	gameRows, err := s.repo.ListFinishedGamesPage(ctx, ListFinishedGamesPageParams{
		UserID:    uid,
		PageLimit: dashboardRecentGameLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("listing recent games: %w", err)
	}

	games := make([]DashboardGameResponse, 0, len(gameRows))
	if len(gameRows) == 0 {
		return games, nil
	}

	gameIDs := make([]pgtype.UUID, len(gameRows))
	for i := range gameRows {
		gameIDs[i] = gameRows[i].ID
	}
	playerRows, err := s.repo.ListPlayersForGames(ctx, gameIDs)
	if err != nil {
		return nil, fmt.Errorf("listing players for recent games: %w", err)
	}
	playersByGame := make(map[pgtype.UUID][]ListPlayersForGamesRow, len(gameRows))
	for i := range playerRows {
		gid := playerRows[i].GameID
		playersByGame[gid] = append(playersByGame[gid], playerRows[i])
	}

	for i := range gameRows {
		games = append(games, toDashboardGame(&gameRows[i], playersByGame[gameRows[i].ID], uid))
	}
	return games, nil
}

// toDashboardGame collapses one finished game into the caller's row of it.
func toDashboardGame(
	g *ListFinishedGamesPageRow, players []ListPlayersForGamesRow, uid pgtype.UUID,
) DashboardGameResponse {
	game := DashboardGameResponse{ID: g.ID.String(), Opponents: []string{}}
	if g.PlaygroupName.Valid {
		name := g.PlaygroupName.String
		game.PlaygroupName = &name
	}
	if g.StartedAt.Valid {
		t := g.StartedAt.Time.Format(time.RFC3339)
		game.StartedAt = &t
	}
	if g.FinishedAt.Valid {
		t := g.FinishedAt.Time.Format(time.RFC3339)
		game.FinishedAt = &t
	}

	for i := range players {
		p := &players[i]
		if p.UserID != uid {
			game.Opponents = append(game.Opponents, p.Username)
			continue
		}
		game.Won = p.Won
		game.Deck = &DashboardDeckRef{
			ID:        p.DeckID.String(),
			Name:      p.DeckName,
			Commander: p.DeckCommander,
			ImageURL:  textToPtr(p.DeckImageUrl),
		}
	}
	return game
}

// dashboardStreak reads the current run of same-result games. StreakWon stays
// nil when there's nothing to report, so the client can tell "no games yet"
// apart from "a losing streak".
func (s *service) dashboardStreak(
	ctx context.Context, uid pgtype.UUID,
) (streak int32, streakWon *bool, err error) {
	row, err := s.repo.GetCurrentStreakForUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("looking up current streak: %w", err)
	}
	if row.TotalGames == 0 {
		return 0, nil, nil
	}
	won := row.StreakWon
	return row.Streak, &won, nil
}

// textToPtr turns a nullable varchar into the omitempty-friendly *string the
// DTOs use.
func textToPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}
