-- name: CreateUser :one
INSERT INTO users (
  username, email, password_hash
) VALUES (
  $1, $2, $3
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
UPDATE users SET google_id = $2
WHERE id = $1
RETURNING *;

-- name: UpdateMoxfieldUsername :one
UPDATE users SET moxfield_username = $2
WHERE id = $1
RETURNING *;
