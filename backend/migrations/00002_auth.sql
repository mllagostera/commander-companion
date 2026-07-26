-- +goose Up
-- +goose StatementBegin

ALTER TABLE users
  ALTER COLUMN password_hash DROP NOT NULL,
  ADD COLUMN google_id varchar UNIQUE;

ALTER TABLE users
  ADD CONSTRAINT users_password_or_google_id_chk
  CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL);

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash varchar UNIQUE NOT NULL,
  expires_at timestamp NOT NULL,
  created_at timestamp DEFAULT (now()),
  revoked_at timestamp
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE refresh_tokens;

ALTER TABLE users
  DROP CONSTRAINT users_password_or_google_id_chk,
  DROP COLUMN google_id,
  ALTER COLUMN password_hash SET NOT NULL;
-- +goose StatementEnd
