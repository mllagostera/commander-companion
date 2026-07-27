package websocket

import (
	"log"

	gameactions "github.com/usuario/commander-companion-backend/internal/game-actions"
)

// Este archivo adapta *Hub a las interfaces Broadcaster que game-actions y games
// definen del lado del consumidor (mismo patrón que games.StatisticsRecalculator, ver
// backend/internal/games/service.go): ninguno de esos dos paquetes importa
// internal/websocket, es websocket el que importa sus tipos para poder satisfacer sus
// interfaces. Ver ADR-0005.

// BroadcastAction implementa gameactions.Broadcaster: retransmite una acción recién
// registrada a todas las conexiones suscriptas a esa partida. Es best-effort y no
// bloquea: nunca debe demorar ni fallar el request HTTP que la originó.
func (h *Hub) BroadcastAction(gameID string, action *gameactions.GameActionResponse) {
	msg, err := encodeEnvelope(EventGameAction, gameID, action.ActorID, action)
	if err != nil {
		// No hay nada razonable que hacer con un error de encoding acá: el request HTTP
		// que registró la acción ya respondió con éxito. Se loguea y se sigue.
		log.Printf("websocket: encoding game_action envelope for game %s: %v", gameID, err)
		return
	}
	h.Broadcast(gameID, msg)
}

// BroadcastGameFinished implementa games.Broadcaster: avisa a toda la sala que la
// partida terminó y cierra sus conexiones (ver ADR-0005: ya no puede haber más
// game_actions para una partida finished, no tiene sentido mantener el socket abierto).
func (h *Hub) BroadcastGameFinished(gameID string) {
	msg, err := encodeEnvelope(EventGameFinished, gameID, "", nil)
	if err != nil {
		log.Printf("websocket: encoding game_finished envelope for game %s: %v", gameID, err)
		return
	}
	h.Broadcast(gameID, msg)
	h.CloseRoom(gameID)
}
