-- +goose Up
-- +goose StatementBegin

-- Quién unió a este jugador a la partida, si no fue él mismo. Null = se unió
-- solo (POST /games/{id}/join de siempre). No-null = otro usuario lo unió
-- como proxy en modo Grupo (ver ADR-0013); ese usuario queda autorizado a
-- registrar acciones en su nombre (internal/game-actions/service.go).
ALTER TABLE game_players ADD COLUMN added_by uuid REFERENCES users(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE game_players DROP COLUMN added_by;

-- +goose StatementEnd
