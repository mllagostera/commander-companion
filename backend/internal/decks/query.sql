-- name: CreateDeck :one
INSERT INTO decks (user_id, name, commander, moxfield_id, image_url)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDeck :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: GetDeckByMoxfieldID :one
-- Resolves a user's already-imported deck from its public Moxfield ID
-- (see internal/sync). The same Moxfield deck can be imported by several
-- users, hence the filter by user_id.
SELECT * FROM decks
WHERE user_id = $1 AND moxfield_id = $2
ORDER BY created_at ASC
LIMIT 1;

-- name: ListDecksPage :many
-- Keyset pagination over (created_at, id) DESC. With cursor_created_at NULL
-- it returns the first page; with a cursor, the rows strictly after it in
-- list order. See internal/common/pagination.go.
SELECT * FROM decks
WHERE user_id = sqlc.arg('user_id')
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: UpdateDeckFromMoxfield :one
-- Re-syncs name, commander, and image for an already-imported deck with what
-- Moxfield returns today (see internal/sync). updated_at marks the last successful sync.
UPDATE decks
SET name = $2, commander = $3, image_url = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDeck :exec
DELETE FROM decks WHERE id = $1;
