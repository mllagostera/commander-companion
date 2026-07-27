-- name: CreateGame :one
INSERT INTO games (playgroup_id, status)
VALUES ($1, $2)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: ListGamesPage :many
-- Paginación keyset sobre (created_at, id) DESC. Con cursor_created_at NULL
-- devuelve la primera página; con cursor, las filas estrictamente posteriores en
-- el orden de la lista. Ver internal/common/pagination.go.
SELECT * FROM games
WHERE (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: StartGame :one
UPDATE games
SET status = 'active', started_at = now()
WHERE id = $1
RETURNING *;

-- name: FinishGame :one
UPDATE games
SET status = 'finished', finished_at = now()
WHERE id = $1
RETURNING *;

-- name: AddGamePlayer :one
INSERT INTO game_players (game_id, user_id, deck_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: RemoveGamePlayer :exec
DELETE FROM game_players WHERE game_id = $1 AND user_id = $2;

-- name: ListGamePlayers :many
SELECT * FROM game_players WHERE game_id = $1;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;
