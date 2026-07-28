-- +goose Up
-- +goose StatementBegin

-- Username de Moxfield vinculado al perfil, para futuro import masivo de decks
-- (ver internal/moxfieldimport, todavía no implementado). Sin constraint de
-- unicidad: no hace falta buscar "quién tiene tal username", solo guardarlo en
-- el perfil propio.
ALTER TABLE users ADD COLUMN moxfield_username varchar;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN moxfield_username;

-- +goose StatementEnd
