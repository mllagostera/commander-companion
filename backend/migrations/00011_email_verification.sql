-- +goose Up
-- +goose StatementBegin

-- Default true: existing accounts and Google sign-ups (already confirmed by the
-- id_token itself, see users.FindOrCreateGoogleUser) end up verified without touching those
-- queries. Only CreateUser (email/password sign-up) explicitly forces false.
ALTER TABLE users
  ADD COLUMN email_verified boolean NOT NULL DEFAULT true;

CREATE TABLE email_verification_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash varchar UNIQUE NOT NULL,
  expires_at timestamp NOT NULL,
  created_at timestamp DEFAULT (now()),
  used_at timestamp
);

CREATE INDEX email_verification_tokens_user_id_idx ON email_verification_tokens (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE email_verification_tokens;

ALTER TABLE users
  DROP COLUMN email_verified;
-- +goose StatementEnd
