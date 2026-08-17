-- name: ListUsersPage :many
-- Keyset pagination over (created_at, id) DESC, same scheme as
-- playgroups.ListPlaygroupsForUserPage (see internal/common/pagination.go). The
-- search filter reuses the same ILIKE-both-fields approach as
-- users.SearchUsersByUsername, but here it also matches email: this listing is
-- admin-only (see auth.RequireAdmin), so the "don't let email be enumerated by
-- partial match" concern that keeps users.SearchUsers email-exact-only doesn't apply.
SELECT * FROM users
WHERE (
    sqlc.narg('search')::text IS NULL
    OR username ILIKE '%' || sqlc.narg('search')::text || '%'
    OR email ILIKE '%' || sqlc.narg('search')::text || '%'
  )
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: GetUserDetail :one
-- A user's profile plus deck/games-played counts, for the admin user detail
-- screen. Counted with correlated subqueries instead of a JOIN + GROUP BY: at
-- one row per call this is simpler to read and just as cheap, and it avoids
-- fan-out duplicating the user's columns per matching deck/game_player row.
SELECT
  u.*,
  (SELECT count(*) FROM decks d WHERE d.user_id = u.id) AS deck_count,
  (SELECT count(*) FROM game_players gp WHERE gp.user_id = u.id) AS games_played_count
FROM users u
WHERE u.id = $1
LIMIT 1;

-- name: UpdateUserActiveStatus :one
UPDATE users SET is_active = $2
WHERE id = $1
RETURNING *;

-- name: GetAdminOverviewStats :one
-- Global counts for the admin dashboard's home page. Live-computed on every
-- call, no summary table — same "live aggregation, no summary table" choice
-- already made for GetPlaygroupStats (internal/statistics); admin-panel
-- traffic is low enough that this doesn't need to be pre-aggregated.
SELECT
  (SELECT count(*) FROM users) AS total_users,
  (SELECT count(*) FROM users WHERE is_active) AS active_users,
  (SELECT count(*) FROM users WHERE email_verified) AS verified_users,
  (SELECT count(*) FROM decks) AS total_decks,
  (SELECT count(*) FROM playgroups) AS total_playgroups,
  (SELECT count(*) FROM games WHERE status = 'finished') AS total_finished_games,
  (SELECT count(*) FROM tournaments) AS total_tournaments;
