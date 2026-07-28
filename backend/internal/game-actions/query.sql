-- name: CreateGameAction :one
INSERT INTO game_actions (game_id, actor_id, target_id, action_type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGameActions :many
SELECT * FROM game_actions WHERE game_id = $1 ORDER BY created_at ASC;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: GetGamePlayer :one
SELECT * FROM game_players WHERE id = $1 LIMIT 1;

-- name: AdjustGamePlayerLife :one
UPDATE game_players
SET life_total = life_total + sqlc.arg(delta)::int
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: AdjustGamePlayerPoison :one
UPDATE game_players
SET poison_counters = poison_counters + sqlc.arg(delta)::int
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetGamePlayerEliminated :one
UPDATE game_players
SET is_eliminated = sqlc.arg(is_eliminated)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpsertCommanderDamage :one
INSERT INTO commander_damage (game_id, attacker_id, defender_id, amount)
VALUES (sqlc.arg(game_id), sqlc.arg(attacker_id), sqlc.arg(defender_id), sqlc.arg(delta))
ON CONFLICT (attacker_id, defender_id)
DO UPDATE SET amount = commander_damage.amount + EXCLUDED.amount
RETURNING *;

-- name: SetCurrentTurnPlayer :one
-- current_turn_player_id nullable: TurnStart lo fija al actor, TurnEnd lo limpia
-- (pasando NULL). Ver internal/game-actions/service.go.
UPDATE games
SET current_turn_player_id = sqlc.narg(current_turn_player_id)
WHERE id = sqlc.arg(id)
RETURNING *;
