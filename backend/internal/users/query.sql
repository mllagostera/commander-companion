-- name: CreateUser :one
-- email_verified is explicit (not the column default): RegisterUser decides its
-- value based on config.RequireEmailVerification (see ADR-0012). CreateUserWithGoogle
-- deliberately doesn't touch it, so it uses the column default (true) — Google already
-- confirms the email in its id_token.
INSERT INTO users (
  username, email, password_hash, email_verified
) VALUES (
  $1, $2, $3, $4
)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users
WHERE google_id = $1 LIMIT 1;

-- name: CreateUserWithGoogle :one
INSERT INTO users (
  username, email, google_id
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: LinkGoogleID :one
-- email_verified is forced to true when linking: FindOrCreateGoogleUser already checked
-- that Google confirms this email before calling this query, so an email/password
-- account that isn't verified yet also gets verified through this path.
UPDATE users SET google_id = $2, email_verified = true
WHERE id = $1
RETURNING *;

-- name: UpdateMoxfieldUsername :one
UPDATE users SET moxfield_username = $2
WHERE id = $1
RETURNING *;

-- name: UpdateUsername :one
UPDATE users SET username = $2
WHERE id = $1
RETURNING *;

-- name: UpdatePasswordHash :one
UPDATE users SET password_hash = $2
WHERE id = $1
RETURNING *;

-- name: CreateEmailVerificationToken :one
INSERT INTO email_verification_tokens (
  user_id, token_hash, expires_at
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: GetEmailVerificationTokenByHash :one
SELECT * FROM email_verification_tokens
WHERE token_hash = $1 LIMIT 1;

-- name: MarkEmailVerificationTokenUsed :exec
UPDATE email_verification_tokens SET used_at = now()
WHERE id = $1;

-- name: SetUserEmailVerified :one
UPDATE users SET email_verified = true
WHERE id = $1
RETURNING *;

-- name: UsernameExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE username = $1) AS exists;

-- name: SearchUsersByUsername :many
-- Partial, case-insensitive username search, to invite people to a playgroup without
-- knowing their UUID (see internal/playgroups). Deliberately does NOT search by email
-- this way (partial): it would allow enumerating other people's email addresses by
-- prefix/substring. Exact email search is handled separately, via GetUserByEmail.
SELECT * FROM users
WHERE username ILIKE '%' || sqlc.arg('pattern') || '%'
ORDER BY username
LIMIT sqlc.arg('result_limit');
