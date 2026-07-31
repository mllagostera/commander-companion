-- +goose Up
-- +goose StatementBegin

-- Moxfield username linked to the profile, for future bulk deck import
-- (see internal/moxfieldimport, not implemented yet). No uniqueness
-- constraint: there's no need to look up "who has this username", just to
-- store it on one's own profile.
ALTER TABLE users ADD COLUMN moxfield_username varchar;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN moxfield_username;

-- +goose StatementEnd
