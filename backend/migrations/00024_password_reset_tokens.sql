-- +goose Up
-- +goose StatementBegin

-- Same shape as email_verification_tokens (see ADR-0012): only the SHA-256 hash of the
-- opaque token is stored, single use (used_at), short TTL decided by the service. See
-- ADR-0022.
CREATE TABLE password_reset_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  token_hash varchar UNIQUE NOT NULL,
  expires_at timestamp NOT NULL,
  created_at timestamp DEFAULT (now()),
  used_at timestamp
);

CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE password_reset_tokens;
-- +goose StatementEnd
