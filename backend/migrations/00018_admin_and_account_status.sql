-- +goose Up
-- +goose StatementBegin

-- Admin role and account status (Phase 1 of the admin dashboard, see
-- ADR-0018). No roles table: is_admin is a plain boolean, promoted by hand
-- in the DB for now (no self-service "become admin" UI yet). is_active
-- backs account deactivation, distinct from deletion: a deactivated account
-- keeps every row (decks, games, statistics) intact and can be reactivated,
-- it just can't log in or refresh a session while false.
ALTER TABLE users ADD COLUMN is_admin boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN is_active boolean NOT NULL DEFAULT true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE users DROP COLUMN is_admin;
ALTER TABLE users DROP COLUMN is_active;

-- +goose StatementEnd
