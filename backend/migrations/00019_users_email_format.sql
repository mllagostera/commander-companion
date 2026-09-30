-- +goose Up
-- +goose StatementBegin

-- Safety net under the backend's own validation (users.normalizeEmail), after an
-- email/password sign-up stored a bare username ("vansidgg") as its email and
-- the account could no longer log in with its real address.
--
-- NOT VALID: the CHECK applies to every INSERT and UPDATE from now on but does
-- not scan the rows already there, so this migration cannot fail on a database
-- that still holds a malformed legacy email. Once those rows are fixed, run
--   ALTER TABLE users VALIDATE CONSTRAINT users_email_format;
-- The pattern is the same one normalizeEmail enforces: something@something.tld,
-- no whitespace, a single @.
ALTER TABLE users
  ADD CONSTRAINT users_email_format
  CHECK (email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$') NOT VALID;

-- Email uniqueness regardless of case: the existing users_email_key still
-- rejects exact duplicates, this one also rejects "A@x.com" next to "a@x.com".
-- It is also the index GetUserByEmail's lower(email) lookup uses. Fails if the
-- table already holds two emails that differ only in case -- find them with
--   SELECT lower(email) FROM users GROUP BY 1 HAVING count(*) > 1;
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX users_email_lower_key;
ALTER TABLE users DROP CONSTRAINT users_email_format;

-- +goose StatementEnd
