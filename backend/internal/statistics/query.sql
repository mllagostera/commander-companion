-- name: GetUserStatistics :one
SELECT * FROM user_statistics_summary
WHERE user_id = $1 LIMIT 1;

-- name: GetDeckStatistics :one
SELECT * FROM deck_statistics_summary
WHERE deck_id = $1 LIMIT 1;

-- name: ListDeckStatisticsForUser :many
-- Every deck owned by the user, LEFT JOINed against its summary row (a deck
-- never played has none) so a single query replaces the GetDeckStats N+1 the
-- web dashboard used to do (one request per deck): see internal/decks for
-- the ordering/definition of "owned by the user".
SELECT
  d.id AS deck_id,
  COALESCE(s.games_played, 0)::int AS games_played,
  COALESCE(s.games_won, 0)::int AS games_won,
  COALESCE(s.highest_life_total_achieved, 0)::int AS highest_life_total_achieved,
  COALESCE(s.total_commander_damage_dealt, 0)::int AS total_commander_damage_dealt
FROM decks d
LEFT JOIN deck_statistics_summary s ON s.deck_id = d.id
WHERE d.user_id = $1
ORDER BY d.created_at DESC;

-- name: GetDeckByID :one
SELECT * FROM decks WHERE id = $1 LIMIT 1;

-- name: ListGamePlayersForGame :many
SELECT * FROM game_players WHERE game_id = $1;

-- name: ListGameActionsForGame :many
SELECT * FROM game_actions WHERE game_id = $1 ORDER BY created_at ASC;

-- name: UpsertUserStatistics :exec
INSERT INTO user_statistics_summary (
  user_id, games_played, games_won, total_damage_dealt, total_commander_damage_dealt, total_eliminations
)
VALUES (sqlc.arg(user_id), sqlc.arg(games_played), sqlc.arg(games_won), sqlc.arg(total_damage_dealt),
        sqlc.arg(total_commander_damage_dealt), sqlc.arg(total_eliminations))
ON CONFLICT (user_id) DO UPDATE SET
  games_played = user_statistics_summary.games_played + EXCLUDED.games_played,
  games_won = user_statistics_summary.games_won + EXCLUDED.games_won,
  total_damage_dealt = user_statistics_summary.total_damage_dealt + EXCLUDED.total_damage_dealt,
  total_commander_damage_dealt = user_statistics_summary.total_commander_damage_dealt + EXCLUDED.total_commander_damage_dealt,
  total_eliminations = user_statistics_summary.total_eliminations + EXCLUDED.total_eliminations,
  last_recalculated_at = now();

-- name: UpsertDeckStatistics :exec
INSERT INTO deck_statistics_summary (
  deck_id, games_played, games_won, highest_life_total_achieved, total_commander_damage_dealt
)
VALUES (sqlc.arg(deck_id), sqlc.arg(games_played), sqlc.arg(games_won), sqlc.arg(highest_life_total_achieved),
        sqlc.arg(total_commander_damage_dealt))
ON CONFLICT (deck_id) DO UPDATE SET
  games_played = deck_statistics_summary.games_played + EXCLUDED.games_played,
  games_won = deck_statistics_summary.games_won + EXCLUDED.games_won,
  highest_life_total_achieved = GREATEST(deck_statistics_summary.highest_life_total_achieved, EXCLUDED.highest_life_total_achieved),
  total_commander_damage_dealt = deck_statistics_summary.total_commander_damage_dealt + EXCLUDED.total_commander_damage_dealt,
  last_recalculated_at = now();

-- name: CountFinishedGamesForPlaygroup :one
SELECT COUNT(*) FROM games WHERE playgroup_id = $1 AND status = 'finished';

-- name: ListPlaygroupMemberGameStats :many
SELECT
  gp.user_id,
  COUNT(DISTINCT gp.game_id)::int AS games_played,
  COUNT(DISTINCT winner.id)::int AS games_won
FROM game_players gp
JOIN games g ON g.id = gp.game_id
LEFT JOIN (
  -- The winner of a game is the only player still alive at the end; the
  -- alive_count = 1 filter selects that row without needing MIN()/MAX()
  -- over uuid (Postgres has no ordering operator for that type).
  SELECT id, game_id
  FROM (
    SELECT id, game_id, COUNT(*) OVER (PARTITION BY game_id) AS alive_count
    FROM game_players
    WHERE NOT is_eliminated
  ) alive
  WHERE alive_count = 1
) winner ON winner.game_id = gp.game_id AND winner.id = gp.id
WHERE g.playgroup_id = $1 AND g.status = 'finished'
GROUP BY gp.user_id;

-- name: ListOpponentStats :many
-- Head-to-head aggregation across every finished game the user has played:
-- for each other user they've shared a seat with, how many games together and
-- how many times each side eliminated the other. Live-computed (no summary
-- table), same choice as ListPlaygroupMemberGameStats above -- read once, on
-- the statistics screen, not on every request like user_statistics_summary.
SELECT
  other.user_id AS opponent_id,
  u.username AS opponent_username,
  COUNT(DISTINCT other.game_id)::int AS games_together,
  COUNT(DISTINCT you_eliminated.id)::int AS times_you_eliminated_them,
  COUNT(DISTINCT they_eliminated.id)::int AS times_eliminated_by_opponent
FROM game_players me
JOIN games g ON g.id = me.game_id AND g.status = 'finished'
JOIN game_players other ON other.game_id = me.game_id AND other.user_id != me.user_id
JOIN users u ON u.id = other.user_id
LEFT JOIN game_actions you_eliminated
  ON you_eliminated.game_id = me.game_id AND you_eliminated.action_type = 'Elimination'
  AND you_eliminated.actor_id = me.id AND you_eliminated.target_id = other.id
LEFT JOIN game_actions they_eliminated
  ON they_eliminated.game_id = me.game_id AND they_eliminated.action_type = 'Elimination'
  AND they_eliminated.actor_id = other.id AND they_eliminated.target_id = me.id
WHERE me.user_id = $1
GROUP BY other.user_id, u.username;

-- name: ListPlaygroupGameCountsForUser :many
-- Every playgroup the user belongs to, with how many finished games they've
-- played within it -- single query replacing the per-group GetPlaygroupStats
-- loop the statistics screens used to do (same "every owned/joined X" shape
-- as ListDeckStatisticsForUser above).
SELECT
  pg.id AS playgroup_id,
  pg.name AS playgroup_name,
  COUNT(DISTINCT gp.game_id)::int AS games_played
FROM playgroup_members pm
JOIN playgroups pg ON pg.id = pm.playgroup_id
LEFT JOIN game_players gp
  ON gp.user_id = pm.user_id
  AND EXISTS (
    SELECT 1 FROM games g
    WHERE g.id = gp.game_id AND g.playgroup_id = pg.id AND g.status = 'finished'
  )
WHERE pm.user_id = $1
GROUP BY pg.id, pg.name;

-- name: ListFinishedGamesPage :many
-- Keyset pagination over (created_at, id) DESC, scoped to finished games where
-- the authenticated user had a seat -- same shape as games.ListGamesPage, but
-- owned here because GET /statistics/games needs the denormalized
-- player/deck/username data GET /games deliberately doesn't carry (that one is
-- shared with the dashboard/join-game flow, kept lean on purpose).
SELECT
  games.id, games.playgroup_id, games.started_at, games.finished_at, games.created_at,
  pg.name AS playgroup_name
FROM games
LEFT JOIN playgroups pg ON pg.id = games.playgroup_id
WHERE games.status = 'finished'
  AND EXISTS (
    SELECT 1 FROM game_players gp
    WHERE gp.game_id = games.id AND gp.user_id = sqlc.arg('user_id')::uuid
  )
  AND (
    sqlc.narg('cursor_created_at')::timestamp IS NULL
    OR (games.created_at, games.id) < (sqlc.narg('cursor_created_at')::timestamp, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY games.created_at DESC, games.id DESC
LIMIT sqlc.arg('page_limit');

-- name: ListGameActionSummaryForGames :many
-- For each game in game_ids: how many turns were played (every TurnStart belongs to
-- one player's turn, so the count across the whole game is the turn count) and the
-- single biggest hit (CombatDamage or CommanderDamage) dealt in it, with who dealt it --
-- powers the finished-games card's "turns" and "biggest hit" stats without the client
-- re-deriving them from the full action log. Left-joined from game_ids (not game_actions)
-- so a game with no actions logged still gets a row (zero turns, no biggest hit).
WITH turns AS (
  SELECT game_id, COUNT(*)::int AS turn_count
  FROM game_actions
  WHERE game_id = ANY(sqlc.arg('game_ids')::uuid[]) AND action_type = 'TurnStart'
  GROUP BY game_id
),
hits AS (
  SELECT DISTINCT ON (game_id)
    game_id,
    actor_id,
    (payload->>'amount')::int AS amount
  FROM game_actions
  WHERE game_id = ANY(sqlc.arg('game_ids')::uuid[])
    AND action_type IN ('CombatDamage', 'CommanderDamage')
  ORDER BY game_id, (payload->>'amount')::int DESC
)
SELECT
  ids.game_id::uuid AS game_id,
  COALESCE(t.turn_count, 0)::int AS turn_count,
  h.amount AS biggest_hit_amount,
  u.username AS biggest_hit_username
FROM unnest(sqlc.arg('game_ids')::uuid[]) AS ids(game_id)
LEFT JOIN turns t ON t.game_id = ids.game_id
LEFT JOIN hits h ON h.game_id = ids.game_id
LEFT JOIN game_players gp ON gp.id = h.actor_id
LEFT JOIN users u ON u.id = gp.user_id;

-- name: CountDecksForUser :one
SELECT COUNT(*) FROM decks WHERE user_id = $1;

-- name: CountPlaygroupsForUser :one
SELECT COUNT(*) FROM playgroup_members WHERE user_id = $1;

-- name: ListDashboardDecksForUser :many
-- The decks the dashboard's "your decks" strip shows: most played first, capped
-- at the handful that fit. Same decks LEFT JOIN summary shape as
-- ListDeckStatisticsForUser (a deck never played has no summary row), but
-- ordered and limited here instead of returning the whole collection for the
-- client to sort and slice.
SELECT
  d.id AS deck_id,
  d.name,
  d.commander,
  d.image_url,
  COALESCE(s.games_played, 0)::int AS games_played,
  COALESCE(s.games_won, 0)::int AS games_won
FROM decks d
LEFT JOIN deck_statistics_summary s ON s.deck_id = d.id
WHERE d.user_id = $1
ORDER BY COALESCE(s.games_played, 0) DESC, d.created_at DESC
LIMIT sqlc.arg('deck_limit');

-- name: GetDashboardBestDeckForUser :one
-- The spotlight card: highest win rate among the user's decks that have
-- actually been played. games_played > 0 both defines "has a win rate" and
-- keeps the division safe. Ties break towards the deck with more games -- a
-- single lucky win shouldn't outrank a long winning record.
SELECT
  d.id AS deck_id,
  d.name,
  d.commander,
  d.image_url,
  COALESCE(s.games_played, 0)::int AS games_played,
  COALESCE(s.games_won, 0)::int AS games_won
FROM decks d
JOIN deck_statistics_summary s ON s.deck_id = d.id
WHERE d.user_id = $1 AND s.games_played > 0
ORDER BY (s.games_won::numeric / s.games_played) DESC, s.games_played DESC
LIMIT 1;

-- name: ListDashboardPlaygroupsForUser :many
-- The group cards, in the same order GET /playgroups uses (newest first), so
-- the dashboard shows the same first groups the playgroups screen does.
-- games_played counts the group's finished games, not the caller's -- that's
-- what the card says ("N games played" for the group).
SELECT
  p.id AS playgroup_id,
  p.name AS playgroup_name,
  (SELECT COUNT(*) FROM playgroup_members m WHERE m.playgroup_id = p.id)::int AS member_count,
  (SELECT COUNT(*) FROM games g WHERE g.playgroup_id = p.id AND g.status = 'finished')::int AS games_played
FROM playgroups p
JOIN playgroup_members pm ON pm.playgroup_id = p.id
WHERE pm.user_id = $1
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg('playgroup_limit');

-- name: ListDashboardPlaygroupMembers :many
-- Members of the groups the dashboard is about to render, batched in one round
-- trip (same game_id = ANY(...) shape as ListPlayersForGames below). Only feeds
-- the avatar strip, so the service keeps the first few per group.
SELECT pm.playgroup_id, pm.user_id, u.username
FROM playgroup_members pm
JOIN users u ON u.id = pm.user_id
WHERE pm.playgroup_id = ANY(sqlc.arg('playgroup_ids')::uuid[])
ORDER BY pm.playgroup_id, pm.joined_at;

-- name: GetCurrentStreakForUser :one
-- How many finished games in a row, counting back from the most recent, ended
-- the same way for this user. Computed in SQL and returned as a single row: the
-- naive version ships every game the user ever played to the client just to
-- count a handful of them off the top.
--
-- `won` uses the same sole-survivor rule as ListPlaygroupMemberGameStats and
-- ListPlayersForGames, so the streak agrees with the win rate shown next to it.
WITH results AS (
  SELECT
    (winner.id IS NOT NULL) AS won,
    ROW_NUMBER() OVER (ORDER BY g.created_at DESC, g.id DESC) AS rn
  FROM game_players gp
  JOIN games g ON g.id = gp.game_id AND g.status = 'finished'
  LEFT JOIN (
    SELECT id, game_id
    FROM (
      SELECT id, game_id, COUNT(*) OVER (PARTITION BY game_id) AS alive_count
      FROM game_players
      WHERE NOT is_eliminated
    ) alive
    WHERE alive_count = 1
  ) winner ON winner.game_id = gp.game_id AND winner.id = gp.id
  WHERE gp.user_id = $1
)
SELECT
  -- The streak ends one game before the first result that differs from the
  -- latest one; if none differs, every game counts.
  COALESCE(
    (SELECT MIN(rn) FROM results WHERE won IS DISTINCT FROM (SELECT won FROM results WHERE rn = 1)) - 1,
    (SELECT COUNT(*) FROM results)
  )::int AS streak,
  COALESCE((SELECT won FROM results WHERE rn = 1), false)::boolean AS streak_won,
  (SELECT COUNT(*) FROM results)::int AS total_games;

-- name: ListPlayersForGames :many
-- Batched fetch of every seat across a page of games (see ListFinishedGamesPage),
-- enriched with username, deck name/commander/image, and a `won` flag -- one
-- round trip for the whole page instead of one per game. The winner subquery
-- (only non-eliminated seat, if there's exactly one) mirrors the identical
-- pattern already used in ListPlaygroupMemberGameStats above -- same rule,
-- same SQL shape, not reinvented.
SELECT
  gp.game_id, gp.user_id, u.username, gp.deck_id, d.name AS deck_name,
  d.commander AS deck_commander, d.image_url AS deck_image_url,
  (winner.id IS NOT NULL)::boolean AS won
FROM game_players gp
JOIN users u ON u.id = gp.user_id
JOIN decks d ON d.id = gp.deck_id
LEFT JOIN (
  SELECT id, game_id
  FROM (
    SELECT id, game_id, COUNT(*) OVER (PARTITION BY game_id) AS alive_count
    FROM game_players
    WHERE NOT is_eliminated
  ) alive
  WHERE alive_count = 1
) winner ON winner.game_id = gp.game_id AND winner.id = gp.id
WHERE gp.game_id = ANY(sqlc.arg('game_ids')::uuid[]);
