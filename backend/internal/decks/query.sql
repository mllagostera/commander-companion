-- name: CreateDeck :one
INSERT INTO decks (user_id, name, commander, moxfield_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeck :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: ListDecks :many
SELECT * FROM decks WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteDeck :exec
DELETE FROM decks WHERE id = $1;
