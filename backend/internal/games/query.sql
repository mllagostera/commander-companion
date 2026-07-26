-- name: CreateGame :one
INSERT INTO games (playgroup_id, status)
VALUES ($1, $2)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: ListGames :many
SELECT * FROM games ORDER BY created_at DESC;

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
