package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	defaultLifeTotal = 40

	actionLifeChange      = "LifeChange"
	actionCombatDamage    = "CombatDamage"
	actionCommanderDamage = "CommanderDamage"
	actionElimination     = "Elimination"
)

// How much of each list the dashboard actually renders. These live here rather
// than as query params because they're the screen's layout, not the caller's
// choice -- see DashboardResponse's doc for why the endpoint is shaped that way.
const (
	dashboardDeckLimit       = 4
	dashboardPlaygroupLimit  = 3
	dashboardRecentGameLimit = 4
	dashboardAvatarLimit     = 4
)

var (
	// ErrDeckNotFound indicates that the deck doesn't exist or doesn't belong to the authenticated user.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrInvalidPlaygroupID indicates the received playgroup ID isn't a valid UUID.
	ErrInvalidPlaygroupID = common.InvalidInput("invalid playgroup id")
	// ErrPlaygroupNotFound indicates that the playgroup doesn't exist or the
	// authenticated user isn't a member — it doesn't distinguish which of the two
	// cases it is (same "don't reveal" criteria as games.ErrPlaygroupNotFound).
	ErrPlaygroupNotFound = common.NotFound("playgroup not found")
)

// PlaygroupMembership is what statistics needs from playgroups to authorize
// GetPlaygroupStats: confirming a user is a member of a group, without this
// package depending on internal/playgroups directly (same pattern as
// games.PlaygroupMembership).
type PlaygroupMembership interface {
	IsMember(ctx context.Context, playgroupID, userID string) (bool, error)
}

// Service defines the business logic of the statistics module.
type Service interface {
	GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error)
	GetDeckStats(ctx context.Context, userID, deckID string) (*DeckStatsResponse, error)
	// ListDeckStats returns the statistics of every deck the user owns in a
	// single query, instead of the caller having to call GetDeckStats once
	// per deck (see ListDeckStatisticsForUser's doc for why this exists).
	ListDeckStats(ctx context.Context, userID string) ([]DeckStatsResponse, error)
	// GetPlaygroupStats returns the aggregated statistics for a playgroup, if
	// userID is a member of it.
	GetPlaygroupStats(ctx context.Context, playgroupID, userID string) (*PlaygroupStatsResponse, error)
	// ListOpponentStats returns the head-to-head record (games together, eliminations
	// each way) against every other user the caller has shared a finished game with.
	ListOpponentStats(ctx context.Context, userID string) ([]OpponentStatsResponse, error)
	// ListPlaygroupGameCounts returns, for every playgroup the user belongs to, how
	// many finished games they've played within it -- in one query, instead of one
	// GetPlaygroupStats call per group.
	ListPlaygroupGameCounts(ctx context.Context, userID string) ([]PlaygroupGameCountResponse, error)
	// ListFinishedGames returns a page of the user's finished-games history,
	// enriched with each seat's username/deck (unlike games.ListGames, which is
	// intentionally lean).
	ListFinishedGames(ctx context.Context, page common.PageRequest, userID string) (*FinishedGameListResponse, error)
	// GetDashboard returns everything the web dashboard renders in one payload,
	// in a fixed number of queries regardless of how many decks, groups or games
	// the account has (see DashboardResponse).
	GetDashboard(ctx context.Context, userID string) (*DashboardResponse, error)
	// RecalculateForGame recalculates the aggregated user and deck statistics from
	// the result and actions of an already-finished game.
	RecalculateForGame(ctx context.Context, gameID string) error
}

type service struct {
	repo       *Queries
	membership PlaygroupMembership
}

// NewService creates a new statistics service.
func NewService(db *pgxpool.Pool, membership PlaygroupMembership) Service {
	return &service{repo: New(db), membership: membership}
}

// GetUserStats returns a user's precalculated global statistics. A user who
// never finished a game doesn't have their own row: zeros are returned
// instead of a 404, because "zero games played" is a valid state.
func (s *service) GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	stats, err := s.repo.GetUserStatistics(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &UserStatsResponse{UserID: userID}, nil
		}
		return nil, fmt.Errorf("looking up user statistics: %w", err)
	}
	return toUserStatsResponse(&stats), nil
}

// GetDeckStats returns a deck's statistics, if it belongs to the given user.
func (s *service) GetDeckStats(ctx context.Context, userID, deckID string) (*DeckStatsResponse, error) {
	did, err := common.ParseUUID(deckID)
	if err != nil {
		return nil, ErrDeckNotFound
	}

	deck, err := s.repo.GetDeckByID(ctx, did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDeckNotFound
		}
		return nil, fmt.Errorf("looking up deck: %w", err)
	}
	if deck.UserID.String() != userID {
		// Doesn't distinguish "doesn't exist" from "not yours": avoids revealing the deck exists.
		return nil, ErrDeckNotFound
	}

	stats, err := s.repo.GetDeckStatistics(ctx, did)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &DeckStatsResponse{DeckID: deckID}, nil
		}
		return nil, fmt.Errorf("looking up deck statistics: %w", err)
	}
	return toDeckStatsResponse(&stats), nil
}

// ListDeckStats returns the statistics of every deck owned by the user (zeros
// for a deck never played, same as GetDeckStats), in a single query.
func (s *service) ListDeckStats(ctx context.Context, userID string) ([]DeckStatsResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListDeckStatisticsForUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing deck statistics: %w", err)
	}

	res := make([]DeckStatsResponse, 0, len(rows))
	for i := range rows {
		res = append(res, DeckStatsResponse{
			DeckID:               rows[i].DeckID.String(),
			GamesPlayed:          rows[i].GamesPlayed,
			GamesWon:             rows[i].GamesWon,
			HighestLifeTotal:     rows[i].HighestLifeTotalAchieved,
			TotalCommanderDamage: rows[i].TotalCommanderDamageDealt,
		})
	}
	return res, nil
}

// GetPlaygroupStats returns statistics aggregated per member within a playgroup,
// calculated live over the finished games (there's no summary table for this).
// Requires userID to be a member of the group, same "don't reveal" criteria
// as playgroups.GetPlaygroup.
func (s *service) GetPlaygroupStats(ctx context.Context, playgroupID, userID string) (*PlaygroupStatsResponse, error) {
	isMember, err := s.membership.IsMember(ctx, playgroupID, userID)
	if err != nil {
		return nil, fmt.Errorf("checking playgroup membership: %w", err)
	}
	if !isMember {
		return nil, ErrPlaygroupNotFound
	}

	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return nil, ErrInvalidPlaygroupID
	}

	gamesPlayed, err := s.repo.CountFinishedGamesForPlaygroup(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("counting playgroup games: %w", err)
	}

	memberStats, err := s.repo.ListPlaygroupMemberGameStats(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup member stats: %w", err)
	}

	res := &PlaygroupStatsResponse{
		PlaygroupID: playgroupID,
		//nolint:gosec // count(*) de partidas de un grupo, nunca se acerca a overflow de int32
		GamesPlayed: int32(gamesPlayed),
		Members:     make([]PlaygroupMemberStats, 0, len(memberStats)),
	}
	for i := range memberStats {
		res.Members = append(res.Members, PlaygroupMemberStats{
			UserID:      memberStats[i].UserID.String(),
			GamesPlayed: memberStats[i].GamesPlayed,
			GamesWon:    memberStats[i].GamesWon,
		})
	}
	return res, nil
}

// ListOpponentStats returns the head-to-head record against every other user
// the caller has shared a finished game with, live-computed like
// GetPlaygroupStats above -- read once, on the statistics screen, not on
// every request like the precalculated summaries.
func (s *service) ListOpponentStats(ctx context.Context, userID string) ([]OpponentStatsResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListOpponentStats(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing opponent statistics: %w", err)
	}

	res := make([]OpponentStatsResponse, 0, len(rows))
	for i := range rows {
		res = append(res, OpponentStatsResponse{
			UserID:                    rows[i].OpponentID.String(),
			Username:                  rows[i].OpponentUsername,
			GamesTogether:             rows[i].GamesTogether,
			TimesYouEliminatedThem:    rows[i].TimesYouEliminatedThem,
			TimesEliminatedByOpponent: rows[i].TimesEliminatedByOpponent,
		})
	}
	return res, nil
}

// ListPlaygroupGameCounts returns, for every playgroup the user belongs to,
// how many finished games they've played within it, in a single query.
func (s *service) ListPlaygroupGameCounts(ctx context.Context, userID string) ([]PlaygroupGameCountResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, err := s.repo.ListPlaygroupGameCountsForUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("listing playgroup game counts: %w", err)
	}

	res := make([]PlaygroupGameCountResponse, 0, len(rows))
	for i := range rows {
		res = append(res, PlaygroupGameCountResponse{
			PlaygroupID:   rows[i].PlaygroupID.String(),
			PlaygroupName: rows[i].PlaygroupName,
			GamesPlayed:   rows[i].GamesPlayed,
		})
	}
	return res, nil
}

// ListFinishedGames returns a page of the user's finished-games history. Unlike
// games.ListGames (kept lean for the dashboard/join-game flow), each entry
// carries every seat's username and deck name/commander/image, resolved here
// in one batched round trip (ListPlayersForGames) instead of per-game.
func (s *service) ListFinishedGames(
	ctx context.Context, page common.PageRequest, userID string,
) (*FinishedGameListResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}

	rows, nextCursor, err := s.fetchFinishedGamesPage(ctx, page, uid)
	if err != nil {
		return nil, err
	}

	items, err := s.enrichFinishedGames(ctx, rows)
	if err != nil {
		return nil, err
	}

	return &FinishedGameListResponse{Items: items, NextCursor: nextCursor}, nil
}

// fetchFinishedGamesPage resolves the cursor (if any), fetches the page, and
// works out whether there's a next one -- same "limit+1" trick as
// games.ListGames, avoiding a separate COUNT(*).
func (s *service) fetchFinishedGamesPage(
	ctx context.Context, page common.PageRequest, uid pgtype.UUID,
) ([]ListFinishedGamesPageRow, *string, error) {
	params := ListFinishedGamesPageParams{UserID: uid, PageLimit: page.Limit + 1}
	if page.Cursor != "" {
		cursorCreatedAt, cursorID, cursorErr := decodeCursor(page.Cursor)
		if cursorErr != nil {
			return nil, nil, cursorErr
		}
		params.CursorCreatedAt = cursorCreatedAt
		params.CursorID = cursorID
	}

	rows, err := s.repo.ListFinishedGamesPage(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("listing finished games: %w", err)
	}

	var nextCursor *string
	if len(rows) > int(page.Limit) {
		rows = rows[:page.Limit]
		last := rows[len(rows)-1]
		encoded := common.EncodeCursor(common.Cursor{CreatedAt: last.CreatedAt.Time, ID: last.ID.String()})
		nextCursor = &encoded
	}
	return rows, nextCursor, nil
}

// enrichFinishedGames batches ListPlayersForGames for the whole page (one
// round trip) instead of one query per game.
func (s *service) enrichFinishedGames(
	ctx context.Context, games []ListFinishedGamesPageRow,
) ([]FinishedGameResponse, error) {
	items := make([]FinishedGameResponse, 0, len(games))
	if len(games) == 0 {
		return items, nil
	}

	gameIDs := make([]pgtype.UUID, len(games))
	for i := range games {
		gameIDs[i] = games[i].ID
	}
	playerRows, err := s.repo.ListPlayersForGames(ctx, gameIDs)
	if err != nil {
		return nil, fmt.Errorf("listing players for finished games: %w", err)
	}
	playersByGame := make(map[pgtype.UUID][]ListPlayersForGamesRow, len(games))
	for i := range playerRows {
		gid := playerRows[i].GameID
		playersByGame[gid] = append(playersByGame[gid], playerRows[i])
	}

	summaryRows, err := s.repo.ListGameActionSummaryForGames(ctx, gameIDs)
	if err != nil {
		return nil, fmt.Errorf("listing action summaries for finished games: %w", err)
	}
	summaryByGame := make(map[pgtype.UUID]ListGameActionSummaryForGamesRow, len(summaryRows))
	for i := range summaryRows {
		summaryByGame[summaryRows[i].GameID] = summaryRows[i]
	}

	for i := range games {
		gid := games[i].ID
		res := toFinishedGameResponse(&games[i], playersByGame[gid])
		if summary, ok := summaryByGame[gid]; ok {
			applyActionSummary(res, &summary)
		}
		items = append(items, *res)
	}
	return items, nil
}

// applyActionSummary fills in a FinishedGameResponse's turn count and biggest hit from
// its ListGameActionSummaryForGames row. BiggestHitAmount/BiggestHitUsername are both
// NULL together (LEFT JOIN, no CombatDamage/CommanderDamage action in the game) or both set.
func applyActionSummary(res *FinishedGameResponse, summary *ListGameActionSummaryForGamesRow) {
	res.TurnCount = summary.TurnCount
	if summary.BiggestHitAmount.Valid && summary.BiggestHitUsername.Valid {
		res.BiggestHit = &BiggestHitResponse{
			Amount:   summary.BiggestHitAmount.Int32,
			Username: summary.BiggestHitUsername.String,
		}
	}
}

// decodeCursor mirrors games.decodeCursor: same opaque cursor format
// (common.EncodeCursor/DecodeCursor), decoded here because statistics owns
// its own pagination over a different query (ListFinishedGamesPage).
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

func toFinishedGameResponse(g *ListFinishedGamesPageRow, players []ListPlayersForGamesRow) *FinishedGameResponse {
	res := &FinishedGameResponse{ID: g.ID.String()}
	if g.PlaygroupID.Valid {
		pid := g.PlaygroupID.String()
		res.PlaygroupID = &pid
	}
	if g.PlaygroupName.Valid {
		name := g.PlaygroupName.String
		res.PlaygroupName = &name
	}
	if g.StartedAt.Valid {
		t := g.StartedAt.Time.Format(time.RFC3339)
		res.StartedAt = &t
	}
	if g.FinishedAt.Valid {
		t := g.FinishedAt.Time.Format(time.RFC3339)
		res.FinishedAt = &t
	}

	res.Players = make([]FinishedGamePlayerResponse, 0, len(players))
	for i := range players {
		p := &players[i]
		player := FinishedGamePlayerResponse{
			UserID:        p.UserID.String(),
			Username:      p.Username,
			DeckID:        p.DeckID.String(),
			DeckName:      p.DeckName,
			DeckCommander: p.DeckCommander,
			Won:           p.Won,
		}
		if p.DeckImageUrl.Valid {
			url := p.DeckImageUrl.String
			player.DeckImageURL = &url
		}
		res.Players = append(res.Players, player)
	}
	return res
}

// RecalculateForGame walks the players and actions of an already-finished
// game and accumulates the result into user_statistics_summary/deck_statistics_summary.
// The winner is the only player still alive (is_eliminated = false) at the end; if
// 0 or 2+ players are left alive (game cut short by hand) it doesn't count as a win
// for anyone, but games_played is still counted for all participants.
func (s *service) RecalculateForGame(ctx context.Context, gameID string) error {
	gid, err := common.ParseUUID(gameID)
	if err != nil {
		return fmt.Errorf("invalid game id: %w", err)
	}

	players, err := s.repo.ListGamePlayersForGame(ctx, gid)
	if err != nil {
		return fmt.Errorf("listing game players: %w", err)
	}
	if len(players) == 0 {
		return nil
	}

	actions, err := s.repo.ListGameActionsForGame(ctx, gid)
	if err != nil {
		return fmt.Errorf("listing game actions: %w", err)
	}

	winnerID, hasWinner := soleSurvivor(players)
	lifePeaks := peakLifeTotals(players, actions)

	for i := range players {
		p := &players[i]
		won := hasWinner && p.ID == winnerID
		dealt, commanderDealt, eliminations := playerActionTotals(p.ID, actions)

		if err := s.repo.UpsertUserStatistics(ctx, UpsertUserStatisticsParams{
			UserID:                    p.UserID,
			GamesPlayed:               pgtype.Int4{Int32: 1, Valid: true},
			GamesWon:                  pgtype.Int4{Int32: boolToInt32(won), Valid: true},
			TotalDamageDealt:          pgtype.Int4{Int32: dealt, Valid: true},
			TotalCommanderDamageDealt: pgtype.Int4{Int32: commanderDealt, Valid: true},
			TotalEliminations:         pgtype.Int4{Int32: eliminations, Valid: true},
		}); err != nil {
			return fmt.Errorf("updating user statistics: %w", err)
		}

		if err := s.repo.UpsertDeckStatistics(ctx, UpsertDeckStatisticsParams{
			DeckID:                    p.DeckID,
			GamesPlayed:               pgtype.Int4{Int32: 1, Valid: true},
			GamesWon:                  pgtype.Int4{Int32: boolToInt32(won), Valid: true},
			HighestLifeTotalAchieved:  pgtype.Int4{Int32: lifePeaks[p.ID], Valid: true},
			TotalCommanderDamageDealt: pgtype.Int4{Int32: commanderDealt, Valid: true},
		}); err != nil {
			return fmt.Errorf("updating deck statistics: %w", err)
		}
	}
	return nil
}

// soleSurvivor returns the ID of the only non-eliminated player, if there's exactly one.
func soleSurvivor(players []GamePlayer) (pgtype.UUID, bool) {
	var survivor pgtype.UUID
	count := 0
	for i := range players {
		if !players[i].IsEliminated.Bool {
			survivor = players[i].ID
			count++
		}
	}
	return survivor, count == 1
}

// peakLifeTotals reconstructs, for each player, the highest life_total reached during
// the game, by replaying the effect of each action in chronological order (the same
// calculation game-actions applies when recording each action, starting from the baseline of 40).
func peakLifeTotals(players []GamePlayer, actions []GameAction) map[pgtype.UUID]int32 {
	life := make(map[pgtype.UUID]int32, len(players))
	peak := make(map[pgtype.UUID]int32, len(players))
	for i := range players {
		life[players[i].ID] = defaultLifeTotal
		peak[players[i].ID] = defaultLifeTotal
	}

	for i := range actions {
		a := &actions[i]
		delta, ok := lifeDelta(a)
		if !ok {
			continue
		}
		subject := a.ActorID
		if a.TargetID.Valid {
			subject = a.TargetID
		}
		if _, tracked := life[subject]; !tracked {
			continue
		}
		life[subject] += delta
		if life[subject] > peak[subject] {
			peak[subject] = life[subject]
		}
	}
	return peak
}

func lifeDelta(a *GameAction) (int32, bool) {
	switch a.ActionType {
	case actionCombatDamage, actionCommanderDamage:
		amount, ok := payloadAmount(a.Payload)
		return -amount, ok
	case actionLifeChange:
		return payloadAmount(a.Payload)
	default:
		return 0, false
	}
}

func payloadAmount(raw []byte) (int32, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, false
	}
	amount, ok := payload["amount"].(float64)
	if !ok {
		return 0, false
	}
	return int32(amount), true
}

// playerActionTotals sums the damage and explicit eliminations attributable to a
// player as an actor within a game. Auto-eliminations from life/poison
// (see game-actions) aren't attributed to a specific actor in the log, so
// only explicit Elimination actions against another player count.
func playerActionTotals(playerID pgtype.UUID, actions []GameAction) (dealt, commanderDealt, eliminations int32) {
	for i := range actions {
		a := &actions[i]
		if a.ActorID != playerID {
			continue
		}
		switch a.ActionType {
		case actionCombatDamage:
			amount, _ := payloadAmount(a.Payload)
			dealt += amount
		case actionCommanderDamage:
			amount, _ := payloadAmount(a.Payload)
			dealt += amount
			commanderDealt += amount
		case actionElimination:
			if a.TargetID.Valid && a.TargetID != a.ActorID {
				eliminations++
			}
		}
	}
	return dealt, commanderDealt, eliminations
}

func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func toUserStatsResponse(s *UserStatisticsSummary) *UserStatsResponse {
	return &UserStatsResponse{
		UserID:               s.UserID.String(),
		GamesPlayed:          s.GamesPlayed.Int32,
		GamesWon:             s.GamesWon.Int32,
		TotalDamageDealt:     s.TotalDamageDealt.Int32,
		TotalCommanderDamage: s.TotalCommanderDamageDealt.Int32,
		TotalEliminations:    s.TotalEliminations.Int32,
	}
}

func toDeckStatsResponse(s *DeckStatisticsSummary) *DeckStatsResponse {
	return &DeckStatsResponse{
		DeckID:               s.DeckID.String(),
		GamesPlayed:          s.GamesPlayed.Int32,
		GamesWon:             s.GamesWon.Int32,
		HighestLifeTotal:     s.HighestLifeTotalAchieved.Int32,
		TotalCommanderDamage: s.TotalCommanderDamageDealt.Int32,
	}
}
