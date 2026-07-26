-- name: CreateGameAction :one
INSERT INTO game_actions (game_id, actor_id, target_id, action_type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGameActions :many
SELECT * FROM game_actions WHERE game_id = $1 ORDER BY created_at ASC;
