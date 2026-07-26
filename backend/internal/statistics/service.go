package statistics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"
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

// ErrDeckNotFound indica que el deck no existe o no pertenece al usuario autenticado.
var ErrDeckNotFound = errors.New("deck not found")

// Service define la lógica de negocio del módulo statistics.
type Service interface {
	GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error)
	GetDeckStats(ctx context.Context, userID, deckID string) (*DeckStatsResponse, error)
	GetPlaygroupStats(ctx context.Context, playgroupID string) (*PlaygroupStatsResponse, error)
	// RecalculateForGame recalcula las estadísticas agregadas de usuario y deck a partir
	// del resultado y las acciones de una partida ya finalizada.
	RecalculateForGame(ctx context.Context, gameID string) error
}

type service struct {
	repo *Queries
}

// NewService crea un nuevo servicio de statistics.
func NewService(db *pgxpool.Pool) Service {
	return &service{repo: New(db)}
}

// GetUserStats devuelve las estadísticas globales precalculadas de un usuario. Un
// usuario que nunca terminó una partida no tiene fila propia: se devuelven ceros
// en vez de un 404, porque "cero partidas jugadas" es un estado válido.
func (s *service) GetUserStats(ctx context.Context, userID string) (*UserStatsResponse, error) {
	uid, err := common.ParseUUID(userID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "invalid user")
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

// GetDeckStats devuelve las estadísticas de un deck, si pertenece al usuario indicado.
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

// GetPlaygroupStats devuelve estadísticas agregadas por miembro dentro de un grupo,
// calculadas en vivo sobre las partidas finalizadas (no hay tabla de resumen para esto).
func (s *service) GetPlaygroupStats(ctx context.Context, playgroupID string) (*PlaygroupStatsResponse, error) {
	pid, err := common.ParseUUID(playgroupID)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid playgroup id")
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
// finalizada y acumula el resultado en user_statistics_summary/deck_statistics_summary.
// El ganador es el único jugador que llega vivo (is_eliminated = false) al final; si
// quedan 0 o 2+ jugadores vivos (partida cortada a mano) no se cuenta como victoria
// de nadie, pero games_played sí se cuenta para todos los participantes.
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

// soleSurvivor devuelve el ID del único jugador no eliminado, si hay exactamente uno.
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

// peakLifeTotals reconstruye, para cada jugador, el mayor life_total alcanzado durante
// la partida, repitiendo el efecto de cada acción en orden cronológico (el mismo cálculo
// que aplica game-actions al registrar cada acción, arrancando del baseline de 40).
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

// playerActionTotals suma el daño y las eliminaciones explícitas atribuibles a un
// jugador como actor dentro de una partida. Las auto-eliminaciones por vida/veneno
// (ver game-actions) no quedan atribuidas a un actor específico en el log, así que
// solo cuentan las acciones Elimination explícitas contra otro jugador.
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
