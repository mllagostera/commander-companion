-- +goose Up
-- +goose StatementBegin

-- Undo for game actions (POST /games/{id}/actions/undo): an undone action stays
-- in the log -- so the audit trail keeps what happened -- but carries the moment
-- it was undone, and everything built on the log (timeline, statistics, head to
-- head, turn times) reads only the rows where this is still NULL.
ALTER TABLE game_actions ADD COLUMN undone_at timestamptz;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE game_actions DROP COLUMN undone_at;

-- +goose StatementEnd
