-- +goose Up
-- +goose StatementBegin

ALTER TABLE decks ADD COLUMN image_url varchar;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE decks DROP COLUMN image_url;

-- +goose StatementEnd
