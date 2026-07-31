package websocket

import (
	"log"

	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
)

// This file adapts *Hub to the Broadcaster interfaces that game-actions and games
// define on the consumer side (same pattern as games.StatisticsRecalculator, see
// backend/internal/games/service.go): neither of those two packages imports
// internal/websocket, it's websocket that imports their types to be able to satisfy their
// interfaces. See ADR-0005.

// BroadcastAction implements gameactions.Broadcaster: it rebroadcasts an action that was
// just recorded to all connections subscribed to that game. It's best-effort and doesn't
// block: it must never delay or fail the HTTP request that originated it.
func (h *Hub) BroadcastAction(gameID string, action *gameactions.GameActionResponse) {
	msg, err := encodeEnvelope(EventGameAction, gameID, action.ActorID, action)
	if err != nil {
		// There's nothing reasonable to do with an encoding error here: the HTTP request
		// that recorded the action already responded successfully. It's logged and execution continues.
		log.Printf("websocket: encoding game_action envelope for game %s: %v", gameID, err)
		return
	}
	h.Broadcast(gameID, msg)
}

// BroadcastGameFinished implements games.Broadcaster: notifies the whole room that the
// game ended and closes its connections (see ADR-0005: there can no longer be
// game_actions for a finished game, so there's no point keeping the socket open).
func (h *Hub) BroadcastGameFinished(gameID string) {
	msg, err := encodeEnvelope(EventGameFinished, gameID, "", nil)
	if err != nil {
		log.Printf("websocket: encoding game_finished envelope for game %s: %v", gameID, err)
		return
	}
	h.Broadcast(gameID, msg)
	h.CloseRoom(gameID)
}
