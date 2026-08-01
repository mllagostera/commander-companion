package gameactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
)

const (
	statusActive = "active"

	// Automatic elimination thresholds per standard Commander rules.
	eliminationLifeTotal      = 0
	eliminationPoisonCounters = 10
	// eliminationCommanderDamage: 21+ commander damage from a single source
	// eliminates, regardless of aggregate life_total (see applyCommanderDamage).
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
	// ErrGameNotFound indicates that the game doesn't exist.
	ErrGameNotFound = common.NotFound("game not found")
	// ErrPlayerNotInGame indicates that the given actor/target doesn't have a seat in that game.
	ErrPlayerNotInGame = common.NotFound("player not found in this game")
	// ErrInvalidActionType indicates that action_type isn't part of the supported vocabulary.
	ErrInvalidActionType = common.InvalidInput("invalid action_type")
	// ErrInvalidActorID indicates that the received actor_id isn't a valid UUID.
	ErrInvalidActorID = common.InvalidInput("invalid actor_id")
	// ErrInvalidTargetID indicates that the received target_id isn't a valid UUID.
	ErrInvalidTargetID = common.InvalidInput("invalid target_id")
	// ErrAmountRequired indicates that the action needs a payload.amount and doesn't have one.
	ErrAmountRequired = common.InvalidInput("payload.amount is required")
	// ErrAmountNotNumeric indicates that payload.amount isn't a (whole) number.
	ErrAmountNotNumeric = common.InvalidInput("payload.amount must be a whole number")
	// ErrAmountOutOfRange indicates that payload.amount's magnitude is
	// unreasonably large (see maxAmountMagnitude).
	ErrAmountOutOfRange = common.InvalidInput("payload.amount is out of range")
	// ErrAmountMustBePositive indicates that payload.amount must be a positive
	// quantity of damage (CombatDamage/CommanderDamage): zero or negative
	// doesn't mean "no damage", it would silently heal or no-op the target.
	ErrAmountMustBePositive = common.InvalidInput("payload.amount must be a positive number")
	// ErrGameNotActive indicates that actions can only be recorded in an active game.
	ErrGameNotActive = common.Conflict("game is not active")
	// ErrCommanderDamageTargetRequired indicates that CommanderDamage needs a
	// target_id different from the actor (it doesn't make sense without an
	// identified defender, since the damage is tracked per attacker-defender pair).
	ErrCommanderDamageTargetRequired = common.InvalidInput("commander damage requires a target_id different from actor_id")
	// ErrNotAuthorizedForActor indicates that the caller isn't the owner of the
	// actor GamePlayer nor the one who added them as a proxy (game_players.added_by, see ADR-0013).
	ErrNotAuthorizedForActor = common.Forbidden("not authorized to act on behalf of this player")
)

// Broadcaster is what game-actions needs to relay a just-recorded action live, via
// WebSocket, to the clients connected to that game (allows mocking it in tests and
// keeps this package from depending on internal/websocket, same pattern as
// games.StatisticsRecalculator). The broadcast is best-effort and
// asynchronous: the implementation must never block or fail RecordAction. See
// ADR-0005 (docs/decisions/0005-websocket-protocol.md).
type Broadcaster interface {
	BroadcastAction(gameID string, action *GameActionResponse)
}

// Service defines the business logic of the game-actions module.
type Service interface {
	// RecordAction records an action. callerUserID is the authenticated user
	// (JWT): they must be the owner of the actor GamePlayer or the one who added
	// them as a proxy (see ADR-0013), or it's rejected with ErrNotAuthorizedForActor.
	RecordAction(ctx context.Context, gameID, callerUserID string, req CreateActionRequest) (*GameActionResponse, error)
	// GetTimeline returns the action history of a game, if callerUserID holds
	// a seat in it.
	GetTimeline(ctx context.Context, gameID, callerUserID string) ([]GameActionResponse, error)
}

type service struct {
	repo        *Queries
	pool        *pgxpool.Pool
	broadcaster Broadcaster
}

// NewService creates a new game-actions service.
func NewService(db *pgxpool.Pool, broadcaster Broadcaster) Service {
	return &service{repo: New(db), pool: db, broadcaster: broadcaster}
}

// RecordAction records a new action (LifeChange, CombatDamage, CommanderDamage,
// PoisonCounter, TurnStart, TurnEnd, Elimination) within an active game, and
// applies its effects to the affected player's state (life, poison, elimination).
//
// The complete write path (actor/target resolution, state mutation, and the
// action log) runs inside a single transaction: before CommanderDamage
// (which needs two atomic writes — the per-source damage table and life_total)
// each step was an independent call, with the possibility that a crash mid-way
// through would leave a life change applied without its corresponding log entry.
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

// resolveActiveGame validates that the game exists and is active, and returns its parsed ID.
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

// resolveActionSubject resolves the actor and, if given, the target of the action. The
// subject the effects are applied to is the target if one was given; if not, the
// actor themselves (e.g. LifeChange on oneself, or Elimination by voluntary concession).
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

// GetTimeline returns the complete action history of a game, if
// callerUserID holds a seat in it. Denies with ErrGameNotFound, not a 403,
// same "don't reveal" pattern as the rest of the module (see
// authorizeActor and ADR-0013).
func (s *service) GetTimeline(ctx context.Context, gameID, callerUserID string) ([]GameActionResponse, error) {
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

	uid, err := common.ParseUUID(callerUserID)
	if err != nil {
		return nil, common.ErrInvalidUser
	}
	if _, membershipErr := s.repo.GetGamePlayerByGameAndUser(
		ctx, GetGamePlayerByGameAndUserParams{GameID: gid, UserID: uid},
	); membershipErr != nil {
		if errors.Is(membershipErr, pgx.ErrNoRows) {
			return nil, ErrGameNotFound
		}
		return nil, fmt.Errorf("checking game membership: %w", membershipErr)
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

// applyAction mutates the affected player's state according to the action type. player
// is the subject of the mutation (target if given, otherwise the actor themselves);
// actorID and hasTarget are needed separately for CommanderDamage, which tracks
// damage per attacker-defender pair and therefore does distinguish the actor from the
// subject (unlike the rest of the actions, for which the subject is enough).
func (s *service) applyAction(
	ctx context.Context, q *Queries, actionType string, gid, actorID pgtype.UUID, hasTarget bool,
	player *GamePlayer, payload map[string]interface{},
) error {
	switch actionType {
	case actionLifeChange:
		return s.applyLifeChange(ctx, q, player.ID, payload)
	case actionCombatDamage:
		return s.applyCombatDamage(ctx, q, player.ID, payload)
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

// applyLifeChange reads payload.amount (signed: the client already chose whether
// this is a gain or a loss) and applies it directly to life_total.
func (s *service) applyLifeChange(
	ctx context.Context, q *Queries, playerID pgtype.UUID, payload map[string]interface{},
) error {
	amount, err := payloadAmount(payload)
	if err != nil {
		return err
	}
	return s.adjustLife(ctx, q, playerID, amount)
}

// applyCombatDamage reads payload.amount and subtracts it from life_total.
// Unlike LifeChange, the amount is never negative here — negative "damage" would
// silently heal the target instead of hurting them (see payloadPositiveAmount).
func (s *service) applyCombatDamage(
	ctx context.Context, q *Queries, playerID pgtype.UUID, payload map[string]interface{},
) error {
	amount, err := payloadPositiveAmount(payload)
	if err != nil {
		return err
	}
	return s.adjustLife(ctx, q, playerID, -amount)
}

// applyCommanderDamage accumulates the attacker's commander damage against the
// defender (commander_damage.amount, atomic upsert) and applies the same amount to
// life_total like any other damage. It eliminates the defender if their life reaches
// 0, same as adjustLife, or if THIS attacker's accumulated damage reaches 21 (real
// Commander rule), even if they still have positive life from other sources. Requires
// a target_id different from the actor (it doesn't make sense to track commander
// damage without an identified defender).
func (s *service) applyCommanderDamage(
	ctx context.Context, q *Queries, gid, attackerID, defenderID pgtype.UUID, hasTarget bool,
	payload map[string]interface{},
) error {
	if !hasTarget || defenderID == attackerID {
		return ErrCommanderDamageTargetRequired
	}
	amount, err := payloadPositiveAmount(payload)
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

// setCurrentTurn sets whose turn it is now (TurnStart). It doesn't model turn
// order (who's next) — it only answers "whose turn is it now".
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

// clearCurrentTurn clears whose turn it is (TurnEnd); the next TurnStart
// sets it again.
func (s *service) clearCurrentTurn(ctx context.Context, q *Queries, gid pgtype.UUID) error {
	_, err := q.SetCurrentTurnPlayer(ctx, SetCurrentTurnPlayerParams{ID: gid})
	if err != nil {
		return fmt.Errorf("clearing current turn player: %w", err)
	}
	return nil
}

// authorizeActor requires that callerUserID be the owner of the actor GamePlayer or
// the one who added them as a proxy (added_by, see ADR-0013). Before this,
// RecordAction didn't validate at all that actor_id belonged to the caller — any
// authenticated user who knew a game_id and an actor_id could record actions on their behalf.
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

// maxAmountMagnitude bounds any single action's payload.amount, in both
// directions. It's set far above anything a real Commander game needs (life
// totals and poison counters never get remotely close to it) purely to reject
// garbage input before it reaches life_total/poison_counters/commander_damage —
// without a bound, encoding/json decodes payload.amount into a float64, and a
// naive int32(amount) conversion on an out-of-range value doesn't error, it
// silently wraps (e.g. 3e9 becomes a large *negative* int32, so "3 billion
// damage" reads as a life gain instead of an instant kill).
const maxAmountMagnitude = 1_000_000

func payloadAmount(payload map[string]interface{}) (int32, error) {
	raw, ok := payload["amount"]
	if !ok {
		return 0, ErrAmountRequired
	}
	amount, ok := raw.(float64)
	if !ok {
		return 0, ErrAmountNotNumeric
	}
	if math.IsNaN(amount) || math.Trunc(amount) != amount {
		return 0, ErrAmountNotNumeric
	}
	if amount < -maxAmountMagnitude || amount > maxAmountMagnitude {
		return 0, ErrAmountOutOfRange
	}
	return int32(amount), nil
}

// payloadPositiveAmount is payloadAmount for the actions where "amount" means
// a quantity of damage dealt, not a signed delta (CombatDamage,
// CommanderDamage): 0 or negative doesn't mean "no damage", it would silently
// heal or no-op the target, so it's rejected instead.
func payloadPositiveAmount(payload map[string]interface{}) (int32, error) {
	amount, err := payloadAmount(payload)
	if err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, ErrAmountMustBePositive
	}
	return amount, nil
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
