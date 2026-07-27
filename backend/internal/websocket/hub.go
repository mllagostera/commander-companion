// Package websocket implementa la sincronización en vivo de partidas por WebSocket
// (ver ADR-0005 en docs/decisions/). El Hub mantiene, en memoria de este proceso, las
// conexiones activas agrupadas por game_id y retransmite mensajes a todas las
// conexiones de una partida. No hay pub/sub externo: si el backend corre en más de una
// réplica, dos jugadores conectados a réplicas distintas no se ven entre sí (limitación
// documentada en la ADR, no un descuido).
package websocket

import "sync"

// Conn es la superficie mínima que el Hub necesita de una conexión cliente: encolar un
// mensaje para enviárselo de forma asíncrona, y cerrarla. La implementación real
// (Client, en client.go) envuelve un *websocket.Conn de gofiber/websocket/v2; los tests
// del Hub usan un fake liviano que no abre ningún socket real.
type Conn interface {
	// Send encola message para envío asíncrono. Es best-effort: si el buffer interno de
	// la conexión está lleno, la implementación puede descartar el mensaje en vez de
	// bloquear (ver ADR-0005).
	Send(message []byte)
	// Close cierra la conexión. Debe ser seguro llamarlo más de una vez.
	Close()
}

// Hub agrupa las conexiones activas por game_id y permite retransmitirles mensajes.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[Conn]struct{}
}

// NewHub crea un Hub vacío.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[Conn]struct{})}
}

// Register añade conn a la sala de gameID.
func (h *Hub) Register(gameID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[gameID] == nil {
		h.rooms[gameID] = make(map[Conn]struct{})
	}
	h.rooms[gameID][conn] = struct{}{}
}

// Unregister remueve conn de la sala de gameID. No hace nada si conn no estaba
// registrada (ej. si la sala entera ya fue cerrada por CloseRoom).
func (h *Hub) Unregister(gameID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients, ok := h.rooms[gameID]
	if !ok {
		return
	}
	delete(clients, conn)
	if len(clients) == 0 {
		delete(h.rooms, gameID)
	}
}

// Broadcast envía message a todas las conexiones registradas en la sala de gameID. No
// bloquea ni devuelve error: la entrega a cada conexión es responsabilidad de Conn.Send
// (ver su doc, y ADR-0005 sección "fuera de alcance" sobre garantías de entrega).
func (h *Hub) Broadcast(gameID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.rooms[gameID] {
		conn.Send(message)
	}
}

// CloseRoom cierra todas las conexiones activas de gameID y elimina la sala. Se usa
// cuando la partida finaliza: ya no puede haber más game_actions que retransmitir (ver
// ADR-0005, ciclo de vida de la conexión).
func (h *Hub) CloseRoom(gameID string) {
	h.mu.Lock()
	clients := h.rooms[gameID]
	delete(h.rooms, gameID)
	h.mu.Unlock()

	for conn := range clients {
		conn.Close()
	}
}

// RoomSize devuelve la cantidad de conexiones activas en la sala de gameID (0 si no hay
// ninguna, incluida una partida sin sala registrada). Pensado para tests e
// introspección.
func (h *Hub) RoomSize(gameID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.rooms[gameID])
}
