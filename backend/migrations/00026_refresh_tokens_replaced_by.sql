-- +goose Up
-- +goose StatementBegin

-- The token a refresh token was rotated into (POST /auth/refresh). NULL when it
-- was revoked any other way — logout, reuse detection, a password reset — or is
-- still active. A rotated token presented again within a short grace window, while
-- its replacement is still active, is a concurrent refresh by the same client
-- rather than a replay, and gets a fresh pair instead of revoking every session of
-- the account. See ADR-0028.
ALTER TABLE refresh_tokens ADD COLUMN replaced_by uuid REFERENCES refresh_tokens(id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE refresh_tokens DROP COLUMN replaced_by;

-- +goose StatementEnd
