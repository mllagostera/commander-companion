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
--
-- online_users counts users whose last authenticated request was in the last
-- 5 minutes (users.last_seen_at, kept fresh by users.ActivityTracker at most
-- once a minute per user). A user with the app open but idle drops off after
-- the window. See ADR-0018's second addendum.
SELECT
  (SELECT count(*) FROM users) AS total_users,
  (SELECT count(*) FROM users WHERE is_active) AS active_users,
  (SELECT count(*) FROM users WHERE email_verified) AS verified_users,
  (SELECT count(*) FROM decks) AS total_decks,
  (SELECT count(*) FROM playgroups) AS total_playgroups,
  (SELECT count(*) FROM games WHERE status = 'finished') AS total_finished_games,
  (SELECT count(*) FROM tournaments) AS total_tournaments,
  (SELECT count(*) FROM users
     WHERE last_seen_at > now() - interval '5 minutes') AS online_users,
  (SELECT count(*) FROM games WHERE status = 'active') AS active_games;

-- name: GetDailyActivity :many
-- Historical series for the admin dashboard's activity chart: per day, how many
-- games were started and how many distinct users played at least one of them.
-- Derived entirely from games/game_players — no new tracking table, see
-- ADR-0018's addendum for why (no scheduler in this backend to run a daily
-- snapshot job). A day with zero games simply doesn't produce a row; the
-- caller (admin.Service.GetDailyActivity) fills the gaps with zero so the
-- chart gets one point per calendar day, not a shorter series with holes.
SELECT
  date_trunc('day', g.started_at)::date AS day,
  count(DISTINCT g.id) AS games_started,
  count(DISTINCT gp.user_id) AS active_users
FROM games g
JOIN game_players gp ON gp.game_id = g.id
WHERE g.started_at >= now() - (sqlc.arg('days_back')::int * interval '1 day')
GROUP BY 1
ORDER BY 1;

-- name: ListUnfinishedGamesPage :many
-- Games that were opened but never finished ('pending' or 'active'), for the
-- admin games screen, so abandoned ones can be found and deleted. Oldest first:
-- the longer a game has been open, the more likely it was abandoned. Keyset
-- pagination over (created_at, id) ASC — same cursor scheme as ListUsersPage,
-- with the comparison flipped for the ascending order. An optional status
-- narrows the list to one of the two unfinished states.
SELECT
  g.id,
  g.status,
  g.created_at,
  g.started_at,
  g.playgroup_id,
  p.name AS playgroup_name
FROM games g
LEFT JOIN playgroups p ON p.id = g.playgroup_id
WHERE g.status IN ('pending', 'active')
  AND (sqlc.narg('status')::text IS NULL OR g.status = sqlc.narg('status')::text)
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (g.created_at, g.id) > (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY g.created_at ASC, g.id ASC
LIMIT sqlc.arg('page_limit');

-- name: ListPlayersForGames :many
-- Every seat across a page of games in one round trip, with the username so the
-- admin can tell who left the game open.
SELECT gp.game_id, u.id AS user_id, u.username
FROM game_players gp
JOIN users u ON u.id = gp.user_id
WHERE gp.game_id = ANY(sqlc.arg('game_ids')::uuid[])
ORDER BY u.username;

-- name: LockGameStatus :one
-- Locks the game row for the rest of DeleteUnfinishedGame's transaction. A
-- concurrent FinishGame (UPDATE ... AND status = 'active') or a new seat/action
-- (whose FK check takes a KEY SHARE lock on this row) waits behind it, so the
-- status checked here can't change before the delete commits.
SELECT status FROM games WHERE id = $1 FOR UPDATE;

-- name: ClearGameCurrentTurn :exec
-- games.current_turn_player_id points back at game_players (00008_current_turn.sql),
-- so it has to be cleared before the seats can be deleted.
UPDATE games SET current_turn_player_id = NULL WHERE id = $1;

-- name: DeleteGameCommanderDamage :exec
DELETE FROM commander_damage WHERE game_id = $1;

-- name: DeleteGameActions :exec
DELETE FROM game_actions WHERE game_id = $1;

-- name: DeleteGamePlayers :exec
DELETE FROM game_players WHERE game_id = $1;

-- name: DeleteGame :exec
DELETE FROM games WHERE id = $1;
