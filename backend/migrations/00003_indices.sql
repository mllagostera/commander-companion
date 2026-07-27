-- +goose Up
-- +goose StatementBegin

-- Lookup/de-dup de decks importados desde Moxfield (internal/decks/service.go: ImportFromMoxfield).
CREATE INDEX decks_moxfield_id_idx ON decks (moxfield_id);

-- Hot path de lectura del timeline de una partida (internal/game-actions: ListGameActions).
CREATE INDEX game_actions_game_id_idx ON game_actions (game_id);

-- Hot path de lectura del estado de jugadores de una partida (internal/games: ListGamePlayers).
CREATE INDEX game_players_game_id_idx ON game_players (game_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX game_players_game_id_idx;
DROP INDEX game_actions_game_id_idx;
DROP INDEX decks_moxfield_id_idx;

-- +goose StatementEnd
