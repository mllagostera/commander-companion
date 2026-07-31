package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

var (
	// ErrDeckNotFound indica que el deck no existe o no pertenece al usuario autenticado.
	ErrDeckNotFound = common.NotFound("deck not found")
	// ErrInvalidPlaygroupID indicates the received playgroup ID isn't a valid UUID.
	ErrInvalidPlaygroupID = common.InvalidInput("invalid playgroup id")
)

// Service defines the business logic of the statistics module.
type Service interface {
	GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error)
	GetDeckStats(ctx context.Context, userID, deckID string) (*DeckStatsResponse, error)
	GetPlaygroupStats(ctx context.Context, playgroupID string) (*PlaygroupStatsResponse, error)
	// RecalculateForGame recalculates the aggregated user and deck statistics from
	// the result and actions of an already-finished game.
	RecalculateForGame(ctx context.Context, gameID string) error
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de statistics.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
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
		// No se distingue "no existe" de "no es tuyo": evita revelar que el deck existe.
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

// GetPlaygroupStats returns statistics aggregated per member within a playgroup,
// calculated live over the finished games (there's no summary table for this).
func (s *service) GetPlaygroupStats(ctx context.Context, playgroupID string) (*PlaygroupStatsResponse, error) {
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

// RecalculateForGame recorre a los jugadores y las acciones de una partida ya
// finished game and accumulates the result into user_statistics_summary/deck_statistics_summary.
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
