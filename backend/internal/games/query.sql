-- name: CreateGame :one
INSERT INTO games (playgroup_id, status)
VALUES ($1, $2)
RETURNING *;

-- name: GetGame :one
SELECT * FROM games WHERE id = $1 LIMIT 1;

-- name: ListGamesPage :many
-- Keyset pagination over (created_at, id) DESC, scoped to games where the
-- authenticated user has (or had) a seat — never the full cross-tenant
-- history. With cursor_created_at NULL it returns the first page; with a
-- cursor, the rows strictly after it in list order. See internal/common/pagination.go.
SELECT games.* FROM games
WHERE EXISTS (
    SELECT 1 FROM game_players gp
    WHERE gp.game_id = games.id AND gp.user_id = sqlc.arg('user_id')::uuid
  )
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: ListGamesForPlaygroup :many
-- Game history for a group. Unpaginated: scoped to a single playgroup,
-- never approaches the volume of ListGamesPage (the global history).
SELECT * FROM games WHERE playgroup_id = $1 ORDER BY created_at DESC;

-- name: StartGame :one
-- The "AND status = 'pending'" guard makes the pending->active transition
-- atomic: without it, two concurrent StartGame calls both read status as
-- pending before either writes, and both succeed. See FinishGame below for
-- the same race with a worse consequence (double-counted statistics).
UPDATE games
SET status = 'active', started_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: FinishGame :one
-- The "AND status = 'active'" guard is load-bearing, not decorative: without
-- it, N concurrent FinishGame calls on the same game (e.g. two players
-- tapping "Finish" at once) all pass the service's earlier status check,
-- all succeed here, and each one separately triggers
-- statistics.RecalculateForGame — which is purely additive (ON CONFLICT DO
-- UPDATE SET games_played = games_played + EXCLUDED.games_played), so
-- games_played/games_won/etc. end up multiplied by however many calls
-- raced. With the guard, only the first UPDATE actually matches a row; the
-- rest affect 0 rows and the service maps that to ErrGameNotActive (409).
UPDATE games
SET status = 'finished', finished_at = now()
WHERE id = $1 AND status = 'active'
RETURNING *;

-- name: AddGamePlayer :one
INSERT INTO game_players (game_id, user_id, deck_id, added_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RemoveGamePlayer :exec
DELETE FROM game_players WHERE game_id = $1 AND user_id = $2;

-- name: ListGamePlayers :many
SELECT * FROM game_players WHERE game_id = $1;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;
