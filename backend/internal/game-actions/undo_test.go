package gameactions_test

import (
	"context"
	"errors"
	"testing"

	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
	"github.com/usuario/commander-companion-backend/internal/games"
	"github.com/usuario/commander-companion-backend/internal/testutil"
)

func mustUndo(t *testing.T, g activeGame, callerID string, actionIDs ...string) []gameactions.GameActionResponse {
	t.Helper()
	res, err := g.actions.UndoActions(context.Background(), g.gameID, callerID, gameactions.UndoActionsRequest{ActionIDs: actionIDs})
	if err != nil {
		t.Fatalf("UndoActions(%v) error = %v, want nil", actionIDs, err)
	}
	return res
}

func mustGetGame(t *testing.T, g activeGame) *games.GameResponse {
	t.Helper()
	game, err := g.games.GetGame(context.Background(), g.gameID, g.user1ID)
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	return game
}

func TestUndoActions_LethalHitAndElimination_BringsThePlayerBack(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	hit := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCombatDamage, Payload: amountPayload(40),
	})
	elimination := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeElimination,
	})
	if !playerByID(t, mustGetGame(t, g).Players, g.player2ID).IsEliminated {
		t.Fatalf("precondition: the lethal hit should eliminate player 2")
	}

	undone := mustUndo(t, g, g.user1ID, elimination.ID, hit.ID)

	if len(undone) != 2 || undone[0].UndoneAt == nil || undone[1].UndoneAt == nil {
		t.Fatalf("UndoActions() = %+v, want both actions returned with undone_at", undone)
	}
	target := playerByID(t, mustGetGame(t, g).Players, g.player2ID)
	if target.LifeTotal != defaultLifeTotalForTest || target.IsEliminated {
		t.Fatalf("after undo: life=%d eliminated=%v, want %d and false", target.LifeTotal, target.IsEliminated, defaultLifeTotalForTest)
	}
	timeline, err := g.actions.GetTimeline(context.Background(), g.gameID, g.user1ID)
	if err != nil || len(timeline) != 0 {
		t.Fatalf("GetTimeline() = %+v, %v, want no actions (both undone)", timeline, err)
	}
}

func TestUndoActions_RevertsLifePoisonAndCommanderDamage(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	life := mustRecordAction(t, g.actions, g.gameID, g.user2ID, gameactions.CreateActionRequest{
		ActorID: g.player2ID, ActionType: actionTypeLifeChange, Payload: amountPayload(5),
	})
	poison := mustRecordAction(t, g.actions, g.gameID, g.user2ID, gameactions.CreateActionRequest{
		ActorID: g.player2ID, ActionType: actionTypePoisonCounter, Payload: amountPayload(3),
	})
	commander := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCommanderDamage, Payload: amountPayload(21),
	})
	if !playerByID(t, mustGetGame(t, g).Players, g.player2ID).IsEliminated {
		t.Fatalf("precondition: 21 commander damage should eliminate player 2")
	}

	mustUndo(t, g, g.user1ID, commander.ID)
	mustUndo(t, g, g.user2ID, poison.ID, life.ID)

	target := playerByID(t, mustGetGame(t, g).Players, g.player2ID)
	if target.LifeTotal != defaultLifeTotalForTest || target.PoisonCounters != 0 || target.IsEliminated {
		t.Fatalf("after undo: %+v, want life=%d poison=0 not eliminated", target, defaultLifeTotalForTest)
	}
	// The per-attacker commander damage is reverted too: 20 more no longer eliminate.
	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCommanderDamage, Payload: amountPayload(20),
	})
	if playerByID(t, mustGetGame(t, g).Players, g.player2ID).IsEliminated {
		t.Fatalf("20 commander damage after undoing 21 should not eliminate")
	}
}

func TestUndoActions_PassTurn_GivesTheTurnBack(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, ActionType: actionTypeTurnStart,
	})
	end := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, ActionType: actionTypeTurnEnd, Payload: map[string]interface{}{"duration_ms": float64(1000)},
	})
	start := mustRecordAction(t, g.actions, g.gameID, g.user2ID, gameactions.CreateActionRequest{
		ActorID: g.player2ID, ActionType: actionTypeTurnStart,
	})

	mustUndo(t, g, g.user2ID, start.ID)
	mustUndo(t, g, g.user1ID, end.ID)

	game := mustGetGame(t, g)
	if game.CurrentTurnPlayerID == nil || *game.CurrentTurnPlayerID != g.player1ID {
		t.Fatalf("CurrentTurnPlayerID = %v, want %s back", game.CurrentTurnPlayerID, g.player1ID)
	}
}

func TestUndoActions_Errors(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	hit := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCombatDamage, Payload: amountPayload(5),
	})

	cases := []struct {
		name    string
		caller  string
		ids     []string
		wantErr error
	}{
		{"no ids", g.user1ID, nil, gameactions.ErrUndoActionIDsRequired},
		{"malformed id", g.user1ID, []string{"nope"}, gameactions.ErrInvalidActionID},
		{"unknown id", g.user1ID, []string{"00000000-0000-0000-0000-000000000000"}, gameactions.ErrActionNotFound},
		// Only who could record the action (its actor's owner) may undo it -- not its target.
		{"not the actor's owner", g.user2ID, []string{hit.ID}, gameactions.ErrNotAuthorizedForActor},
	}
	for _, tc := range cases {
		_, err := g.actions.UndoActions(context.Background(), g.gameID, tc.caller, gameactions.UndoActionsRequest{ActionIDs: tc.ids})
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("%s: UndoActions() error = %v, want %v", tc.name, err, tc.wantErr)
		}
	}

	mustUndo(t, g, g.user1ID, hit.ID)
	_, err := g.actions.UndoActions(context.Background(), g.gameID, g.user1ID, gameactions.UndoActionsRequest{ActionIDs: []string{hit.ID}})
	if !errors.Is(err, gameactions.ErrActionAlreadyUndone) {
		t.Fatalf("undoing twice: error = %v, want ErrActionAlreadyUndone", err)
	}
}

func TestUndoActions_FinishedGame_IsRejected(t *testing.T) {
	pool := testutil.DB(t)
	truncateGameActionsTables(t, pool)
	g := setupActiveGame(t, pool)

	hit := mustRecordAction(t, g.actions, g.gameID, g.user1ID, gameactions.CreateActionRequest{
		ActorID: g.player1ID, TargetID: g.player2ID, ActionType: actionTypeCombatDamage, Payload: amountPayload(40),
	})
	if _, err := g.games.FinishGame(context.Background(), g.gameID, g.user1ID); err != nil {
		t.Fatalf("FinishGame() error = %v", err)
	}

	_, err := g.actions.UndoActions(context.Background(), g.gameID, g.user1ID, gameactions.UndoActionsRequest{ActionIDs: []string{hit.ID}})
	if !errors.Is(err, gameactions.ErrGameNotActive) {
		t.Fatalf("UndoActions() on a finished game: error = %v, want ErrGameNotActive", err)
	}
}
