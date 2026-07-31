-- +goose Up
-- +goose StatementBegin

-- Who added this player to the game, if it wasn't themselves. Null = they joined
-- on their own (the usual POST /games/{id}/join). Non-null = another user added
-- them as a proxy in Group mode (see ADR-0013); that user is authorized to
-- record actions on their behalf (internal/game-actions/service.go).
ALTER TABLE game_players ADD COLUMN added_by uuid REFERENCES users(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE game_players DROP COLUMN added_by;

-- +goose StatementEnd
