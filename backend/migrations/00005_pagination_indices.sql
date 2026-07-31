-- +goose Up
-- +goose StatementBegin

-- Indexes supporting the keyset pagination of /decks and /games
-- (internal/decks/query.sql: ListDecksPage, internal/games/query.sql: ListGamesPage).
-- Column order exactly mirrors those queries' ORDER BY, so the
-- planner resolves both the cursor filter and the ordering with an index scan and no
-- sort, at any page depth.

CREATE INDEX decks_user_id_created_at_id_idx ON decks (user_id, created_at DESC, id DESC);

CREATE INDEX games_created_at_id_idx ON games (created_at DESC, id DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX games_created_at_id_idx;
DROP INDEX decks_user_id_created_at_id_idx;

-- +goose StatementEnd
