-- name: CreateUser :one
-- email_verified es explícito (no el default de la columna): RegisterUser decide su
-- valor según config.RequireEmailVerification (ver ADR-0012). CreateUserWithGoogle no
-- lo toca a propósito, así que usa el default de la columna (true) — Google ya confirma
-- el email en su id_token.
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
-- email_verified se fuerza a true al vincular: FindOrCreateGoogleUser ya comprobó que
-- Google confirma este email antes de llamar a esta query, así que una cuenta
-- email/password todavía no verificada queda verificada por esta vía también.
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

-- name: SearchUsersByUsername :many
-- Búsqueda parcial case-insensitive de username, para invitar gente a un playgroup sin
-- conocer su UUID (ver internal/playgroups). A propósito NO busca por email de esta
-- forma (parcial): permitiría enumerar direcciones de correo ajenas por prefijo/substring.
-- La búsqueda por email exacta se resuelve aparte, con GetUserByEmail.
SELECT * FROM users
WHERE username ILIKE '%' || sqlc.arg('pattern') || '%'
ORDER BY username
LIMIT sqlc.arg('result_limit');
