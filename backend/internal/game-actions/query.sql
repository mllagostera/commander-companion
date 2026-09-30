-- name: CreateGameAction :one
INSERT INTO game_actions (game_id, actor_id, target_id, action_type, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListGameActions :many
-- The timeline: undone actions are left out, as if they had never happened.
SELECT * FROM game_actions WHERE game_id = $1 AND undone_at IS NULL ORDER BY created_at ASC;

-- name: GetGameActionForUpdate :one
-- Locks the row so two concurrent undos of the same action can't both revert it.
SELECT * FROM game_actions WHERE id = $1 AND game_id = $2 FOR UPDATE;

-- name: MarkGameActionUndone :one
UPDATE game_actions SET undone_at = now() WHERE id = $1 RETURNING *;

-- name: CountActiveEliminationsOf :one
-- Explicit Elimination actions still in force against a player: undoing a hit
-- only brings its target back if nothing else still eliminates them.
SELECT COUNT(*)::int FROM game_actions
WHERE game_id = $1 AND target_id = $2 AND action_type = 'Elimination' AND undone_at IS NULL;

-- name: GetLatestTurnAction :one
-- The most recent TurnStart/TurnEnd still in force, to work out whose turn it is
-- after undoing one of them (TurnStart: the actor's; TurnEnd: nobody's).
SELECT * FROM game_actions
WHERE game_id = $1 AND action_type IN ('TurnStart', 'TurnEnd') AND undone_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: SubtractCommanderDamage :exec
-- Undo of a CommanderDamage action. A plain UPDATE rather than UpsertCommanderDamage
-- with a negative delta: the INSERT half of the upsert would trip
-- commander_damage_amount_chk (amount >= 0) before ON CONFLICT is even considered.
-- The row always exists here (the action being undone created or grew it).
UPDATE commander_damage
SET amount = GREATEST(amount - sqlc.arg(amount)::int, 0)
WHERE attacker_id = sqlc.arg(attacker_id) AND defender_id = sqlc.arg(defender_id);

-- name: ListCommanderDamageAgainst :many
SELECT * FROM commander_damage WHERE game_id = $1 AND defender_id = $2;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: GetGamePlayer :one
SELECT * FROM game_players WHERE id = $1 LIMIT 1;

-- name: GetGamePlayerByGameAndUser :one
SELECT * FROM game_players WHERE game_id = $1 AND user_id = $2 LIMIT 1;

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
-- current_turn_player_id is nullable: TurnStart sets it to the actor, TurnEnd clears
-- it (passing NULL). See internal/game-actions/service.go.
UPDATE games
SET current_turn_player_id = sqlc.narg(current_turn_player_id)
WHERE id = sqlc.arg(id)
RETURNING *;
