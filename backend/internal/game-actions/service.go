package gameactions

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
	statusActive = "active"

	// Umbrales de eliminación automática según las reglas estándar de Commander.
	eliminationLifeTotal      = 0
	eliminationPoisonCounters = 10

	actionLifeChange      = "LifeChange"
	actionCombatDamage    = "CombatDamage"
	actionCommanderDamage = "CommanderDamage"
	actionPoisonCounter   = "PoisonCounter"
	actionTurnStart       = "TurnStart"
	actionTurnEnd         = "TurnEnd"
	actionElimination     = "Elimination"
)

func isValidActionType(actionType string) bool {
	switch actionType {
	case actionLifeChange, actionCombatDamage, actionCommanderDamage,
		actionPoisonCounter, actionTurnStart, actionTurnEnd, actionElimination:
		return true
	default:
		return false
	}
}

var (
	// ErrGameNotFound indica que la partida no existe.
	ErrGameNotFound = common.NotFound("game not found")
	// ErrPlayerNotInGame indica que el actor/target indicado no tiene asiento en esa partida.
	ErrPlayerNotInGame = common.NotFound("player not found in this game")
	// ErrInvalidActionType indica que el action_type no pertenece al vocabulario soportado.
	ErrInvalidActionType = common.InvalidInput("invalid action_type")
	// ErrInvalidActorID indica que el actor_id recibido no es un UUID válido.
	ErrInvalidActorID = common.InvalidInput("invalid actor_id")
	// ErrInvalidTargetID indica que el target_id recibido no es un UUID válido.
	ErrInvalidTargetID = common.InvalidInput("invalid target_id")
	// ErrAmountRequired indica que la acción necesita un payload.amount y no lo trae.
	ErrAmountRequired = common.InvalidInput("payload.amount is required")
	// ErrAmountNotNumeric indica que payload.amount no es un número.
	ErrAmountNotNumeric = common.InvalidInput("payload.amount must be a number")
	// ErrGameNotActive indica que solo se pueden registrar acciones en una partida activa.
	ErrGameNotActive = common.Conflict("game is not active")
)

// Broadcaster es lo que game-actions necesita para retransmitir en vivo, por
// WebSocket, una acción recién registrada a los clientes conectados a esa partida
// (permite mockearlo en tests y evita que este paquete dependa de internal/websocket,
// mismo patrón que games.StatisticsRecalculator). El broadcast es best-effort y
// asíncrono: la implementación nunca debe bloquear ni fallar RecordAction. Ver
// ADR-0005 (docs/decisions/0005-websocket-protocol.md).
type Broadcaster interface {
	BroadcastAction(gameID string, action *GameActionResponse)
}

// Service define la lógica de negocio del módulo game-actions.
type Service interface {
	RecordAction(ctx context.Context, gameID string, req CreateActionRequest) (*GameActionResponse, error)
	GetTimeline(ctx context.Context, gameID string) ([]GameActionResponse, error)
}

type service struct {
	repo        *Queries
	broadcaster Broadcaster
}

// NewService crea un nuevo servicio de game-actions.
func NewService(db *pgxpool.Pool, broadcaster Broadcaster) Service {
	return &service{repo: New(db), broadcaster: broadcaster}
}

// RecordAction registra una nueva acción (LifeChange, CombatDamage, CommanderDamage,
// PoisonCounter, TurnStart, TurnEnd, Elimination) dentro de una partida activa, y
// aplica sus efectos sobre el estado del jugador afectado (vida, veneno, eliminación).
func (s *service) RecordAction(
	ctx context.Context, gameID string, req CreateActionRequest,
) (*GameActionResponse, error) {
	if !isValidActionType(req.ActionType) {
		return nil, ErrInvalidActionType
	}

	gid, err := s.resolveActiveGame(ctx, gameID)
	if err != nil {
		return nil, err
	}

	actorID, subject, targetID, err := s.resolveActionSubject(ctx, gid, req)
	if err != nil {
		return nil, err
	}

	err = s.applyAction(ctx, req.ActionType, subject, req.Payload)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("encoding payload: %w", err)
	}

	action, err := s.repo.CreateGameAction(ctx, CreateGameActionParams{
		GameID:     gid,
		ActorID:    actorID,
		TargetID:   targetID,
		ActionType: req.ActionType,
		Payload:    payload,
	})
	if err != nil {
		return nil, fmt.Errorf("recording action: %w", err)
	}

	res := toGameActionResponse(&action)
	s.broadcaster.BroadcastAction(gameID, res)
	return res, nil
}

// resolveActiveGame valida que la partida exista y esté activa, y devuelve su ID parseado.
func (s *service) resolveActiveGame(ctx context.Context, gameID string) (pgtype.UUID, error) {
	gid, err := common.ParseUUID(gameID)
	if err != nil {
		return pgtype.UUID{}, ErrGameNotFound
	}

	game, err := s.repo.GetGame(ctx, gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, ErrGameNotFound
		}
		return pgtype.UUID{}, fmt.Errorf("looking up game: %w", err)
	}
	if game.Status != statusActive {
		return pgtype.UUID{}, ErrGameNotActive
	}
	return gid, nil
}

// resolveActionSubject resuelve el actor y, si se indicó, el target de la acción. El
// sujeto sobre el que se aplican los efectos es el target si se indicó uno; si no, el
// propio actor (p. ej. LifeChange sobre uno mismo, o Elimination por rendición voluntaria).
func (s *service) resolveActionSubject(
	ctx context.Context, gid pgtype.UUID, req CreateActionRequest,
) (actorID pgtype.UUID, subject *GamePlayer, targetID pgtype.UUID, err error) {
	actorID, err = common.ParseUUID(req.ActorID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, ErrInvalidActorID
	}
	actor, err := s.getGamePlayer(ctx, gid, actorID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, err
	}

	if req.TargetID == "" {
		return actorID, actor, pgtype.UUID{}, nil
	}

	targetID, err = common.ParseUUID(req.TargetID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, ErrInvalidTargetID
	}
	target, err := s.getGamePlayer(ctx, gid, targetID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, err
	}
	return actorID, target, targetID, nil
}

// GetTimeline devuelve el historial completo de acciones de una partida.
func (s *service) GetTimeline(ctx context.Context, gameID string) ([]GameActionResponse, error) {
	gid, err := common.ParseUUID(gameID)
	if err != nil {
		return nil, ErrGameNotFound
	}
	_, err = s.repo.GetGame(ctx, gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGameNotFound
		}
		return nil, fmt.Errorf("looking up game: %w", err)
	}

	actions, err := s.repo.ListGameActions(ctx, gid)
	if err != nil {
		return nil, fmt.Errorf("listing game actions: %w", err)
	}

	result := make([]GameActionResponse, 0, len(actions))
	for i := range actions {
		result = append(result, *toGameActionResponse(&actions[i]))
	}
	return result, nil
}

// applyAction muta el estado del jugador afectado según el tipo de acción.
func (s *service) applyAction(
	ctx context.Context, actionType string, player *GamePlayer, payload map[string]interface{},
) error {
	switch actionType {
	case actionLifeChange:
		amount, err := payloadAmount(payload)
		if err != nil {
			return err
		}
		return s.adjustLife(ctx, player.ID, amount)
	case actionCombatDamage, actionCommanderDamage:
		// Nota: no se distingue el origen del daño de comandante por oponente
		// (regla de 21 letal de una sola fuente); el esquema hoy solo trackea
		// life_total agregado. Deferido: requeriría una tabla de daño por par
		// jugador-comandante, fuera de alcance de esta pasada.
		amount, err := payloadAmount(payload)
		if err != nil {
			return err
		}
		return s.adjustLife(ctx, player.ID, -amount)
	case actionPoisonCounter:
		amount, err := payloadAmount(payload)
		if err != nil {
			return err
		}
		return s.adjustPoison(ctx, player.ID, amount)
	case actionElimination:
		return s.eliminate(ctx, player.ID)
	case actionTurnStart, actionTurnEnd:
		// Marcadores de log puro: el esquema no trackea de quién es el turno actual.
		return nil
	default:
		return ErrInvalidActionType
	}
}

func (s *service) adjustLife(ctx context.Context, playerID pgtype.UUID, delta int32) error {
	updated, err := s.repo.AdjustGamePlayerLife(ctx, AdjustGamePlayerLifeParams{ID: playerID, Delta: delta})
	if err != nil {
		return fmt.Errorf("adjusting life total: %w", err)
	}
	if !updated.IsEliminated.Bool && updated.LifeTotal.Int32 <= eliminationLifeTotal {
		return s.eliminate(ctx, updated.ID)
	}
	return nil
}

func (s *service) adjustPoison(ctx context.Context, playerID pgtype.UUID, delta int32) error {
	updated, err := s.repo.AdjustGamePlayerPoison(ctx, AdjustGamePlayerPoisonParams{ID: playerID, Delta: delta})
	if err != nil {
		return fmt.Errorf("adjusting poison counters: %w", err)
	}
	if !updated.IsEliminated.Bool && updated.PoisonCounters.Int32 >= eliminationPoisonCounters {
		return s.eliminate(ctx, updated.ID)
	}
	return nil
}

func (s *service) eliminate(ctx context.Context, playerID pgtype.UUID) error {
	_, err := s.repo.SetGamePlayerEliminated(ctx, SetGamePlayerEliminatedParams{
		ID:           playerID,
		IsEliminated: pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("eliminating player: %w", err)
	}
	return nil
}

func (s *service) getGamePlayer(ctx context.Context, gameID, playerID pgtype.UUID) (*GamePlayer, error) {
	player, err := s.repo.GetGamePlayer(ctx, playerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlayerNotInGame
		}
		return nil, fmt.Errorf("looking up game player: %w", err)
	}
	if player.GameID != gameID {
		return nil, ErrPlayerNotInGame
	}
	return &player, nil
}

func payloadAmount(payload map[string]interface{}) (int32, error) {
	raw, ok := payload["amount"]
	if !ok {
		return 0, ErrAmountRequired
	}
	amount, ok := raw.(float64)
	if !ok {
		return 0, ErrAmountNotNumeric
	}
	return int32(amount), nil
}

func toGameActionResponse(action *GameAction) *GameActionResponse {
	res := &GameActionResponse{
		ID:         action.ID.String(),
		GameID:     action.GameID.String(),
		ActorID:    action.ActorID.String(),
		ActionType: action.ActionType,
		CreatedAt:  action.CreatedAt.Time.Format(time.RFC3339),
	}
	if action.TargetID.Valid {
		tid := action.TargetID.String()
		res.TargetID = &tid
	}
	if len(action.Payload) > 0 {
		var payload map[string]interface{}
		if err := json.Unmarshal(action.Payload, &payload); err == nil {
			res.Payload = payload
		}
	}
	return res
}
