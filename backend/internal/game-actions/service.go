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
	// eliminationCommanderDamage: 21+ de daño de comandante de una misma fuente
	// elimina, independiente del life_total agregado (ver applyCommanderDamage).
	eliminationCommanderDamage = 21

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
	// ErrCommanderDamageTargetRequired indica que CommanderDamage necesita un
	// target_id distinto del actor (no tiene sentido sin un defensor identificado,
	// ya que el daño se trackea por par atacante-defensor).
	ErrCommanderDamageTargetRequired = common.InvalidInput("commander damage requires a target_id different from actor_id")
	// ErrNotAuthorizedForActor indica que el caller no es dueño del GamePlayer actor
	// ni quien lo unió como proxy (game_players.added_by, ver ADR-0013).
	ErrNotAuthorizedForActor = common.Forbidden("not authorized to act on behalf of this player")
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
	// RecordAction registra una acción. callerUserID es el usuario autenticado
	// (JWT): debe ser el dueño del GamePlayer actor o quien lo unió como proxy
	// (ver ADR-0013), o se rechaza con ErrNotAuthorizedForActor.
	RecordAction(ctx context.Context, gameID, callerUserID string, req CreateActionRequest) (*GameActionResponse, error)
	GetTimeline(ctx context.Context, gameID string) ([]GameActionResponse, error)
}

type service struct {
	repo        *Queries
	pool        *pgxpool.Pool
	broadcaster Broadcaster
}

// NewService crea un nuevo servicio de game-actions.
func NewService(db *pgxpool.Pool, broadcaster Broadcaster) Service {
	return &service{repo: New(db), pool: db, broadcaster: broadcaster}
}

// RecordAction registra una nueva acción (LifeChange, CombatDamage, CommanderDamage,
// PoisonCounter, TurnStart, TurnEnd, Elimination) dentro de una partida activa, y
// aplica sus efectos sobre el estado del jugador afectado (vida, veneno, eliminación).
//
// El camino de escritura completo (resolución de actor/target, mutación de estado, y
// el log de la acción) corre dentro de una única transacción: antes de CommanderDamage
// (que necesita dos escrituras atómicas — la tabla de daño por fuente y life_total)
// cada paso era una llamada independiente, con la posibilidad de que un crash a mitad
// de camino dejara un cambio de vida aplicado sin su entrada de log correspondiente.
func (s *service) RecordAction(
	ctx context.Context, gameID, callerUserID string, req CreateActionRequest,
) (*GameActionResponse, error) {
	if !isValidActionType(req.ActionType) {
		return nil, ErrInvalidActionType
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.repo.WithTx(tx)

	gid, err := s.resolveActiveGame(ctx, q, gameID)
	if err != nil {
		return nil, err
	}

	actorID, subject, targetID, err := s.resolveActionSubject(ctx, q, gid, callerUserID, req)
	if err != nil {
		return nil, err
	}

	err = s.applyAction(ctx, q, req.ActionType, gid, actorID, targetID.Valid, subject, req.Payload)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("encoding payload: %w", err)
	}

	action, err := q.CreateGameAction(ctx, CreateGameActionParams{
		GameID:     gid,
		ActorID:    actorID,
		TargetID:   targetID,
		ActionType: req.ActionType,
		Payload:    payload,
	})
	if err != nil {
		return nil, fmt.Errorf("recording action: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	res := toGameActionResponse(&action)
	s.broadcaster.BroadcastAction(gameID, res)
	return res, nil
}

// resolveActiveGame valida que la partida exista y esté activa, y devuelve su ID parseado.
func (s *service) resolveActiveGame(ctx context.Context, q *Queries, gameID string) (pgtype.UUID, error) {
	gid, err := common.ParseUUID(gameID)
	if err != nil {
		return pgtype.UUID{}, ErrGameNotFound
	}

	game, err := q.GetGame(ctx, gid)
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
	ctx context.Context, q *Queries, gid pgtype.UUID, callerUserID string, req CreateActionRequest,
) (actorID pgtype.UUID, subject *GamePlayer, targetID pgtype.UUID, err error) {
	actorID, err = common.ParseUUID(req.ActorID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, ErrInvalidActorID
	}
	actor, err := s.getGamePlayer(ctx, q, gid, actorID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, err
	}
	if authErr := authorizeActor(actor, callerUserID); authErr != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, authErr
	}

	if req.TargetID == "" {
		return actorID, actor, pgtype.UUID{}, nil
	}

	targetID, err = common.ParseUUID(req.TargetID)
	if err != nil {
		return pgtype.UUID{}, nil, pgtype.UUID{}, ErrInvalidTargetID
	}
	target, err := s.getGamePlayer(ctx, q, gid, targetID)
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

// applyAction muta el estado del jugador afectado según el tipo de acción. player es
// el sujeto de la mutación (target si se indicó, si no el propio actor); actorID y
// hasTarget se necesitan aparte para CommanderDamage, que trackea daño por par
// atacante-defensor y por eso sí distingue al actor del sujeto (a diferencia del
// resto de las acciones, para las que alcanza con el sujeto).
func (s *service) applyAction(
	ctx context.Context, q *Queries, actionType string, gid, actorID pgtype.UUID, hasTarget bool,
	player *GamePlayer, payload map[string]interface{},
) error {
	switch actionType {
	case actionLifeChange:
		return s.applyLifeDelta(ctx, q, player.ID, payload, 1)
	case actionCombatDamage:
		return s.applyLifeDelta(ctx, q, player.ID, payload, -1)
	case actionCommanderDamage:
		return s.applyCommanderDamage(ctx, q, gid, actorID, player.ID, hasTarget, payload)
	case actionPoisonCounter:
		amount, err := payloadAmount(payload)
		if err != nil {
			return err
		}
		return s.adjustPoison(ctx, q, player.ID, amount)
	case actionElimination:
		return s.eliminate(ctx, q, player.ID)
	case actionTurnStart:
		return s.setCurrentTurn(ctx, q, gid, actorID)
	case actionTurnEnd:
		return s.clearCurrentTurn(ctx, q, gid)
	default:
		return ErrInvalidActionType
	}
}

func (s *service) adjustLife(ctx context.Context, q *Queries, playerID pgtype.UUID, delta int32) error {
	updated, err := q.AdjustGamePlayerLife(ctx, AdjustGamePlayerLifeParams{ID: playerID, Delta: delta})
	if err != nil {
		return fmt.Errorf("adjusting life total: %w", err)
	}
	if !updated.IsEliminated.Bool && updated.LifeTotal.Int32 <= eliminationLifeTotal {
		return s.eliminate(ctx, q, updated.ID)
	}
	return nil
}

// applyLifeDelta lee payload.amount y lo aplica a life_total multiplicado por sign:
// 1 para LifeChange (el amount ya viene con el signo que quiere el cliente), -1 para
// CombatDamage (amount es siempre "cuánto daño", nunca negativo).
func (s *service) applyLifeDelta(
	ctx context.Context, q *Queries, playerID pgtype.UUID, payload map[string]interface{}, sign int32,
) error {
	amount, err := payloadAmount(payload)
	if err != nil {
		return err
	}
	return s.adjustLife(ctx, q, playerID, sign*amount)
}

// applyCommanderDamage acumula el daño de comandante del atacante contra el defensor
// (commander_damage.amount, upsert atómico) y aplica la misma cantidad a life_total
// como cualquier otro daño. Elimina al defensor si su vida llega a 0, igual que
// adjustLife, o si el daño acumulado de ESTE atacante llega a 21 (regla real de
// Commander), aunque le quede vida positiva de otras fuentes. Requiere un target_id
// distinto del actor (no tiene sentido trackear daño de comandante sin un defensor
// identificado).
func (s *service) applyCommanderDamage(
	ctx context.Context, q *Queries, gid, attackerID, defenderID pgtype.UUID, hasTarget bool,
	payload map[string]interface{},
) error {
	if !hasTarget || defenderID == attackerID {
		return ErrCommanderDamageTargetRequired
	}
	amount, err := payloadAmount(payload)
	if err != nil {
		return err
	}

	cd, err := q.UpsertCommanderDamage(ctx, UpsertCommanderDamageParams{
		GameID:     gid,
		AttackerID: attackerID,
		DefenderID: defenderID,
		Delta:      amount,
	})
	if err != nil {
		return fmt.Errorf("recording commander damage: %w", err)
	}

	updated, err := q.AdjustGamePlayerLife(ctx, AdjustGamePlayerLifeParams{ID: defenderID, Delta: -amount})
	if err != nil {
		return fmt.Errorf("adjusting life total: %w", err)
	}

	if !updated.IsEliminated.Bool &&
		(updated.LifeTotal.Int32 <= eliminationLifeTotal || cd.Amount >= eliminationCommanderDamage) {
		return s.eliminate(ctx, q, defenderID)
	}
	return nil
}

func (s *service) adjustPoison(ctx context.Context, q *Queries, playerID pgtype.UUID, delta int32) error {
	updated, err := q.AdjustGamePlayerPoison(ctx, AdjustGamePlayerPoisonParams{ID: playerID, Delta: delta})
	if err != nil {
		return fmt.Errorf("adjusting poison counters: %w", err)
	}
	if !updated.IsEliminated.Bool && updated.PoisonCounters.Int32 >= eliminationPoisonCounters {
		return s.eliminate(ctx, q, updated.ID)
	}
	return nil
}

func (s *service) eliminate(ctx context.Context, q *Queries, playerID pgtype.UUID) error {
	_, err := q.SetGamePlayerEliminated(ctx, SetGamePlayerEliminatedParams{
		ID:           playerID,
		IsEliminated: pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("eliminating player: %w", err)
	}
	return nil
}

// setCurrentTurn fija de quién es el turno ahora (TurnStart). No modela orden de
// turno (quién sigue) — solo responde "de quién es el turno ahora".
func (s *service) setCurrentTurn(ctx context.Context, q *Queries, gid, playerID pgtype.UUID) error {
	_, err := q.SetCurrentTurnPlayer(ctx, SetCurrentTurnPlayerParams{
		ID:                  gid,
		CurrentTurnPlayerID: playerID,
	})
	if err != nil {
		return fmt.Errorf("setting current turn player: %w", err)
	}
	return nil
}

// clearCurrentTurn limpia de quién es el turno (TurnEnd); el siguiente TurnStart lo
// vuelve a fijar.
func (s *service) clearCurrentTurn(ctx context.Context, q *Queries, gid pgtype.UUID) error {
	_, err := q.SetCurrentTurnPlayer(ctx, SetCurrentTurnPlayerParams{ID: gid})
	if err != nil {
		return fmt.Errorf("clearing current turn player: %w", err)
	}
	return nil
}

// authorizeActor exige que callerUserID sea el dueño del GamePlayer actor o quien lo
// unió como proxy (added_by, ver ADR-0013). Antes de esto, RecordAction no validaba
// en absoluto que actor_id perteneciera al caller — cualquier usuario autenticado que
// conociera un game_id y un actor_id podía registrar acciones en su nombre.
func authorizeActor(actor *GamePlayer, callerUserID string) error {
	if actor.UserID.String() == callerUserID {
		return nil
	}
	if actor.AddedBy.Valid && actor.AddedBy.String() == callerUserID {
		return nil
	}
	return ErrNotAuthorizedForActor
}

func (s *service) getGamePlayer(ctx context.Context, q *Queries, gameID, playerID pgtype.UUID) (*GamePlayer, error) {
	player, err := q.GetGamePlayer(ctx, playerID)
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
