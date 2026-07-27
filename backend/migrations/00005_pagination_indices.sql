-- +goose Up
-- +goose StatementBegin

-- Índices de apoyo a la paginación keyset de /decks y /games
-- (internal/decks/query.sql: ListDecksPage, internal/games/query.sql: ListGamesPage).
-- El orden de las columnas replica exactamente el ORDER BY de esas queries, así el
-- planner resuelve tanto el filtro del cursor como el orden con un index scan y sin
-- sort, a cualquier profundidad de página.

CREATE INDEX decks_user_id_created_at_id_idx ON decks (user_id, created_at DESC, id DESC);

CREATE INDEX games_created_at_id_idx ON games (created_at DESC, id DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX games_created_at_id_idx;
DROP INDEX decks_user_id_created_at_id_idx;

-- +goose StatementEnd
