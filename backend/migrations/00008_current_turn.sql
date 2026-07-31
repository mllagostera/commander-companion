-- +goose Up
-- +goose StatementBegin

-- Who currently has the turn in the game. Nullable: no turn assigned
-- until the first TurnStart. Doesn't model "turn order" (who's next) -- it only
-- answers "whose turn is it now", which is the only thing TurnStart/TurnEnd
-- (internal/game-actions/service.go) currently use.
ALTER TABLE games ADD COLUMN current_turn_player_id uuid REFERENCES game_players(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE games DROP COLUMN current_turn_player_id;

-- +goose StatementEnd
