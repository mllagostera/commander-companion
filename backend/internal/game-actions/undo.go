package gameactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/usuario/commander-companion-backend/internal/common"
)

// maxUndoBatch bounds how many actions a single undo request may revert. Far above a
// normal "undo the last thing" (one to three actions), but high enough for a client
// resetting a whole game's worth of its own actions in one request.
const maxUndoBatch = 1000

var (
	// ErrUndoActionIDsRequired indicates an undo request with no action_ids, or too many.
	ErrUndoActionIDsRequired = common.InvalidInput("action_ids must list between 1 and 1000 actions")
	// ErrInvalidActionID indicates that an entry of action_ids isn't a valid UUID.
	ErrInvalidActionID = common.InvalidInput("invalid action id")
	// ErrActionNotFound indicates that an action doesn't exist in that game.
	ErrActionNotFound = common.NotFound("action not found in this game")
	// ErrActionAlreadyUndone indicates that an action was already undone.
	ErrActionAlreadyUndone = common.Conflict("action already undone")
)

// UndoActions reverts the given actions of an active game, in the order given (clients
// send the most recent first), all or nothing in a single transaction. Each action's
// effects on the game state are reverted -- life, poison, commander damage -- and it's
// marked undone rather than deleted, so the log keeps it while the timeline and the
// statistics stop counting it (see ListGameActions and the statistics queries).
//
// Authorization is the same as for recording: the caller must own the action's actor
// GamePlayer or have proxy-joined it (see authorizeActor/ADR-0013). Afterwards, every
// player whose state changed is re-evaluated for elimination (undoing the lethal hit
// brings them back), and whose turn it is is recomputed if a turn action was undone.
func (s *service) UndoActions(
	ctx context.Context, gameID, callerUserID string, req UndoActionsRequest,
) ([]GameActionResponse, error) {
	if len(req.ActionIDs) == 0 || len(req.ActionIDs) > maxUndoBatch {
		return nil, ErrUndoActionIDsRequired
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

	undone := make([]GameAction, 0, len(req.ActionIDs))
	touched := make(map[pgtype.UUID]struct{})
	turnTouched := false
	for _, rawID := range req.ActionIDs {
		action, err := s.undoOne(ctx, q, gid, callerUserID, rawID)
		if err != nil {
			return nil, err
		}
		undone = append(undone, *action)
		touched[actionSubject(action)] = struct{}{}
		if action.ActionType == actionTurnStart || action.ActionType == actionTurnEnd {
			turnTouched = true
		}
	}

	for playerID := range touched {
		if err := s.reevaluateElimination(ctx, q, gid, playerID); err != nil {
			return nil, err
		}
	}
	if turnTouched {
		if err := s.restoreCurrentTurn(ctx, q, gid); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	res := make([]GameActionResponse, 0, len(undone))
	for i := range undone {
		r := toGameActionResponse(&undone[i])
		s.broadcaster.BroadcastActionUndone(gameID, r)
		res = append(res, *r)
	}
	return res, nil
}

// undoOne locks, authorizes, reverts and marks a single action.
func (s *service) undoOne(
	ctx context.Context, q *Queries, gid pgtype.UUID, callerUserID, rawID string,
) (*GameAction, error) {
	aid, err := common.ParseUUID(rawID)
	if err != nil {
		return nil, ErrInvalidActionID
	}
	action, err := q.GetGameActionForUpdate(ctx, GetGameActionForUpdateParams{ID: aid, GameID: gid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("looking up action: %w", err)
	}
	if action.UndoneAt.Valid {
		return nil, ErrActionAlreadyUndone
	}

	actor, err := s.getGamePlayer(ctx, q, gid, action.ActorID)
	if err != nil {
		return nil, err
	}
	if err := authorizeActor(actor, callerUserID); err != nil {
		return nil, err
	}

	if err := s.revertAction(ctx, q, &action); err != nil {
		return nil, err
	}
	marked, err := q.MarkGameActionUndone(ctx, aid)
	if err != nil {
		return nil, fmt.Errorf("marking action undone: %w", err)
	}
	return &marked, nil
}

// actionSubject is the player an action's effects were applied to: its target if it has
// one, otherwise the actor (same rule as resolveActionSubject).
func actionSubject(action *GameAction) pgtype.UUID {
	if action.TargetID.Valid {
		return action.TargetID
	}
	return action.ActorID
}

// revertAction applies the inverse of an action's effects. Elimination and turn actions
// have no direct effect to invert: their consequences are recomputed afterwards (see
// reevaluateElimination and restoreCurrentTurn). Life is adjusted without the automatic
// elimination adjustLife applies, since elimination is re-evaluated as a whole at the end.
func (s *service) revertAction(ctx context.Context, q *Queries, action *GameAction) error {
	switch action.ActionType {
	case actionLifeChange, actionCombatDamage, actionCommanderDamage, actionPoisonCounter:
	default:
		return nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(action.Payload, &payload); err != nil {
		return fmt.Errorf("decoding payload of action %s: %w", action.ID.String(), err)
	}
	amount, err := payloadAmount(payload)
	if err != nil {
		return err
	}
	subject := actionSubject(action)

	switch action.ActionType {
	case actionLifeChange:
		return s.adjustLifeOnly(ctx, q, subject, -amount)
	case actionCombatDamage:
		return s.adjustLifeOnly(ctx, q, subject, amount)
	case actionCommanderDamage:
		if err := q.SubtractCommanderDamage(ctx, SubtractCommanderDamageParams{
			Amount:     amount,
			AttackerID: action.ActorID,
			DefenderID: subject,
		}); err != nil {
			return fmt.Errorf("reverting commander damage: %w", err)
		}
		return s.adjustLifeOnly(ctx, q, subject, amount)
	default: // actionPoisonCounter
		if _, err := q.AdjustGamePlayerPoison(ctx, AdjustGamePlayerPoisonParams{ID: subject, Delta: -amount}); err != nil {
			return fmt.Errorf("reverting poison counters: %w", err)
		}
		return nil
	}
}

func (s *service) adjustLifeOnly(ctx context.Context, q *Queries, playerID pgtype.UUID, delta int32) error {
	if _, err := q.AdjustGamePlayerLife(ctx, AdjustGamePlayerLifeParams{ID: playerID, Delta: delta}); err != nil {
		return fmt.Errorf("reverting life total: %w", err)
	}
	return nil
}

// reevaluateElimination sets a player's is_eliminated from their state after an undo:
// eliminated if life is at 0 or below, 10+ poison, 21+ commander damage from a single
// attacker, or an explicit Elimination action against them still in force.
func (s *service) reevaluateElimination(ctx context.Context, q *Queries, gid, playerID pgtype.UUID) error {
	player, err := q.GetGamePlayer(ctx, playerID)
	if err != nil {
		return fmt.Errorf("looking up game player: %w", err)
	}
	eliminated := player.LifeTotal.Int32 <= eliminationLifeTotal ||
		player.PoisonCounters.Int32 >= eliminationPoisonCounters

	if !eliminated {
		damage, err := q.ListCommanderDamageAgainst(ctx, ListCommanderDamageAgainstParams{GameID: gid, DefenderID: playerID})
		if err != nil {
			return fmt.Errorf("listing commander damage: %w", err)
		}
		for _, d := range damage {
			if d.Amount >= eliminationCommanderDamage {
				eliminated = true
				break
			}
		}
	}
	if !eliminated {
		count, err := q.CountActiveEliminationsOf(ctx, CountActiveEliminationsOfParams{GameID: gid, TargetID: playerID})
		if err != nil {
			return fmt.Errorf("counting eliminations: %w", err)
		}
		eliminated = count > 0
	}

	if eliminated == player.IsEliminated.Bool {
		return nil
	}
	if _, err := q.SetGamePlayerEliminated(ctx, SetGamePlayerEliminatedParams{
		ID:           playerID,
		IsEliminated: pgtype.Bool{Bool: eliminated, Valid: true},
	}); err != nil {
		return fmt.Errorf("updating elimination: %w", err)
	}
	return nil
}

// restoreCurrentTurn recomputes whose turn it is from the turn actions still in force:
// the actor of the latest TurnStart, or nobody if the latest is a TurnEnd (or there's none).
func (s *service) restoreCurrentTurn(ctx context.Context, q *Queries, gid pgtype.UUID) error {
	latest, err := q.GetLatestTurnAction(ctx, gid)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return s.clearCurrentTurn(ctx, q, gid)
	case err != nil:
		return fmt.Errorf("looking up latest turn action: %w", err)
	case latest.ActionType == actionTurnStart:
		return s.setCurrentTurn(ctx, q, gid, latest.ActorID)
	default:
		return s.clearCurrentTurn(ctx, q, gid)
	}
}
