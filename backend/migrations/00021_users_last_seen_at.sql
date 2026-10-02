-- +goose Up
-- +goose StatementBegin

-- Last time the user made an authenticated API request, so the admin overview's
-- "online users" can mean "did something in the last few minutes" instead of
-- "has an unexpired refresh token" (which stays true for up to 30 days after
-- the last login). Written by users.ActivityTracker, at most once per minute per
-- user. NULL = never seen since this column was added.
ALTER TABLE users ADD COLUMN last_seen_at timestamptz;

-- Backs the "last_seen_at > now() - interval" range count in
-- admin.GetAdminOverviewStats.
CREATE INDEX users_last_seen_at_idx ON users (last_seen_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX users_last_seen_at_idx;
ALTER TABLE users DROP COLUMN last_seen_at;

-- +goose StatementEnd
